package handlers

import (
	"context"
	"io"
	"job-applications/internal/db"
	"job-applications/internal/utils"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const settingsTestUserID = int32(7)

type MockSettingsStore struct {
	User              db.User
	ErrGetUser        error
	EnabledProviders  []db.ListEnabledProvidersRow
	ErrListProviders  error
	ModelsByName      map[string][]string
	ErrListModels     error
	EffortsByOffering map[string][]string
	ErrListEfforts    error
	AISettings        db.UserAiSetting
	ErrGetAISettings  error
	ErrUpsert         error
	ErrUpdateUsername error
	ErrUpdateEmail    error

	UpsertCalled bool
	LastUpsert   db.UpsertUserAISettingsParams
	LastUsername db.UpdateUsernameParams
	LastEmail    db.UpdateUserEmailParams
}

func (m *MockSettingsStore) GetUserByID(_ context.Context, _ int32) (db.User, error) {
	return m.User, m.ErrGetUser
}

func (m *MockSettingsStore) UpdateUsername(_ context.Context, arg db.UpdateUsernameParams) error {
	m.LastUsername = arg
	return m.ErrUpdateUsername
}

func (m *MockSettingsStore) UpdateUserEmail(_ context.Context, arg db.UpdateUserEmailParams) error {
	m.LastEmail = arg
	return m.ErrUpdateEmail
}

func (m *MockSettingsStore) GetUserAISettings(_ context.Context, _ int32) (db.UserAiSetting, error) {
	return m.AISettings, m.ErrGetAISettings
}

func (m *MockSettingsStore) UpsertUserAISettings(_ context.Context, arg db.UpsertUserAISettingsParams) error {
	m.UpsertCalled = true
	m.LastUpsert = arg
	return m.ErrUpsert
}

func (m *MockSettingsStore) ListEnabledProviders(_ context.Context) ([]db.ListEnabledProvidersRow, error) {
	return m.EnabledProviders, m.ErrListProviders
}

func (m *MockSettingsStore) ListModelsByProviderName(_ context.Context, name string) ([]string, error) {
	if m.ErrListModels != nil {
		return nil, m.ErrListModels
	}
	return m.ModelsByName[name], nil
}

func (m *MockSettingsStore) ListEffortsByProviderAndModel(_ context.Context, arg db.ListEffortsByProviderAndModelParams) ([]string, error) {
	if m.ErrListEfforts != nil {
		return nil, m.ErrListEfforts
	}
	return m.EffortsByOffering[offeringKey(arg.Name, arg.Model)], nil
}

type spyRenderer struct {
	name string
	data any
	err  error
}

func (s *spyRenderer) Render(_ io.Writer, name string, data any) error {
	s.name = name
	s.data = data
	return s.err
}

func settingsFormRequest(method, path string, values url.Values) *http.Request {
	return withUserID(formRequest(method, path, values), settingsTestUserID)
}

func settingsGetRequest(path string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	return withUserID(req, settingsTestUserID)
}

func uniqueViolation() error {
	return &pgconn.PgError{Code: "23505"}
}

func foreignKeyViolation() error {
	return &pgconn.PgError{Code: "23503"}
}

func openAICatalog() []db.ListEnabledProvidersRow {
	return []db.ListEnabledProvidersRow{
		{Name: "OpenAI", Model: "gpt-5.6-sol", EffortLevel: "low"},
		{Name: "OpenAI", Model: "gpt-5.6-sol", EffortLevel: "high"},
		{Name: "OpenAI", Model: "gpt-6-astra", EffortLevel: "none"},
		{Name: "OpenAI", Model: "gpt-6-astra", EffortLevel: "medium"},
		{Name: "OpenAI", Model: "gpt-6-astra", EffortLevel: "max"},
		{Name: "Anthropic", Model: "claude-opus-5", EffortLevel: "low"},
		{Name: "Anthropic", Model: "claude-opus-5", EffortLevel: "max"},
	}
}

func TestSettingsHandlerImplementsStore(t *testing.T) {
	store := &MockSettingsStore{}
	var _ SettingsStore = store
}

func TestPage_MissingUserID(t *testing.T) {
	handler := NewSettingsHandler(&MockSettingsStore{}, &spyRenderer{})

	req := httptest.NewRequest(http.MethodGet, "/account/settings", nil)
	w := httptest.NewRecorder()

	handler.Page(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestPage_GetUserError(t *testing.T) {
	handler := NewSettingsHandler(
		&MockSettingsStore{ErrGetUser: pgx.ErrNoRows},
		&spyRenderer{},
	)

	w := httptest.NewRecorder()
	handler.Page(w, settingsGetRequest("/account/settings"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestPage_ListProvidersError(t *testing.T) {
	handler := NewSettingsHandler(
		&MockSettingsStore{
			User:             db.User{Username: "alice", Email: "alice@example.com"},
			ErrListProviders: pgx.ErrNoRows,
		},
		&spyRenderer{},
	)

	w := httptest.NewRecorder()
	handler.Page(w, settingsGetRequest("/account/settings"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestPage_GetAISettingsError(t *testing.T) {
	handler := NewSettingsHandler(
		&MockSettingsStore{
			User:             db.User{Username: "alice", Email: "alice@example.com"},
			ErrGetAISettings: uniqueViolation(),
		},
		&spyRenderer{},
	)

	w := httptest.NewRecorder()
	handler.Page(w, settingsGetRequest("/account/settings"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestPage_NoSettings_RendersDefaults(t *testing.T) {
	renderer := &spyRenderer{}
	handler := NewSettingsHandler(
		&MockSettingsStore{
			User:             db.User{Username: "alice", Email: "alice@example.com"},
			EnabledProviders: openAICatalog(),
			ErrGetAISettings: pgx.ErrNoRows,
		},
		renderer,
	)

	w := httptest.NewRecorder()
	handler.Page(w, settingsGetRequest("/account/settings"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if renderer.name != "account-settings" {
		t.Errorf("rendered template = %q, want %q", renderer.name, "account-settings")
	}

	data, ok := renderer.data.(accountSettingsPageData)
	if !ok {
		t.Fatalf("renderer data type = %T, want accountSettingsPageData", renderer.data)
	}
	if data.ActivePage != "account-settings" {
		t.Errorf("ActivePage = %q, want account-settings", data.ActivePage)
	}
	if data.Username != "alice" || data.Email != "alice@example.com" {
		t.Errorf("profile = %+v, want alice/alice@example.com", data)
	}
	if !reflect.DeepEqual(data.Providers, []string{"OpenAI", "Anthropic"}) {
		t.Errorf("Providers = %v, want distinct [OpenAI Anthropic]", data.Providers)
	}
	if data.HasKey || data.Enabled {
		t.Errorf("expected no key and disabled, got HasKey=%v Enabled=%v", data.HasKey, data.Enabled)
	}
	if data.ModelName != "" || data.EffortLevel != "" {
		t.Errorf("expected empty selection, got model=%q effort=%q", data.ModelName, data.EffortLevel)
	}
}

func TestPage_WithSavedSettings_DistinctModelsAndEfforts(t *testing.T) {
	renderer := &spyRenderer{}
	handler := NewSettingsHandler(
		&MockSettingsStore{
			User:             db.User{Username: "alice", Email: "alice@example.com"},
			EnabledProviders: openAICatalog(),
			AISettings: db.UserAiSetting{
				ProviderName: "OpenAI",
				ModelName:    "gpt-6-astra",
				EffortLevel:  "medium",
				Key:          utils.PGText("stored-key"),
				Enabled:      true,
			},
		},
		renderer,
	)

	w := httptest.NewRecorder()
	handler.Page(w, settingsGetRequest("/account/settings"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	data := renderer.data.(accountSettingsPageData)
	if !data.HasKey || !data.Enabled {
		t.Errorf("expected key present and enabled, got HasKey=%v Enabled=%v", data.HasKey, data.Enabled)
	}
	if data.ProviderName != "OpenAI" || data.ModelName != "gpt-6-astra" {
		t.Errorf("selection = %s/%s, want OpenAI/gpt-6-astra", data.ProviderName, data.ModelName)
	}
	if data.EffortLevel != "medium" {
		t.Errorf("EffortLevel = %q, want medium", data.EffortLevel)
	}
	if !reflect.DeepEqual(data.Models, []string{"gpt-5.6-sol", "gpt-6-astra"}) {
		t.Errorf("Models = %v, want distinct [gpt-5.6-sol gpt-6-astra]", data.Models)
	}
	if !reflect.DeepEqual(data.Efforts, []string{"none", "medium", "max"}) {
		t.Errorf("Efforts = %v, want provider-ordered [none medium max]", data.Efforts)
	}
}

func TestPage_RenderError(t *testing.T) {
	handler := NewSettingsHandler(
		&MockSettingsStore{User: db.User{Username: "alice", Email: "alice@example.com"}},
		&spyRenderer{err: pgx.ErrNoRows},
	)

	w := httptest.NewRecorder()
	handler.Page(w, settingsGetRequest("/account/settings"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestConfigOptions_RendersModelsForProvider(t *testing.T) {
	renderer := &spyRenderer{}
	handler := NewSettingsHandler(
		&MockSettingsStore{
			ModelsByName: map[string][]string{"OpenAI": {"gpt-5.6-sol", "gpt-6-astra"}},
		},
		renderer,
	)

	w := httptest.NewRecorder()
	handler.ConfigOptions(w, settingsGetRequest("/account/settings/ai/config?provider_name=OpenAI"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if renderer.name != "ai-config-fields" {
		t.Errorf("rendered template = %q, want ai-config-fields", renderer.name)
	}
	data, ok := renderer.data.(accountSettingsPageData)
	if !ok {
		t.Fatalf("renderer data type = %T, want accountSettingsPageData", renderer.data)
	}
	if data.ProviderName != "OpenAI" {
		t.Errorf("ProviderName = %q, want OpenAI", data.ProviderName)
	}
	if !reflect.DeepEqual(data.Models, []string{"gpt-5.6-sol", "gpt-6-astra"}) {
		t.Errorf("Models = %v, want [gpt-5.6-sol gpt-6-astra]", data.Models)
	}
}

func TestConfigOptions_ListError(t *testing.T) {
	handler := NewSettingsHandler(
		&MockSettingsStore{ErrListModels: pgx.ErrNoRows},
		&spyRenderer{},
	)

	w := httptest.NewRecorder()
	handler.ConfigOptions(w, settingsGetRequest("/account/settings/ai/config?provider_name=OpenAI"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestConfigOptions_RenderError(t *testing.T) {
	handler := NewSettingsHandler(
		&MockSettingsStore{},
		&spyRenderer{err: pgx.ErrNoRows},
	)

	w := httptest.NewRecorder()
	handler.ConfigOptions(w, settingsGetRequest("/account/settings/ai/config?provider_name=OpenAI"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestEffortOptions_RendersEffortsForModel(t *testing.T) {
	renderer := &spyRenderer{}
	handler := NewSettingsHandler(
		&MockSettingsStore{
			EffortsByOffering: map[string][]string{
				offeringKey("Anthropic", "claude-opus-5"): {"low", "max", "high", "xhigh", "medium"},
			},
		},
		renderer,
	)

	w := httptest.NewRecorder()
	handler.EffortOptions(w, settingsGetRequest("/account/settings/ai/efforts?provider_name=Anthropic&model_name=claude-opus-5"))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if renderer.name != "ai-effort-options" {
		t.Errorf("rendered template = %q, want ai-effort-options", renderer.name)
	}
	data, ok := renderer.data.(accountSettingsPageData)
	if !ok {
		t.Fatalf("renderer data type = %T, want accountSettingsPageData", renderer.data)
	}
	if !reflect.DeepEqual(data.Efforts, []string{"max", "xhigh", "high", "medium", "low"}) {
		t.Errorf("Efforts = %v, want provider-ordered [max xhigh high medium low]", data.Efforts)
	}
}

func TestEffortOptions_MissingParams(t *testing.T) {
	handler := NewSettingsHandler(&MockSettingsStore{}, &spyRenderer{})

	w := httptest.NewRecorder()
	handler.EffortOptions(w, settingsGetRequest("/account/settings/ai/efforts?provider_name=OpenAI"))

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestEffortOptions_ListError(t *testing.T) {
	handler := NewSettingsHandler(
		&MockSettingsStore{ErrListEfforts: pgx.ErrNoRows},
		&spyRenderer{},
	)

	w := httptest.NewRecorder()
	handler.EffortOptions(w, settingsGetRequest("/account/settings/ai/efforts?provider_name=OpenAI&model_name=gpt-6-astra"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestEffortOptions_RenderError(t *testing.T) {
	handler := NewSettingsHandler(
		&MockSettingsStore{},
		&spyRenderer{err: pgx.ErrNoRows},
	)

	w := httptest.NewRecorder()
	handler.EffortOptions(w, settingsGetRequest("/account/settings/ai/efforts?provider_name=OpenAI&model_name=gpt-6-astra"))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestUpdateUsername_MissingUserID(t *testing.T) {
	handler := NewSettingsHandler(&MockSettingsStore{}, &spyRenderer{})

	req := formRequest(http.MethodPut, "/account/username", url.Values{"username": {"bob"}})
	w := httptest.NewRecorder()

	handler.UpdateUsername(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestUpdateUsername_EmptyUsername(t *testing.T) {
	handler := NewSettingsHandler(&MockSettingsStore{}, &spyRenderer{})

	w := httptest.NewRecorder()
	handler.UpdateUsername(w, settingsFormRequest(http.MethodPut, "/account/username", url.Values{"username": {"   "}}))

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateUsername_Success(t *testing.T) {
	store := &MockSettingsStore{}
	handler := NewSettingsHandler(store, &spyRenderer{})

	w := httptest.NewRecorder()
	handler.UpdateUsername(w, settingsFormRequest(http.MethodPut, "/account/username", url.Values{"username": {"bob"}}))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if store.LastUsername.ID != settingsTestUserID || store.LastUsername.Username != "bob" {
		t.Errorf("update params = %+v, want user %d with username bob", store.LastUsername, settingsTestUserID)
	}
}

func TestUpdateUsername_UniqueViolation(t *testing.T) {
	handler := NewSettingsHandler(
		&MockSettingsStore{ErrUpdateUsername: uniqueViolation()},
		&spyRenderer{},
	)

	w := httptest.NewRecorder()
	handler.UpdateUsername(w, settingsFormRequest(http.MethodPut, "/account/username", url.Values{"username": {"taken"}}))

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", w.Code, http.StatusConflict)
	}
}

func TestUpdateUsername_StoreError(t *testing.T) {
	handler := NewSettingsHandler(
		&MockSettingsStore{ErrUpdateUsername: pgx.ErrNoRows},
		&spyRenderer{},
	)

	w := httptest.NewRecorder()
	handler.UpdateUsername(w, settingsFormRequest(http.MethodPut, "/account/username", url.Values{"username": {"bob"}}))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestUpdateEmail_InvalidEmail(t *testing.T) {
	handler := NewSettingsHandler(&MockSettingsStore{}, &spyRenderer{})

	w := httptest.NewRecorder()
	handler.UpdateEmail(w, settingsFormRequest(http.MethodPut, "/account/email", url.Values{"email": {"not-an-email"}}))

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateEmail_Success(t *testing.T) {
	store := &MockSettingsStore{}
	handler := NewSettingsHandler(store, &spyRenderer{})

	w := httptest.NewRecorder()
	handler.UpdateEmail(w, settingsFormRequest(http.MethodPut, "/account/email", url.Values{"email": {"bob@example.com"}}))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if store.LastEmail.ID != settingsTestUserID || store.LastEmail.Email != "bob@example.com" {
		t.Errorf("update params = %+v, want user %d with email bob@example.com", store.LastEmail, settingsTestUserID)
	}
}

func TestUpdateEmail_UniqueViolation(t *testing.T) {
	handler := NewSettingsHandler(
		&MockSettingsStore{ErrUpdateEmail: uniqueViolation()},
		&spyRenderer{},
	)

	w := httptest.NewRecorder()
	handler.UpdateEmail(w, settingsFormRequest(http.MethodPut, "/account/email", url.Values{"email": {"taken@example.com"}}))

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", w.Code, http.StatusConflict)
	}
}

func TestUpdateEmail_StoreError(t *testing.T) {
	handler := NewSettingsHandler(
		&MockSettingsStore{ErrUpdateEmail: pgx.ErrNoRows},
		&spyRenderer{},
	)

	w := httptest.NewRecorder()
	handler.UpdateEmail(w, settingsFormRequest(http.MethodPut, "/account/email", url.Values{"email": {"bob@example.com"}}))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestUpdateAISettings_GetSettingsError(t *testing.T) {
	store := &MockSettingsStore{ErrGetAISettings: uniqueViolation()}
	handler := NewSettingsHandler(store, &spyRenderer{})

	values := url.Values{
		"provider_name": {"OpenAI"},
		"model_name":    {"gpt-6-astra"},
		"effort_level":  {"high"},
		"encrypted_key": {"sk-test"},
		"enabled":       {"true"},
	}
	w := httptest.NewRecorder()
	handler.UpdateAISettings(w, settingsFormRequest(http.MethodPut, "/account/settings", values))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
	if store.UpsertCalled {
		t.Error("expected no upsert when settings lookup fails")
	}
}

func TestUpdateAISettings_MissingEffort(t *testing.T) {
	store := &MockSettingsStore{ErrGetAISettings: pgx.ErrNoRows}
	handler := NewSettingsHandler(store, &spyRenderer{})

	values := url.Values{
		"provider_name": {"OpenAI"},
		"model_name":    {"gpt-6-astra"},
		"effort_level":  {""},
		"enabled":       {"false"},
	}
	w := httptest.NewRecorder()
	handler.UpdateAISettings(w, settingsFormRequest(http.MethodPut, "/account/settings", values))

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if store.UpsertCalled {
		t.Error("expected no upsert for missing effort level")
	}
}

func TestUpdateAISettings_EnableWithoutKey(t *testing.T) {
	store := &MockSettingsStore{ErrGetAISettings: pgx.ErrNoRows}
	handler := NewSettingsHandler(store, &spyRenderer{})

	values := url.Values{
		"provider_name": {"OpenAI"},
		"model_name":    {"gpt-6-astra"},
		"effort_level":  {"high"},
		"encrypted_key": {""},
		"enabled":       {"true"},
	}
	w := httptest.NewRecorder()
	handler.UpdateAISettings(w, settingsFormRequest(http.MethodPut, "/account/settings", values))

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if store.UpsertCalled {
		t.Error("expected no upsert when enabling without a key")
	}
}

func TestUpdateAISettings_ProviderChangedEnabledWithoutNewKey(t *testing.T) {
	store := &MockSettingsStore{
		AISettings: db.UserAiSetting{
			ProviderName: "OpenAI",
			ModelName:    "gpt-6-astra",
			EffortLevel:  "high",
			Key:          utils.PGText("old"),
		},
	}
	handler := NewSettingsHandler(store, &spyRenderer{})

	values := url.Values{
		"provider_name": {"Anthropic"},
		"model_name":    {"claude-opus-5"},
		"effort_level":  {"max"},
		"encrypted_key": {""},
		"enabled":       {"true"},
	}
	w := httptest.NewRecorder()
	handler.UpdateAISettings(w, settingsFormRequest(http.MethodPut, "/account/settings", values))

	// Provider changed => stored key is cleared, so enabling without a new key is rejected.
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	if store.UpsertCalled {
		t.Error("expected no upsert when enabling after provider change without a key")
	}
}

func TestUpdateAISettings_SaveWithNewKey(t *testing.T) {
	store := &MockSettingsStore{ErrGetAISettings: pgx.ErrNoRows}
	handler := NewSettingsHandler(store, &spyRenderer{})

	values := url.Values{
		"provider_name": {"OpenAI"},
		"model_name":    {"gpt-6-astra"},
		"effort_level":  {"max"},
		"encrypted_key": {"sk-test"},
		"enabled":       {"true"},
	}
	w := httptest.NewRecorder()
	handler.UpdateAISettings(w, settingsFormRequest(http.MethodPut, "/account/settings", values))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !store.UpsertCalled {
		t.Fatal("expected upsert to be called")
	}
	if !store.LastUpsert.Enabled {
		t.Error("Enabled = false, want true")
	}
	if store.LastUpsert.EffortLevel != "max" {
		t.Errorf("EffortLevel = %q, want max", store.LastUpsert.EffortLevel)
	}
	if string(store.LastUpsert.Key.String) != "sk-test" {
		t.Errorf("Key = %s, want sk-test", store.LastUpsert.Key.String)
	}
}

func TestUpdateAISettings_EffortChangeKeepsKey(t *testing.T) {
	store := &MockSettingsStore{
		AISettings: db.UserAiSetting{
			ProviderName: "OpenAI",
			ModelName:    "gpt-6-astra",
			EffortLevel:  "low",
			Key:          utils.PGText("stored-key"),
			Enabled:      true,
		},
	}
	handler := NewSettingsHandler(store, &spyRenderer{})

	values := url.Values{
		"provider_name": {"OpenAI"},
		"model_name":    {"gpt-6-astra"},
		"effort_level":  {"max"},
		"encrypted_key": {""},
		"enabled":       {"true"},
	}
	w := httptest.NewRecorder()
	handler.UpdateAISettings(w, settingsFormRequest(http.MethodPut, "/account/settings", values))

	// Only the effort changed, so the key is still valid for the provider/model and must be kept.
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if store.LastUpsert.EffortLevel != "max" {
		t.Errorf("EffortLevel = %q, want max", store.LastUpsert.EffortLevel)
	}
	if !strings.EqualFold(store.LastUpsert.Key.String, "stored-key") {
		t.Errorf("Key = %q, want stored-key kept on effort change", store.LastUpsert.Key.String)
	}
}

func TestUpdateAISettings_ProviderChangeClearsKey(t *testing.T) {
	store := &MockSettingsStore{
		AISettings: db.UserAiSetting{
			ProviderName: "OpenAI",
			ModelName:    "gpt-6-astra",
			EffortLevel:  "high",
			Key:          utils.PGText("stored-key"),
			Enabled:      true,
		},
	}
	handler := NewSettingsHandler(store, &spyRenderer{})

	values := url.Values{
		"provider_name": {"Anthropic"},
		"model_name":    {"claude-opus-5"},
		"effort_level":  {"low"},
		"encrypted_key": {""},
		"enabled":       {"false"},
	}
	w := httptest.NewRecorder()
	handler.UpdateAISettings(w, settingsFormRequest(http.MethodPut, "/account/settings", values))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if len(store.LastUpsert.Key.String) != 0 {
		t.Errorf("Key = %s, want cleared after provider change", store.LastUpsert.Key.String)
	}
	if store.LastUpsert.ProviderName != "Anthropic" || store.LastUpsert.EffortLevel != "low" {
		t.Errorf("upsert = %s/%s/%s, want Anthropic/claude-opus-5/low", store.LastUpsert.ProviderName, store.LastUpsert.ModelName, store.LastUpsert.EffortLevel)
	}
}

func TestUpdateAISettings_NewKeyReplacesOldAfterProviderChange(t *testing.T) {
	store := &MockSettingsStore{
		AISettings: db.UserAiSetting{
			ProviderName: "OpenAI",
			ModelName:    "gpt-6-astra",
			EffortLevel:  "high",
			Key:          utils.PGText("old-key"),
			Enabled:      true,
		},
	}
	handler := NewSettingsHandler(store, &spyRenderer{})

	values := url.Values{
		"provider_name": {"Anthropic"},
		"model_name":    {"claude-opus-5"},
		"effort_level":  {"max"},
		"encrypted_key": {"sk-anthropic"},
		"enabled":       {"true"},
	}
	w := httptest.NewRecorder()
	handler.UpdateAISettings(w, settingsFormRequest(http.MethodPut, "/account/settings", values))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if store.LastUpsert.Key.String != "sk-anthropic" {
		t.Errorf("Key = %s, want sk-anthropic", store.LastUpsert.Key.String)
	}
}

func TestUpdateAISettings_ForeignKeyViolation(t *testing.T) {
	store := &MockSettingsStore{
		ErrGetAISettings: pgx.ErrNoRows,
		ErrUpsert:        foreignKeyViolation(),
	}
	handler := NewSettingsHandler(store, &spyRenderer{})

	values := url.Values{
		"provider_name": {"Unknown"},
		"model_name":    {"ghost-model"},
		"effort_level":  {"max"},
		"encrypted_key": {"sk-x"},
		"enabled":       {"true"},
	}
	w := httptest.NewRecorder()
	handler.UpdateAISettings(w, settingsFormRequest(http.MethodPut, "/account/settings", values))

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateAISettings_StoreError(t *testing.T) {
	store := &MockSettingsStore{
		ErrGetAISettings: pgx.ErrNoRows,
		ErrUpsert:        pgx.ErrNoRows,
	}
	handler := NewSettingsHandler(store, &spyRenderer{})

	values := url.Values{
		"provider_name": {"OpenAI"},
		"model_name":    {"gpt-6-astra"},
		"effort_level":  {"high"},
		"encrypted_key": {"sk-x"},
		"enabled":       {"true"},
	}
	w := httptest.NewRecorder()
	handler.UpdateAISettings(w, settingsFormRequest(http.MethodPut, "/account/settings", values))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestRemoveKey_NoSettingsRow(t *testing.T) {
	store := &MockSettingsStore{ErrGetAISettings: pgx.ErrNoRows}
	handler := NewSettingsHandler(store, &spyRenderer{})

	w := httptest.NewRecorder()
	handler.RemoveKey(w, settingsFormRequest(http.MethodDelete, "/account/settings/key", url.Values{}))

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if store.UpsertCalled {
		t.Error("expected no upsert when no settings row exists")
	}
}

func TestRemoveKey_DisablesAndClearsKey(t *testing.T) {
	store := &MockSettingsStore{
		AISettings: db.UserAiSetting{
			ProviderName: "OpenAI",
			ModelName:    "gpt-6-astra",
			EffortLevel:  "high",
			Key:          utils.PGText("stored-key"),
			Enabled:      true,
		},
	}
	handler := NewSettingsHandler(store, &spyRenderer{})

	w := httptest.NewRecorder()
	handler.RemoveKey(w, settingsFormRequest(http.MethodDelete, "/account/settings/key", url.Values{}))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !store.UpsertCalled {
		t.Fatal("expected upsert to be called")
	}
	if len(store.LastUpsert.Key.String) != 0 {
		t.Errorf("Key = %s, want cleared", store.LastUpsert.Key.String)
	}
	if store.LastUpsert.Enabled {
		t.Error("Enabled = true, want disabled after key removal")
	}
	if store.LastUpsert.ProviderName != "OpenAI" || store.LastUpsert.ModelName != "gpt-6-astra" || store.LastUpsert.EffortLevel != "high" {
		t.Errorf("preserved settings = %s/%s/%s, want OpenAI/gpt-6-astra/high",
			store.LastUpsert.ProviderName, store.LastUpsert.ModelName, store.LastUpsert.EffortLevel)
	}
	if w.Header().Get("HX-Refresh") != "true" {
		t.Error("HX-Refresh header not set to true")
	}
}

func TestRemoveKey_GetSettingsError(t *testing.T) {
	store := &MockSettingsStore{ErrGetAISettings: uniqueViolation()}
	handler := NewSettingsHandler(store, &spyRenderer{})

	w := httptest.NewRecorder()
	handler.RemoveKey(w, settingsFormRequest(http.MethodDelete, "/account/settings/key", url.Values{}))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestRemoveKey_UpsertError(t *testing.T) {
	store := &MockSettingsStore{
		AISettings: db.UserAiSetting{
			ProviderName: "OpenAI",
			ModelName:    "gpt-6-astra",
			EffortLevel:  "high",
			Key:          utils.PGText("stored-key"),
		},
		ErrUpsert: pgx.ErrNoRows,
	}
	handler := NewSettingsHandler(store, &spyRenderer{})

	w := httptest.NewRecorder()
	handler.RemoveKey(w, settingsFormRequest(http.MethodDelete, "/account/settings/key", url.Values{}))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestOrderedEfforts_ProviderPreference(t *testing.T) {
	cases := map[string]struct {
		input []string
		want  []string
	}{
		"OpenAI":    {[]string{"max", "none", "high", "medium", "low", "xhigh"}, []string{"none", "low", "medium", "high", "xhigh", "max"}},
		"Anthropic": {[]string{"medium", "low", "xhigh", "max", "high"}, []string{"max", "xhigh", "high", "medium", "low"}},
		"DeepSeek":  {[]string{"low", "max", "high"}, []string{"max", "high", "low"}},
		"Google":    {[]string{"low"}, []string{"low"}},
	}
	for provider, tc := range cases {
		if got := orderedEfforts(provider, tc.input); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("orderedEfforts(%q) = %v, want %v", provider, got, tc.want)
		}
	}
}

func TestOrderedEfforts_UnknownEffortsFallBackAlphabetically(t *testing.T) {
	input := []string{"custom-a", "low", "custom-b"}
	want := []string{"low", "custom-a", "custom-b"}
	if got := orderedEfforts("OpenAI", input); !reflect.DeepEqual(got, want) {
		t.Errorf("orderedEfforts = %v, want %v", got, want)
	}
}

func TestOrderedEfforts_KnownBeforeUnknown(t *testing.T) {
	// A known effort always sorts ahead of an unknown one, regardless of input
	// order. The two-element slice deterministically exercises both comparison
	// directions (known-first and unknown-first).
	for _, tt := range []struct {
		name  string
		input []string
	}{
		{"known first", []string{"low", "custom"}},
		{"unknown first", []string{"custom", "low"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := orderedEfforts("OpenAI", tt.input); !reflect.DeepEqual(got, []string{"low", "custom"}) {
				t.Errorf("orderedEfforts(%v) = %v, want [low custom]", tt.input, got)
			}
		})
	}
}

func TestIsValidEmail(t *testing.T) {
	valid := []string{"a@b.co", "alice+tag@example.com"}
	invalid := []string{"", "   ", "not-an-email", "@missing-user.com", "no-at-sign"}

	for _, email := range valid {
		if !isValidEmail(email) {
			t.Errorf("isValidEmail(%q) = false, want true", email)
		}
	}
	for _, email := range invalid {
		if isValidEmail(email) {
			t.Errorf("isValidEmail(%q) = true, want false", email)
		}
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if !isUniqueViolation(uniqueViolation()) {
		t.Error("expected 23505 to be treated as a unique violation")
	}
	if isUniqueViolation(foreignKeyViolation()) {
		t.Error("expected 23503 not to be treated as a unique violation")
	}
	if isUniqueViolation(pgx.ErrNoRows) {
		t.Error("expected unrelated errors not to be unique violations")
	}
	if isUniqueViolation(nil) {
		t.Error("expected nil not to be a unique violation")
	}
}

func TestIsForeignKeyViolation(t *testing.T) {
	if !isForeignKeyViolation(foreignKeyViolation()) {
		t.Error("expected 23503 to be treated as a foreign key violation")
	}
	if isForeignKeyViolation(uniqueViolation()) {
		t.Error("expected 23505 not to be treated as a foreign key violation")
	}
	if isForeignKeyViolation(pgx.ErrNoRows) {
		t.Error("expected unrelated errors not to be foreign key violations")
	}
}

func TestUpdateEmail_MissingUserID(t *testing.T) {
	handler := NewSettingsHandler(&MockSettingsStore{}, &spyRenderer{})

	req := formRequest(http.MethodPut, "/account/email", url.Values{"email": {"a@b.com"}})
	w := httptest.NewRecorder()

	handler.UpdateEmail(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestUpdateAISettings_MissingUserID(t *testing.T) {
	handler := NewSettingsHandler(&MockSettingsStore{}, &spyRenderer{})

	req := formRequest(http.MethodPut, "/account/settings/ai", url.Values{
		"provider_name": {"OpenAI"},
		"model_name":    {"gpt-5.6-sol"},
		"effort_level":  {"low"},
	})
	w := httptest.NewRecorder()

	handler.UpdateAISettings(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestRemoveKey_MissingUserID(t *testing.T) {
	handler := NewSettingsHandler(&MockSettingsStore{}, &spyRenderer{})

	req := httptest.NewRequest(http.MethodDelete, "/account/settings/ai/key", nil)
	w := httptest.NewRecorder()

	handler.RemoveKey(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}
