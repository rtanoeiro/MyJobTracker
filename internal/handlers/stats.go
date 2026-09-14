package handlers

import (
	"context"
	"job-applications/internal/middlewares"
	"job-applications/internal/templates"
	"log/slog"
	"net/http"
)

type StatsStore interface {
	CountNumberApplications(ctx context.Context, userID int32) (int64, error)
	CountNumberInterviews(ctx context.Context, userID int32) (int64, error)
	CountNumberPending(ctx context.Context, userID int32) (int64, error)
}

type StatsHandler struct {
	Store    StatsStore
	Renderer templates.Renderer
}

func NewStatsHandler(store StatsStore, renderer templates.Renderer) *StatsHandler {
	return &StatsHandler{
		Store:    store,
		Renderer: renderer,
	}
}

type StatsData struct {
	TotalApplications int64
	TotalInterviews   int64
	TotalPending      int64
}

// GET /stats/bar - Returns stats bar with 4 stat cards
func (handler *StatsHandler) TopBarStats(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID from context")
		return
	}

	totalApplications, errTotalApplications := handler.Store.CountNumberApplications(request.Context(), userID)
	if errTotalApplications != nil {
		slog.Error("Failed to get total applications", "error", errTotalApplications)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get total applications")
		return
	}

	totalInterviews, errTotalInterviews := handler.Store.CountNumberInterviews(request.Context(), userID)
	if errTotalInterviews != nil {
		slog.Error("Failed to get total interviews", "error", errTotalInterviews)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get total interviews")
		return
	}

	totalPending, errTotalPending := handler.Store.CountNumberPending(request.Context(), userID)
	if errTotalPending != nil {
		slog.Error("Failed to get total pending", "error", errTotalPending)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get total pending")
		return
	}
	statsData := StatsData{
		TotalApplications: totalApplications,
		TotalInterviews:   totalInterviews,
		TotalPending:      totalPending,
	}

	if err := handler.Renderer.Render(writer, "stats-bar", statsData); err != nil {
		slog.Error("Failed to render stats bar", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render stats bar")
		return
	}
	slog.Info("Stats bar requested", "user_id", userID)
}
