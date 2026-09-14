package handlers

import (
	"context"
	"fmt"
	"job-applications/internal/db"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type MockApplicationsStore struct {
	ShouldFailGetAll                                bool
	ShouldFailGetByID                               bool
	ShouldFailCreate                                bool
	ShouldFailUpdate                                bool
	ShouldFailDelete                                bool
	ShouldFailIsOwner                               bool
	ShouldFailGetAllUserApplicationsByStatus        bool
	ShouldFailGetAllUserApplicationsByNameOrCompany bool
	ShouldFailGetAllUserApplicationsByAllFilters    bool
	IsOwnerResult                                   bool
}

func (m *MockApplicationsStore) GetAllUserApplications(_ context.Context, _ int32) ([]db.GetAllUserApplicationsRow, error) {
	if m.ShouldFailGetAll {
		return nil, fmt.Errorf("mock error: failed to get applications")
	}
	return []db.GetAllUserApplicationsRow{
		{ID: 1, CompanyName: "Acme", Role: "Engineer"},
	}, nil
}

func (m *MockApplicationsStore) GetApplicationByID(_ context.Context, _ int32) (db.GetApplicationByIDRow, error) {
	if m.ShouldFailGetByID {
		return db.GetApplicationByIDRow{}, fmt.Errorf("mock error: application not found")
	}
	return db.GetApplicationByIDRow{ID: 1, CompanyName: "Acme", Role: "Engineer"}, nil
}

func (m *MockApplicationsStore) CreateApplication(_ context.Context, _ db.CreateApplicationParams) (int32, error) {
	if m.ShouldFailCreate {
		return 0, fmt.Errorf("mock error: failed to create application")
	}
	return 1, nil
}

func (m *MockApplicationsStore) UpdateApplication(_ context.Context, _ db.UpdateApplicationParams) error {
	if m.ShouldFailUpdate {
		return fmt.Errorf("mock error: failed to update application")
	}
	return nil
}

func (m *MockApplicationsStore) UpdateFollowUpDate(_ context.Context, _ db.UpdateFollowUpDateParams) error {
	if m.ShouldFailUpdate {
		return fmt.Errorf("mock error: failed to update follow-up date")
	}
	return nil
}

func (m *MockApplicationsStore) DeleteApplication(_ context.Context, _ db.DeleteApplicationParams) error {
	if m.ShouldFailDelete {
		return fmt.Errorf("mock error: failed to delete application")
	}
	return nil
}

func (m *MockApplicationsStore) IsOwner(_ context.Context, _ db.IsOwnerParams) (bool, error) {
	if m.ShouldFailIsOwner {
		return false, fmt.Errorf("mock error: failed to check ownership")
	}
	return m.IsOwnerResult, nil
}

func (m *MockApplicationsStore) GetAllUserApplicationsByStatus(_ context.Context, _ db.GetAllUserApplicationsByStatusParams) ([]db.GetAllUserApplicationsByStatusRow, error) {
	if m.ShouldFailGetAllUserApplicationsByStatus {
		return nil, fmt.Errorf("mock error: failed to get all user applications by status")
	}
	return []db.GetAllUserApplicationsByStatusRow{
		{ID: 1, CompanyName: "Acme", Role: "Engineer"},
	}, nil
}

func (m *MockApplicationsStore) GetAllUserApplicationsByNameOrCompany(_ context.Context, _ db.GetAllUserApplicationsByNameOrCompanyParams) ([]db.GetAllUserApplicationsByNameOrCompanyRow, error) {
	if m.ShouldFailGetAllUserApplicationsByNameOrCompany {
		return nil, fmt.Errorf("mock error: failed to get all user applications by name or company")
	}
	return []db.GetAllUserApplicationsByNameOrCompanyRow{
		{ID: 1, CompanyName: "Acme", Role: "Engineer"},
	}, nil
}

func (m *MockApplicationsStore) GetAllUserApplicationsByAllFilters(_ context.Context, _ db.GetAllUserApplicationsByAllFiltersParams) ([]db.GetAllUserApplicationsByAllFiltersRow, error) {
	if m.ShouldFailGetAllUserApplicationsByAllFilters {
		return nil, fmt.Errorf("mock error: failed to get all user applications by name or company")
	}
	return []db.GetAllUserApplicationsByAllFiltersRow{
		{ID: 1, CompanyName: "Acme", Role: "Engineer"},
	}, nil
}

// --- createParams / updateParams unit tests ---

func TestCreateParams_OptionalFieldsEmpty(t *testing.T) {
	params := createParams(1, "Acme", "https://www.acme.com", "Engineer", "Remote", "", "", "", "2025-03-01", "", "", "")

	if params.Sector.Valid {
		t.Error("expected Sector.Valid to be false when empty")
	}
	if params.Location.Valid {
		t.Error("expected Location.Valid to be false when empty")
	}
	if params.Salary.Valid {
		t.Error("expected Salary.Valid to be false when empty")
	}
	if params.Link.Valid {
		t.Error("expected Link.Valid to be false when empty")
	}
	if params.ContactLinkedinProfile.Valid {
		t.Error("expected ContactLinkedinProfile.Valid to be false when empty")
	}
	if params.InterviewDate.Valid {
		t.Error("expected InterviewDate.Valid to be false when empty")
	}
}

func TestCreateParams_OptionalFieldsFilled(t *testing.T) {
	params := createParams(1, "Acme", "https://www.acme.com", "Engineer", "Remote", "Tech", "Berlin", "80k", "2025-03-01", "https://example.com", "john", "2025-04-01")

	if !params.Sector.Valid || params.Sector.String != "Tech" {
		t.Errorf("Sector = %+v, want Valid=true, String=Tech", params.Sector)
	}
	if !params.Location.Valid || params.Location.String != "Berlin" {
		t.Errorf("Location = %+v, want Valid=true, String=Berlin", params.Location)
	}
	if !params.Salary.Valid || params.Salary.String != "80k" {
		t.Errorf("Salary = %+v, want Valid=true, String=80k", params.Salary)
	}
	if !params.InterviewDate.Valid {
		t.Error("expected InterviewDate.Valid to be true when provided")
	}
}

func TestCreateParams_RequiredFields(t *testing.T) {
	params := createParams(42, "Google", "https://www.google.com", "SRE", "Hybrid", "", "", "", "2025-06-15", "", "", "")

	if params.UserID != 42 {
		t.Errorf("UserID = %d, want 42", params.UserID)
	}
	if params.CompanyName != "Google" {
		t.Errorf("CompanyName = %q, want Google", params.CompanyName)
	}
	if params.Status != db.TypApplicationStatusApplied {
		t.Errorf("Status = %q, want %q", params.Status, db.TypApplicationStatusApplied)
	}
}

func TestCreateParams_DateParsing(t *testing.T) {
	params := createParams(1, "Co", "https://www.co.com", "Dev", "Remote", "", "", "", "2025-03-15", "", "", "")

	if !params.Date.Valid {
		t.Fatal("expected Date.Valid to be true")
	}
	if params.Date.Time.Year() != 2025 || params.Date.Time.Month() != 3 || params.Date.Time.Day() != 15 {
		t.Errorf("Date = %v, want 2025-03-15", params.Date.Time)
	}
}

func TestUpdateParams_StatusMapping(t *testing.T) {
	params := updateParams(1, 1, "Co", "https://www.co.com", "Dev", "Remote", "", "", "", "2025-01-01", "", "", "Interview", "", "2024-12-28")

	if params.Status != db.TypApplicationStatusInterview {
		t.Errorf("Status = %q, want %q", params.Status, db.TypApplicationStatusInterview)
	}
}

func TestUpdateParams_OptionalFieldsEmpty(t *testing.T) {
	params := updateParams(1, 1, "Co", "https://www.co.com", "Dev", "Remote", "", "", "", "2025-01-01", "", "", "Applied", "", "")

	if params.Sector.Valid {
		t.Error("expected Sector.Valid to be false when empty")
	}
	if params.InterviewDate.Valid {
		t.Error("expected InterviewDate.Valid to be false when empty")
	}
}

// --- Handler HTTP tests ---

func TestApplicationsCreate_ValidationFailure(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	values := url.Values{
		"company":    {""},
		"role":       {"Engineer"},
		"work_model": {"Remote"},
		"date":       {"2025-01-01"},
	}
	req := formRequest(http.MethodPost, "/applications", values)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestApplicationsCreate_NoUserID(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	values := url.Values{
		"company":    {"Acme"},
		"role":       {"Dev"},
		"work_model": {"Remote"},
		"date":       {"2025-01-01"},
	}
	req := formRequest(http.MethodPost, "/applications", values)
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestApplicationsCreate_Success(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	values := url.Values{
		"company":    {"Acme"},
		"role":       {"Engineer"},
		"work_model": {"Remote"},
		"date":       {"2025-01-01"},
	}
	req := formRequest(http.MethodPost, "/applications", values)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Header().Get("HX-Trigger") != "stats-updated" {
		t.Errorf("HX-Trigger = %q, want stats-updated", w.Header().Get("HX-Trigger"))
	}
}

func TestApplicationsCreate_DBError(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{ShouldFailCreate: true},
		newMockRenderer(false),
	)

	values := url.Values{
		"company":    {"Acme"},
		"role":       {"Engineer"},
		"work_model": {"Remote"},
		"date":       {"2025-01-01"},
	}
	req := formRequest(http.MethodPost, "/applications", values)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestApplicationsDelete_InvalidID(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := requestWithURLParam(http.MethodDelete, "/applications/abc", "id", "abc")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestApplicationsDelete_NoUserID(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := requestWithURLParam(http.MethodDelete, "/applications/1", "id", "1")
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestApplicationsDelete_NotOwner(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{IsOwnerResult: false},
		newMockRenderer(false),
	)

	req := requestWithURLParam(http.MethodDelete, "/applications/1", "id", "1")
	req = withUserID(req, 99)
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestApplicationsDelete_IsOwnerDBError(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{ShouldFailIsOwner: true},
		newMockRenderer(false),
	)

	req := requestWithURLParam(http.MethodDelete, "/applications/1", "id", "1")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d (owner check errors deny access)", w.Code, http.StatusForbidden)
	}
}

func TestApplicationsDelete_Success(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{IsOwnerResult: true},
		newMockRenderer(false),
	)

	req := requestWithURLParam(http.MethodDelete, "/applications/5", "id", "5")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Header().Get("HX-Trigger") != "stats-updated" {
		t.Errorf("HX-Trigger = %q, want stats-updated", w.Header().Get("HX-Trigger"))
	}
}

func TestApplicationsDelete_DBError(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{IsOwnerResult: true, ShouldFailDelete: true},
		newMockRenderer(false),
	)

	req := requestWithURLParam(http.MethodDelete, "/applications/5", "id", "5")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Delete(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestApplicationsUpdate_InvalidID(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := requestWithURLParam(http.MethodPut, "/applications/xyz", "id", "xyz")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestApplicationsUpdate_NoUserID(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := requestWithURLParam(http.MethodPut, "/applications/1", "id", "1")
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestApplicationsUpdate_NotOwner(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{IsOwnerResult: false},
		newMockRenderer(false),
	)

	req := requestWithURLParam(http.MethodPut, "/applications/1", "id", "1")
	req = withUserID(req, 99)
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestApplicationsUpdate_Success(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{IsOwnerResult: true},
		newMockRenderer(false),
	)

	values := url.Values{
		"company":    {"Acme"},
		"role":       {"Dev"},
		"work_model": {"Remote"},
		"date":       {"2025-01-01"},
		"status":     {"Applied"},
	}
	req := formRequestWithURLParam(http.MethodPut, "/applications/1", values, "id", "1")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Header().Get("HX-Trigger") != "stats-updated" {
		t.Errorf("HX-Trigger = %q, want stats-updated", w.Header().Get("HX-Trigger"))
	}
}

func TestApplicationsUpdate_DBError(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{ShouldFailUpdate: true, IsOwnerResult: true},
		newMockRenderer(false),
	)

	values := url.Values{
		"company":    {"Acme"},
		"role":       {"Dev"},
		"work_model": {"Remote"},
		"date":       {"2025-01-01"},
		"status":     {"Applied"},
	}
	req := formRequestWithURLParam(http.MethodPut, "/applications/1", values, "id", "1")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Update(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestApplicationsEditForm_InvalidID(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := requestWithURLParam(http.MethodGet, "/applications/abc/edit", "id", "abc")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.EditForm(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestApplicationsEditForm_NoUserID(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := requestWithURLParam(http.MethodGet, "/applications/1/edit", "id", "1")
	w := httptest.NewRecorder()

	handler.EditForm(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestApplicationsEditForm_NotOwner(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{IsOwnerResult: false},
		newMockRenderer(false),
	)

	req := requestWithURLParam(http.MethodGet, "/applications/1/edit", "id", "1")
	req = withUserID(req, 99)
	w := httptest.NewRecorder()

	handler.EditForm(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestApplicationsEditForm_DBError(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{ShouldFailGetByID: true, IsOwnerResult: true},
		newMockRenderer(false),
	)

	req := requestWithURLParam(http.MethodGet, "/applications/1/edit", "id", "1")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.EditForm(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestApplicationsEditForm_RenderError(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{IsOwnerResult: true},
		newMockRenderer(true),
	)

	req := requestWithURLParam(http.MethodGet, "/applications/1/edit", "id", "1")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.EditForm(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestApplicationsEditForm_Success(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{IsOwnerResult: true},
		newMockRenderer(false),
	)

	req := requestWithURLParam(http.MethodGet, "/applications/1/edit", "id", "1")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.EditForm(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestApplicationsList_NoUserID(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := httptest.NewRequest(http.MethodGet, "/applications", nil)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestApplicationsList_RenderError(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(true))

	req := httptest.NewRequest(http.MethodGet, "/applications", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestApplicationsList_DBError(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{ShouldFailGetAll: true},
		newMockRenderer(false),
	)

	req := httptest.NewRequest(http.MethodGet, "/applications", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestApplicationsList_Success(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := httptest.NewRequest(http.MethodGet, "/applications", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.List(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestApplicationsFilter_NoUserID(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := httptest.NewRequest(http.MethodGet, "/applications/filter", nil)
	w := httptest.NewRecorder()

	handler.Filter(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestApplicationsFilter_NoFilters_ReturnsAll(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := httptest.NewRequest(http.MethodGet, "/applications/filter", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Filter(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestApplicationsFilter_StatusAll_ReturnsAll(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := httptest.NewRequest(http.MethodGet, "/applications/filter?status=all", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Filter(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestApplicationsFilter_StatusOnly(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := httptest.NewRequest(http.MethodGet, "/applications/filter?status=Applied", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Filter(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestApplicationsFilter_StatusAndName(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := httptest.NewRequest(http.MethodGet, "/applications/filter?status=Applied&nameOrCompany=Acme", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Filter(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestApplicationsFilter_NameOnly_FallsBackToAll(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := httptest.NewRequest(http.MethodGet, "/applications/filter?nameOrCompany=Acme", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Filter(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestApplicationsFilter_DBError_Status(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{ShouldFailGetAllUserApplicationsByStatus: true},
		newMockRenderer(false),
	)

	req := httptest.NewRequest(http.MethodGet, "/applications/filter?status=Applied", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Filter(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestApplicationsFilter_DBError_NameOrCompany(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{ShouldFailGetAllUserApplicationsByNameOrCompany: true},
		newMockRenderer(false),
	)

	req := httptest.NewRequest(http.MethodGet, "/applications/filter?status=all&nameOrCompany=Acme", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Filter(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestApplicationsFilter_RenderError(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{},
		newMockRenderer(true),
	)

	req := httptest.NewRequest(http.MethodGet, "/applications/filter?status=Applied", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Filter(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestApplicationsNewForm_Success(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := httptest.NewRequest(http.MethodGet, "/applications/new", nil)
	w := httptest.NewRecorder()

	handler.NewForm(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestApplicationsNewForm_RenderError(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(true))

	req := httptest.NewRequest(http.MethodGet, "/applications/new", nil)
	w := httptest.NewRecorder()

	handler.NewForm(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestFollowUpBadge_InvalidID(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := requestWithURLParam(http.MethodGet, "/applications/abc/follow-up", "id", "abc")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.FollowUpBadge(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestFollowUpBadge_NoUserID(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := requestWithURLParam(http.MethodGet, "/applications/1/follow-up", "id", "1")
	w := httptest.NewRecorder()

	handler.FollowUpBadge(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestFollowUpBadge_NotOwner(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{IsOwnerResult: false},
		newMockRenderer(false),
	)

	req := requestWithURLParam(http.MethodGet, "/applications/1/follow-up", "id", "1")
	req = withUserID(req, 99)
	w := httptest.NewRecorder()

	handler.FollowUpBadge(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestFollowUpBadge_DBError(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{ShouldFailGetByID: true, IsOwnerResult: true},
		newMockRenderer(false),
	)

	req := requestWithURLParam(http.MethodGet, "/applications/1/follow-up", "id", "1")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.FollowUpBadge(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestFollowUpBadge_RenderError(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{IsOwnerResult: true},
		newMockRenderer(true),
	)

	req := requestWithURLParam(http.MethodGet, "/applications/1/follow-up", "id", "1")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.FollowUpBadge(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestFollowUpBadge_Success(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{IsOwnerResult: true},
		newMockRenderer(false),
	)

	req := requestWithURLParam(http.MethodGet, "/applications/5/follow-up", "id", "5")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.FollowUpBadge(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestFollowUpEditForm_InvalidID(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := requestWithURLParam(http.MethodGet, "/applications/xyz/follow-up/edit", "id", "xyz")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.FollowUpEditForm(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestFollowUpEditForm_NoUserID(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := requestWithURLParam(http.MethodGet, "/applications/1/follow-up/edit", "id", "1")
	w := httptest.NewRecorder()

	handler.FollowUpEditForm(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestFollowUpEditForm_NotOwner(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{IsOwnerResult: false},
		newMockRenderer(false),
	)

	req := requestWithURLParam(http.MethodGet, "/applications/1/follow-up/edit", "id", "1")
	req = withUserID(req, 99)
	w := httptest.NewRecorder()

	handler.FollowUpEditForm(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestFollowUpEditForm_DBError(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{ShouldFailGetByID: true, IsOwnerResult: true},
		newMockRenderer(false),
	)

	req := requestWithURLParam(http.MethodGet, "/applications/1/follow-up/edit", "id", "1")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.FollowUpEditForm(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestFollowUpEditForm_RenderError(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{IsOwnerResult: true},
		newMockRenderer(true),
	)

	req := requestWithURLParam(http.MethodGet, "/applications/1/follow-up/edit", "id", "1")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.FollowUpEditForm(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestFollowUpEditForm_Success(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{IsOwnerResult: true},
		newMockRenderer(false),
	)

	req := requestWithURLParam(http.MethodGet, "/applications/3/follow-up/edit", "id", "3")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.FollowUpEditForm(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUpdateFollowUp_InvalidID(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := requestWithURLParam(http.MethodPut, "/applications/bad/follow-up", "id", "bad")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.UpdateFollowUp(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateFollowUp_NoUserID(t *testing.T) {
	handler := NewApplicationsHandler(&MockApplicationsStore{}, newMockRenderer(false))

	req := requestWithURLParam(http.MethodPut, "/applications/1/follow-up", "id", "1")
	w := httptest.NewRecorder()

	handler.UpdateFollowUp(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestUpdateFollowUp_NotOwner(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{IsOwnerResult: false},
		newMockRenderer(false),
	)

	values := url.Values{"follow_up_date": {"2025-05-01"}}
	req := formRequestWithURLParam(http.MethodPut, "/applications/1/follow-up", values, "id", "1")
	req = withUserID(req, 99)
	w := httptest.NewRecorder()

	handler.UpdateFollowUp(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestUpdateFollowUp_DBError(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{ShouldFailUpdate: true, IsOwnerResult: true},
		newMockRenderer(false),
	)

	values := url.Values{"follow_up_date": {"2025-05-01"}}
	req := formRequestWithURLParam(http.MethodPut, "/applications/1/follow-up", values, "id", "1")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.UpdateFollowUp(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestUpdateFollowUp_RenderError(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{IsOwnerResult: true},
		newMockRenderer(true),
	)

	values := url.Values{"follow_up_date": {"2025-05-01"}}
	req := formRequestWithURLParam(http.MethodPut, "/applications/1/follow-up", values, "id", "1")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.UpdateFollowUp(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestUpdateFollowUp_Success(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{IsOwnerResult: true},
		newMockRenderer(false),
	)

	values := url.Values{"follow_up_date": {"2025-05-01"}}
	req := formRequestWithURLParam(http.MethodPut, "/applications/2/follow-up", values, "id", "2")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.UpdateFollowUp(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestUpdateFollowUp_EmptyDate(t *testing.T) {
	handler := NewApplicationsHandler(
		&MockApplicationsStore{IsOwnerResult: true},
		newMockRenderer(false),
	)

	values := url.Values{"follow_up_date": {""}}
	req := formRequestWithURLParam(http.MethodPut, "/applications/2/follow-up", values, "id", "2")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.UpdateFollowUp(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestValidateApplication(t *testing.T) {
	tests := []struct {
		name      string
		company   string
		role      string
		workModel string
		date      string
		want      bool
	}{
		{"all fields present", "Acme", "Engineer", "Remote", "2025-01-01", true},
		{"missing company", "", "Engineer", "Remote", "2025-01-01", false},
		{"missing role", "Acme", "", "Remote", "2025-01-01", false},
		{"missing work model", "Acme", "Engineer", "", "2025-01-01", false},
		{"missing date", "Acme", "Engineer", "Remote", "", false},
		{"all empty", "", "", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateApplication(tt.company, tt.role, tt.workModel, tt.date)
			if got != tt.want {
				t.Errorf("validateApplication(%q, %q, %q, %q) = %v, want %v",
					tt.company, tt.role, tt.workModel, tt.date, got, tt.want)
			}
		})
	}
}
