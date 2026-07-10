package session

import (
	"context"
	"log/slog"
	"reflect"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/repository"
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

// postgresPageSize controls how many messages are fetched per pagination round-trip.
const postgresPageSize = 50

type PostgresLoader struct {
	repo                repository.MessageRepository
	next                ConversationLoader
	contextWindowTokens int
}

func NewPostgresLoader(repo repository.MessageRepository, next ConversationLoader, contextWindowTokens int) *PostgresLoader {
	return &PostgresLoader{repo: repo, next: chainNext(next), contextWindowTokens: contextWindowTokens}
}

// Load paginates newest→oldest until accumulated tokens satisfy the context window or all rows are fetched.
// On partial pagination error it returns whatever was collected; on a first-page error it delegates to next.
func (l *PostgresLoader) Load(ctx context.Context, sessionID string) ([]model.Message, bool, error) {
	var collected []model.Message
	var totalTokens int64
	offset := 0

	for {
		page, err := l.repo.GetRecentMessages(ctx, sessionID, postgresPageSize, offset)
		if err != nil {
			if len(collected) > 0 {
				slog.Warn("postgres pagination error, using partial results", "session_id", sessionID, "error", err)
				break
			}
			slog.Warn("postgres miss, delegating", "session_id", sessionID, "error", err)
			return l.next.Load(ctx, sessionID)
		}
		if len(page) == 0 {
			break
		}

		for _, m := range page {
			cost := m.InputToken + m.OutputToken
			if cost == 0 {
				cost = int64(len(m.Content) / 4)
			}
			totalTokens += cost
		}

		// page is ASC (oldest-in-page first); prepend to keep collected in chronological order.
		collected = append(page, collected...)

		if totalTokens >= int64(l.contextWindowTokens) || len(page) < postgresPageSize {
			break
		}
		offset += postgresPageSize
	}

	if len(collected) == 0 {
		return l.next.Load(ctx, sessionID)
	}
	return collected, true, nil
}

// CacheWarmingLoader wraps an inner loader and warms the cache on hit.
// Applies BuildWindow before pushing so Redis stores exactly the token-bounded slice.
// Redis unavailable → logs warning, returns data normally.
type CacheWarmingLoader struct {
	inner               ConversationLoader
	cache               repository.MessageCache
	contextWindowTokens int
}

func NewCacheWarmingLoader(inner ConversationLoader, cache repository.MessageCache, contextWindowTokens int) *CacheWarmingLoader {
	return &CacheWarmingLoader{inner: inner, cache: cache, contextWindowTokens: contextWindowTokens}
}

func (l *CacheWarmingLoader) Load(ctx context.Context, sessionID string) ([]model.Message, bool, error) {
	msgs, found, err := l.inner.Load(ctx, sessionID)
	if found && err == nil {
		window := BuildWindow(msgs, l.contextWindowTokens)
		if warmErr := l.cache.PushMessages(ctx, sessionID, window); warmErr != nil {
			slog.Warn("failed to warm redis after postgres load", "session_id", sessionID, "error", warmErr)
		}
		return window, found, err
	}
	return msgs, found, err
}
