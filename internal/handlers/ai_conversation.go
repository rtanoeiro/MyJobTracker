package handlers

import (
	"encoding/json"
	"fmt"
	"job-applications/internal/ai"
	"job-applications/internal/db"
	"strings"
	"time"
)

type CVAIForm struct {
	Profile        cvProfile
	Experiences    []cvExperience
	Education      string
	Certifications string
	Academic       string
	Skills         string
}

// appendConversation returns history with a single {role, content} message
// appended. Roles must be lowercase ("user"/"assistant") because
// ai-chat-thread.html maps them to CSS classes and data-role attributes, and the
// DOM re-serialization on the next request reads them back verbatim.
func appendConversation(history []AIConversation, msg, role string) []AIConversation {
	newMessage := AIConversation{
		Role:    role,
		Content: msg,
		Time:    time.Now(),
	}
	history = append(history, newMessage)
	return history
}

// unmarshalTranscript decodes the "transcript" form value that the chat page
// resubmits as JSON: [{"role":"user","content":"..."}, ...]. An empty or
// absent transcript yields an empty thread, not an error.
func unmarshalTranscript(transcript string) ([]AIConversation, error) {
	if transcript == "" {
		return []AIConversation{}, nil
	}
	var history []AIConversation
	if err := json.Unmarshal([]byte(transcript), &history); err != nil {
		return nil, err
	}
	return history, nil
}

// marshalTranscript is the inverse of unmarshalTranscript: it serializes a
// conversation to the compact JSON shape echoed back in the hidden
// #chat-transcript field. Time is omitted so each request round-trip stays
// byte-stable with what the server originally received.
func marshalTranscript(history []AIConversation) (string, error) {
	type transcriptMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	messages := make([]transcriptMessage, 0, len(history))
	for _, turn := range history {
		messages = append(messages, transcriptMessage{Role: turn.Role, Content: turn.Content})
	}
	encoded, err := json.Marshal(messages)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// toAIMessages assembles what the model sees: the system instructions, the CV as
// a leading user context turn, then every conversation turn in order. Roles that
// are neither user nor assistant nor system are dropped.
func toAIMessages(systemPrompt, resumeText string, conversation []AIConversation) []ai.Message {
	messages := make([]ai.Message, 0, len(conversation)+2)
	messages = append(messages, ai.Message{Role: "system", Content: systemPrompt})
	if resumeText != "" {
		messages = append(messages, ai.Message{Role: "user", Content: resumeText})
	}
	for _, turn := range conversation {
		switch turn.Role {
		case "user", "assistant", "system":
			messages = append(messages, ai.Message{Role: turn.Role, Content: turn.Content})
		}
	}
	return messages
}

// cvToText renders a stored CV as plain text so it can be sent to the model as
// context. The exact formatting is a product decision; see cv_mapper.go for the
// typed fields available on db.GetCVByIDRow (profile, experiences, education,
// certifications, academic contributions, Skills).
func cvToText(cv db.GetCVByIDRow) (string, error) {
	form := CVAIForm{}
	decode := func(raw []byte, into any, section string) error {
		if err := json.Unmarshal(raw, into); err != nil {
			return fmt.Errorf("decode %s: %w", section, err)
		}
		return nil
	}

	if err := decode(cv.Profile, &form.Profile, "profile"); err != nil {
		return "", err
	}
	if err := decode(cv.Experiences, &form.Experiences, "experiences"); err != nil {
		return "", err
	}

	form.Education = getTextFromPgTypeText(cv.Education)
	form.Certifications = getTextFromPgTypeText(cv.Certifications)
	form.Academic = getTextFromPgTypeText(cv.AcademicContributions)
	form.Skills = getTextFromPgTypeText(cv.Skills)

	var text strings.Builder
	fmt.Fprintf(&text, "----- Name ----- : %s\n", form.Profile.Name)
	fmt.Fprintf(&text, "----- Location ----- : %s\n", form.Profile.Location)

	if len(form.Experiences) > 0 {
		text.WriteString("\n----- Experience -----\n")
		for index, experience := range form.Experiences {
			fmt.Fprintf(&text, "----- Experience %d -----\n", index)
			fmt.Fprintf(&text, "   - Company: %s\n", experience.Company)
			fmt.Fprintf(&text, "   - Role/Field: %s\n", experience.Field)
			if experience.Start != "" {
				fmt.Fprintf(&text, "   - Start Date: %s\n", experience.Start)
			}
			if experience.End != "" || experience.Current {
				endDate := experience.End
				if experience.Current {
					endDate = "Current"
				}
				fmt.Fprintf(&text, "   - End Date: %s\n", endDate)
			}
			for _, bullet := range experience.Bullets {
				if strings.TrimSpace(bullet) != "" {
					fmt.Fprintf(&text, "       - %s\n", bullet)
				}
			}
		}
	}

	writeAICVListSection(&text, "Education", form.Education)
	writeAICVListSection(&text, "Certifications", form.Certifications)
	writeAICVListSection(&text, "Academic Contributions", form.Academic)
	writeAICVListSection(&text, "Skills", form.Skills)

	return text.String(), nil
}

// writeAICVListSection appends a titled semicolon-separated section to the AI
// resume text, one bullet per non-blank item. The title is skipped when the
// section is empty so the model only sees populated sections.
func writeAICVListSection(text *strings.Builder, title string, block string) {
	items := make([]string, 0)
	for _, item := range strings.Split(block, ";") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return
	}

	fmt.Fprintf(text, "\n----- %s -----\n", title)
	for _, item := range items {
		fmt.Fprintf(text, "   - %s\n", item)
	}
}
