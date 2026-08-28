package httpapi

import (
	"errors"
	"net/http"

	"github.com/Alumos/Clipmesh/backend/internal/store"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	var request loginRequest
	if err := decodeJSON(w, r, 16<<10, &request); err != nil {
		writeError(w, http.StatusBadRequest, "invalid login body")
		return
	}
	user, err := s.authManager.Authenticate(r.Context(), request.Username, request.Password)
	if errors.Is(err, store.ErrInvalidCredentials) {
		// Keep the response generic so it cannot disclose which credential failed.
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
	if !allowMethod(w, r, http.MethodPost) {
		return
	}
	if err := s.authManager.RevokeSession(r.Context(), r); err != nil {
		s.logger.Warn("revoke auth session", "error", err)
	}
	s.authManager.ClearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	if !allowMethod(w, r, http.MethodGet) {
		return
	}
	user, ok := requestUser(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "login required")
		return
	}
	writeJSON(w, http.StatusOK, user)
}
