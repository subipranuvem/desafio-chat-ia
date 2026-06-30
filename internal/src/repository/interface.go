package repository

import (
	"context"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

type MessageRepository interface {
	SaveMessages(ctx context.Context, sessionID string, messages []model.Message) error
}
