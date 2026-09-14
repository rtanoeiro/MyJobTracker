package handlers

import (
	"context"
	"fmt"
	"job-applications/internal/db"
	"job-applications/internal/session"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

type MockLoginStore struct {
	ShouldFailGetUser       bool
	ShouldFailCreateSession bool
	ShouldFailDeleteSession bool
}

func (m *MockLoginStore) GetUserByEmailOrUsername(_ context.Context, _ string) (db.User, error) {
	if m.ShouldFailGetUser {
		return db.User{}, fmt.Errorf("mock error: user not found")
	}
	return db.User{
		ID:       1,
		Email:    "user@test.com",
		Username: "username",
	}, nil
}

func (m *MockLoginStore) CreateSession(_ context.Context, _ db.CreateSessionParams) (pgconn.CommandTag, error) {
	if m.ShouldFailCreateSession {
		return pgconn.CommandTag{}, fmt.Errorf("mock error: session creation failed")
	}
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (m *MockLoginStore) DeleteSession(_ context.Context, _ string) error {
	if m.ShouldFailDeleteSession {
		return fmt.Errorf("mock error: delete session failed")
	}
	return nil
}

func TestLoginHandler_MissingFields(t *testing.T) {
	handler := NewLoginHandler(&MockLoginStore{}, newMockRenderer(false))

	tests := []struct {
		name   string
		values url.Values
	}{
		{"empty email", url.Values{"emailOrUsername": {""}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := formRequest(http.MethodPost, "/login", tt.values)
			w := httptest.NewRecorder()

			handler.Login(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestLoginHandler_UserLookupFailure(t *testing.T) {
	handler := NewLoginHandler(
		&MockLoginStore{ShouldFailGetUser: true},
		newMockRenderer(false),
	)

	values := url.Values{"emailOrUsername": {"missing@user.com"}}
	req := formRequest(http.MethodPost, "/login", values)
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestLoginHandler_Success(t *testing.T) {
	handler := NewLoginHandler(
		&MockLoginStore{},
		newMockRenderer(false),
	)

	values := url.Values{"emailOrUsername": {"user@test.com"}}
	req := formRequest(http.MethodPost, "/login", values)
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Header().Get("HX-Redirect") != "/account/home" {
		t.Errorf("HX-Redirect = %q, want /account/home", w.Header().Get("HX-Redirect"))
	}
}

func TestLoginHandler_SessionCreateError(t *testing.T) {
	handler := NewLoginHandler(
		&MockLoginStore{ShouldFailCreateSession: true},
		newMockRenderer(false),
	)

	values := url.Values{"emailOrUsername": {"user@test.com"}}
	req := formRequest(http.MethodPost, "/login", values)
	w := httptest.NewRecorder()

	handler.Login(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestLogout_NoSessionInContext(t *testing.T) {
	handler := NewLoginHandler(&MockLoginStore{}, newMockRenderer(false))

	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	w := httptest.NewRecorder()

	handler.Logout(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestLogout_Success(t *testing.T) {
	handler := NewLoginHandler(&MockLoginStore{}, newMockRenderer(false))

	sess := &session.Session{ID: "sess-123", UserID: 5}
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req = withSession(req, sess)
	w := httptest.NewRecorder()

	handler.Logout(w, req)

	if w.Header().Get("HX-Redirect") != "/" {
		t.Errorf("HX-Redirect = %q, want /", w.Header().Get("HX-Redirect"))
	}
}

func TestLogout_DeleteSessionError(t *testing.T) {
	handler := NewLoginHandler(
		&MockLoginStore{ShouldFailDeleteSession: true},
		newMockRenderer(false),
	)

	sess := &session.Session{ID: "sess-123", UserID: 5}
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req = withSession(req, sess)
	w := httptest.NewRecorder()

	handler.Logout(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestLoginHandler_RenderError(t *testing.T) {
	handler := NewLoginHandler(&MockLoginStore{}, newMockRenderer(true))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	handler.Page(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}
