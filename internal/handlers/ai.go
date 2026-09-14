package handlers

import (
	"context"
	"job-applications/internal/ai"
	"job-applications/internal/db"
	"job-applications/internal/middlewares"
	"job-applications/internal/templates"
	"job-applications/internal/utils"
	"job-applications/prompts"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type AIStore interface {
	// Used to pre-populate model, effort, and API Key
	GetUserAISettings(ctx context.Context, userID int32) (db.UserAiSetting, error)
	GetModelInformationByNameAndEffort(ctx context.Context, params db.GetModelInformationByNameAndEffortParams) (db.GetModelInformationByNameAndEffortRow, error)
	// This may not be needed
	GetAllUserCVs(ctx context.Context, userID int32) ([]db.GetAllUserCVsRow, error)

	// This is used to get CV Information and Application information to be used as a User Message
	GetCVByID(ctx context.Context, arg db.GetCVByIDParams) (db.GetCVByIDRow, error)
}

type AIHandler struct {
	AIStore     AIStore
	Renderer    templates.Renderer
	ChatFactory ai.ChatFactory
}

type AIPageData struct {
	ActivePage string
	AISettings db.UserAiSetting
	Prompts    map[string]string
	CVs        []db.GetAllUserCVsRow
}

// AIChatResponseData carries a finished chat turn to the ai-chat-response
// fragment: the full thread for the swap, plus the serialized transcript that
// is echoed into the hidden #chat-transcript field for the next request.
type AIChatResponseData struct {
	Thread     []AIConversation
	Transcript string
}

type AIConversation struct {
	Role    string
	Content string
	Time    time.Time
}

func NewAIHandler(aiStore AIStore, chatFactory ai.ChatFactory, renderer templates.Renderer) *AIHandler {
	return &AIHandler{
		AIStore:     aiStore,
		Renderer:    renderer,
		ChatFactory: chatFactory,
	}
}

func getPrompts() map[string]string {
	return prompts.Prompts()
}

func (handler *AIHandler) Chat(writer http.ResponseWriter, request *http.Request) {
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID")
		return
	}
	errParse := request.ParseForm()
	if errParse != nil {
		slog.Error("Failed to parse input from chat")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to parse input from chat")
		return
	}
	transcript := request.PostForm.Get("transcript")
	jobDescription := request.PostForm.Get("job_description")
	followUp := request.PostForm.Get("follow_up")
	cvID := request.PostForm.Get("cv_id")
	prompt := request.PostForm.Get("prompt")
	parsedCVID, errCVParse := utils.ConvertFromStrToInt32(cvID)
	if errCVParse != nil || parsedCVID == 0 {
		slog.Error("Invalid CV id", "cv_id", cvID)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Choose a CV to review")
		return
	}

	history, errTranscript := unmarshalTranscript(transcript)
	if errTranscript != nil {
		slog.Error("Failed to parse chat transcript", "error", errTranscript)
		templates.RenderResponseMessage(writer, http.StatusBadRequest, "Failed to parse conversation history")
		return
	}

	// The current turn is a follow-up prompt when one was typed, otherwise the
	// job description (first review, or a re-review after editing the JD).
	userContent := strings.TrimSpace(followUp)
	if userContent == "" {
		userContent = strings.TrimSpace(jobDescription)
	}
	conversationData := appendConversation(history, userContent, "user")

	cv, errCV := handler.AIStore.GetCVByID(request.Context(), db.GetCVByIDParams{ID: parsedCVID, UserID: userID})
	if errCV != nil {
		slog.Error("Failed to get user CV", "error", errCV)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get CV")
		return
	}
	resumeText, errResume := cvToText(cv)
	if errResume != nil {
		slog.Error("Failed to serialize CV for AI request", "error", errResume)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to prepare CV for review")
		return
	}

	aiSettings, errSettings := handler.AIStore.GetUserAISettings(request.Context(), userID)
	if errSettings != nil {
		slog.Error("Failed to get user AI Settings", "error", errSettings)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user AI Settings")
		return
	}
	if !validateAISettings(aiSettings) {
		slog.Error("AI Settings Validation Failed", "error", errSettings)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to set AI Settings, please verify your chosen Provider, Model Name and API Key")
		return
	}

	params := db.GetModelInformationByNameAndEffortParams{Name: aiSettings.ProviderName, Model: aiSettings.ModelName, EffortLevel: aiSettings.EffortLevel}
	modelData, errModelData := handler.AIStore.GetModelInformationByNameAndEffort(request.Context(), params)
	if errModelData != nil {
		slog.Error("Failed to get model AI Information", "error", errModelData)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get model AI Information")
		return
	}

	// Order the model sees: system instructions, the CV as context, then the
	// full conversation history (whose first user turn is the job description).
	modelMessages := toAIMessages(prompt, resumeText, conversationData)
	chatClient := handler.ChatFactory(aiSettings.ProviderName)
	response, errResponse := chatClient.Send(
		request.Context(),
		modelData.BaseUrl,
		aiSettings.Key.String,
		modelMessages,
		modelData.Model,
		modelData.EffortLevel,
	)
	if errResponse != nil {
		slog.Error("Failed to get model AI Response", "error", errResponse)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get model AI Response")
		return
	}

	conversationData = appendConversation(conversationData, response, "assistant")

	transcript, errMarshal := marshalTranscript(conversationData)
	if errMarshal != nil {
		slog.Error("Failed to serialize conversation transcript", "error", errMarshal)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to serialize conversation history")
		return
	}
	responseData := AIChatResponseData{
		Thread:     conversationData,
		Transcript: transcript,
	}
	if errRender := handler.Renderer.Render(writer, "ai-chat-response", responseData); errRender != nil {
		slog.Error("Failed to render response from AI Model", "error", errRender)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render response from AI Model")
		return
	}

	slog.Info("Chat Endpoint reached by user", "user_id", userID)
}

// GET /ai/chat/clear
func (handler *AIHandler) ClearThread(writer http.ResponseWriter, request *http.Request) {
	if err := handler.Renderer.Render(writer, "ai-empty-chat", nil); err != nil {
		slog.Error("Failed to render ai empty chat", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render ai empty chat")
		return
	}

	writer.WriteHeader(http.StatusOK)
}

func (handler *AIHandler) Page(writer http.ResponseWriter, request *http.Request) {
	prompts := getPrompts()
	userID, ok := middlewares.GetUserIDFromContext(request.Context())
	if !ok {
		slog.Error("Failed to get user ID from context")
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get user ID")
		return
	}
	userAISettings, errSettings := handler.AIStore.GetUserAISettings(request.Context(), userID)
	if errSettings != nil {
		slog.Error("Failed to get user AI Settings", "error", errSettings)
	}

	userCVs, errCVs := handler.AIStore.GetAllUserCVs(request.Context(), userID)
	if errCVs != nil {
		slog.Error("Failed to get User CVs", "error", errCVs)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to get User CVs")
		return
	}

	data := AIPageData{
		ActivePage: "ai-chat",
		AISettings: userAISettings,
		CVs:        userCVs,
		Prompts:    prompts,
	}
	if err := handler.Renderer.Render(writer, "ai-chat", data); err != nil {
		slog.Error("Failed to render ai chat page", "error", err)
		templates.RenderResponseMessage(writer, http.StatusInternalServerError, "Failed to render ai chat page")
		return
	}
	slog.Info("List applications", "user_id", userID)
}

func validateAISettings(settings db.UserAiSetting) bool {
	if settings.ProviderName == "" || settings.ModelName == "" || settings.EffortLevel == "" || !settings.Key.Valid {
		return false
	}
	return true
}
