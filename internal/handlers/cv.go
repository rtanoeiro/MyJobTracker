package handlers

import (
	"context"
	"errors"
	"fmt"
	"job-applications/internal/db"
	"job-applications/internal/middlewares"
	"job-applications/internal/templates"
	"job-applications/internal/utils"
	"log/slog"
	"net/http"
	"slices"

	"github.com/go-chi/chi/v5"
)

type CVStore interface {
	CreateCV(ctx context.Context, arg db.CreateCVParams) (int32, error)
	DeleteCV(ctx context.Context, arg db.DeleteCVParams) error
	GetAllUserCVs(ctx context.Context, userID int32) ([]db.GetAllUserCVsRow, error)
	GetCVByID(ctx context.Context, arg db.GetCVByIDParams) (db.GetCVByIDRow, error)
	IsCVOwner(ctx context.Context, arg db.IsCVOwnerParams) (bool, error)
	UpdateCV(ctx context.Context, arg db.UpdateCVParams) error
}

type CVHandler struct {
	Store    CVStore
	Renderer templates.Renderer
}

func NewCVHandler(store CVStore, renderer templates.Renderer) *CVHandler {
	return &CVHandler{
		Store:    store,
		Renderer: renderer,
	}
}

func (handler *CVHandler) Page(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID")
		return
	}
	slog.Info("User accessed Main CV Endpoint", "user_id", userID)

	if err := handler.Renderer.Render(writer, "cvs", nil); err != nil {
		slog.Error("Failed to render applications list", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render applications list")
		return
	}
}

func (handler *CVHandler) CreateCVForm(writer http.ResponseWriter, request *http.Request) {
	if err := handler.Renderer.Render(writer, "cv-form", nil); err != nil {
		slog.Error("Failed to render applications list", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render applications list")
		return
	}
}

func (handler *CVHandler) CreateCV(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID")
		return
	}
	slog.Info("User accessed Create CV Endpoint", "user_id", userID)

	params, errParams := buildCreateCVParams(request, userID)
	if errParams != nil {
		slog.Error("Failed to process CV form", "error", errParams)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to process CV form")
		return
	}

	if _, errCreate := handler.Store.CreateCV(request.Context(), params); errCreate != nil {
		slog.Error("Failed to create CV", "error", errCreate, "user_id", userID)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to create CV")
		return
	}
	slog.Info("CV created", "user_id", userID)

	writer.Header().Set("HX-Redirect", "/account/cvs")
}

// POST /cvs/{id}/copy
func (handler *CVHandler) CopyCV(writer http.ResponseWriter, request *http.Request) {
	cvID, userID, isOwner := getCVAndOwnership(request, handler)
	if cvID == 0 {
		slog.Error("Failed to get user CV", "cv_id", cvID)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Failed to get user CV")
		return
	}
	if !isOwner {
		slog.Error("User trying to copy CV from another user", "cv_id", cvID)
		templates.RenderResponseMessage(writer, http.StatusForbidden, "Unable to render CV")
		return
	}

	cvData, errCV := handler.Store.GetCVByID(request.Context(), db.GetCVByIDParams{ID: cvID, UserID: userID})
	if errCV != nil {
		slog.Error("Failed to get CV Data", "CV_ID", cvID)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get CV Data")
		return
	}

	createParams := db.CreateCVParams{
		UserID:                cvData.UserID,
		Title:                 fmt.Sprintf("Copy of %s", cvData.Title),
		Summary:               cvData.Summary,
		Profile:               cvData.Profile,
		Experiences:           cvData.Experiences,
		Education:             cvData.Education,
		Certifications:        cvData.Certifications,
		AcademicContributions: cvData.AcademicContributions,
		Skills:                cvData.Skills,
	}
	_, errCreate := handler.Store.CreateCV(request.Context(), createParams)
	if errCreate != nil {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID")
		return

	}
	writer.Header().Set("HX-Redirect", "/account/cvs")
}

func getCVAndOwnership(request *http.Request, handler *CVHandler) (int32, int32, bool) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		return 0, 0, false
	}
	cvID, errConvert := utils.ConvertFromStrToInt32(chi.URLParam(request, "id"))
	if errConvert != nil {
		return 0, 0, false
	}

	isOwner := handler.isCVOwner(request.Context(), cvID, userID)
	return cvID, userID, isOwner
}

func (handler *CVHandler) List(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID")
		return
	}

	slog.Info("User accessed List Endpoint", "user_id", userID)
	userCVs, errCVs := handler.Store.GetAllUserCVs(request.Context(), userID)
	if errCVs != nil {
		slog.Error("Failed to get user CVs", "user_id", userID, "error", errCVs)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Error getting user CVs")
		return
	}

	if err := handler.Renderer.Render(writer, "cvs-list", userCVs); err != nil {
		slog.Error("Failed to render applications list", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render applications list")
		return
	}
}

func (handler *CVHandler) RenderEditForm(writer http.ResponseWriter, request *http.Request) {
	cvID, userID, isOwner := getCVAndOwnership(request, handler)
	if cvID == 0 {
		slog.Error("Failed to get user CV", "cv_id", cvID)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Failed to get user CV")
		return
	}
	if !isOwner {
		slog.Error("User trying to edit CV from another user", "cv_id", cvID)
		templates.RenderResponseMessage(writer, http.StatusForbidden, "Unable to render CV")
		return
	}

	currentCV, errCV := handler.Store.GetCVByID(request.Context(), db.GetCVByIDParams{ID: cvID, UserID: userID})
	if errCV != nil {
		slog.Error("Failed to get user CV", "error", errCV, "cv_id", cvID)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Failed to get user CV")
		return
	}

	data, errDecode := buildCVEditFormData(currentCV)
	if errDecode != nil {
		slog.Error("Failed to decode CV for edit form", "error", errDecode, "cv_id", cvID)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to load CV Edit Form")
		return
	}

	if err := handler.Renderer.Render(writer, "cv-edit-form", data); err != nil {
		slog.Error("Failed to render cv edit form", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render CV Edit Form")
		return
	}
}

func (handler *CVHandler) UpdateCV(writer http.ResponseWriter, request *http.Request) {
	cvID, userID, isOwner := getCVAndOwnership(request, handler)
	if cvID == 0 {
		slog.Error("Failed to get user CV", "cv_id", cvID)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Failed to get user CV")
		return
	}
	if !isOwner {
		slog.Error("User trying to update CV from another user", "cv_id", cvID)
		templates.RenderResponseMessage(writer, http.StatusForbidden, "Unable to render CV")
		return
	}
	// Updating a CV is just like creating a new CV, but the argument passed down to the store is different
	params, errParams := buildCreateCVParams(request, userID)
	if errParams != nil {
		slog.Error("Failed to process CV form", "error", errParams)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to process CV form")
		return
	}
	updateParams := convertCreateFormToUpdateForm(params, cvID, userID)
	if errUpdate := handler.Store.UpdateCV(request.Context(), updateParams); errUpdate != nil {
		slog.Error("Failed to Update CV", "error", errUpdate, "user_id", userID)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to update CV")
		return
	}
	slog.Info("CV Updated", "user_id", userID)
	writer.Header().Set("HX-Redirect", "/account/cvs")
}

func (handler *CVHandler) DeleteCV(writer http.ResponseWriter, request *http.Request) {
	cvID, userID, isOwner := getCVAndOwnership(request, handler)
	if cvID == 0 {
		slog.Error("Failed to get user CV", "cv_id", cvID, "userID", userID)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Failed to get user CV")
		return
	}
	if !isOwner {
		slog.Error("User trying to delete CV from another user", "cv_id", cvID)
		templates.RenderResponseMessage(writer, http.StatusForbidden, "Unable to render CV")
		return
	}
	slog.Info("User accessed Delete CV Endpoint", "user_id", userID)

	if errDelete := handler.Store.DeleteCV(request.Context(), db.DeleteCVParams{ID: cvID, UserID: userID}); errDelete != nil {
		slog.Error("Failed to Delete CV", "error", errDelete, "user_id", userID)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to create CV")
		return
	}
	slog.Info("CV Updated", "user_id", userID)
	writer.Header().Set("HX-Redirect", "/account/cvs")
}

var ALLOWED_SECTIONS = []string{"experiences"}

// POST /cvs/snippets/{section}/add
func (handler *CVHandler) AddSnippetBlock(writer http.ResponseWriter, request *http.Request) {
	section := chi.URLParam(request, "section")
	nextIndex, errIndex := utils.ConvertFromStrToInt32(request.FormValue("next_index"))
	if errIndex != nil || nextIndex < 0 {
		slog.Error("Invalid next_index", "next_index", request.FormValue("next_index"))
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Invalid block index")
		return
	}
	slog.Info("trying to add a new snippet block", "section", section, "next_index", nextIndex)

	if errSection := validateSections(section); errSection != nil {
		slog.Error("Invalid section", "section", section)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Invalid section to Add")
		return
	}

	switch section {
	case "experiences":
		handler.handleAddBlockAndButton(writer, "experiences", int(nextIndex))
	default:
		slog.Error("Unhandled section", "section", section)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Invalid Section to Add")
	}
}

// RemoveSnippetBlock removes a single already-rendered block from the DOM. The
// form has not been submitted yet, so there is nothing to persist: we only need
// a response whose (empty) body lets the block's hx-swap="outerHTML" delete it.
//
// DELETE /cvs/snippets/{section}/{index}
func (handler *CVHandler) RemoveSnippetBlock(writer http.ResponseWriter, request *http.Request) {
	index := chi.URLParam(request, "index")
	indexInt, errIndex := utils.ConvertFromStrToInt32(index)
	if errIndex != nil {
		slog.Error("Invalid Item to Remove", "section", errIndex)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Invalid Item to Remove")
		return
	}
	if indexInt < 0 {
		slog.Error("Invalid Current Index", "currIndex", request.FormValue("index"))
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Invalid block index")
		return
	}
	writer.WriteHeader(http.StatusOK)
}

// Adding a block means adding the block to fill it in, and the + Add section button
func (handler *CVHandler) handleAddBlockAndButton(writer http.ResponseWriter, section string, nextIndex int) {
	blockName := fmt.Sprintf("cv-%s-block", section)
	addButtonBlock := fmt.Sprintf("cv-add-%s-button", section)

	var block any
	switch section {
	case "experiences":
		block = cvExperience{
			Index:   nextIndex,
			Months:  cvMonths(),
			Years:   cvYears(),
			Bullets: make([]string, cvExperienceBulletsLimit),
		}
	default:
		slog.Error("Unhandled section", "section", section)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Invalid Section to Add")
		return
	}

	if err := handler.Renderer.Render(writer, blockName, block); err != nil {
		slog.Error("Failed to render block", "error", err, "section", section)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render block")
		return
	}
	if err := handler.Renderer.Render(writer, addButtonBlock, nextIndex+1); err != nil {
		slog.Error("Failed to render add button", "error", err, "section", section)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render add button")
		return
	}
}

func validateSections(section string) error {
	if !slices.Contains(ALLOWED_SECTIONS, section) {
		return errors.New("invalid section")
	}
	return nil
}

func (handler *CVHandler) isCVOwner(context context.Context, intCVID int32, userID int32) bool {
	isCVOwner, errIsOwner := handler.Store.IsCVOwner(context, db.IsCVOwnerParams{ID: intCVID, UserID: userID})
	if errIsOwner != nil {
		slog.Error("Failed to get CV Owner Identity")
		return false
	}
	if !isCVOwner {
		slog.Error("User tried to CV from another person", "user_id", userID, "cvID", intCVID)
		return false
	}
	return true
}
