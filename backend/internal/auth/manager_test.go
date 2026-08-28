package auth

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestCredentialsAndSession(t *testing.T) {
	manager := New("admin", "a-strong-local-password", time.Hour, false)
	if !manager.ValidCredentials("admin", "a-strong-local-password") {
		t.Fatal("valid credentials were rejected")
	}
	if manager.ValidCredentials("admin", "wrong-password") {
		t.Fatal("invalid password was accepted")
	}

	token, err := manager.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	manager.SetCookie(response, token)
	request := httptest.NewRequest("GET", "/api/auth/me", nil)
	request.AddCookie(response.Result().Cookies()[0])
	if !manager.Authenticated(request) {
		t.Fatal("fresh session was rejected")
	}
}

func TestExpiredSessionIsRejected(t *testing.T) {
	manager := New("admin", "password", time.Nanosecond, false)
	token, err := manager.CreateSession()
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	manager.SetCookie(response, token)
	time.Sleep(2 * time.Millisecond)
	request := httptest.NewRequest("GET", "/api/auth/me", nil)
	request.AddCookie(response.Result().Cookies()[0])
	if manager.Authenticated(request) {
		t.Fatal("expired session was accepted")
	}
}
