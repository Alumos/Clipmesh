package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Alumos/Clipmesh/backend/internal/events"
	"github.com/Alumos/Clipmesh/backend/internal/model"
	"github.com/Alumos/Clipmesh/backend/internal/store"
)

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
		clips, err := s.store.ListForUser(
			r.Context(),
			user.ID,
			strings.TrimSpace(r.URL.Query().Get("q")),
			r.URL.Query().Get("kind"),
			limit,
		)
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
	if err := decodeJSON(w, r, 2<<20, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if request.Kind != "" && request.Kind != "text" {
		writeError(w, http.StatusBadRequest, "only text clips use this endpoint")
		return
	}
	clip, err := s.store.CreateTextForUser(
		r.Context(),
		userID,
		request.DeviceID,
		request.DeviceName,
		request.Formats,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.hub.Publish(events.Event{Type: "created", UserID: userID, Clip: &clip})
	writeJSON(w, http.StatusCreated, clip)
}

func (s *Server) clipByID(w http.ResponseWriter, r *http.Request) {
	user, _ := requestUser(r)
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/clips/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusNotFound, "clip not found")
		return
	}
	id := parts[0]
	if len(parts) == 2 && parts[1] == "file" {
		if !allowMethod(w, r, http.MethodGet) {
			return
		}
		s.downloadFile(w, r, user.ID, id)
		return
	}
	if len(parts) != 1 {
		writeError(w, http.StatusNotFound, "clip not found")
		return
	}
	if !allowMethod(w, r, http.MethodDelete) {
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
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	user, _ := requestUser(r)

	// Leave room for multipart headers, then enforce the payload limit again
	// while copying the selected file to disk.
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
		mimeType = mime.TypeByExtension(filepath.Ext(name))
		if mimeType == "" {
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
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{
		"filename": clip.Name,
	}))
	http.ServeFile(w, r, clip.StoragePath)
}

func safeFilename(name string) string {
	name = strings.ReplaceAll(strings.TrimSpace(name), "\\", "/")
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "\r", "")
	name = strings.ReplaceAll(name, "\n", "")
	name = strings.ToValidUTF8(name, "_")
	if len(name) <= 240 {
		return name
	}

	extension := filepath.Ext(name)
	if len(extension) > 16 {
		extension = ""
	}
	base := strings.TrimSuffix(name, filepath.Ext(name))
	return truncateUTF8(base, 240-len(extension)) + extension
}

func truncateUTF8(value string, maxBytes int) string {
	for len(value) > maxBytes {
		_, size := utf8.DecodeLastRuneInString(value)
		value = value[:len(value)-size]
	}
	return value
}

func fileExtension(name string) string {
	extension := filepath.Ext(name)
	if len(extension) < 2 || len(extension) > 16 {
		return ".bin"
	}
	for _, char := range extension[1:] {
		if !((char >= 'a' && char <= 'z') ||
			(char >= 'A' && char <= 'Z') ||
			(char >= '0' && char <= '9')) {
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
