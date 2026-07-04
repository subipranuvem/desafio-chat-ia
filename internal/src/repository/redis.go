package repository

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/database"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

// windowSize defines how many messages are kept per session in the sliding window cache.
const windowSize = 20

type redisMessageCache struct {
	db database.RedisDB
}

func NewRedisMessageCache(db database.RedisDB) MessageCache {
	return &redisMessageCache{db: db}
}

func sessionKey(sessionID string) string {
	return "chat:session:" + sessionID
}

func (c *redisMessageCache) PushMessages(ctx context.Context, sessionID string, messages []model.Message) error {
	client := c.db.Client()
	key := sessionKey(sessionID)

	for _, msg := range messages {
		data, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("marshal message: %w", err)
		}
		if err := client.RPush(ctx, key, data).Err(); err != nil {
			return err
		}
	}

	// Keep only the last windowSize messages.
	return client.LTrim(ctx, key, -windowSize, -1).Err()
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
