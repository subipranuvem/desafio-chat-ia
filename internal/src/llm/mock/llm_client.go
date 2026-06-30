package mock

import (
	"context"

	testifymock "github.com/stretchr/testify/mock"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

type LLMClient struct {
	testifymock.Mock
}

func (m *LLMClient) SendMessage(ctx context.Context, chat model.Chat, onChunk func(model.MessageChunk) error) error {
	args := m.Called(ctx, chat, onChunk)
	return args.Error(0)
}
