package llm

import (
	"context"
	"testing"

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
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c == nil {
			t.Fatal("expected client, got nil")
		}
	})

	t.Run("returns error for unknown model", func(t *testing.T) {
		r := NewRegistry()

		_, err := r.For("nonexistent-model")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("register overwrites existing model", func(t *testing.T) {
		r := NewRegistry()
		first := &stubClient{}
		second := &stubClient{}

		r.Register("model", first)
		r.Register("model", second)

		c, err := r.For("model")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if c != second {
			t.Fatal("expected second client after overwrite")
		}
	})
}
