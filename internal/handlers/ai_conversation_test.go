package handlers

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// The transcript JSON echoed into the hidden #chat-transcript field must round
// trip through unmarshalTranscript without loss or drift, so each request
// stays byte-stable with what the server originally produced.
func TestMarshalTranscriptRoundTripsThroughUnmarshal(t *testing.T) {
	history := []AIConversation{
		{Role: "user", Content: "Paste a job description here & <stuff>", Time: time.Now()},
		{Role: "assistant", Content: `<h1>Review</h1><p>Thanks</p>`},
		{Role: "user", Content: "and a follow-up"},
	}

	encoded, err := marshalTranscript(history)
	if err != nil {
		t.Fatalf("marshalTranscript failed: %v", err)
	}

	decoded, err := unmarshalTranscript(encoded)
	if err != nil {
		t.Fatalf("unmarshalTranscript(%q) failed: %v", encoded, err)
	}

	if len(decoded) != len(history) {
		t.Fatalf("round trip changed message count: got %d, want %d", len(decoded), len(history))
	}
	for i, turn := range decoded {
		if turn.Role != history[i].Role || turn.Content != history[i].Content {
			t.Errorf("message %d drifted: got %+v, want role=%q content=%q", i, turn, history[i].Role, history[i].Content)
		}
		if !turn.Time.IsZero() {
			t.Errorf("message %d carried a Time value %v, want zero so the echo stays stable", i, turn.Time)
		}
	}
}

// The emitted shape must stay exactly the compact [{role,content}] form, not
// accidentally grow extra keys that the template/echo relies on.
func TestMarshalTranscriptEmitsOnlyRoleAndContent(t *testing.T) {
	encoded, err := marshalTranscript([]AIConversation{{Role: "user", Content: "hi"}})
	if err != nil {
		t.Fatalf("marshalTranscript failed: %v", err)
	}

	var raw []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(encoded), &raw); err != nil {
		t.Fatalf("encoded output %q is not valid JSON: %v", encoded, err)
	}
	if len(raw) != 1 {
		t.Fatalf("expected one message, got %d", len(raw))
	}
	if len(raw[0]) != 2 {
		t.Errorf("message has %d keys, want exactly role and content: %v", len(raw[0]), raw[0])
	}
}

func TestMarshalTranscriptEmptyHistory(t *testing.T) {
	encoded, err := marshalTranscript(nil)
	if err != nil {
		t.Fatalf("marshalTranscript(nil) failed: %v", err)
	}
	if !strings.HasPrefix(encoded, "[") {
		t.Errorf("expected an empty JSON array, got %q", encoded)
	}
	decoded, err := unmarshalTranscript(encoded)
	if err != nil {
		t.Fatalf("unmarshalTranscript(%q) failed: %v", encoded, err)
	}
	if len(decoded) != 0 {
		t.Errorf("expected empty history, got %d messages", len(decoded))
	}
}

func TestUnmarshalTranscriptEmptyInput(t *testing.T) {
	history, err := unmarshalTranscript("")
	if err != nil {
		t.Fatalf("unmarshalTranscript(\"\") failed: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected empty history for empty transcript, got %d messages", len(history))
	}
}

func TestUnmarshalTranscriptInvalidJSON(t *testing.T) {
	if _, err := unmarshalTranscript(`[{not valid`); err == nil {
		t.Error("expected error for malformed transcript JSON")
	}
}

func TestAppendConversation(t *testing.T) {
	history := []AIConversation{
		{Role: "user", Content: "first"},
		{Role: "assistant", Content: "reply"},
	}

	got := appendConversation(history, "follow-up", "user")

	if len(got) != 3 {
		t.Fatalf("expected 3 messages after append, got %d", len(got))
	}
	last := got[len(got)-1]
	if last.Role != "user" || last.Content != "follow-up" {
		t.Errorf("appended message = %+v, want role=user content=follow-up", last)
	}
	if last.Time.IsZero() {
		t.Error("appended message should carry a Time so the bubble renders in order")
	}
	if got[0] != history[0] || got[1] != history[1] {
		t.Error("appendConversation must not mutate or reorder existing messages")
	}
}

func TestAppendConversationToNilHistory(t *testing.T) {
	got := appendConversation(nil, "first message", "user")
	if len(got) != 1 || got[0].Role != "user" || got[0].Content != "first message" {
		t.Fatalf("append to nil = %+v, want a single user message", got)
	}
}

func TestToAIMessages_OrderingAndRoleFiltering(t *testing.T) {
	conversation := []AIConversation{
		{Role: "user", Content: "job description"},
		{Role: "assistant", Content: "review"},
		{Role: "tool", Content: "must be dropped"},
	}

	messages := toAIMessages("system prompt", "resume text", conversation)

	wantRoles := []string{"system", "user", "user", "assistant"}
	if len(messages) != len(wantRoles) {
		t.Fatalf("got %d messages, want %d: %+v", len(messages), len(wantRoles), messages)
	}
	for i, want := range wantRoles {
		if messages[i].Role != want {
			t.Errorf("message %d role = %q, want %q", i, messages[i].Role, want)
		}
	}
	if messages[1].Content != "resume text" {
		t.Errorf("resume context = %q, want resume text", messages[1].Content)
	}
	for _, turn := range messages {
		if turn.Role == "tool" {
			t.Errorf("unexpected role survived filtering: %+v", turn)
		}
	}
}

func TestToAIMessages_OmitsResumeWhenEmpty(t *testing.T) {
	messages := toAIMessages("system prompt", "", []AIConversation{{Role: "user", Content: "hi"}})

	if len(messages) != 2 {
		t.Fatalf("got %d messages, want 2 (system + user): %+v", len(messages), messages)
	}
	if messages[0].Role != "system" || messages[1].Role != "user" {
		t.Errorf("unexpected roles: %+v", messages)
	}
}

func TestToAIMessages_KeepsSystemConversationTurns(t *testing.T) {
	conversation := []AIConversation{{Role: "system", Content: "override"}}
	messages := toAIMessages("system prompt", "resume", conversation)
	if len(messages) != 3 {
		t.Fatalf("got %d messages, want 3: %+v", len(messages), messages)
	}
	if messages[2].Content != "override" {
		t.Errorf("system turn content = %q, want override", messages[2].Content)
	}
}

func TestCVToText_FullRendering(t *testing.T) {
	text, err := cvToText(cvRow())
	if err != nil {
		t.Fatalf("cvToText: %v", err)
	}

	for _, want := range []string{
		"----- Name ----- : Jane",
		"----- Location ----- : London",
		"----- Experience -----",
		"   - Company: ACME",
		"   - Role/Field: Engineer",
		"   - Start Date: 2021-03",
		"   - End Date: 2022-06",
		"       - won big",
		"       - helped",
		"----- Education -----",
		"   - UCL, CS, 2015",
		"----- Certifications -----",
		"   - AWS, Amazon",
		"----- Academic Contributions -----",
		"   - a paper",
		"----- Skills -----",
		"   - volunteer",
		"   - languages",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("cvToText output missing %q:\n%s", want, text)
		}
	}
}

func TestCVToText_CurrentRoleRendersCurrent(t *testing.T) {
	row := cvRow()
	row.Experiences = []byte(`[{"company":"ACME","field":"Eng","start":"2020-01","current":true,"bullets":["still there"]}]`)

	text, err := cvToText(row)
	if err != nil {
		t.Fatalf("cvToText: %v", err)
	}

	if !strings.Contains(text, "   - End Date: Current") {
		t.Errorf("current role should render \"End Date: Current\":\n%s", text)
	}
	if strings.Contains(text, "   - End Date: \n") {
		t.Errorf("empty end date leaked into output:\n%s", text)
	}
}

func TestCVToText_EmptySectionsOmitHeaders(t *testing.T) {
	row := cvRow()
	row.Experiences = []byte(`[]`)
	row.Education = pgtype.Text{}
	row.Certifications = pgtype.Text{}
	row.AcademicContributions = pgtype.Text{}
	row.Skills = pgtype.Text{}

	text, err := cvToText(row)
	if err != nil {
		t.Fatalf("cvToText: %v", err)
	}

	for _, header := range []string{"Experience", "Education", "Certifications", "Academic Contributions", "Skills"} {
		if strings.Contains(text, "----- "+header) {
			t.Errorf("empty %s section rendered a header:\n%s", header, text)
		}
	}
}

func TestCVToText_SkipsBlankListValues(t *testing.T) {
	row := cvRow()
	row.AcademicContributions = pgtype.Text{String: "a paper;   ;", Valid: true}
	row.Skills = pgtype.Text{String: "  languages  ;   ", Valid: true}

	text, err := cvToText(row)
	if err != nil {
		t.Fatalf("cvToText: %v", err)
	}

	if !strings.Contains(text, "   - a paper") {
		t.Errorf("academic contributions missing from output:\n%s", text)
	}
	if !strings.Contains(text, "   - languages") {
		t.Errorf("skill value should be trimmed and rendered:\n%s", text)
	}
	// Blank entries must not render as bare empty bullets.
	if strings.Contains(text, "   - \n") {
		t.Errorf("blank list value rendered an empty bullet:\n%s", text)
	}
}

func TestCVToText_MalformedSectionReturnsError(t *testing.T) {
	row := cvRow()
	row.Profile = []byte(`{"name":`)

	if _, err := cvToText(row); err == nil {
		t.Error("expected error for malformed profile JSON")
	} else if !strings.Contains(err.Error(), "profile") {
		t.Errorf("error should name the failing section, got %q", err)
	}

	row = cvRow()
	row.Experiences = []byte(`[{"company":`)
	if _, err := cvToText(row); err == nil {
		t.Error("expected error for malformed experiences JSON")
	} else if !strings.Contains(err.Error(), "experiences") {
		t.Errorf("error should name the failing section, got %q", err)
	}
}

func TestWriteAICVListSection_EmptyIsNoOp(t *testing.T) {
	var text strings.Builder
	writeAICVListSection(&text, "Skills", "")
	if text.String() != "" {
		t.Errorf("empty list should write nothing, got %q", text.String())
	}
}

func TestWriteAICVListSection_SkipsBlankValues(t *testing.T) {
	block := "first;  ; last"

	var text strings.Builder
	writeAICVListSection(&text, "Academic Contributions", block)

	got := text.String()
	if !strings.Contains(got, "----- Academic Contributions -----") {
		t.Errorf("missing title in output:\n%s", got)
	}
	if !strings.Contains(got, "   - first") || !strings.Contains(got, "   - last") {
		t.Errorf("populated values missing from output:\n%s", got)
	}
	if strings.Count(got, "   - ") != 2 {
		t.Errorf("blank values should be skipped, got:\n%s", got)
	}
}
