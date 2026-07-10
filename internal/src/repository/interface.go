package repository

import (
	"context"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

type MessageRepository interface {
	SaveMessages(ctx context.Context, sessionID string, messages []model.Message) error
	GetMessages(ctx context.Context, sessionID string, limit, offset int) (model.MessageQuery, error)
	GetRecentMessages(ctx context.Context, sessionID string, limit, offset int) ([]model.Message, error)
}

type MessageCache interface {
	PushMessages(ctx context.Context, sessionID string, messages []model.Message) error
	GetRecentMessages(ctx context.Context, sessionID string) ([]model.Message, error)
}
