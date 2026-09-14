package middlewares

import (
	"context"
	"errors"
	"job-applications/internal/db"
	"job-applications/internal/session"
	"net/http"
	"net/http/httptest"
	"testing"
)

// --- mock session store ---

type mockSessionStore struct {
	getFunc    func(ctx context.Context, id string) (db.Session, error)
	updateFunc func(ctx context.Context, arg db.UpdateSessionActivityParams) error
	deleteFunc func(ctx context.Context, id string) error
}

func (m *mockSessionStore) GetSessionByID(ctx context.Context, id string) (db.Session, error) {
	return m.getFunc(ctx, id)
}

func (m *mockSessionStore) UpdateSessionActivity(ctx context.Context, arg db.UpdateSessionActivityParams) error {
	if m.updateFunc != nil {
		return m.updateFunc(ctx, arg)
	}
	return nil
}

func (m *mockSessionStore) DeleteSession(ctx context.Context, id string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, id)
	}
	return nil
}

func TestRequireAuth_NoCookie_Redirects(t *testing.T) {
	store := &mockSessionStore{}
	mgr := session.NewManager(store)

	handler := RequireAuth(*mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not have been called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/home", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}
	if loc := w.Header().Get("Location"); loc != "/" {
		t.Errorf("Location = %q, want /", loc)
	}
}

func TestRequireAuth_InvalidSession_Redirects(t *testing.T) {
	store := &mockSessionStore{
		getFunc: func(_ context.Context, _ string) (db.Session, error) {
			return db.Session{}, errors.New("not found")
		},
	}
	mgr := session.NewManager(store)

	handler := RequireAuth(*mgr)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not have been called")
	}))

	req := httptest.NewRequest(http.MethodGet, "/home", nil)
	req.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: "bad-session"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", w.Code, http.StatusSeeOther)
	}
}

func TestRequireAuth_ValidSession_PassesThrough(t *testing.T) {
	store := &mockSessionStore{
		getFunc: func(_ context.Context, id string) (db.Session, error) {
			return db.Session{
				ID:     id,
				UserID: 5,
			}, nil
		},
		updateFunc: func(_ context.Context, _ db.UpdateSessionActivityParams) error {
			return nil
		},
	}
	mgr := session.NewManager(store)

	var capturedUserID int32
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, ok := GetUserIDFromContext(r.Context())
		if !ok {
			t.Error("expected user_id in context")
		}
		capturedUserID = uid
		w.WriteHeader(http.StatusOK)
	})

	handler := RequireAuth(*mgr)(inner)

	req := httptest.NewRequest(http.MethodGet, "/home", nil)
	req.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: "valid-session"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if capturedUserID != 5 {
		t.Errorf("user_id = %d, want 5", capturedUserID)
	}
}

func TestRequireAuth_UpdateActivityError_StillPassesThrough(t *testing.T) {
	store := &mockSessionStore{
		getFunc: func(_ context.Context, id string) (db.Session, error) {
			return db.Session{ID: id, UserID: 1}, nil
		},
		updateFunc: func(_ context.Context, _ db.UpdateSessionActivityParams) error {
			return errors.New("update failed")
		},
	}
	mgr := session.NewManager(store)

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := RequireAuth(*mgr)(inner)
	req := httptest.NewRequest(http.MethodGet, "/home", nil)
	req.AddCookie(&http.Cookie{Name: session.SessionCookieName, Value: "sess"})
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("inner handler should still be called even if UpdateActivity fails")
	}
}

func TestGetSessionFromContext_Present(t *testing.T) {
	sess := &session.Session{ID: "abc", UserID: 10}
	ctx := context.WithValue(context.Background(), SessionContextKey, sess)

	got, ok := GetSessionFromContext(ctx)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got.ID != "abc" {
		t.Errorf("ID = %q, want abc", got.ID)
	}
}

func TestGetSessionFromContext_Missing(t *testing.T) {
	_, ok := GetSessionFromContext(context.Background())
	if ok {
		t.Error("expected ok=false for empty context")
	}
}

func TestGetUserIDFromContext_Present(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserIDContextKey, int32(42))

	uid, ok := GetUserIDFromContext(ctx)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if uid != 42 {
		t.Errorf("uid = %d, want 42", uid)
	}
}

func TestGetUserIDFromContext_Missing(t *testing.T) {
	_, ok := GetUserIDFromContext(context.Background())
	if ok {
		t.Error("expected ok=false for empty context")
	}
}

func TestGetUserIDFromContext_WrongType(t *testing.T) {
	ctx := context.WithValue(context.Background(), UserIDContextKey, "not-an-int")
	_, ok := GetUserIDFromContext(ctx)
	if ok {
		t.Error("expected ok=false for wrong type in context")
	}
}
