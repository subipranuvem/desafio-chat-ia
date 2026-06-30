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
