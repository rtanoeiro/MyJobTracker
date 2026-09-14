package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

type MockStatsStore struct {
	ShouldFailCountApplications bool
	ShouldFailCountInterviews   bool
	ShouldFailCountPending      bool
	TotalApplications           int64
	TotalInterviews             int64
	TotalPending                int64
}

func (m *MockStatsStore) CountNumberApplications(_ context.Context, _ int32) (int64, error) {
	if m.ShouldFailCountApplications {
		return 0, fmt.Errorf("mock error: failed to count applications")
	}
	return m.TotalApplications, nil
}

func (m *MockStatsStore) CountNumberInterviews(_ context.Context, _ int32) (int64, error) {
	if m.ShouldFailCountInterviews {
		return 0, fmt.Errorf("mock error: failed to count interviews")
	}
	return m.TotalInterviews, nil
}

func (m *MockStatsStore) CountNumberPending(_ context.Context, _ int32) (int64, error) {
	if m.ShouldFailCountPending {
		return 0, fmt.Errorf("mock error: failed to count pending")
	}
	return m.TotalPending, nil
}

func TestTopBarStats_NoUserID(t *testing.T) {
	handler := NewStatsHandler(&MockStatsStore{}, newMockRenderer(false))

	req := httptest.NewRequest(http.MethodGet, "/stats/bar", nil)
	w := httptest.NewRecorder()

	handler.TopBarStats(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestTopBarStats_CountApplicationsError(t *testing.T) {
	handler := NewStatsHandler(
		&MockStatsStore{ShouldFailCountApplications: true},
		newMockRenderer(false),
	)

	req := httptest.NewRequest(http.MethodGet, "/stats/bar", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.TopBarStats(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestTopBarStats_CountInterviewsError(t *testing.T) {
	handler := NewStatsHandler(
		&MockStatsStore{ShouldFailCountInterviews: true},
		newMockRenderer(false),
	)

	req := httptest.NewRequest(http.MethodGet, "/stats/bar", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.TopBarStats(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestTopBarStats_CountPendingError(t *testing.T) {
	handler := NewStatsHandler(
		&MockStatsStore{ShouldFailCountPending: true},
		newMockRenderer(false),
	)

	req := httptest.NewRequest(http.MethodGet, "/stats/bar", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.TopBarStats(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestTopBarStats_RenderError(t *testing.T) {
	handler := NewStatsHandler(&MockStatsStore{}, newMockRenderer(true))

	req := httptest.NewRequest(http.MethodGet, "/stats/bar", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.TopBarStats(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestTopBarStats_Success(t *testing.T) {
	handler := NewStatsHandler(
		&MockStatsStore{
			TotalApplications: 10,
			TotalInterviews:   3,
			TotalPending:      5,
		},
		newMockRenderer(false),
	)

	req := httptest.NewRequest(http.MethodGet, "/stats/bar", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.TopBarStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}
