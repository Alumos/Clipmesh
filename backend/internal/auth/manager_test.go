package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Alumos/Clipmesh/backend/internal/model"
)

var errInvalidCredentials = errors.New("invalid credentials")

type testSession struct {
	userID    string
	expiresAt time.Time
}

type testRepository struct {
	user     model.User
	password string
	sessions map[string]testSession
}

func newTestRepository() *testRepository {
	return &testRepository{
		user:     model.User{ID: "user-1", Username: "admin", Role: "admin"},
		password: "a-strong-local-password",
		sessions: make(map[string]testSession),
	}
}

func (r *testRepository) AuthenticateUser(_ context.Context, username, password string) (model.User, error) {
	if username != r.user.Username || password != r.password {
		return model.User{}, errInvalidCredentials
	}
	return r.user, nil
}

func (r *testRepository) SaveSession(_ context.Context, tokenHash, userID string, expiresAt time.Time) error {
	r.sessions[tokenHash] = testSession{userID: userID, expiresAt: expiresAt}
	return nil
}

func (r *testRepository) UserForSession(_ context.Context, tokenHash string) (model.User, error) {
	session, ok := r.sessions[tokenHash]
	if !ok || session.userID != r.user.ID || !session.expiresAt.After(time.Now()) {
		return model.User{}, errInvalidCredentials
	}
	return r.user, nil
}

func (r *testRepository) DeleteSession(_ context.Context, tokenHash string) error {
	delete(r.sessions, tokenHash)
	return nil
}

func TestCredentialsAndSession(t *testing.T) {
	repository := newTestRepository()
	manager := New(repository, time.Hour, false)
	user, err := manager.Authenticate(context.Background(), "admin", repository.password)
	if err != nil {
		t.Fatalf("valid credentials were rejected: %v", err)
	}
	if _, err := manager.Authenticate(context.Background(), "admin", "wrong-password"); err == nil {
		t.Fatal("invalid password was accepted")
	}

	token, err := manager.CreateSessionForUser(context.Background(), user)
	if err != nil {
		t.Fatal(err)
	}
	request := requestWithSessionCookie(t, manager, token)
	currentUser, ok := manager.CurrentUser(context.Background(), request)
	if !ok || currentUser.ID != user.ID {
		t.Fatal("fresh session was rejected")
	}

	if err := manager.RevokeSession(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if _, ok := manager.CurrentUser(context.Background(), request); ok {
		t.Fatal("revoked session was accepted")
	}
}

func TestExpiredSessionIsRejected(t *testing.T) {
	repository := newTestRepository()
	manager := New(repository, time.Nanosecond, false)
	token, err := manager.CreateSessionForUser(context.Background(), repository.user)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)
	if _, ok := manager.CurrentUser(context.Background(), requestWithSessionCookie(t, manager, token)); ok {
		t.Fatal("expired session was accepted")
	}
}

func requestWithSessionCookie(t *testing.T, manager *Manager, token string) *http.Request {
	t.Helper()
	response := httptest.NewRecorder()
	manager.SetCookie(response, token)
	request := httptest.NewRequest("GET", "/api/auth/me", nil)
	request.AddCookie(response.Result().Cookies()[0])
	return request
}
