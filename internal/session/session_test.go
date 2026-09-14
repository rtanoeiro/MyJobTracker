package session

import (
	"context"
	"crypto/tls"
	"errors"
	"job-applications/internal/db"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestSetCookie(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	SetCookie(w, req, "abc123", 24*time.Hour)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	c := cookies[0]
	if c.Name != SessionCookieName {
		t.Errorf("cookie name = %q, want %q", c.Name, SessionCookieName)
	}
	if c.Value != "abc123" {
		t.Errorf("cookie value = %q, want abc123", c.Value)
	}
	if c.MaxAge != 86400 {
		t.Errorf("MaxAge = %d, want 86400", c.MaxAge)
	}
	if !c.HttpOnly {
		t.Error("expected HttpOnly to be true")
	}
	if c.Path != "/" {
		t.Errorf("Path = %q, want /", c.Path)
	}
}

func TestClearCookie(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	ClearCookie(w, req)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}

	c := cookies[0]
	if c.Value != "" {
		t.Errorf("cookie value = %q, want empty", c.Value)
	}
	if c.MaxAge != -1 {
		t.Errorf("MaxAge = %d, want -1", c.MaxAge)
	}
}

func TestSessionCookie_Attributes(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	SetCookie(w, req, "sess", time.Hour)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	c := cookies[0]
	if !c.HttpOnly {
		t.Error("expected HttpOnly to be true")
	}
	if c.SameSite != http.SameSiteLaxMode {
		t.Errorf("SameSite = %v, want %v", c.SameSite, http.SameSiteLaxMode)
	}
	if c.Secure {
		t.Error("expected Secure to be false over plain HTTP")
	}
}

func TestSessionCookie_SecureOverTLS(t *testing.T) {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.TLS = &tls.ConnectionState{}
	SetCookie(w, req, "sess", time.Hour)

	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if !cookies[0].Secure {
		t.Error("expected Secure to be true over TLS")
	}
}

func TestGetSessionIDFromRequest_Present(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "session-xyz"})

	id, err := GetSessionIDFromRequest(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "session-xyz" {
		t.Errorf("got %q, want session-xyz", id)
	}
}

func TestGetSessionIDFromRequest_Missing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	_, err := GetSessionIDFromRequest(req)
	if err == nil {
		t.Error("expected error for missing cookie, got nil")
	}
}

// --- Manager tests with mock store ---

type mockSessionStore struct {
	getFunc            func(ctx context.Context, id string) (db.Session, error)
	updateActivityFunc func(ctx context.Context, arg db.UpdateSessionActivityParams) error
	deleteFunc         func(ctx context.Context, id string) error
}

func (m *mockSessionStore) GetSessionByID(ctx context.Context, id string) (db.Session, error) {
	return m.getFunc(ctx, id)
}

func (m *mockSessionStore) UpdateSessionActivity(ctx context.Context, arg db.UpdateSessionActivityParams) error {
	return m.updateActivityFunc(ctx, arg)
}

func (m *mockSessionStore) DeleteSession(ctx context.Context, id string) error {
	return m.deleteFunc(ctx, id)
}

func TestManager_Get_Success(t *testing.T) {
	store := &mockSessionStore{
		getFunc: func(_ context.Context, id string) (db.Session, error) {
			return db.Session{
				ID:     id,
				UserID: 7,
			}, nil
		},
	}

	mgr := NewManager(store)
	sess, err := mgr.Get(context.Background(), "test-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sess.ID != "test-id" {
		t.Errorf("ID = %q, want test-id", sess.ID)
	}
	if sess.UserID != 7 {
		t.Errorf("UserID = %d, want 7", sess.UserID)
	}
}

func TestManager_Get_NotFound(t *testing.T) {
	store := &mockSessionStore{
		getFunc: func(_ context.Context, _ string) (db.Session, error) {
			return db.Session{}, pgx.ErrNoRows
		},
	}

	mgr := NewManager(store)
	_, err := mgr.Get(context.Background(), "missing")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestManager_Get_DBError(t *testing.T) {
	dbErr := errors.New("connection refused")
	store := &mockSessionStore{
		getFunc: func(_ context.Context, _ string) (db.Session, error) {
			return db.Session{}, dbErr
		},
	}

	mgr := NewManager(store)
	_, err := mgr.Get(context.Background(), "any")
	if !errors.Is(err, dbErr) {
		t.Errorf("expected wrapped db error, got %v", err)
	}
}

func TestManager_UpdateActivity(t *testing.T) {
	var captured db.UpdateSessionActivityParams
	store := &mockSessionStore{
		updateActivityFunc: func(_ context.Context, arg db.UpdateSessionActivityParams) error {
			captured = arg
			return nil
		},
	}

	mgr := NewManager(store)
	err := mgr.UpdateActivity(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if captured.ID != "sess-1" {
		t.Errorf("captured ID = %q, want sess-1", captured.ID)
	}
	if !captured.ExpiresAt.Valid {
		t.Error("expected ExpiresAt to be valid")
	}
}

func TestManager_Delete(t *testing.T) {
	var deletedID string
	store := &mockSessionStore{
		deleteFunc: func(_ context.Context, id string) error {
			deletedID = id
			return nil
		},
	}

	mgr := NewManager(store)
	err := mgr.Delete(context.Background(), "sess-to-delete")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deletedID != "sess-to-delete" {
		t.Errorf("deleted ID = %q, want sess-to-delete", deletedID)
	}
}

func TestManager_Delete_Error(t *testing.T) {
	store := &mockSessionStore{
		deleteFunc: func(_ context.Context, _ string) error {
			return errors.New("delete failed")
		},
	}

	mgr := NewManager(store)
	err := mgr.Delete(context.Background(), "any")
	if err == nil {
		t.Error("expected error, got nil")
	}
}
