package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireHTMX_GET_PassesThrough(t *testing.T) {
	called := false
	handler := RequireHTMX(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("GET request should pass through")
	}
}

func TestRequireHTMX_POST_WithoutHeader_Forbidden(t *testing.T) {
	handler := RequireHTMX(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not have been called")
	}))

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestRequireHTMX_POST_WithHeader_PassesThrough(t *testing.T) {
	called := false
	handler := RequireHTMX(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("POST with HX-Request header should pass through")
	}
}
