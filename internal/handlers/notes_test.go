package handlers

import (
	"context"
	"fmt"
	"job-applications/internal/db"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type MockNotesStore struct {
	ShouldFailGetAllNotes bool
	ShouldFailGetNoteByID bool
	ShouldFailCreateNote  bool
	ShouldFailUpdateNote  bool
	ShouldFailDeleteNote  bool
}

func (m *MockNotesStore) GetAllUserNotes(_ context.Context, _ int32) ([]db.Note, error) {
	if m.ShouldFailGetAllNotes {
		return nil, fmt.Errorf("mock error: failed to get notes")
	}
	return []db.Note{{ID: 1, NoteHeader: "Test Note"}}, nil
}

func (m *MockNotesStore) GetNoteByID(_ context.Context, _ db.GetNoteByIDParams) (db.Note, error) {
	if m.ShouldFailGetNoteByID {
		return db.Note{}, fmt.Errorf("mock error: note not found")
	}
	return db.Note{ID: 1, NoteHeader: "Test Note"}, nil
}

func (m *MockNotesStore) CreateNote(_ context.Context, arg db.CreateNoteParams) (db.Note, error) {
	if m.ShouldFailCreateNote {
		return db.Note{}, fmt.Errorf("mock error: failed to create note")
	}
	return db.Note{ID: 10, NoteHeader: arg.NoteHeader}, nil
}

func (m *MockNotesStore) UpdateNote(_ context.Context, _ db.UpdateNoteParams) error {
	if m.ShouldFailUpdateNote {
		return fmt.Errorf("mock error: failed to update note")
	}
	return nil
}

func (m *MockNotesStore) DeleteNote(_ context.Context, _ db.DeleteNoteParams) error {
	if m.ShouldFailDeleteNote {
		return fmt.Errorf("mock error: failed to delete note")
	}
	return nil
}

func TestNotesNewForm_Success(t *testing.T) {
	handler := NewNotesHandler(&MockNotesStore{}, newMockRenderer(false))

	req := httptest.NewRequest(http.MethodGet, "/notes/new", nil)
	w := httptest.NewRecorder()

	handler.NewForm(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestNotesNewForm_RenderError(t *testing.T) {
	handler := NewNotesHandler(&MockNotesStore{}, newMockRenderer(true))

	req := httptest.NewRequest(http.MethodGet, "/notes/new", nil)
	w := httptest.NewRecorder()

	handler.NewForm(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestNotesCreate_NoUserID(t *testing.T) {
	handler := NewNotesHandler(&MockNotesStore{}, newMockRenderer(false))

	values := url.Values{"note_header": {"My Note"}, "note_text": {"content"}}
	req := formRequest(http.MethodPost, "/notes", values)
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestNotesCreate_EmptyHeader(t *testing.T) {
	handler := NewNotesHandler(&MockNotesStore{}, newMockRenderer(false))

	values := url.Values{"note_header": {""}, "note_text": {"some text"}}
	req := formRequest(http.MethodPost, "/notes", values)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if !strings.Contains(w.Body.String(), "Note header is required") {
		t.Errorf("body = %q, expected header required message", w.Body.String())
	}
}

func TestNotesCreate_Success(t *testing.T) {
	handler := NewNotesHandler(&MockNotesStore{}, newMockRenderer(false))

	values := url.Values{"note_header": {"My Note"}, "note_text": {"content"}}
	req := formRequest(http.MethodPost, "/notes", values)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Header().Get("HX-Trigger") != "notes-updated" {
		t.Errorf("HX-Trigger = %q, want notes-updated", w.Header().Get("HX-Trigger"))
	}
}

func TestNotesCreate_DBError(t *testing.T) {
	handler := NewNotesHandler(
		&MockNotesStore{ShouldFailCreateNote: true},
		newMockRenderer(false),
	)

	values := url.Values{"note_header": {"My Note"}}
	req := formRequest(http.MethodPost, "/notes", values)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestNotesEditForm_InvalidID(t *testing.T) {
	handler := NewNotesHandler(&MockNotesStore{}, newMockRenderer(false))

	req := requestWithURLParam(http.MethodGet, "/notes/abc/edit", "id", "abc")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.EditForm(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestNotesEditForm_DBError(t *testing.T) {
	handler := NewNotesHandler(
		&MockNotesStore{ShouldFailGetNoteByID: true},
		newMockRenderer(false),
	)

	req := requestWithURLParam(http.MethodGet, "/notes/1/edit", "id", "1")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.EditForm(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestNotesEditForm_RenderError(t *testing.T) {
	handler := NewNotesHandler(&MockNotesStore{}, newMockRenderer(true))

	req := requestWithURLParam(http.MethodGet, "/notes/1/edit", "id", "1")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.EditForm(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestNotesEditForm_Success(t *testing.T) {
	handler := NewNotesHandler(&MockNotesStore{}, newMockRenderer(false))

	req := requestWithURLParam(http.MethodGet, "/notes/1/edit", "id", "1")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.EditForm(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestNotesDelete_InvalidID(t *testing.T) {
	handler := NewNotesHandler(&MockNotesStore{}, newMockRenderer(false))

	req := requestWithURLParam(http.MethodDelete, "/notes/abc", "id", "abc")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestNotesDelete_Success(t *testing.T) {
	handler := NewNotesHandler(&MockNotesStore{}, newMockRenderer(false))

	req := requestWithURLParam(http.MethodDelete, "/notes/5", "id", "5")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestNotesDelete_DBError(t *testing.T) {
	handler := NewNotesHandler(
		&MockNotesStore{ShouldFailDeleteNote: true},
		newMockRenderer(false),
	)

	req := requestWithURLParam(http.MethodDelete, "/notes/5", "id", "5")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestNotesUpdate_InvalidID(t *testing.T) {
	handler := NewNotesHandler(&MockNotesStore{}, newMockRenderer(false))

	req := requestWithURLParam(http.MethodPut, "/notes/xyz", "id", "xyz")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestNotesUpdate_EmptyHeader(t *testing.T) {
	handler := NewNotesHandler(&MockNotesStore{}, newMockRenderer(false))

	values := url.Values{"note_header": {""}, "note_text": {"text"}}
	req := formRequestWithURLParam(http.MethodPut, "/notes/1", values, "id", "1")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestNotesUpdate_Success(t *testing.T) {
	handler := NewNotesHandler(&MockNotesStore{}, newMockRenderer(false))

	values := url.Values{"note_header": {"Updated"}, "note_text": {"new text"}}
	req := formRequestWithURLParam(http.MethodPut, "/notes/1", values, "id", "1")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Header().Get("HX-Trigger") != "notes-updated" {
		t.Errorf("HX-Trigger = %q, want notes-updated", w.Header().Get("HX-Trigger"))
	}
}

func TestNotesUpdate_DBError(t *testing.T) {
	handler := NewNotesHandler(
		&MockNotesStore{ShouldFailUpdateNote: true},
		newMockRenderer(false),
	)

	values := url.Values{"note_header": {"Updated"}, "note_text": {"text"}}
	req := formRequestWithURLParam(http.MethodPut, "/notes/1", values, "id", "1")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestNotesPage_Success(t *testing.T) {
	handler := NewNotesHandler(&MockNotesStore{}, newMockRenderer(false))

	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	w := httptest.NewRecorder()

	handler.Page(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestNotesPage_RenderError(t *testing.T) {
	handler := NewNotesHandler(&MockNotesStore{}, newMockRenderer(true))

	req := httptest.NewRequest(http.MethodGet, "/notes", nil)
	w := httptest.NewRecorder()

	handler.Page(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestNotesList_RenderError(t *testing.T) {
	handler := NewNotesHandler(&MockNotesStore{}, newMockRenderer(true))

	req := httptest.NewRequest(http.MethodGet, "/notes/list", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestNotesList_NoUserID(t *testing.T) {
	handler := NewNotesHandler(&MockNotesStore{}, newMockRenderer(false))

	req := httptest.NewRequest(http.MethodGet, "/notes/list", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestNotesList_DBError(t *testing.T) {
	handler := NewNotesHandler(
		&MockNotesStore{ShouldFailGetAllNotes: true},
		newMockRenderer(false),
	)

	req := httptest.NewRequest(http.MethodGet, "/notes/list", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestNotesList_Success(t *testing.T) {
	handler := NewNotesHandler(&MockNotesStore{}, newMockRenderer(false))

	req := httptest.NewRequest(http.MethodGet, "/notes/list", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
