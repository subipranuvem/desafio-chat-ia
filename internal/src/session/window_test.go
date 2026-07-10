package session_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/session"
)

func msgOfTokens(tokens int) model.Message {
	return model.Message{Role: model.RoleUser, Content: strings.Repeat("a", tokens*4)}
}

func TestBuildWindow(t *testing.T) {
	t.Run("empty input returns empty", func(t *testing.T) {
		got := session.BuildWindow(nil, 8000)
		require.Empty(t, got)
	})

	t.Run("all messages fit returns all", func(t *testing.T) {
		msgs := []model.Message{
			msgOfTokens(10),
			msgOfTokens(5),
		}
		got := session.BuildWindow(msgs, 8000)
		require.Equal(t, msgs, got)
	})

	t.Run("budget exactly met returns all", func(t *testing.T) {
		msgs := []model.Message{
			msgOfTokens(2000),
			msgOfTokens(2000),
		}
		got := session.BuildWindow(msgs, 4000)
		require.Equal(t, msgs, got)
	})

	t.Run("trims oldest messages when budget exceeded", func(t *testing.T) {
		msgs := []model.Message{
			msgOfTokens(100), // oldest — dropped
			msgOfTokens(100), // middle — kept
			msgOfTokens(100), // newest — kept first
		}
		got := session.BuildWindow(msgs, 250)
		require.Len(t, got, 2)
		require.Equal(t, msgs[1], got[0])
		require.Equal(t, msgs[2], got[1])
	})

	t.Run("single message exceeding budget returns empty", func(t *testing.T) {
		msgs := []model.Message{msgOfTokens(200)}
		got := session.BuildWindow(msgs, 100)
		require.Empty(t, got)
	})

	t.Run("large content trims correctly", func(t *testing.T) {
		content := strings.Repeat("a", 400)
		msgs := []model.Message{
			{Role: model.RoleUser, Content: content},
			{Role: model.RoleUser, Content: content},
			{Role: model.RoleUser, Content: content},
		}
		got := session.BuildWindow(msgs, 150)
		require.Len(t, got, 1)
		require.Equal(t, msgs[2], got[0])
	})

	t.Run("keeps chronological order in output", func(t *testing.T) {
		first := model.Message{Role: model.RoleUser, Content: "first"}
		second := model.Message{Role: model.RoleUser, Content: "second"}
		third := model.Message{Role: model.RoleUser, Content: "third"}
		got := session.BuildWindow([]model.Message{first, second, third}, 8000)
		require.Equal(t, "first", got[0].Content)
		require.Equal(t, "second", got[1].Content)
		require.Equal(t, "third", got[2].Content)
	})
}
