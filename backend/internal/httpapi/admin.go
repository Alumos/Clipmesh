package httpapi

import (
	"errors"
	"net/http"
	"os"
	"strings"

	"github.com/Alumos/Clipmesh/backend/internal/store"
)

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
		if err := decodeJSON(w, r, 16<<10, &request); err != nil {
			writeError(w, http.StatusBadRequest, "invalid user body")
			return
		}
		user, err := s.store.CreateUser(
			r.Context(),
			request.Username,
			request.Password,
			request.Role,
		)
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
	if !allowMethod(w, r, http.MethodDelete) {
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
