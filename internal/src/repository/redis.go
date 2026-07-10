package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/database"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

type redisMessageCache struct {
	db  database.RedisDB
	ttl time.Duration
}

func NewRedisMessageCache(db database.RedisDB, ttl time.Duration) MessageCache {
	return &redisMessageCache{db: db, ttl: ttl}
}

func sessionKey(sessionID string) string {
	return "chat:session:" + sessionID
}

// PushMessages replaces the stored window atomically: DEL + RPush + Expire in a single transaction.
// Callers are responsible for passing a pre-computed, token-bounded slice.
func (c *redisMessageCache) PushMessages(ctx context.Context, sessionID string, messages []model.Message) error {
	serialized := make([]any, len(messages))
	for i, msg := range messages {
		data, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("marshal message: %w", err)
		}
		serialized[i] = data
	}

	client := c.db.Client()
	key := sessionKey(sessionID)

	_, err := client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.Del(ctx, key)
		if len(serialized) > 0 {
			pipe.RPush(ctx, key, serialized...)
		}
		pipe.Expire(ctx, key, c.ttl)
		return nil
	})
	return err
}

func (c *redisMessageCache) GetRecentMessages(ctx context.Context, sessionID string) ([]model.Message, error) {
	data, err := c.db.Client().LRange(ctx, sessionKey(sessionID), 0, -1).Result()
	if err != nil {
		return nil, err
	}

	messages := make([]model.Message, 0, len(data))
	for _, item := range data {
		var msg model.Message
		if err := json.Unmarshal([]byte(item), &msg); err != nil {
			return nil, fmt.Errorf("unmarshal message: %w", err)
		}
		messages = append(messages, msg)
	}
	return messages, nil
}
