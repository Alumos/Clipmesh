package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"time"

	"github.com/Alumos/Clipmesh/backend/internal/model"
)

const sessionCookieName = "clipmesh_session"

// UserRepository keeps authentication independent from the concrete database.
type UserRepository interface {
	AuthenticateUser(context.Context, string, string) (model.User, error)
	SaveSession(context.Context, string, string, time.Time) error
	UserForSession(context.Context, string) (model.User, error)
	DeleteSession(context.Context, string) error
}

type Manager struct {
	repo         UserRepository
	cookieSecure bool
	sessionTTL   time.Duration
}

func New(repo UserRepository, sessionTTL time.Duration, cookieSecure bool) *Manager {
	return &Manager{
		repo:         repo,
		cookieSecure: cookieSecure,
		sessionTTL:   sessionTTL,
	}
}

func (m *Manager) Authenticate(ctx context.Context, username, password string) (model.User, error) {
	return m.repo.AuthenticateUser(ctx, username, password)
}

func (m *Manager) CreateSessionForUser(ctx context.Context, user model.User) (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	expiresAt := time.Now().Add(m.sessionTTL)
	if err := m.repo.SaveSession(ctx, hashToken(token), user.ID, expiresAt); err != nil {
		return "", err
	}
	return token, nil
}

func (m *Manager) CurrentUser(ctx context.Context, r *http.Request) (model.User, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return model.User{}, false
	}
	user, err := m.repo.UserForSession(ctx, hashToken(cookie.Value))
	return user, err == nil
}

func (m *Manager) RevokeSession(ctx context.Context, r *http.Request) error {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil
	}
	return m.repo.DeleteSession(ctx, hashToken(cookie.Value))
}

func (m *Manager) SetCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(m.sessionTTL / time.Second),
		HttpOnly: true,
		Secure:   m.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func (m *Manager) ClearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   m.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}
