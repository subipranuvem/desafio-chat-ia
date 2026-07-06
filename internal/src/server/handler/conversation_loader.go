package handler

import (
	"context"
	"log/slog"
	"reflect"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/repository"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/server/param"
)

// ConversationLoader fetches prior messages for a session.
// Each link tries its source and delegates to next on miss.
type ConversationLoader interface {
	Load(ctx context.Context, sessionID string) (messages []model.Message, found bool, err error)
}

// noopLoader terminates the chain: always returns not found.
type noopLoader struct{}

func (noopLoader) Load(_ context.Context, _ string) ([]model.Message, bool, error) {
	return nil, false, nil
}

func chainNext(next ConversationLoader) ConversationLoader {
	if next == nil {
		return noopLoader{}
	}
	// Guard against typed-nil: an interface wrapping a nil pointer is non-nil
	// but would panic on dispatch. reflect.ValueOf detects the underlying nil.
	v := reflect.ValueOf(next)
	if v.Kind() == reflect.Pointer && v.IsNil() {
		return noopLoader{}
	}
	return next
}

type RedisLoader struct {
	cache repository.MessageCache
	next  ConversationLoader
}

func NewRedisLoader(cache repository.MessageCache, next ConversationLoader) *RedisLoader {
	return &RedisLoader{cache: cache, next: chainNext(next)}
}

func (l *RedisLoader) Load(ctx context.Context, sessionID string) ([]model.Message, bool, error) {
	msgs, err := l.cache.GetRecentMessages(ctx, sessionID)
	if err != nil {
		slog.Warn("redis miss, delegating", "session_id", sessionID, "error", err)
	}
	if len(msgs) > 0 {
		return msgs, true, nil
	}
	return l.next.Load(ctx, sessionID)
}

type PostgresLoader struct {
	repo  repository.MessageRepository
	cache repository.MessageCache
	next  ConversationLoader
}

func NewPostgresLoader(repo repository.MessageRepository, cache repository.MessageCache, next ConversationLoader) *PostgresLoader {
	return &PostgresLoader{repo: repo, cache: cache, next: chainNext(next)}
}

func (l *PostgresLoader) Load(ctx context.Context, sessionID string) ([]model.Message, bool, error) {
	msgs, err := l.repo.GetRecentMessages(ctx, sessionID, param.DefaultWindowSize)
	if err != nil {
		slog.Warn("postgres miss, delegating", "session_id", sessionID, "error", err)
	}
	if len(msgs) == 0 {
		return l.next.Load(ctx, sessionID)
	}
	if warmErr := l.cache.PushMessages(ctx, sessionID, msgs); warmErr != nil {
		slog.Warn("failed to warm redis after postgres load", "session_id", sessionID, "error", warmErr)
	}
	return msgs, true, nil
}
