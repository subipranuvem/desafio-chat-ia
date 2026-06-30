package llm

import (
	"testing"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

func TestNewOpenAIClient(t *testing.T) {
	t.Run("empty api key returns error", func(t *testing.T) {
		_, err := NewOpenAIClient("", "https://api.openai.com/v1", "gpt-4o")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("empty model id returns error", func(t *testing.T) {
		_, err := NewOpenAIClient("sk-test", "https://api.openai.com/v1", "")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("empty base url is allowed", func(t *testing.T) {
		c, err := NewOpenAIClient("sk-test", "", "gpt-4o")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c == nil {
			t.Fatal("expected client, got nil")
		}
	})
}

func TestBuildOpenAIMessages(t *testing.T) {
	t.Run("maps user role", func(t *testing.T) {
		chat := model.Chat{Messages: []model.Message{
			{Role: model.RoleUser, Content: "hello"},
		}}
		msgs := buildOpenAIMessages(chat)
		if len(msgs) != 1 {
			t.Fatalf("expected 1 message, got %d", len(msgs))
		}
	})

	t.Run("maps assistant role", func(t *testing.T) {
		chat := model.Chat{Messages: []model.Message{
			{Role: model.RoleAssistant, Content: "hi"},
		}}
		msgs := buildOpenAIMessages(chat)
		if len(msgs) != 1 {
			t.Fatalf("expected 1 message, got %d", len(msgs))
		}
	})

	t.Run("maps system role", func(t *testing.T) {
		chat := model.Chat{Messages: []model.Message{
			{Role: model.RoleSystem, Content: "you are a helpful assistant"},
		}}
		msgs := buildOpenAIMessages(chat)
		if len(msgs) != 1 {
			t.Fatalf("expected 1 message, got %d", len(msgs))
		}
	})

	t.Run("preserves message order", func(t *testing.T) {
		chat := model.Chat{Messages: []model.Message{
			{Role: model.RoleSystem, Content: "system"},
			{Role: model.RoleUser, Content: "first"},
			{Role: model.RoleAssistant, Content: "second"},
			{Role: model.RoleUser, Content: "third"},
		}}
		msgs := buildOpenAIMessages(chat)
		if len(msgs) != 4 {
			t.Fatalf("expected 4 messages, got %d", len(msgs))
		}
	})

	t.Run("empty chat returns empty slice", func(t *testing.T) {
		msgs := buildOpenAIMessages(model.Chat{})
		if len(msgs) != 0 {
			t.Fatalf("expected 0 messages, got %d", len(msgs))
		}
	})
}
