package llm

import (
	"context"
	"testing"

	"google.golang.org/genai"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

func TestNewGeminiClient(t *testing.T) {
	t.Run("empty api key returns error", func(t *testing.T) {
		_, err := NewGeminiClient(context.Background(), "", "gemini-2.5-flash")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("empty model id returns error", func(t *testing.T) {
		_, err := NewGeminiClient(context.Background(), "fake-key", "")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestBuildGeminiContents(t *testing.T) {
	t.Run("system message extracted as sysPrompt", func(t *testing.T) {
		chat := model.Chat{Messages: []model.Message{
			{Role: model.RoleSystem, Content: "you are helpful"},
		}}
		contents, sysPrompt := buildGeminiContents(chat)
		if sysPrompt != "you are helpful" {
			t.Fatalf("expected sysPrompt %q, got %q", "you are helpful", sysPrompt)
		}
		if len(contents) != 0 {
			t.Fatalf("expected 0 contents, got %d", len(contents))
		}
	})

	t.Run("user message mapped with user role", func(t *testing.T) {
		chat := model.Chat{Messages: []model.Message{
			{Role: model.RoleUser, Content: "hello"},
		}}
		contents, _ := buildGeminiContents(chat)
		if len(contents) != 1 {
			t.Fatalf("expected 1 content, got %d", len(contents))
		}
		if contents[0].Role != string(genai.RoleUser) {
			t.Fatalf("expected role %q, got %q", genai.RoleUser, contents[0].Role)
		}
		if contents[0].Parts[0].Text != "hello" {
			t.Fatalf("expected text %q, got %q", "hello", contents[0].Parts[0].Text)
		}
	})

	t.Run("assistant message mapped with model role", func(t *testing.T) {
		chat := model.Chat{Messages: []model.Message{
			{Role: model.RoleAssistant, Content: "hi there"},
		}}
		contents, _ := buildGeminiContents(chat)
		if len(contents) != 1 {
			t.Fatalf("expected 1 content, got %d", len(contents))
		}
		if contents[0].Role != string(genai.RoleModel) {
			t.Fatalf("expected role %q, got %q", genai.RoleModel, contents[0].Role)
		}
	})

	t.Run("system message does not appear in contents", func(t *testing.T) {
		chat := model.Chat{Messages: []model.Message{
			{Role: model.RoleSystem, Content: "sys"},
			{Role: model.RoleUser, Content: "msg"},
		}}
		contents, _ := buildGeminiContents(chat)
		if len(contents) != 1 {
			t.Fatalf("expected 1 content (system excluded), got %d", len(contents))
		}
	})

	t.Run("preserves message order", func(t *testing.T) {
		chat := model.Chat{Messages: []model.Message{
			{Role: model.RoleUser, Content: "first"},
			{Role: model.RoleAssistant, Content: "second"},
			{Role: model.RoleUser, Content: "third"},
		}}
		contents, _ := buildGeminiContents(chat)
		if len(contents) != 3 {
			t.Fatalf("expected 3 contents, got %d", len(contents))
		}
		if contents[0].Parts[0].Text != "first" {
			t.Fatalf("expected %q at index 0, got %q", "first", contents[0].Parts[0].Text)
		}
		if contents[2].Parts[0].Text != "third" {
			t.Fatalf("expected %q at index 2, got %q", "third", contents[2].Parts[0].Text)
		}
	})
}
