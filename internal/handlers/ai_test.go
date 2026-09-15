package handlers

import (
	"context"
	"fmt"
	"job-applications/internal/ai"
	"job-applications/internal/db"
	"job-applications/internal/utils"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

type MockAIStore struct {
	AISettings       db.UserAiSetting
	ErrGetAISettings error

	ErrModelInfo error

	CVs       []db.GetAllUserCVsRow
	ErrGetCVs error

	CV        db.GetCVByIDRow
	ErrGetCV  error
	ErrGetApp error
}

func (m *MockAIStore) GetUserAISettings(_ context.Context, _ int32) (db.UserAiSetting, error) {
	return m.AISettings, m.ErrGetAISettings
}

func (m *MockAIStore) GetModelInformationByNameAndEffort(_ context.Context, _ db.GetModelInformationByNameAndEffortParams) (db.GetModelInformationByNameAndEffortRow, error) {
	if m.ErrModelInfo != nil {
		return db.GetModelInformationByNameAndEffortRow{}, m.ErrModelInfo
	}
	return db.GetModelInformationByNameAndEffortRow{BaseUrl: "https://example.test"}, nil
}

func (m *MockAIStore) GetAllUserCVs(_ context.Context, _ int32) ([]db.GetAllUserCVsRow, error) {
	if m.ErrGetCVs != nil {
		return nil, m.ErrGetCVs
	}
	return m.CVs, nil
}

func (m *MockAIStore) GetCVByID(_ context.Context, _ db.GetCVByIDParams) (db.GetCVByIDRow, error) {
	if m.ErrGetCV != nil {
		return db.GetCVByIDRow{}, m.ErrGetCV
	}
	return m.CV, nil
}

func newAIHandler(aiStore AIStore, chatClient ai.ChatClient, renderShouldFail bool) *AIHandler {
	return NewAIHandler(aiStore, func(string) ai.ChatClient { return chatClient }, newMockRenderer(renderShouldFail))
}

func TestAIPage_NoUserID(t *testing.T) {
	handler := newAIHandler(&MockAIStore{}, ai.OpenAIClient{}, false)
	req := httptest.NewRequest(http.MethodGet, "/ai/chat", nil)
	w := httptest.NewRecorder()

	handler.Page(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestAIPage_CVStoreError(t *testing.T) {
	handler := newAIHandler(&MockAIStore{ErrGetCVs: fmt.Errorf("mock error: no cvs")}, ai.OpenAIClient{}, false)
	req := httptest.NewRequest(http.MethodGet, "/ai/chat", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Page(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestAIPage_RenderError(t *testing.T) {
	handler := newAIHandler(&MockAIStore{}, ai.OpenAIClient{}, true)
	req := httptest.NewRequest(http.MethodGet, "/ai/chat", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Page(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestAIPage_Success(t *testing.T) {
	handler := newAIHandler(&MockAIStore{}, ai.OpenAIClient{}, false)
	req := httptest.NewRequest(http.MethodGet, "/ai/chat", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Page(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

// A failed settings lookup is only logged, never fatal: the chat page should
// still render so the user can configure their AI provider.
func TestAIPage_SettingsErrorStillRenders(t *testing.T) {
	handler := newAIHandler(&MockAIStore{ErrGetAISettings: fmt.Errorf("mock error: no settings")}, ai.OpenAIClient{}, false)
	req := httptest.NewRequest(http.MethodGet, "/ai/chat", nil)
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Page(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d (settings failure must not block the page)", w.Code, http.StatusOK)
	}
}

func TestAIClearThread(t *testing.T) {
	handler := newAIHandler(&MockAIStore{}, ai.OpenAIClient{}, false)
	req := httptest.NewRequest(http.MethodGet, "/ai/chat/clear", nil)
	w := httptest.NewRecorder()

	handler.ClearThread(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestAIClearThread_RenderError(t *testing.T) {
	handler := newAIHandler(&MockAIStore{}, ai.OpenAIClient{}, true)
	req := httptest.NewRequest(http.MethodGet, "/ai/chat/clear", nil)
	w := httptest.NewRecorder()

	handler.ClearThread(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestAIChat_NoUserID(t *testing.T) {
	handler := newAIHandler(&MockAIStore{}, ai.OpenAIClient{}, false)
	req := formRequest(http.MethodPost, "/ai/chat", url.Values{"cv_id": {"1"}})
	w := httptest.NewRecorder()

	handler.Chat(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestAIChat_InvalidCV(t *testing.T) {
	handler := newAIHandler(&MockAIStore{}, ai.OpenAIClient{}, false)

	for _, cvID := range []string{"", "abc", "0"} {
		req := formRequest(http.MethodPost, "/ai/chat", url.Values{"cv_id": {cvID}})
		req = withUserID(req, 1)
		w := httptest.NewRecorder()

		handler.Chat(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("cv_id %q: status = %d, want %d", cvID, w.Code, http.StatusBadRequest)
		}
	}
}

func TestAIChat_InvalidTranscript(t *testing.T) {
	handler := newAIHandler(&MockAIStore{}, ai.OpenAIClient{}, false)
	req := formRequest(http.MethodPost, "/ai/chat", url.Values{"cv_id": {"1"}, "transcript": {"[not json"}})
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Chat(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAIChat_ParseFormError(t *testing.T) {
	handler := newAIHandler(&MockAIStore{}, ai.OpenAIClient{}, false)
	// An invalid percent-escape in a form-encoded body makes ParseForm fail.
	req := httptest.NewRequest(http.MethodPost, "/ai/chat", strings.NewReader("cv_id=%zz"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withUserID(req, 1)
	w := httptest.NewRecorder()

	handler.Chat(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// stubAIChat is the fake network client injected into AIHandler: tests decide
// exactly what the "model" returns, so Chat's full flow runs offline.
type stubAIChat struct {
	reply string
	err   error
}

func (s stubAIChat) Send(_ context.Context, _, _ string, _ []ai.Message, _, _ string) (string, error) {
	return s.reply, s.err
}

// aiChatCV returns a CV row that cvToText can render, so Chat reaches the
// settings/model/client steps without tripping over malformed stored JSON.
func aiChatCV() db.GetCVByIDRow {
	return db.GetCVByIDRow{
		ID:                    1,
		Title:                 "Generalist",
		Profile:               []byte(`{"name":"Jane","location":"London"}`),
		Experiences:           []byte(`[]`),
		Education:             pgtype.Text{},
		Certifications:        pgtype.Text{},
		AcademicContributions: pgtype.Text{},
		Skills:                pgtype.Text{},
	}
}

func aiChatRequest() *http.Request {
	req := formRequest(http.MethodPost, "/ai/chat", url.Values{"cv_id": {"1"}})
	return withUserID(req, 1)
}

func TestAIChat_GetCVError(t *testing.T) {
	handler := newAIHandler(&MockAIStore{ErrGetCV: fmt.Errorf("mock error: no cv")}, stubAIChat{}, false)
	req := aiChatRequest()
	w := httptest.NewRecorder()

	handler.Chat(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestAIChat_CVSerializationError(t *testing.T) {
	bad := aiChatCV()
	bad.Profile = []byte(`{"name":`)
	handler := newAIHandler(&MockAIStore{CV: bad}, stubAIChat{}, false)
	req := aiChatRequest()
	w := httptest.NewRecorder()

	handler.Chat(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestAIChat_GetAISettingsError(t *testing.T) {
	handler := newAIHandler(&MockAIStore{CV: aiChatCV(), ErrGetAISettings: fmt.Errorf("mock error: no settings")}, stubAIChat{}, false)
	req := aiChatRequest()
	w := httptest.NewRecorder()

	handler.Chat(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestAIChat_InvalidAISettings(t *testing.T) {
	handler := newAIHandler(&MockAIStore{CV: aiChatCV(), AISettings: db.UserAiSetting{}}, stubAIChat{}, false)
	req := aiChatRequest()
	w := httptest.NewRecorder()

	handler.Chat(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestAIChat_ModelInfoError(t *testing.T) {
	store := &MockAIStore{
		CV: aiChatCV(),
		AISettings: db.UserAiSetting{
			ProviderName: "provider",
			ModelName:    "model",
			EffortLevel:  "medium",
			Key:          utils.PGText("key"),
		},
		ErrModelInfo: fmt.Errorf("mock error: no model"),
	}
	handler := newAIHandler(store, stubAIChat{}, false)
	req := aiChatRequest()
	w := httptest.NewRecorder()

	handler.Chat(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestAIChat_NetworkError(t *testing.T) {
	store := &MockAIStore{
		CV: aiChatCV(),
		AISettings: db.UserAiSetting{
			ProviderName: "provider",
			ModelName:    "model",
			EffortLevel:  "medium",
			Key:          utils.PGText("key"),
		},
	}
	handler := newAIHandler(store, stubAIChat{err: fmt.Errorf("mock error: api down")}, false)
	req := aiChatRequest()
	w := httptest.NewRecorder()

	handler.Chat(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestAIChat_RenderError(t *testing.T) {
	store := &MockAIStore{
		CV: aiChatCV(),
		AISettings: db.UserAiSetting{
			ProviderName: "provider",
			ModelName:    "model",
			EffortLevel:  "medium",
			Key:          utils.PGText("key"),
		},
	}
	handler := NewAIHandler(store, func(string) ai.ChatClient { return stubAIChat{reply: "reply"} }, &spyRenderer{err: fmt.Errorf("mock error: render failed")})
	req := aiChatRequest()
	w := httptest.NewRecorder()

	handler.Chat(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestAIChat_Success(t *testing.T) {
	spy := &spyRenderer{}
	store := &MockAIStore{
		CV: aiChatCV(),
		AISettings: db.UserAiSetting{
			ProviderName: "provider",
			ModelName:    "model",
			EffortLevel:  "medium",
			Key:          utils.PGText("key"),
		},
	}
	handler := NewAIHandler(store, func(string) ai.ChatClient { return stubAIChat{reply: "This looks strong"} }, spy)
	req := aiChatRequest()
	w := httptest.NewRecorder()

	handler.Chat(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if spy.name != "ai-chat-response" {
		t.Fatalf("rendered template = %q, want ai-chat-response", spy.name)
	}
	data, ok := spy.data.(AIChatResponseData)
	if !ok {
		t.Fatalf("render data = %T, want AIChatResponseData", spy.data)
	}
	if len(data.Thread) != 2 {
		t.Fatalf("thread has %d messages, want 2 (user turn + assistant reply)", len(data.Thread))
	}
	assistant := data.Thread[1]
	if assistant.Role != "assistant" || assistant.Content != "This looks strong" {
		t.Errorf("assistant turn = %+v, want role=assistant content=This looks strong", assistant)
	}
	if data.Transcript == "" {
		t.Error("expected a non-empty transcript to echo into the next request")
	}
}

func TestValidateAISettings(t *testing.T) {
	tests := []struct {
		name     string
		settings db.UserAiSetting
		want     bool
	}{
		{"all present", db.UserAiSetting{ProviderName: "p", ModelName: "m", EffortLevel: "e", Key: utils.PGText("key")}, true},
		{"missing provider", db.UserAiSetting{ModelName: "m", EffortLevel: "e", Key: utils.PGText("key")}, false},
		{"missing model", db.UserAiSetting{ProviderName: "p", EffortLevel: "e", Key: utils.PGText("key")}, false},
		{"missing effort", db.UserAiSetting{ProviderName: "p", ModelName: "m", Key: utils.PGText("key")}, false},
		{"missing key", db.UserAiSetting{ProviderName: "p", ModelName: "m", EffortLevel: "e"}, false},
		{"all empty", db.UserAiSetting{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateAISettings(tt.settings); got != tt.want {
				t.Errorf("validateAISettings(%+v) = %v, want %v", tt.settings, got, tt.want)
			}
		})
	}
}
