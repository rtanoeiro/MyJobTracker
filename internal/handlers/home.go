package handlers

import (
	"job-applications/internal/templates"
	"log/slog"
	"net/http"
)

type HomeHandler struct {
	Renderer templates.Renderer
}

func NewHomeHandler(renderer templates.Renderer) *HomeHandler {
	return &HomeHandler{
		Renderer: renderer,
	}
}

func (handler *HomeHandler) Page(writer http.ResponseWriter, request *http.Request) {
	data := map[string]any{
		"ActivePage": "applications",
	}
	if err := handler.Renderer.Render(writer, "home", data); err != nil {
		slog.Error("Failed to render login page", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render login page")
		return
	}
}
