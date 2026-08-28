package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/Alumos/Clipmesh/backend/internal/auth"
	"github.com/Alumos/Clipmesh/backend/internal/config"
	"github.com/Alumos/Clipmesh/backend/internal/events"
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

func New(cfg config.Config, clips *store.Store, hub *events.Hub, logger *slog.Logger, version string) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	s := &Server{
		cfg:         cfg,
		store:       clips,
		hub:         hub,
		logger:      logger,
		authManager: auth.New(clips, cfg.SessionTTL, cfg.CookieSecure),
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

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    "ok",
		"version":   s.version,
		"uptime":    time.Since(s.startedAt).Round(time.Second).String(),
		"startedAt": s.startedAt,
	})
}

func (s *Server) config(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"textLimit":      s.cfg.TextLimit,
		"fileTtlSeconds": int64(s.cfg.FileTTL / time.Second),
		"maxUploadBytes": s.cfg.MaxUploadSize,
		"pageSize":       6,
	})
}
