package llm

import (
	"context"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

type LLMClient interface {
	SendMessage(ctx context.Context, chat model.Chat, onChunk func(model.MessageChunk) error) error
	// CountTokens returns the token count for a single message as it would be sent to the model.
	CountTokens(ctx context.Context, message model.Message) (int64, error)
}
