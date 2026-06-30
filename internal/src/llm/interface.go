package llm

import (
	"context"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

type LLMClient interface {
	SendMessage(ctx context.Context, chat model.Chat, onChunk func(model.MessageChunk) error) error
}
