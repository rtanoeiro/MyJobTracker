package handlers

import (
	"context"
	"job-applications/internal/db"
	"job-applications/internal/templates"
	"log/slog"
	"net/http"
)

type RegisterStore interface {
	CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error)
}

type RegisterHandler struct {
	Store    RegisterStore
	Renderer templates.Renderer
}

func NewRegisterHandler(store RegisterStore, renderer templates.Renderer) *RegisterHandler {
	return &RegisterHandler{
		Store:    store,
		Renderer: renderer,
	}
}

func (handler *RegisterHandler) Page(writer http.ResponseWriter, request *http.Request) {
	if err := handler.Renderer.Render(writer, "register", nil); err != nil {
		slog.Error("Failed to render register page", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render register page")
		return
	}
}

func (handler *RegisterHandler) RegisterAccount(writer http.ResponseWriter, request *http.Request) {
	email := request.FormValue("email")
	username := request.FormValue("username")
	if email == "" || username == "" {
		slog.Error("Failed to create user", "error", "Empty email or username, both should be provided")
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Failed to create user, please provide both an username and an email")
		return
	}

	createdUser, errCreate := handler.Store.CreateUser(request.Context(), db.CreateUserParams{
		Email:    email,
		Username: username,
	})
	if errCreate != nil {
		slog.Error("Failed to create user", "error", errCreate)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to create user, please contact support if you continue to experience this issue")
		return
	}

	slog.Info("User created", "user", createdUser)
	writer.Header().Set("HX-Redirect", "/")
}
