package handlers

import (
	"context"
	"fmt"
	"job-applications/internal/db"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type MockRegisterStore struct {
	ShouldFailCreateUser bool
}

func (m *MockRegisterStore) CreateUser(_ context.Context, _ db.CreateUserParams) (db.User, error) {
	if m.ShouldFailCreateUser {
		return db.User{}, fmt.Errorf("mock error: unique constraint violation")
	}
	return db.User{ID: 1, Email: "new@user.com"}, nil
}

func TestRegisterHandler_MissingFields(t *testing.T) {
	handler := NewRegisterHandler(&MockRegisterStore{}, newMockRenderer(false))

	tests := []struct {
		name   string
		values url.Values
	}{
		{"empty email and user", url.Values{"email": {""}, "username": {""}}},
		{"empty email", url.Values{"email": {""}, "username": {"username"}}},
		{"empty user", url.Values{"email": {"a@b.com"}, "username": {""}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := formRequest(http.MethodPost, "/register", tt.values)
			w := httptest.NewRecorder()

			handler.RegisterAccount(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestRegisterHandler_DBError(t *testing.T) {
	handler := NewRegisterHandler(
		&MockRegisterStore{ShouldFailCreateUser: true},
		newMockRenderer(false),
	)

	values := url.Values{
		"email":    {"a@b.com"},
		"username": {"username"},
	}
	req := formRequest(http.MethodPost, "/register", values)
	w := httptest.NewRecorder()

	handler.RegisterAccount(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestRegisterHandler_Success(t *testing.T) {
	handler := NewRegisterHandler(&MockRegisterStore{}, newMockRenderer(false))

	values := url.Values{
		"email":    {"new@user.com"},
		"username": {"username"},
	}
	req := formRequest(http.MethodPost, "/register", values)
	w := httptest.NewRecorder()

	handler.RegisterAccount(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRegisterHandler_RenderError(t *testing.T) {
	handler := NewRegisterHandler(&MockRegisterStore{}, newMockRenderer(true))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/register", nil)

	handler.Page(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}
