package llm

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

func TestNewOpenAIClient(t *testing.T) {
	t.Run("empty api key returns error", func(t *testing.T) {
		_, err := NewOpenAIClient("", "https://api.openai.com/v1", "gpt-4o")
		require.Error(t, err)
	})

	t.Run("empty model id returns error", func(t *testing.T) {
		_, err := NewOpenAIClient("sk-test", "https://api.openai.com/v1", "")
		require.Error(t, err)
	})

	t.Run("empty base url is allowed", func(t *testing.T) {
		c, err := NewOpenAIClient("sk-test", "", "gpt-4o")
		require.NoError(t, err)
		require.NotNil(t, c)
	})
}

func TestBuildOpenAIMessages(t *testing.T) {
	t.Run("maps user role", func(t *testing.T) {
		chat := model.Chat{Messages: []model.Message{
			{Role: model.RoleUser, Content: "hello"},
		}}
		require.Len(t, buildOpenAIMessages(chat), 1)
	})

	t.Run("maps assistant role", func(t *testing.T) {
		chat := model.Chat{Messages: []model.Message{
			{Role: model.RoleAssistant, Content: "hi"},
		}}
		require.Len(t, buildOpenAIMessages(chat), 1)
	})

	t.Run("maps system role", func(t *testing.T) {
		chat := model.Chat{Messages: []model.Message{
			{Role: model.RoleSystem, Content: "you are a helpful assistant"},
		}}
		require.Len(t, buildOpenAIMessages(chat), 1)
	})

	t.Run("preserves message order", func(t *testing.T) {
		chat := model.Chat{Messages: []model.Message{
			{Role: model.RoleSystem, Content: "system"},
			{Role: model.RoleUser, Content: "first"},
			{Role: model.RoleAssistant, Content: "second"},
			{Role: model.RoleUser, Content: "third"},
		}}
		require.Len(t, buildOpenAIMessages(chat), 4)
	})

	t.Run("empty chat returns empty slice", func(t *testing.T) {
		require.Empty(t, buildOpenAIMessages(model.Chat{}))
	})
}
