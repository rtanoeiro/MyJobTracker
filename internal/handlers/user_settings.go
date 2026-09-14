package handlers

import (
	"context"
	"errors"
	"job-applications/internal/db"
	"job-applications/internal/middlewares"
	"job-applications/internal/templates"
	"job-applications/internal/utils"
	"log/slog"
	"net/http"
	"net/mail"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type SettingsStore interface {
	GetUserByID(ctx context.Context, id int32) (db.User, error)
	UpdateUsername(ctx context.Context, arg db.UpdateUsernameParams) error
	UpdateUserEmail(ctx context.Context, arg db.UpdateUserEmailParams) error
	GetUserAISettings(ctx context.Context, userID int32) (db.UserAiSetting, error)
	UpsertUserAISettings(ctx context.Context, arg db.UpsertUserAISettingsParams) error
	ListEnabledProviders(ctx context.Context) ([]db.ListEnabledProvidersRow, error)
	ListModelsByProviderName(ctx context.Context, name string) ([]string, error)
	ListEffortsByProviderAndModel(ctx context.Context, arg db.ListEffortsByProviderAndModelParams) ([]string, error)
}

type SettingsHandler struct {
	Store    SettingsStore
	Renderer templates.Renderer
}

type accountSettingsPageData struct {
	ActivePage   string
	Username     string
	Email        string
	Providers    []string
	ProviderName string
	Models       []string
	ModelName    string
	Efforts      []string
	EffortLevel  string
	HasKey       bool
	Enabled      bool
}

func NewSettingsHandler(store SettingsStore, renderer templates.Renderer) *SettingsHandler {
	return &SettingsHandler{
		Store:    store,
		Renderer: renderer,
	}
}

var providerEffortOrder = map[string][]string{
	"OpenAI":    {"none", "low", "medium", "high", "xhigh", "max"},
	"Anthropic": {"max", "xhigh", "high", "medium", "low"},
	"DeepSeek":  {"max", "high", "low"},
	"Google":    {"low"},
}

func orderedEfforts(provider string, efforts []string) []string {
	rank := make(map[string]int, len(efforts))
	for index, effort := range providerEffortOrder[provider] {
		rank[effort] = index
	}
	ordered := append([]string(nil), efforts...)
	sort.SliceStable(ordered, func(i, j int) bool {
		rankI, knownI := rank[ordered[i]]
		rankJ, knownJ := rank[ordered[j]]
		switch {
		case knownI && knownJ:
			return rankI < rankJ
		case knownI:
			return true
		case knownJ:
			return false
		default:
			return ordered[i] < ordered[j]
		}
	})
	return ordered
}

type aiCatalog struct {
	providers []string
	models    map[string][]string
	efforts   map[string][]string
}

func buildAICatalog(rows []db.ListEnabledProvidersRow) aiCatalog {
	catalog := aiCatalog{
		models:  make(map[string][]string),
		efforts: make(map[string][]string),
	}
	lastModel := make(map[string]string)
	lastEffort := make(map[string]string)
	for _, provider := range rows {
		if len(catalog.providers) == 0 || catalog.providers[len(catalog.providers)-1] != provider.Name {
			catalog.providers = append(catalog.providers, provider.Name)
		}
		if lastModel[provider.Name] != provider.Model {
			catalog.models[provider.Name] = append(catalog.models[provider.Name], provider.Model)
			lastModel[provider.Name] = provider.Model
		}
		offeringKey := offeringKey(provider.Name, provider.Model)
		if lastEffort[offeringKey] != provider.EffortLevel {
			catalog.efforts[offeringKey] = append(catalog.efforts[offeringKey], provider.EffortLevel)
			lastEffort[offeringKey] = provider.EffortLevel
		}
	}
	for _, provider := range catalog.providers {
		for _, model := range catalog.models[provider] {
			catalog.efforts[offeringKey(provider, model)] = orderedEfforts(provider, catalog.efforts[offeringKey(provider, model)])
		}
	}
	return catalog
}

func offeringKey(provider, model string) string {
	return provider + "\x00" + model
}

// GET /account/settings
func (handler *SettingsHandler) Page(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID")
		return
	}
	slog.Info("User accessed Account Settings Endpoint", "user_id", userID)

	user, errUser := handler.Store.GetUserByID(request.Context(), userID)
	if errUser != nil {
		slog.Error("Failed to get user", "error", errUser, "user_id", userID)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to load account settings")
		return
	}

	data := accountSettingsPageData{
		ActivePage: "account-settings",
		Username:   user.Username,
		Email:      user.Email,
	}

	providers, errProviders := handler.Store.ListEnabledProviders(request.Context())
	if errProviders != nil {
		slog.Error("Failed to list enabled AI providers", "error", errProviders, "user_id", userID)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to load AI settings")
		return
	}
	catalog := buildAICatalog(providers)
	data.Providers = catalog.providers

	settings, errSettings := handler.Store.GetUserAISettings(request.Context(), userID)
	if errSettings != nil && !errors.Is(errSettings, pgx.ErrNoRows) {
		slog.Error("Failed to get user AI settings", "error", errSettings, "user_id", userID)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to load AI settings")
		return
	}
	if errSettings == nil {
		data.ProviderName = settings.ProviderName
		data.ModelName = settings.ModelName
		data.EffortLevel = settings.EffortLevel
		data.Enabled = settings.Enabled
		data.HasKey = len(settings.Key.String) > 0
	}
	data.Models = catalog.models[data.ProviderName]
	data.Efforts = catalog.efforts[offeringKey(data.ProviderName, data.ModelName)]

	if err := handler.Renderer.Render(writer, "account-settings", data); err != nil {
		slog.Error("Failed to render account settings page", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render account settings")
		return
	}
}

// GET /account/settings/ai/config?provider_name=<name>
func (handler *SettingsHandler) ConfigOptions(writer http.ResponseWriter, request *http.Request) {
	providerName := strings.TrimSpace(request.FormValue("provider_name"))

	data := accountSettingsPageData{ActivePage: "account-settings", ProviderName: providerName}
	if providerName != "" {
		models, errModels := handler.Store.ListModelsByProviderName(request.Context(), providerName)
		if errModels != nil {
			slog.Error("Failed to list provider models", "error", errModels, "provider", providerName)
			templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to load models")
			return
		}
		data.Models = models
	}

	if err := handler.Renderer.Render(writer, "ai-config-fields", data); err != nil {
		slog.Error("Failed to render model and effort fields", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render models")
		return
	}
}

// GET /account/settings/ai/efforts?provider_name=<name>&model_name=<model>
func (handler *SettingsHandler) EffortOptions(writer http.ResponseWriter, request *http.Request) {
	providerName := strings.TrimSpace(request.FormValue("provider_name"))
	modelName := strings.TrimSpace(request.FormValue("model_name"))
	if providerName == "" || modelName == "" {
		slog.Error("Failed to list efforts", "error", "provider and model are required")
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Choose a provider and model first")
		return
	}

	efforts, errEfforts := handler.Store.ListEffortsByProviderAndModel(request.Context(), db.ListEffortsByProviderAndModelParams{
		Name:  providerName,
		Model: modelName,
	})
	if errEfforts != nil {
		slog.Error("Failed to list provider efforts", "error", errEfforts, "provider", providerName, "model", modelName)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to load efforts")
		return
	}

	data := accountSettingsPageData{
		ProviderName: providerName,
		ModelName:    modelName,
		Efforts:      orderedEfforts(providerName, efforts),
	}
	if err := handler.Renderer.Render(writer, "ai-effort-options", data); err != nil {
		slog.Error("Failed to render effort options", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render efforts")
		return
	}
}

// PUT /account/username
func (handler *SettingsHandler) UpdateUsername(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID")
		return
	}

	username := strings.TrimSpace(request.FormValue("username"))
	if username == "" {
		slog.Error("Failed to update username", "error", "username must not be empty", "user_id", userID)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Username cannot be empty")
		return
	}
	slog.Info("User updating username", "user_id", userID)

	errUpdate := handler.Store.UpdateUsername(request.Context(), db.UpdateUsernameParams{
		ID:       userID,
		Username: username,
	})
	if errUpdate != nil {
		if isUniqueViolation(errUpdate) {
			slog.Error("Failed to update username", "error", "username already taken", "user_id", userID)
			templates.RenderResponseMessage(writer, http.StatusConflict, "That username is already taken")
			return
		}
		slog.Error("Failed to update username", "error", errUpdate, "user_id", userID)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to update username")
		return
	}

	slog.Info("Username updated", "user_id", userID)
	templates.RenderResponseMessage(writer, http.StatusOK, "Username updated")
}

// PUT /account/email
func (handler *SettingsHandler) UpdateEmail(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID")
		return
	}

	email := strings.TrimSpace(request.FormValue("email"))
	if !isValidEmail(email) {
		slog.Error("Failed to update email", "error", "email must be valid", "user_id", userID)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Please enter a valid email address")
		return
	}
	slog.Info("User updating email", "user_id", userID)

	errUpdate := handler.Store.UpdateUserEmail(request.Context(), db.UpdateUserEmailParams{
		ID:    userID,
		Email: email,
	})
	if errUpdate != nil {
		if isUniqueViolation(errUpdate) {
			slog.Error("Failed to update email", "error", "email already in use", "user_id", userID)
			templates.RenderResponseMessage(writer, http.StatusConflict, "That email is already in use")
			return
		}
		slog.Error("Failed to update email", "error", errUpdate, "user_id", userID)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to update email")
		return
	}

	slog.Info("Email updated", "user_id", userID)
	templates.RenderResponseMessage(writer, http.StatusOK, "Email updated")
}

// UpdateAISettings persists the provider, model, effort level, optional key and
// enabled flag. The stored key is only kept when neither the provider nor the
// model changed; changing either clears it unless a replacement key was
// submitted. Changing only the effort level keeps the key.
//
// PUT /account/settings/ai
func (handler *SettingsHandler) UpdateAISettings(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID")
		return
	}

	providerName := strings.TrimSpace(request.FormValue("provider_name"))
	modelName := strings.TrimSpace(request.FormValue("model_name"))
	effortLevel := strings.TrimSpace(request.FormValue("effort_level"))
	enabled := request.FormValue("enabled") == "true"
	newKey := request.FormValue("encrypted_key")

	if providerName == "" || modelName == "" || effortLevel == "" {
		slog.Error("Failed to save AI settings", "error", "provider, model and effort are required", "user_id", userID)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Choose a provider, a model and an effort level before saving")
		return
	}
	slog.Info("User saving AI settings", "user_id", userID, "provider", providerName, "model", modelName, "effort", effortLevel, "enabled", enabled)

	current, errCurrent := handler.Store.GetUserAISettings(request.Context(), userID)
	hasCurrent := errCurrent == nil
	if errCurrent != nil && !errors.Is(errCurrent, pgx.ErrNoRows) {
		slog.Error("Failed to get user AI settings", "error", errCurrent, "user_id", userID)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to save AI settings")
		return
	}

	var key string
	switch {
	case newKey != "":
		// New key submitted: store it
		key = newKey
	case hasCurrent && current.ProviderName == providerName && current.ModelName == modelName:
		// Provider/model unchanged: keep the previously saved key.
		key = current.Key.String
	default:
		// Provider or model changed: the old key may belong to another provider, clear it.
		key = ""
	}

	if enabled && len(key) == 0 {
		slog.Error("Failed to save AI settings", "error", "a key is required to enable AI matching", "user_id", userID)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "An API key is required to enable AI matching")
		return
	}

	errUpsert := handler.Store.UpsertUserAISettings(request.Context(), db.UpsertUserAISettingsParams{
		UserID:       userID,
		ProviderName: providerName,
		ModelName:    modelName,
		EffortLevel:  effortLevel,
		Key:          utils.PGText(key),
		Enabled:      enabled,
	})
	if errUpsert != nil {
		if isForeignKeyViolation(errUpsert) {
			slog.Error("Failed to save AI settings", "error", "provider/model/effort combination not available", "user_id", userID)
			templates.RenderResponseMessage(writer, http.StatusBadRequest, "The selected provider, model or effort level is not available")
			return
		}
		slog.Error("Failed to save AI settings", "error", errUpsert, "user_id", userID)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to save AI settings")
		return
	}

	slog.Info("AI settings saved", "user_id", userID, "enabled", enabled, "has_key", key != "")
	templates.RenderResponseMessage(writer, http.StatusOK, "AI settings saved")
}

// DELETE /account/settings/ai/key
func (handler *SettingsHandler) RemoveKey(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID")
		return
	}
	slog.Info("User removing API key", "user_id", userID)

	current, errCurrent := handler.Store.GetUserAISettings(request.Context(), userID)
	if errCurrent != nil {
		if errors.Is(errCurrent, pgx.ErrNoRows) {
			templates.RenderResponseMessage(writer, http.StatusOK, "No API key saved to remove")
			return
		}
		slog.Error("Failed to get user AI settings", "error", errCurrent, "user_id", userID)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to remove API key")
		return
	}

	errUpsert := handler.Store.UpsertUserAISettings(request.Context(), db.UpsertUserAISettingsParams{
		UserID:       userID,
		ProviderName: current.ProviderName,
		ModelName:    current.ModelName,
		EffortLevel:  current.EffortLevel,
		Key:          utils.PGText(""),
		Enabled:      false,
	})
	if errUpsert != nil {
		slog.Error("Failed to remove API key", "error", errUpsert, "user_id", userID)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to remove API key")
		return
	}

	slog.Info("API key removed and AI matching disabled", "user_id", userID)
	writer.Header().Set("HX-Refresh", "true")
	templates.RenderResponseMessage(writer, http.StatusOK, "API key removed and AI matching disabled")
}

func isValidEmail(email string) bool {
	parsed, err := mail.ParseAddress(email)
	return err == nil && parsed.Address == email
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
