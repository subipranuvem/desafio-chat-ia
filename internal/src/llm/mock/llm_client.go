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

func (m *LLMClient) CountTokens(ctx context.Context, message model.Message) (int64, error) {
	args := m.Called(ctx, message)
	return args.Get(0).(int64), args.Error(1)
}
