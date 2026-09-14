package handlers

import (
	"context"
	"job-applications/internal/db"
	"job-applications/internal/middlewares"
	"job-applications/internal/templates"
	"job-applications/internal/utils"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type ApplicationsStore interface {
	GetAllUserApplications(ctx context.Context, userID int32) ([]db.GetAllUserApplicationsRow, error)
	GetAllUserApplicationsByStatus(ctx context.Context, arg db.GetAllUserApplicationsByStatusParams) ([]db.GetAllUserApplicationsByStatusRow, error)
	GetAllUserApplicationsByNameOrCompany(ctx context.Context, arg db.GetAllUserApplicationsByNameOrCompanyParams) ([]db.GetAllUserApplicationsByNameOrCompanyRow, error)
	GetAllUserApplicationsByAllFilters(ctx context.Context, arg db.GetAllUserApplicationsByAllFiltersParams) ([]db.GetAllUserApplicationsByAllFiltersRow, error)
	GetApplicationByID(ctx context.Context, id int32) (db.GetApplicationByIDRow, error)
	CreateApplication(ctx context.Context, arg db.CreateApplicationParams) (int32, error)
	UpdateApplication(ctx context.Context, arg db.UpdateApplicationParams) error
	UpdateFollowUpDate(ctx context.Context, arg db.UpdateFollowUpDateParams) error
	DeleteApplication(ctx context.Context, arg db.DeleteApplicationParams) error
	IsOwner(ctx context.Context, arg db.IsOwnerParams) (bool, error)
}

type FollowUpData struct {
	ID                    int32
	LastFollowUpContactAt pgtype.Date
}

type ApplicationsHandler struct {
	Store    ApplicationsStore
	Renderer templates.Renderer
}

func NewApplicationsHandler(store ApplicationsStore, renderer templates.Renderer) *ApplicationsHandler {
	return &ApplicationsHandler{
		Store:    store,
		Renderer: renderer,
	}
}

// GET /applications - Returns table rows for all applications
func (handler *ApplicationsHandler) List(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID")
		return
	}

	applications, err := handler.Store.GetAllUserApplications(request.Context(), userID)
	if err != nil {
		slog.Error("Failed to get all user applications", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get all user applications")
		return
	}

	if err := handler.Renderer.Render(writer, "applications-list", applications); err != nil {
		slog.Error("Failed to render applications list", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render applications list")
		return
	}
	slog.Info("List applications", "user_id", userID)
}

// GET /applications/new - Returns modal HTML with empty form
func (handler *ApplicationsHandler) NewForm(writer http.ResponseWriter, request *http.Request) {
	today := time.Now().Format("2006-01-02")
	if err := handler.Renderer.Render(writer, "application-form", map[string]string{"Today": string(today)}); err != nil {
		slog.Error("Failed to render applications list", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render applications list")
		return
	}
	slog.Info("New application form requested")
}

// POST /applications - Creates new application
func (handler *ApplicationsHandler) Create(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID")
		return
	}

	company := request.FormValue("company")
	companyWebsite := request.FormValue("company_website")
	role := request.FormValue("role")
	workModel := request.FormValue("work_model")
	sector := request.FormValue("sector")
	location := request.FormValue("location")
	salary := request.FormValue("salary")
	date := request.FormValue("date")
	link := request.FormValue("link")
	contact := request.FormValue("contact")
	interviewDate := request.FormValue("interview_date")
	slog.Info("Creating application", "user_id", userID, "company", company, "role", role, "workModel", workModel, "sector", sector, "location", location, "salary", salary, "date", date, "link", link, "contact", contact, "interviewDate", interviewDate)

	isValid := validateApplication(company, role, workModel, date)
	if !isValid {
		slog.Error("Invalid application data")
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Invalid application data")
		return
	}

	params := createParams(userID, company, companyWebsite, role, workModel, sector, location, salary, date, link, contact, interviewDate)

	createApplication, errAdd := handler.Store.CreateApplication(request.Context(), params)
	if errAdd != nil {
		slog.Error("Failed to create application", "error", errAdd)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to create application")
		return
	}
	slog.Info("Application created", "application", createApplication)

	writer.Header().Set("HX-Trigger", "stats-updated")
	slog.Info("Create application", "user_id", userID)
}

// GET /applications/{id}/edit - Returns modal HTML with pre-filled form
func (handler *ApplicationsHandler) EditForm(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID")
		return
	}
	id := chi.URLParam(request, "id")
	idInt, errConvert := utils.ConvertFromStrToInt32(id)
	if errConvert != nil {
		slog.Error("Failed to convert application ID to int", "error", errConvert)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Failed to convert application ID to int")
		return
	}
	isOwner := isApplicationOwner(handler.Store, request.Context(), userID, idInt)
	if !isOwner {
		slog.Error("User attempting to render edit application from another user", "applcationID", idInt)
		templates.RenderResponseMessage(writer, http.StatusForbidden, "Failed to get application by ID")
		return
	}
	application, err := handler.Store.GetApplicationByID(request.Context(), idInt)
	if err != nil {
		slog.Error("Failed to get application by ID", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get application by ID")
		return
	}
	if err := handler.Renderer.Render(writer, "application-edit-form", application); err != nil {
		slog.Error("Failed to render application edit form", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render applications list")
		return
	}
}

// PUT /applications/{id} - Updates existing application
func (handler *ApplicationsHandler) Update(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user")
		return
	}

	company := request.FormValue("company")
	companyWebsite := request.FormValue("company_website")
	role := request.FormValue("role")
	workModel := request.FormValue("work_model")
	sector := request.FormValue("sector")
	location := request.FormValue("location")
	salary := request.FormValue("salary")
	date := request.FormValue("date")
	link := request.FormValue("link")
	contact := request.FormValue("contact")
	status := request.FormValue("status")
	interviewDate := request.FormValue("interview_date")
	lastFollowUpDate := request.FormValue("last_follow_up_contact_at")
	id := chi.URLParam(request, "id")
	idInt, errConvert := utils.ConvertFromStrToInt32(id)
	if errConvert != nil {
		slog.Error("Failed to convert application ID to int", "error", errConvert)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Failed to convert application ID to int")
		return
	}

	isOwner := isApplicationOwner(handler.Store, request.Context(), userID, idInt)
	if !isOwner {
		slog.Error("User attempting to render edit application from another user", "user_id", userID, "applcationID", idInt)
		templates.RenderResponseMessage(writer, http.StatusForbidden, "Failed to get application by ID")
		return
	}

	updateParams := updateParams(idInt, userID, company, companyWebsite, role, workModel, sector, location, salary, date, link, contact, status, interviewDate, lastFollowUpDate)
	errUpdate := handler.Store.UpdateApplication(request.Context(), updateParams)
	if errUpdate != nil {
		slog.Error("Failed to update application", "error", errUpdate)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to update application")
		return
	}
	slog.Info("Application updated", "application_id", idInt)
	writer.Header().Set("HX-Trigger", "stats-updated")
}

// DELETE /applications/{id} - Deletes application
func (handler *ApplicationsHandler) Delete(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user")
		return
	}
	id := chi.URLParam(request, "id")
	idInt, errConvert := utils.ConvertFromStrToInt32(id)
	if errConvert != nil {
		slog.Error("Failed to convert application ID to int", "error", idInt)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Failed to convert application ID to int")
		return
	}
	isOwner := isApplicationOwner(handler.Store, request.Context(), userID, idInt)
	if !isOwner {
		slog.Error("User attempting to delete application from another user", "applcationID", idInt)
		templates.RenderResponseMessage(writer, http.StatusForbidden, "Failed to get application by ID")
		return
	}

	errDelete := handler.Store.DeleteApplication(request.Context(), db.DeleteApplicationParams{ID: idInt, UserID: userID})
	if errDelete != nil {
		slog.Error("Failed to delete application", "error", errDelete)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to delete application")
		return
	}

	writer.Header().Set("HX-Trigger", "stats-updated")
}

func (handler *ApplicationsHandler) Filter(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID")
		return
	}

	nameOrCompany := request.URL.Query().Get("nameOrCompany")
	status := request.URL.Query().Get("status")

	slog.Info("Filter applications", "nameOrCompany", nameOrCompany, "status", status)
	hasStatus := status != "" && status != "all"
	hasSearch := nameOrCompany != ""

	var applications any
	var err error

	switch {
	case hasStatus && hasSearch:
		slog.Info("Filter applications by status and name or company", "status", status, "nameOrCompany", nameOrCompany)
		applications, err = handler.Store.GetAllUserApplicationsByAllFilters(
			request.Context(),
			db.GetAllUserApplicationsByAllFiltersParams{
				UserID:  userID,
				Status:  db.TypApplicationStatus(status),
				Column3: pgtype.Text{String: strings.ToLower(nameOrCompany), Valid: true},
			},
		)
	case hasStatus:
		slog.Info("Filter applications by status", "status", status)
		applications, err = handler.Store.GetAllUserApplicationsByStatus(
			request.Context(),
			db.GetAllUserApplicationsByStatusParams{UserID: userID, Status: db.TypApplicationStatus(status)},
		)
	case hasSearch:
		slog.Info("Filter applications by name or company", "nameOrCompany", nameOrCompany)
		applications, err = handler.Store.GetAllUserApplicationsByNameOrCompany(
			request.Context(),
			db.GetAllUserApplicationsByNameOrCompanyParams{UserID: userID, Column2: pgtype.Text{String: strings.ToLower(nameOrCompany), Valid: true}},
		)
	default:
		slog.Info("Filter applications by all", "status", status, "nameOrCompany", nameOrCompany)
		applications, err = handler.Store.GetAllUserApplications(request.Context(), userID)
	}

	if err != nil {
		slog.Error("Failed to filter applications", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to filter applications")
		return
	}

	slog.Info("Filter applications", "user_id", userID, "status", status, "nameOrCompany", nameOrCompany)
	if err := handler.Renderer.Render(writer, "applications-list", applications); err != nil {
		slog.Error("Failed to render applications list", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render applications list")
		return
	}
}

// GET /applications/{id}/follow-up - Returns the follow-up badge cell
func (handler *ApplicationsHandler) FollowUpBadge(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user")
		return
	}
	id := chi.URLParam(request, "id")
	idInt, errConvert := utils.ConvertFromStrToInt32(id)
	if errConvert != nil {
		slog.Error("Failed to convert application ID to int", "error", errConvert)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Failed to convert application ID to int")
		return
	}

	isOwner := isApplicationOwner(handler.Store, request.Context(), userID, idInt)
	if !isOwner {
		slog.Error("User attempting to get follow-up badges from another user", "applcationID", idInt)
		templates.RenderResponseMessage(writer, http.StatusForbidden, "Failed to get application by ID")
		return
	}

	application, err := handler.Store.GetApplicationByID(request.Context(), idInt)
	if err != nil {
		slog.Error("Failed to get application by ID", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get application by ID")
		return
	}
	data := FollowUpData{ID: idInt, LastFollowUpContactAt: application.LastFollowUpContactAt}
	if err := handler.Renderer.Render(writer, "follow-up-badge", data); err != nil {
		slog.Error("Failed to render follow-up badge", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render follow-up badge")
		return
	}
}

// GET /applications/{id}/follow-up/edit - Returns the inline follow-up date form cell
func (handler *ApplicationsHandler) FollowUpEditForm(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user")
		return
	}
	id := chi.URLParam(request, "id")
	idInt, errConvert := utils.ConvertFromStrToInt32(id)
	if errConvert != nil {
		slog.Error("Failed to convert application ID to int", "error", errConvert)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Failed to convert application ID to int")
		return
	}

	isOwner := isApplicationOwner(handler.Store, request.Context(), userID, idInt)
	if !isOwner {
		slog.Error("User attempting to edit follow up form another user", "applcationID", idInt)
		templates.RenderResponseMessage(writer, http.StatusForbidden, "Failed to get application by ID")
		return
	}

	application, err := handler.Store.GetApplicationByID(request.Context(), idInt)
	if err != nil {
		slog.Error("Failed to get application by ID", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get application by ID")
		return
	}
	data := FollowUpData{ID: idInt, LastFollowUpContactAt: application.LastFollowUpContactAt}
	if err := handler.Renderer.Render(writer, "follow-up-edit-form", data); err != nil {
		slog.Error("Failed to render follow-up edit form", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render follow-up edit form")
		return
	}
}

// PUT /applications/{id}/follow-up - Updates last_follow_up_contact_at, returns badge cell
func (handler *ApplicationsHandler) UpdateFollowUp(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user")
		return
	}
	id := chi.URLParam(request, "id")
	idInt, errConvert := utils.ConvertFromStrToInt32(id)
	if errConvert != nil {
		slog.Error("Failed to convert application ID to int", "error", errConvert)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Failed to convert application ID to int")
		return
	}

	isOwner := isApplicationOwner(handler.Store, request.Context(), userID, idInt)
	if !isOwner {
		slog.Error("User attempting to update follow up application from another user", "applcationID", idInt)
		templates.RenderResponseMessage(writer, http.StatusForbidden, "Failed to get application by ID")
		return
	}

	followUpDate := request.FormValue("follow_up_date")

	err := handler.Store.UpdateFollowUpDate(request.Context(), db.UpdateFollowUpDateParams{
		ID:                    idInt,
		LastFollowUpContactAt: utils.ConvertFromStrToPGTypeDate(followUpDate),
		UserID:                userID,
	})
	if err != nil {
		slog.Error("Failed to update follow-up date", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to update follow-up date")
		return
	}
	slog.Info("Follow-up date updated", "application_id", idInt, "date", followUpDate)

	data := FollowUpData{ID: idInt, LastFollowUpContactAt: utils.ConvertFromStrToPGTypeDate(followUpDate)}
	if err := handler.Renderer.Render(writer, "follow-up-badge", data); err != nil {
		slog.Error("Failed to render follow-up badge", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render follow-up badge")
		return
	}
}

func createParams(userID int32, company, companyWebsite, role, workModel, sector, location, salary, date, link, contact, interviewDate string) db.CreateApplicationParams {
	companyWebsiteAdd := utils.PGText(companyWebsite)
	sectorAdd := utils.PGText(sector)
	locationAdd := utils.PGText(location)
	salaryAdd := utils.PGText(salary)
	linkAdd := utils.PGText(link)
	contactAdd := utils.PGText(contact)

	return db.CreateApplicationParams{
		UserID:                 userID,
		CompanyName:            company,
		CompanyWebsite:         companyWebsiteAdd,
		Role:                   role,
		WorkModel:              workModel,
		Sector:                 sectorAdd,
		Location:               locationAdd,
		Salary:                 salaryAdd,
		Link:                   linkAdd,
		ContactLinkedinProfile: contactAdd,
		Date:                   utils.ConvertFromStrToPGTypeDate(date),
		Status:                 db.TypApplicationStatusApplied,
		InterviewDate:          utils.ConvertFromStrToPGTypeDate(interviewDate),
	}
}

func updateParams(ID, userID int32, company, companyWebsite, role, workModel, sector, location, salary, date, link, contact, status, interviewDate, lastFollowUpDate string) db.UpdateApplicationParams {
	companyWebsiteAdd := utils.PGText(companyWebsite)
	sectorAdd := utils.PGText(sector)
	locationAdd := utils.PGText(location)
	salaryAdd := utils.PGText(salary)
	linkAdd := utils.PGText(link)
	contactAdd := utils.PGText(contact)

	return db.UpdateApplicationParams{
		ID:                     ID,
		UserID:                 userID,
		CompanyName:            company,
		CompanyWebsite:         companyWebsiteAdd,
		Role:                   role,
		WorkModel:              workModel,
		Sector:                 sectorAdd,
		Location:               locationAdd,
		Salary:                 salaryAdd,
		Link:                   linkAdd,
		ContactLinkedinProfile: contactAdd,
		Date:                   utils.ConvertFromStrToPGTypeDate(date),
		Status:                 db.TypApplicationStatus(status),
		InterviewDate:          utils.ConvertFromStrToPGTypeDate(interviewDate),
		LastFollowUpContactAt:  utils.ConvertFromStrToPGTypeDate(lastFollowUpDate),
	}
}

func isApplicationOwner(handler ApplicationsStore, context context.Context, userId, applicationID int32) bool {
	isOwner, _ := handler.IsOwner(context, db.IsOwnerParams{
		UserID: userId,
		ID:     applicationID,
	})
	return isOwner
}

func validateApplication(company, role, workModel, date string) bool {
	if company == "" || role == "" || workModel == "" || date == "" {
		return false
	}
	return true
}
