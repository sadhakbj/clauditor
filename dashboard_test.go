package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetSessionMessages_Codex(t *testing.T) {
	oldProjectsDir, oldCodexDir := projectsDir, codexDir
	t.Cleanup(func() {
		projectsDir = oldProjectsDir
		codexDir = oldCodexDir
	})

	tmpDir := t.TempDir()
	projectsDir = filepath.Join(tmpDir, "projects")
	codexDir = filepath.Join(tmpDir, "sessions")

	sessDir := filepath.Join(codexDir, "2026", "04", "24")
	if err := os.MkdirAll(sessDir, 0750); err != nil {
		t.Fatal(err)
	}

	sessionID := "codex-msgs-1"
	content := `{"timestamp":"2026-04-24T13:52:04.867Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"<environment_context>\n  <cwd>/tmp/repo</cwd>\n</environment_context>"}]}}
{"timestamp":"2026-04-24T13:52:16.749Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello codex"}]}}
{"timestamp":"2026-04-24T13:52:18.277Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello."}],"phase":"final_answer"}}
`
	filePath := filepath.Join(sessDir, "rollout-2026-04-24T13-52-04-"+sessionID+".jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	msgs, err := getSessionMessages(sessionID)
	if err != nil {
		t.Fatalf("getSessionMessages error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("want 2 user messages, got %d", len(msgs))
	}
	if msgs[0].Content != "<environment_context>\n  <cwd>/tmp/repo</cwd>\n</environment_context>" {
		t.Errorf("first message = %q", msgs[0].Content)
	}
	if msgs[1].Content != "hello codex" {
		t.Errorf("second message = %q, want %q", msgs[1].Content, "hello codex")
	}
}

func TestGetSessionConversation_Codex(t *testing.T) {
	oldProjectsDir, oldCodexDir := projectsDir, codexDir
	t.Cleanup(func() {
		projectsDir = oldProjectsDir
		codexDir = oldCodexDir
	})

	tmpDir := t.TempDir()
	projectsDir = filepath.Join(tmpDir, "projects")
	codexDir = filepath.Join(tmpDir, "sessions")

	sessDir := filepath.Join(codexDir, "2026", "04", "24")
	if err := os.MkdirAll(sessDir, 0750); err != nil {
		t.Fatal(err)
	}

	sessionID := "codex-convo-1"
	content := `{"timestamp":"2026-04-24T13:52:16.749Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"hello codex"}]}}
{"timestamp":"2026-04-24T13:52:18.277Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Hello."}],"phase":"final_answer"}}
`
	filePath := filepath.Join(sessDir, "rollout-2026-04-24T13-52-16-"+sessionID+".jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	entries, err := getSessionConversation(sessionID)
	if err != nil {
		t.Fatalf("getSessionConversation error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("want 2 entries, got %d", len(entries))
	}
	if entries[0].Role != "user" || entries[0].Content != "hello codex" {
		t.Errorf("entry[0] = %#v", entries[0])
	}
	if entries[1].Role != "assistant" || entries[1].Content != "Hello." {
		t.Errorf("entry[1] = %#v", entries[1])
	}
}

func TestGetSessionMessages_CodexEventUserMessage(t *testing.T) {
	oldProjectsDir, oldCodexDir := projectsDir, codexDir
	t.Cleanup(func() {
		projectsDir = oldProjectsDir
		codexDir = oldCodexDir
	})

	tmpDir := t.TempDir()
	projectsDir = filepath.Join(tmpDir, "projects")
	codexDir = filepath.Join(tmpDir, "sessions")

	sessDir := filepath.Join(codexDir, "2026", "04", "24")
	if err := os.MkdirAll(sessDir, 0750); err != nil {
		t.Fatal(err)
	}

	sessionID := "codex-event-msgs-1"
	content := `{"timestamp":"2026-04-24T13:58:00.000Z","type":"event_msg","payload":{"type":"user_message","message":"first prompt"}}
{"timestamp":"2026-04-24T13:59:00.000Z","type":"event_msg","payload":{"type":"token_count","message":""}}
{"timestamp":"2026-04-24T14:00:00.000Z","type":"event_msg","payload":{"type":"user_message","message":"second prompt"}}
`
	filePath := filepath.Join(sessDir, "rollout-2026-04-24T13-58-00-"+sessionID+".jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	msgs, err := getSessionMessages(sessionID)
	if err != nil {
		t.Fatalf("getSessionMessages error: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("want 2 user messages, got %d", len(msgs))
	}
	if msgs[0].Content != "first prompt" || msgs[1].Content != "second prompt" {
		t.Fatalf("unexpected messages: %#v", msgs)
	}
}

func TestGetSessionConversation_CodexEventUserMessage(t *testing.T) {
	oldProjectsDir, oldCodexDir := projectsDir, codexDir
	t.Cleanup(func() {
		projectsDir = oldProjectsDir
		codexDir = oldCodexDir
	})

	tmpDir := t.TempDir()
	projectsDir = filepath.Join(tmpDir, "projects")
	codexDir = filepath.Join(tmpDir, "sessions")

	sessDir := filepath.Join(codexDir, "2026", "04", "24")
	if err := os.MkdirAll(sessDir, 0750); err != nil {
		t.Fatal(err)
	}

	sessionID := "codex-event-convo-1"
	content := `{"timestamp":"2026-04-24T13:58:00.000Z","type":"event_msg","payload":{"type":"user_message","message":"first prompt"}}
{"timestamp":"2026-04-24T13:58:01.000Z","type":"response_item","payload":{"type":"message","role":"assistant","content":[{"type":"output_text","text":"reply"}],"phase":"final_answer"}}
{"timestamp":"2026-04-24T14:00:00.000Z","type":"event_msg","payload":{"type":"user_message","message":"second prompt"}}
`
	filePath := filepath.Join(sessDir, "rollout-2026-04-24T13-58-00-"+sessionID+".jsonl")
	if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	entries, err := getSessionConversation(sessionID)
	if err != nil {
		t.Fatalf("getSessionConversation error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("want 3 entries, got %d", len(entries))
	}
	if entries[0].Role != "user" || entries[0].Content != "first prompt" {
		t.Errorf("entry[0] = %#v", entries[0])
	}
	if entries[2].Role != "user" || entries[2].Content != "second prompt" {
		t.Errorf("entry[2] = %#v", entries[2])
	}
}

func TestAssistantTexts_SingleAssistantMessageBecomesResponse(t *testing.T) {
	preAction, response := assistantTexts("Yes, I work.", "Yes, I work.", 0)
	if preAction != "" {
		t.Errorf("preAction = %q, want empty", preAction)
	}
	if response != "Yes, I work." {
		t.Errorf("response = %q, want %q", response, "Yes, I work.")
	}
}

func TestAssistantTexts_ThinkingKeepsPreActionSeparate(t *testing.T) {
	preAction, response := assistantTexts("Planning...", "Planning...", 120)
	if preAction != "Planning..." {
		t.Errorf("preAction = %q, want %q", preAction, "Planning...")
	}
	if response != "" {
		t.Errorf("response = %q, want empty", response)
	}
}

func TestShouldReplaceDisplayMessage_PrefersNewerRealPrompt(t *testing.T) {
	current := &userMessage{TimestampTs: 200, Content: "actual prompt"}
	candidate := userMessage{TimestampTs: 100, Content: "older prompt"}
	if shouldReplaceDisplayMessage(current, candidate) {
		t.Fatal("should not replace newer real prompt with older candidate")
	}
}

func TestShouldReplaceDisplayMessage_ReplacesSyntheticPrompt(t *testing.T) {
	current := &userMessage{TimestampTs: 200, Content: "<task-notification>synthetic</task-notification>"}
	candidate := userMessage{TimestampTs: 100, Content: "real prompt"}
	if !shouldReplaceDisplayMessage(current, candidate) {
		t.Fatal("should replace synthetic prompt")
	}
}
