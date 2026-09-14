package handlers

import (
	"context"
	"job-applications/internal/db"
	"job-applications/internal/middlewares"
	"job-applications/internal/templates"
	"job-applications/internal/utils"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type NotesStore interface {
	GetAllUserNotes(ctx context.Context, userID int32) ([]db.Note, error)
	GetNoteByID(ctx context.Context, arg db.GetNoteByIDParams) (db.Note, error)
	CreateNote(ctx context.Context, arg db.CreateNoteParams) (db.Note, error)
	UpdateNote(ctx context.Context, arg db.UpdateNoteParams) error
	DeleteNote(ctx context.Context, arg db.DeleteNoteParams) error
}

type NotesHandler struct {
	Store    NotesStore
	Renderer templates.Renderer
}

func NewNotesHandler(store NotesStore, renderer templates.Renderer) *NotesHandler {
	return &NotesHandler{
		Store:    store,
		Renderer: renderer,
	}
}

// GET /notes - Returns list of all notes
func (handler *NotesHandler) Page(writer http.ResponseWriter, request *http.Request) {
	data := map[string]any{
		"ActivePage": "notes",
	}
	if err := handler.Renderer.Render(writer, "notes", data); err != nil {
		slog.Error("Failed to render notes list", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render notes list")
		return
	}
	slog.Info("List notes")
}

// GET /notes/list - Returns list of all notes
func (handler *NotesHandler) List(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID")
		return
	}

	notes, err := handler.Store.GetAllUserNotes(request.Context(), userID)
	if err != nil {
		slog.Error("Failed to get all notes", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get all notes")
		return
	}

	if err := handler.Renderer.Render(writer, "notes-list", notes); err != nil {
		slog.Error("Failed to render notes list", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render notes list")
		return
	}
	slog.Info("List notes")
}

// GET /notes/new - Returns modal HTML with empty form
func (handler *NotesHandler) NewForm(writer http.ResponseWriter, request *http.Request) {
	if err := handler.Renderer.Render(writer, "note-form", nil); err != nil {
		slog.Error("Failed to render note form", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render note form")
		return
	}
	slog.Info("New note form requested")
}

// POST /notes - Creates new note
func (handler *NotesHandler) Create(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get userID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get userID from context")
		return
	}

	noteHeader := request.FormValue("note_header")
	noteText := request.FormValue("note_text")

	if noteHeader == "" {
		slog.Error("Note header is required")
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Note header is required")
		return
	}

	params := db.CreateNoteParams{
		UserID:     userID,
		NoteHeader: noteHeader,
		NoteText:   utils.PGText(noteText),
	}

	createdNote, err := handler.Store.CreateNote(request.Context(), params)
	if err != nil {
		slog.Error("Failed to create note", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to create note")
		return
	}

	writer.Header().Set("HX-Trigger", "notes-updated")
	slog.Info("Note created", "note_id", createdNote.ID)
}

// GET /notes/{id}/edit - Returns modal HTML with pre-filled form
func (handler *NotesHandler) EditForm(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID")
		return
	}
	id := chi.URLParam(request, "id")
	idInt, err := utils.ConvertFromStrToInt32(id)
	if err != nil {
		slog.Error("Failed to convert note ID to int", "error", err)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Invalid note ID")
		return
	}

	note, err := handler.Store.GetNoteByID(request.Context(), db.GetNoteByIDParams{ID: idInt, UserID: userID})
	if err != nil {
		slog.Error("Failed to get note by ID", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get note")
		return
	}

	if err := handler.Renderer.Render(writer, "note-edit-form", note); err != nil {
		slog.Error("Failed to render note edit form", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render note edit form")
		return
	}

	slog.Info("Edit note form requested", "note_id", id)
}

// PUT /notes/{id} - Updates existing note
func (handler *NotesHandler) Update(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID")
		return
	}
	id := chi.URLParam(request, "id")
	idInt, err := utils.ConvertFromStrToInt32(id)
	if err != nil {
		slog.Error("Failed to convert note ID to int", "error", err)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Invalid note ID")
		return
	}

	noteHeader := request.FormValue("note_header")
	noteText := request.FormValue("note_text")

	if noteHeader == "" {
		slog.Error("Note header is required")
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Note header is required")
		return
	}

	params := db.UpdateNoteParams{
		ID:         idInt,
		NoteHeader: noteHeader,
		NoteText:   utils.PGText(noteText),
		UserID:     userID,
	}

	if err := handler.Store.UpdateNote(request.Context(), params); err != nil {
		slog.Error("Failed to update note", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to update note")
		return
	}

	writer.Header().Set("HX-Trigger", "notes-updated")
	slog.Info("Note updated", "note_id", id)
}

// DELETE /notes/{id} - Deletes note
func (handler *NotesHandler) Delete(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID")
		return
	}
	id := chi.URLParam(request, "id")
	idInt, err := utils.ConvertFromStrToInt32(id)
	if err != nil {
		slog.Error("Failed to convert note ID to int", "error", err)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Invalid note ID")
		return
	}

	if err := handler.Store.DeleteNote(request.Context(), db.DeleteNoteParams{ID: idInt, UserID: userID}); err != nil {
		slog.Error("Failed to delete note", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to delete note")
		return
	}

	slog.Info("Note deleted", "note_id", id)
}
