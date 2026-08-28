package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Alumos/Clipmesh/backend/internal/auth"
	"github.com/Alumos/Clipmesh/backend/internal/config"
	"github.com/Alumos/Clipmesh/backend/internal/events"
	"github.com/Alumos/Clipmesh/backend/internal/model"
	"github.com/Alumos/Clipmesh/backend/internal/store"
)

type Server struct {
	cfg         config.Config
	store       *store.Store
	hub         *events.Hub
	logger      *slog.Logger
	authManager *auth.Manager
	startedAt   time.Time
	version     string
}

type contextKey string

const userContextKey contextKey = "clipmesh.current-user"

func New(cfg config.Config, clips *store.Store, hub *events.Hub, logger *slog.Logger, version string) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	// New is also used by tests and small embedders, so make sure the first
	// account exists even when they do not run cmd/server/main.go directly.
	if _, err := clips.EnsureAdmin(context.Background(), cfg.AdminUsername, cfg.AdminPassword); err != nil {
		logger.Error("ensure administrator", "error", err)
	}
	s := &Server{
		cfg:         cfg,
		store:       clips,
		hub:         hub,
		logger:      logger,
		authManager: auth.NewWithStore(clips, cfg.SessionTTL, cfg.CookieSecure),
		startedAt:   time.Now().UTC(),
		version:     version,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.health)
	mux.HandleFunc("/api/auth/login", s.login)
	mux.HandleFunc("/api/auth/logout", s.logout)
	mux.HandleFunc("/api/auth/me", s.me)
	mux.HandleFunc("/api/config", s.config)
	mux.HandleFunc("/api/events", s.events)
	mux.HandleFunc("/api/clips", s.clips)
	mux.HandleFunc("/api/clips/file", s.uploadFile)
	mux.HandleFunc("/api/clips/", s.clipByID)
	mux.HandleFunc("/api/admin/users", s.adminUsers)
	mux.HandleFunc("/api/admin/users/", s.adminUserByID)
	return s.cors(s.auth(mux))
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"version":   s.version,
		"uptime":    time.Since(s.startedAt).Round(time.Second).String(),
		"startedAt": s.startedAt,
	})
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var request loginRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid login body")
		return
	}
	user, err := s.authManager.Authenticate(r.Context(), request.Username, request.Password)
	if errors.Is(err, store.ErrInvalidCredentials) {
		// Keep this response deliberately generic so it does not disclose which
		// credential failed.
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	if err != nil {
		s.internalError(w, "authenticate user", err)
		return
	}
	session, err := s.authManager.CreateSessionForUser(r.Context(), user)
	if err != nil {
		s.internalError(w, "create auth session", err)
		return
	}
	s.authManager.SetCookie(w, session)
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.authManager.RevokeSession(r.Context(), r); err != nil {
		s.logger.Warn("revoke auth session", "error", err)
	}
	s.authManager.ClearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	user, ok := requestUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "login required")
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, user)
}

func (s *Server) config(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"textLimit":      s.cfg.TextLimit,
		"fileTtlSeconds": int64(s.cfg.FileTTL / time.Second),
		"maxUploadBytes": s.cfg.MaxUploadSize,
		"authEnabled":    true,
		"pageSize":       20,
	})
}

func (s *Server) clips(w http.ResponseWriter, r *http.Request) {
	user, _ := requestUser(r)
	switch r.Method {
	case http.MethodGet:
		limit := 100
		if raw := r.URL.Query().Get("limit"); raw != "" {
			if parsed, err := strconv.Atoi(raw); err == nil {
				limit = parsed
			}
		}
		clips, err := s.store.ListForUser(r.Context(), user.ID, strings.TrimSpace(r.URL.Query().Get("q")), r.URL.Query().Get("kind"), limit)
		if err != nil {
			s.internalError(w, "list clips", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": clips})
	case http.MethodPost:
		s.createText(w, r, user.ID)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

type createTextRequest struct {
	Kind       string            `json:"kind"`
	DeviceID   string            `json:"deviceId"`
	DeviceName string            `json:"deviceName"`
	Formats    map[string]string `json:"formats"`
}

func (s *Server) createText(w http.ResponseWriter, r *http.Request, userID string) {
	var request createTextRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if request.Kind != "" && request.Kind != "text" {
		writeError(w, http.StatusBadRequest, "only text clips use this endpoint")
		return
	}
	clip, err := s.store.CreateTextForUser(r.Context(), userID, request.DeviceID, request.DeviceName, request.Formats)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.hub.Publish(events.Event{Type: "created", UserID: userID, Clip: &clip})
	writeJSON(w, http.StatusCreated, clip)
}

func (s *Server) clipByID(w http.ResponseWriter, r *http.Request) {
	user, _ := requestUser(r)
	path := strings.TrimPrefix(r.URL.Path, "/api/clips/")
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "clip not found")
		return
	}
	id := parts[0]
	if len(parts) == 2 && parts[1] == "file" {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		s.downloadFile(w, r, user.ID, id)
		return
	}
	if len(parts) != 1 || r.Method != http.MethodDelete {
		w.Header().Set("Allow", "DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	storagePath, err := s.store.DeleteForUser(r.Context(), user.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "clip not found")
		return
	}
	if err != nil {
		s.internalError(w, "delete clip", err)
		return
	}
	if storagePath != "" {
		if err := os.Remove(storagePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.logger.Warn("remove clip file", "path", storagePath, "error", err)
		}
	}
	s.hub.Publish(events.Event{Type: "deleted", UserID: user.ID, ID: id})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) uploadFile(w http.ResponseWriter, r *http.Request) {
	user, _ := requestUser(r)
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Leave room for multipart headers while enforcing the selected file's
	// actual payload limit again while copying it to disk.
	r.Body = http.MaxBytesReader(w, r.Body, s.cfg.MaxUploadSize+(1<<20))
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	formats := map[string]string{}
	if raw := strings.TrimSpace(r.FormValue("formats")); raw != "" {
		if err := json.Unmarshal([]byte(raw), &formats); err != nil {
			writeError(w, http.StatusBadRequest, "formats must be a JSON object")
			return
		}
	}
	deviceName := strings.TrimSpace(r.FormValue("deviceName"))
	deviceID := strings.TrimSpace(r.FormValue("deviceId"))
	name := safeFilename(header.Filename)
	if name == "" {
		name = "clipboard-file"
	}
	mimeType := strings.TrimSpace(header.Header.Get("Content-Type"))
	if mimeType == "" || mimeType == "application/octet-stream" {
		if detected := mime.TypeByExtension(filepath.Ext(name)); detected != "" {
			mimeType = detected
		} else {
			mimeType = "application/octet-stream"
		}
	}

	id, err := model.NewID()
	if err != nil {
		s.internalError(w, "create file id", err)
		return
	}
	temporary, err := os.CreateTemp(s.cfg.FilesDir, ".upload-*")
	if err != nil {
		s.internalError(w, "create temporary file", err)
		return
	}
	temporaryPath := temporary.Name()
	removeTemporary := true
	defer func() {
		if removeTemporary {
			_ = os.Remove(temporaryPath)
		}
	}()

	limited := io.LimitReader(file, s.cfg.MaxUploadSize+1)
	bytesWritten, copyErr := io.Copy(temporary, limited)
	closeErr := temporary.Close()
	if copyErr != nil || closeErr != nil {
		s.internalError(w, "save uploaded file", firstError(copyErr, closeErr))
		return
	}
	if bytesWritten > s.cfg.MaxUploadSize {
		writeError(w, http.StatusRequestEntityTooLarge, "file exceeds the configured upload limit")
		return
	}

	destination := filepath.Join(s.cfg.FilesDir, id+fileExtension(name))
	if err := os.Rename(temporaryPath, destination); err != nil {
		s.internalError(w, "finalize uploaded file", err)
		return
	}
	removeTemporary = false
	if formats["file"] == "" {
		formats["file"] = mimeType
	}
	clip, err := s.store.CreateFile(r.Context(), store.FileInput{
		ID:          id,
		UserID:      user.ID,
		DeviceID:    deviceID,
		DeviceName:  deviceName,
		MimeType:    mimeType,
		Name:        name,
		Size:        bytesWritten,
		Formats:     formats,
		StoragePath: destination,
		ExpiresAt:   time.Now().Add(s.cfg.FileTTL),
	})
	if err != nil {
		_ = os.Remove(destination)
		s.internalError(w, "insert file clip", err)
		return
	}
	s.hub.Publish(events.Event{Type: "created", UserID: user.ID, Clip: &clip})
	writeJSON(w, http.StatusCreated, clip)
}

func (s *Server) downloadFile(w http.ResponseWriter, r *http.Request, userID, id string) {
	clip, err := s.store.GetForUser(r.Context(), userID, id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "clip not found")
		return
	}
	if err != nil {
		s.internalError(w, "get file clip", err)
		return
	}
	if clip.Kind != "file" || clip.StoragePath == "" {
		writeError(w, http.StatusNotFound, "file content not found")
		return
	}
	if _, err := os.Stat(clip.StoragePath); errors.Is(err, os.ErrNotExist) {
		writeError(w, http.StatusNotFound, "file content has expired")
		return
	} else if err != nil {
		s.internalError(w, "stat file clip", err)
		return
	}
	w.Header().Set("Content-Type", clip.MimeType)
	w.Header().Set("Content-Disposition", "attachment; filename="+strconv.Quote(clip.Name))
	http.ServeFile(w, r, clip.StoragePath)
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	user, _ := requestUser(r)
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming is not supported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	after := parseEventSequence(r)
	client, unsubscribe := s.hub.Subscribe(user.ID, after)
	defer unsubscribe()

	writeSSE(w, "ready", map[string]any{"version": s.version, "lastSequence": s.hub.Latest(user.ID)})
	flusher.Flush()
	keepAlive := time.NewTicker(25 * time.Second)
	defer keepAlive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case event := <-client:
			writeSSEID(w, "clip", event.Sequence, event)
			flusher.Flush()
		case <-keepAlive.C:
			_, _ = io.WriteString(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

func parseEventSequence(r *http.Request) uint64 {
	raw := strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	if raw == "" {
		raw = strings.TrimSpace(r.URL.Query().Get("since"))
	}
	if raw == "" {
		return 0
	}
	sequence, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}
	return sequence
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		users, err := s.store.ListUsers(r.Context())
		if err != nil {
			s.internalError(w, "list users", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": users})
	case http.MethodPost:
		var request createUserRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid user body")
			return
		}
		user, err := s.store.CreateUser(r.Context(), request.Username, request.Password, request.Role)
		if errors.Is(err, store.ErrUsernameTaken) {
			writeError(w, http.StatusConflict, "username already exists")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, user)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) adminUserByID(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/users/"), "/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if r.Method != http.MethodDelete {
		w.Header().Set("Allow", "DELETE")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	current, _ := requestUser(r)
	if current.ID == id {
		writeError(w, http.StatusBadRequest, "cannot delete the current account")
		return
	}
	paths, err := s.store.DeleteUser(r.Context(), id)
	if errors.Is(err, store.ErrUserNotFound) {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	if errors.Is(err, store.ErrLastAdmin) {
		writeError(w, http.StatusConflict, "cannot delete the last administrator")
		return
	}
	if err != nil {
		s.internalError(w, "delete user", err)
		return
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			s.logger.Warn("remove deleted user file", "path", path, "error", err)
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	user, ok := requestUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "login required")
		return false
	}
	if !user.IsAdmin() {
		writeError(w, http.StatusForbidden, "administrator access required")
		return false
	}
	return true
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" || r.URL.Path == "/api/auth/login" || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}
		user, ok := s.authManager.CurrentUser(r.Context(), r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "login required")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	})
}

func requestUser(r *http.Request) (model.User, bool) {
	user, ok := r.Context().Value(userContextKey).(model.User)
	return user, ok && user.ID != ""
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := origin == ""
		specificOrigin := false
		for _, configured := range s.cfg.CORSOrigins {
			if configured == "*" || configured == origin {
				allowed = true
				if configured == "*" {
					origin = "*"
				} else {
					specificOrigin = true
				}
				break
			}
		}
		if allowed && origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			if specificOrigin {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) internalError(w http.ResponseWriter, operation string, err error) {
	s.logger.Error(operation, "error", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}

func writeSSE(w io.Writer, eventName string, payload any) {
	writeSSEID(w, eventName, 0, payload)
}

func writeSSEID(w io.Writer, eventName string, id uint64, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	if id > 0 {
		_, _ = fmt.Fprintf(w, "id: %d\n", id)
	}
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, data)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func safeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "\r", "")
	name = strings.ReplaceAll(name, "\n", "")
	if len(name) > 240 {
		name = name[:240]
	}
	return name
}

func fileExtension(name string) string {
	extension := filepath.Ext(name)
	if len(extension) < 2 || len(extension) > 16 {
		return ".bin"
	}
	for _, char := range extension[1:] {
		if !((char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9')) {
			return ".bin"
		}
	}
	return extension
}

func firstError(errorsToCheck ...error) error {
	for _, err := range errorsToCheck {
		if err != nil {
			return err
		}
	}
	return nil
}
