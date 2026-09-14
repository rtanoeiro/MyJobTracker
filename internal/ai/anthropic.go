package ai

import (
	"context"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// AnthropicClient talks to the native Anthropic Messages API. It differs from
// the OpenAI-compatible shape in three ways that the Send body must bridge:
//   - the system prompt is a top-level field, not a "system" message
//   - reasoning effort is expressed as a thinking token budget, not a label
//   - replies come back as a slice of content blocks, not a single string
type AnthropicClient struct{}

func (handler AnthropicClient) Send(ctx context.Context, baseURL, apiKey string, messages []Message, model, effort string) (string, error) {
	client := anthropic.NewClient(
		option.WithAPIKey(apiKey),
		option.WithBaseURL(baseURL),
	)
	_ = client // TODO: use once the request below is implemented

	// TODO: split "system" messages out of `messages` into MessageNewParams.System
	// TODO: map remaining "user"/"assistant" messages to anthropic.MessageParam turns
	// TODO: map effort via anthropicEffortFor(effort) into a thinking budget,
	//       and set MaxTokens so it stays above the budget
	_ = anthropicEffortFor(effort) // TODO: wire into the thinking budget
	// TODO: call client.Messages.New(ctx, params)
	// TODO: concatenate the text content blocks of the response
	// TODO: return formatResponse(joined), nil
	return "", nil
}

// anthropicEffortFor translates the stored effort level into a thinking token
// budget for the Messages API.
func anthropicEffortFor(effort string) int64 {
	// TODO: implement the agreed mapping, e.g.
	//   "low"   -> 1024
	//   "medium"-> 4096
	//   "high"  -> 8192
	//   "xhigh" -> 16384
	//   "max"   -> 32000
	//   ""/unknown -> 0 (no thinking)
	return 0
}
