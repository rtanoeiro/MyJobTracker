package ai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"

	"github.com/microcosm-cc/bluemonday"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
)

// Message is a single chat turn sent to the provider. Role is one of
// "system", "user" or "assistant".
type Message struct {
	Role    string
	Content string
}

type OpenAIClient struct{}

func (handler OpenAIClient) Send(ctx context.Context, baseURL, apiKey string, messages []Message, model, effort string) (string, error) {
	client := openai.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)

	apiMessages := make([]openai.ChatCompletionMessageParamUnion, 0, len(messages))
	for _, message := range messages {
		switch message.Role {
		case "system":
			apiMessages = append(apiMessages, openai.SystemMessage(message.Content))
		case "assistant":
			apiMessages = append(apiMessages, openai.AssistantMessage(message.Content))
		default:
			apiMessages = append(apiMessages, openai.UserMessage(message.Content))
		}
	}

	params := openai.ChatCompletionNewParams{
		Messages: apiMessages,
		Model:    model,
	}
	if reasoningEffort := reasoningEffortFor(effort); reasoningEffort != "" {
		params.ReasoningEffort = reasoningEffort
	}

	chatCompletion, err := client.Chat.Completions.New(ctx, params)
	if err != nil {
		slog.Info("Error from API", "error", err)
		return "", fmt.Errorf("failed getting response from API %w", err)
	}

	if len(chatCompletion.Choices) == 0 {
		slog.Info("No response from AI.")
		return "", errors.New("no response from AI")
	}
	reply := chatCompletion.Choices[0].Message.Content

	slog.Debug("Response from API", "tokens_used", chatCompletion.Usage.TotalTokens, "reply", reply)
	return formatResponse(reply), nil
}

// reasoningEffortFor maps the stored effort level onto OpenAI's reasoning_effort
// parameter. OpenAI only accepts a coarser scale than what the catalogue offers,
// so unsupported levels are clamped; an empty string means "omit the parameter".
func reasoningEffortFor(effort string) shared.ReasoningEffort {
	// TODO: implement the agreed mapping:
	//   "low"   -> shared.ReasoningEffortLow
	//   "medium"-> shared.ReasoningEffortMedium
	//   "high"  -> shared.ReasoningEffortHigh
	//   "xhigh" -> high (clamped)
	//   "max"   -> high (clamped)
	//   "none"/""/unknown -> omit
	return ""
}

// formatResponse renders a provider's raw markdown reply into sanitised HTML.
// AI responses most often come back as markdown (headers, lists, emojis); the
// HTML is consumed via {{ safeHTML .Content }}, so it must be sanitised here.
func formatResponse(reply string) string {
	return sanitise(parseMarkdownToHTML([]byte(reply)))
}

func parseMarkdownToHTML(text []byte) string {
	extensions := parser.CommonExtensions | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(text)

	htmlFlags := html.CommonFlags | html.HrefTargetBlank | html.SkipHTML | html.SkipImages
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)
	renderedText := markdown.Render(doc, renderer)

	return string(renderedText)
}

func sanitise(text string) string {
	parser := bluemonday.UGCPolicy()
	return parser.Sanitize(text)
}
