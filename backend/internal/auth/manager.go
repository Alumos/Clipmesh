package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/Alumos/Clipmesh/backend/internal/model"
)

const sessionCookieName = "clipmesh_session"

// UserRepository is implemented by the SQLite store. Keeping the interface in
// the auth package makes the session manager easy to exercise without HTTP or
// database details.
type UserRepository interface {
	AuthenticateUser(context.Context, string, string) (model.User, error)
	SaveSession(context.Context, string, string, time.Time) error
	UserForSession(context.Context, string) (model.User, error)
	DeleteSession(context.Context, string) error
}

type legacySession struct {
	expiresAt time.Time
	user      model.User
}

type Manager struct {
	repo               UserRepository
	legacyUser         model.User
	legacyPasswordHash [32]byte
	cookieSecure       bool
	sessionTTL         time.Duration

	mu       sync.Mutex
	sessions map[string]legacySession
}

// New remains available for compatibility with the original unit tests and
// small embedders. Production uses NewWithStore, which persists users and
// sessions in SQLite.
func New(username, password string, sessionTTL time.Duration, cookieSecure bool) *Manager {
	return &Manager{
		legacyUser:         model.User{ID: "legacy-admin", Username: username, Role: "admin"},
		legacyPasswordHash: sha256.Sum256([]byte(password)),
		cookieSecure:       cookieSecure,
		sessionTTL:         sessionTTL,
		sessions:           make(map[string]legacySession),
	}
}

func NewWithStore(repo UserRepository, sessionTTL time.Duration, cookieSecure bool) *Manager {
	return &Manager{
		repo:         repo,
		cookieSecure: cookieSecure,
		sessionTTL:   sessionTTL,
		sessions:     make(map[string]legacySession),
	}
}

func (m *Manager) Username() string {
	return m.legacyUser.Username
}

func (m *Manager) Authenticate(ctx context.Context, username, password string) (model.User, error) {
	if m.repo != nil {
		return m.repo.AuthenticateUser(ctx, username, password)
	}
	if !equalSecret(username, m.legacyUser.Username) || !equalSecretHash(password, m.legacyPasswordHash) {
		// This branch is only for callers using the backwards-compatible
		// in-memory constructor; production uses the repository path.
		return model.User{}, errors.New("invalid username or password")
	}
	return m.legacyUser, nil
}

// ValidCredentials is retained for compatibility. Repository-backed callers
// should prefer Authenticate so the request context is propagated.
func (m *Manager) ValidCredentials(username, password string) bool {
	if m.repo == nil {
		return equalSecret(username, m.legacyUser.Username) && equalSecretHash(password, m.legacyPasswordHash)
	}
	_, err := m.repo.AuthenticateUser(context.Background(), username, password)
	return err == nil
}

func (m *Manager) CreateSession() (string, error) {
	return m.createSession(context.Background(), m.legacyUser)
}

func (m *Manager) CreateSessionForUser(ctx context.Context, user model.User) (string, error) {
	return m.createSession(ctx, user)
}

func (m *Manager) createSession(ctx context.Context, user model.User) (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	expiresAt := time.Now().Add(m.sessionTTL)
	if m.repo != nil {
		if err := m.repo.SaveSession(ctx, hashToken(token), user.ID, expiresAt); err != nil {
			return "", err
		}
		return token, nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for existing, session := range m.sessions {
		if !session.expiresAt.After(now) {
			delete(m.sessions, existing)
		}
	}
	m.sessions[token] = legacySession{expiresAt: expiresAt, user: user}
	return token, nil
}

func (m *Manager) CurrentUser(ctx context.Context, r *http.Request) (model.User, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return model.User{}, false
	}
	if m.repo != nil {
		user, err := m.repo.UserForSession(ctx, hashToken(cookie.Value))
		return user, err == nil
	}
	now := time.Now()
	m.mu.Lock()
	defer m.mu.Unlock()
	session, ok := m.sessions[cookie.Value]
	if !ok || !session.expiresAt.After(now) {
		delete(m.sessions, cookie.Value)
		return model.User{}, false
	}
	return session.user, true
}

func (m *Manager) Authenticated(r *http.Request) bool {
	_, ok := m.CurrentUser(context.Background(), r)
	return ok
}

func (m *Manager) RevokeSession(ctx context.Context, r *http.Request) error {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil
	}
	if m.repo != nil {
		return m.repo.DeleteSession(ctx, hashToken(cookie.Value))
	}
	m.mu.Lock()
	delete(m.sessions, cookie.Value)
	m.mu.Unlock()
	return nil
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

func equalSecret(got, want string) bool {
	gotHash := sha256.Sum256([]byte(got))
	wantHash := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gotHash[:], wantHash[:]) == 1
}

func equalSecretHash(got string, want [32]byte) bool {
	gotHash := sha256.Sum256([]byte(got))
	return subtle.ConstantTimeCompare(gotHash[:], want[:]) == 1
}
