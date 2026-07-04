package mock

import (
	"context"

	testifymock "github.com/stretchr/testify/mock"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

type MessageCache struct {
	testifymock.Mock
}

func (m *MessageCache) PushMessages(ctx context.Context, sessionID string, messages []model.Message) error {
	args := m.Called(ctx, sessionID, messages)
	return args.Error(0)
}

func (m *MessageCache) GetRecentMessages(ctx context.Context, sessionID string) ([]model.Message, error) {
	args := m.Called(ctx, sessionID)
	msgs, _ := args.Get(0).([]model.Message)
	return msgs, args.Error(1)
}
