package handlers

import (
	"context"
	"job-applications/internal/db"
	"job-applications/internal/middlewares"
	"job-applications/internal/session"
	"job-applications/internal/templates"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

type LoginStore interface {
	GetUserByEmailOrUsername(ctx context.Context, email string) (db.User, error)
	CreateSession(ctx context.Context, arg db.CreateSessionParams) (pgconn.CommandTag, error)
	DeleteSession(ctx context.Context, id string) error
}

type LoginHandler struct {
	Store    LoginStore
	Renderer templates.Renderer
}

func NewLoginHandler(store LoginStore, renderer templates.Renderer) *LoginHandler {
	return &LoginHandler{
		Store:    store,
		Renderer: renderer,
	}
}

func (handler *LoginHandler) Page(writer http.ResponseWriter, request *http.Request) {
	if err := handler.Renderer.Render(writer, "login", nil); err != nil {
		slog.Error("Failed to render login page", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render login page")
		return

	}
}

func (handler *LoginHandler) Login(writer http.ResponseWriter, request *http.Request) {
	emailOrUsername := request.FormValue("emailOrUsername")
	if emailOrUsername == "" {
		slog.Error("Email or Username are required to login")
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Email or Usernameme are required during Login")
		return
	}

	user, errUser := handler.Store.GetUserByEmailOrUsername(request.Context(), strings.ToLower(emailOrUsername))
	if errUser != nil {
		slog.Error("Failed to get user by email or username", "error", errUser)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Failed login, try again")
		return
	}

	sessionID := uuid.New().String()
	_, errSession := handler.Store.CreateSession(request.Context(), db.CreateSessionParams{
		ID:           sessionID,
		UserID:       user.ID,
		IpAddress:    middleware.GetClientIP(request.Context()),
		UserAgent:    request.UserAgent(),
		CreatedAt:    pgtype.Timestamp{Time: time.Now(), Valid: true},
		LastActivity: pgtype.Timestamp{Time: time.Now(), Valid: true},
		ExpiresAt:    pgtype.Timestamp{Time: time.Now().Add(time.Hour * 24), Valid: true},
	})
	if errSession != nil {
		slog.Error("Failed to create session", "error", errSession)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to create session, please try again")
		return
	}
	session.SetCookie(writer, request, sessionID, session.DefaultExpiration)

	slog.Info("login successful", "emailOrUsername", emailOrUsername, "user_id", user.ID)
	writer.Header().Set("HX-Redirect", "/account/home")
}

// POST /logout
func (handler *LoginHandler) Logout(writer http.ResponseWriter, request *http.Request) {

	sessionData, ok := middlewares.GetSessionFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get session from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get session from context")
		return
	}

	if err := handler.Store.DeleteSession(request.Context(), sessionData.ID); err != nil {
		slog.Error("Failed to delete session", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to delete session")
	}

	session.ClearCookie(writer, request)
	slog.Info("User logged out", "userID", sessionData.UserID, "sessionID", sessionData.ID)
	writer.Header().Set("HX-Redirect", "/")
	templates.RenderResponseMessage(writer, http.StatusOK, "Logout successfully")
}
