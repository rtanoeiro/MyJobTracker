package server

import (
	"job-applications/internal/ai"
	"job-applications/internal/database"
	"job-applications/internal/handlers"
	"job-applications/internal/middlewares"
	"job-applications/internal/session"
	"job-applications/internal/templates"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func setupRoutes(r chi.Router, db *database.DB, renderer *templates.BaseRenderer) {
	// Global middleware: wraps every request, including public and static routes
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(middlewares.RequireHTMX)

	// Manager is a struct that has access to the Store that satisfies the functions that connect to DB
	sessionManager := session.NewManager(db)
	authMiddleware := middlewares.RequireAuth(*sessionManager)

	// Handlers
	loginHandler := handlers.NewLoginHandler(db, renderer)
	registerHandler := handlers.NewRegisterHandler(db, renderer)
	homeHandler := handlers.NewHomeHandler(renderer)
	statsHandler := handlers.NewStatsHandler(db, renderer)
	applicationsHandler := handlers.NewApplicationsHandler(db, renderer)
	notesHandler := handlers.NewNotesHandler(db, renderer)
	cvHandler := handlers.NewCVHandler(db, renderer)
	settingsHandler := handlers.NewSettingsHandler(db, renderer)
	aiHandler := handlers.NewAIHandler(db, ai.NewChatClient, renderer)

	r.Group(func(r chi.Router) {
		r.Use(middleware.Timeout(15 * time.Second))

		cssServer := http.FileServer(http.Dir("static"))
		r.Handle("/static/*", http.StripPrefix("/static/", cssServer))

		r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, errWrite := w.Write([]byte("OK"))
			if errWrite != nil {
				slog.Error("Failed to write health check response", "error", errWrite)
			}
		})

		r.Get("/register", registerHandler.Page)
		r.Post("/register", registerHandler.RegisterAccount)

		r.Get("/", loginHandler.Page)
		r.Post("/login", loginHandler.Login)
	})

	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.Timeout(15 * time.Second))
		r.Get("/account/home", homeHandler.Page)
		r.Post("/logout", loginHandler.Logout)
		r.Get("/stats/bar", statsHandler.TopBarStats)

		r.Get("/applications/list", applicationsHandler.List)
		r.Get("/applications/filter", applicationsHandler.Filter)
		r.Post("/applications", applicationsHandler.Create)
		r.Delete("/applications/{id}", applicationsHandler.Delete)
		r.Get("/applications/{id}/edit", applicationsHandler.EditForm)
		r.Put("/applications/{id}", applicationsHandler.Update)
		r.Get("/applications/new", applicationsHandler.NewForm)
		r.Get("/applications/{id}/follow-up", applicationsHandler.FollowUpBadge)
		r.Get("/applications/{id}/follow-up/edit", applicationsHandler.FollowUpEditForm)
		r.Put("/applications/{id}/follow-up", applicationsHandler.UpdateFollowUp)

		r.Get("/account/notes", notesHandler.Page)
		r.Get("/notes/list", notesHandler.List)
		r.Post("/notes", notesHandler.Create)
		r.Delete("/notes/{id}", notesHandler.Delete)
		r.Get("/notes/{id}/edit", notesHandler.EditForm)
		r.Put("/notes/{id}", notesHandler.Update)
		r.Get("/notes/new", notesHandler.NewForm)

		// CV Routes
		r.Get("/account/cvs", cvHandler.Page)
		r.Get("/cvs/list", cvHandler.List)
		r.Get("/cvs/new", cvHandler.CreateCVForm)
		r.Post("/cvs", cvHandler.CreateCV)
		r.Get("/cvs/{id}/edit", cvHandler.RenderEditForm)
		r.Put("/cvs/{id}", cvHandler.UpdateCV)
		r.Post("/cvs/{id}/copy", cvHandler.CopyCV)
		r.Delete("/cvs/{id}", cvHandler.DeleteCV)
		r.Post("/cvs/snippets/{section}/add", cvHandler.AddSnippetBlock)
		r.Delete("/cvs/snippets/{section}/{index}", cvHandler.RemoveSnippetBlock)

		// AI Chat Routes
		r.Get("/account/ai", aiHandler.Page)
		r.Get("/ai/chat/clear", aiHandler.ClearThread)

		// Account Settings Routes
		r.Get("/account/settings", settingsHandler.Page)
		r.Put("/account/username", settingsHandler.UpdateUsername)
		r.Put("/account/email", settingsHandler.UpdateEmail)
		r.Put("/account/settings/ai", settingsHandler.UpdateAISettings)
		r.Delete("/account/settings/ai/key", settingsHandler.RemoveKey)
		r.Get("/account/settings/ai/config", settingsHandler.ConfigOptions)
		r.Get("/account/settings/ai/efforts", settingsHandler.EffortOptions)
	})

	// Dedicated group with bigger timeout, as it needs to wait for AI Response
	r.Group(func(r chi.Router) {
		r.Use(authMiddleware)
		r.Use(middleware.Timeout(3 * time.Minute))
		r.Post("/ai/chat", aiHandler.Chat)
	})
}
