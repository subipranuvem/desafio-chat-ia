package llm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

type stubClient struct{}

func (s *stubClient) SendMessage(_ context.Context, _ model.Chat, _ func(model.MessageChunk) error) error {
	return nil
}

func TestRegistry_For(t *testing.T) {
	t.Run("returns registered client", func(t *testing.T) {
		r := NewRegistry()
		r.Register("gpt-4o", &stubClient{})

		c, err := r.For("gpt-4o")
		require.NoError(t, err)
		require.NotNil(t, c)
	})

	t.Run("returns error for unknown model", func(t *testing.T) {
		r := NewRegistry()

		_, err := r.For("nonexistent-model")
		require.Error(t, err)
	})

	t.Run("register overwrites existing model", func(t *testing.T) {
		r := NewRegistry()
		first := &stubClient{}
		second := &stubClient{}

		r.Register("model", first)
		r.Register("model", second)

		c, err := r.For("model")
		require.NoError(t, err)
		require.Equal(t, second, c)
	})
}
