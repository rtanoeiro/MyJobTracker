package ai

import "context"

// ChatClient is the provider-agnostic seam the handler talks to. Implementations
// translate the shared []Message transcript into whatever shape their provider
// expects, and return an already-rendered HTML reply.
type ChatClient interface {
	Send(ctx context.Context, baseURL, apiKey string, messages []Message, model, effort string) (string, error)
}

// ChatFactory selects a ChatClient for a provider name. The handler holds one of
// these so provider choice can stay per-user (settings are loaded per request).
type ChatFactory func(providerName string) ChatClient

// NewChatClient returns the native client for providerName. For now it only returns
// the Anthropic Client for Anthropic and OpenAI compatible AI Providers
func NewChatClient(providerName string) ChatClient {
	switch providerName {
	case "Anthropic":
		return AnthropicClient{}
	default:
		return OpenAIClient{}
	}
}
