package llm

import (
	"context"
	"testing"

	"google.golang.org/genai"

	"github.com/stretchr/testify/require"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

func TestNewGeminiClient(t *testing.T) {
	t.Run("empty api key returns error", func(t *testing.T) {
		_, err := NewGeminiClient(context.Background(), "", "gemini-2.5-flash")
		require.Error(t, err)
	})

	t.Run("empty model id returns error", func(t *testing.T) {
		_, err := NewGeminiClient(context.Background(), "fake-key", "")
		require.Error(t, err)
	})
}

func TestBuildGeminiContents(t *testing.T) {
	t.Run("system message extracted as sysPrompt", func(t *testing.T) {
		chat := model.Chat{Messages: []model.Message{
			{Role: model.RoleSystem, Content: "you are helpful"},
		}}
		contents, sysPrompt := buildGeminiContents(chat)
		require.Equal(t, "you are helpful", sysPrompt)
		require.Empty(t, contents)
	})

	t.Run("user message mapped with user role", func(t *testing.T) {
		chat := model.Chat{Messages: []model.Message{
			{Role: model.RoleUser, Content: "hello"},
		}}
		contents, _ := buildGeminiContents(chat)
		require.Len(t, contents, 1)
		require.Equal(t, string(genai.RoleUser), contents[0].Role)
		require.Equal(t, "hello", contents[0].Parts[0].Text)
	})

	t.Run("assistant message mapped with model role", func(t *testing.T) {
		chat := model.Chat{Messages: []model.Message{
			{Role: model.RoleAssistant, Content: "hi there"},
		}}
		contents, _ := buildGeminiContents(chat)
		require.Len(t, contents, 1)
		require.Equal(t, string(genai.RoleModel), contents[0].Role)
	})

	t.Run("system message does not appear in contents", func(t *testing.T) {
		chat := model.Chat{Messages: []model.Message{
			{Role: model.RoleSystem, Content: "sys"},
			{Role: model.RoleUser, Content: "msg"},
		}}
		contents, _ := buildGeminiContents(chat)
		require.Len(t, contents, 1)
	})

	t.Run("preserves message order", func(t *testing.T) {
		chat := model.Chat{Messages: []model.Message{
			{Role: model.RoleUser, Content: "first"},
			{Role: model.RoleAssistant, Content: "second"},
			{Role: model.RoleUser, Content: "third"},
		}}
		contents, _ := buildGeminiContents(chat)
		require.Len(t, contents, 3)
		require.Equal(t, "first", contents[0].Parts[0].Text)
		require.Equal(t, "third", contents[2].Parts[0].Text)
	})
}
