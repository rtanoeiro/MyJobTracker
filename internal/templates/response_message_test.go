package templates

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRenderResponseMessage_ErrorClass(t *testing.T) {
	codes := []int{http.StatusBadRequest, http.StatusNotFound, http.StatusInternalServerError}

	for _, code := range codes {
		w := httptest.NewRecorder()
		RenderResponseMessage(w, code, "something went wrong")

		body := w.Body.String()
		if !strings.Contains(body, `alert-error`) {
			t.Errorf("status %d: expected alert-error class, got %q", code, body)
		}
		if !strings.Contains(body, "something went wrong") {
			t.Errorf("status %d: expected message in body, got %q", code, body)
		}
		if w.Code != code {
			t.Errorf("expected status %d, got %d", code, w.Code)
		}
	}
}

func TestRenderResponseMessage_SuccessClass(t *testing.T) {
	w := httptest.NewRecorder()
	RenderResponseMessage(w, http.StatusOK, "all good")

	body := w.Body.String()
	if !strings.Contains(body, `alert-success`) {
		t.Errorf("expected alert-success class, got %q", body)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestRenderResponseMessage_RedirectCode_NoClass(t *testing.T) {
	w := httptest.NewRecorder()
	RenderResponseMessage(w, http.StatusFound, "redirecting")

	body := w.Body.String()
	// 3xx codes fall into neither the error nor success range, so messageClass is ""
	if !strings.Contains(body, `class="alert alert-"`) {
		t.Errorf("expected empty message class for 3xx code, got %q", body)
	}
	if w.Code != http.StatusFound {
		t.Errorf("expected status %d, got %d", http.StatusFound, w.Code)
	}
}

func TestRenderResponseMessage_ContentType(t *testing.T) {
	w := httptest.NewRecorder()
	RenderResponseMessage(w, http.StatusOK, "test")

	ct := w.Header().Get("Content-Type")
	if ct != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8", ct)
	}
}
