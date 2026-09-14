package templates

import (
	"strings"
	"testing"
)

type threadMessage struct {
	Role    string
	Content string
}

// The ai-chat thread template only renders assistant replies as trusted HTML;
// user content must stay autoescaped, even if it contains markup.
func TestAIChatThread_EscapesUserAndTrustsAssistant(t *testing.T) {
	r := New("../../templates/*.html")
	if r.templates == nil {
		t.Fatal("failed to parse production templates")
	}

	data := []threadMessage{
		{Role: "user", Content: `<script>alert(1)</script> job & description`},
		{Role: "assistant", Content: `<h1>Ramon</h1><p>Location: Melbourne</p>`},
	}

	var buf strings.Builder
	if err := r.Render(&buf, "ai-chat-thread", data); err != nil {
		t.Fatalf("failed to render ai-chat-thread: %v", err)
	}
	body := buf.String()

	if !strings.Contains(body, `<h1>Ramon</h1><p>Location: Melbourne</p>`) {
		t.Errorf("assistant HTML was not emitted unescaped, got %q", body)
	}
	if strings.Contains(body, `&lt;script&gt;`) == false {
		t.Errorf("user markup was not escaped, got %q", body)
	}
	if strings.Contains(body, `chat-msg-html`) == false {
		t.Errorf("expected rendered assistant bubble to use chat-msg-html class, got %q", body)
	}
}

// The Clear Thread flow swaps the ai-chat-thread fragment with an empty
// conversation; that branch must render the "no conversation" placeholder
// rather than nothing.
func TestAIChatThread_EmptyRendersEmptyPlaceholder(t *testing.T) {
	r := New("../../templates/*.html")
	if r.templates == nil {
		t.Fatal("failed to parse production templates")
	}

	var buf strings.Builder
	if err := r.Render(&buf, "ai-chat-thread", []threadMessage{}); err != nil {
		t.Fatalf("failed to render empty ai-chat-thread: %v", err)
	}
	body := buf.String()

	if !strings.Contains(body, "chat-empty") {
		t.Errorf("expected empty conversation to render the chat-empty placeholder, got %q", body)
	}
	if !strings.Contains(body, "No conversation yet.") {
		t.Errorf("expected empty conversation to render the empty-state message, got %q", body)
	}
}
