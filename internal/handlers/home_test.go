package handlers

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// TestMain is used as an entry point to Test Functions, and used here to discard log lines
func TestMain(m *testing.M) {
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	os.Exit(m.Run())

}

func TestHomePage_RenderError(t *testing.T) {
	handler := NewHomeHandler(newMockRenderer(true))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/home", nil)

	handler.Page(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}
