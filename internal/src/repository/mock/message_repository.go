package mock

import (
	"context"

	testifymock "github.com/stretchr/testify/mock"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

type MessageRepository struct {
	testifymock.Mock
}

func (m *MessageRepository) SaveMessages(ctx context.Context, sessionID string, messages []model.Message) error {
	args := m.Called(ctx, sessionID, messages)
	return args.Error(0)
}

func (m *MessageRepository) GetMessages(ctx context.Context, sessionID string, limit, offset int) (model.MessageQuery, error) {
	args := m.Called(ctx, sessionID, limit, offset)
	page, _ := args.Get(0).(model.MessageQuery)
	return page, args.Error(1)
}

func (m *MessageRepository) GetRecentMessages(ctx context.Context, sessionID string, limit, offset int) ([]model.Message, error) {
	args := m.Called(ctx, sessionID, limit, offset)
	msgs, _ := args.Get(0).([]model.Message)
	return msgs, args.Error(1)
}
