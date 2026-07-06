package handler_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
	repomock "github.com/subipranuvem/desafio-chat-ia/internal/src/repository/mock"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/server/handler"
)

type stubLoader struct {
	load func() ([]model.Message, bool, error)
}

func (s *stubLoader) Load(_ context.Context, _ string) ([]model.Message, bool, error) {
	return s.load()
}

func TestRedisLoader(t *testing.T) {
	t.Run("returns messages on cache hit", func(t *testing.T) {
		msgs := []model.Message{{Role: model.RoleUser, Content: "hello"}}
		cache := &repomock.MessageCache{}
		cache.On("GetRecentMessages", mock.Anything, "s1").Return(msgs, nil)

		loader := handler.NewRedisLoader(cache, nil)
		got, found, err := loader.Load(context.Background(), "s1")

		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, msgs, got)
		cache.AssertExpectations(t)
	})

	t.Run("returns not found when cache miss and no next", func(t *testing.T) {
		cache := &repomock.MessageCache{}
		cache.On("GetRecentMessages", mock.Anything, "s1").Return(nil, nil)

		loader := handler.NewRedisLoader(cache, nil)
		_, found, err := loader.Load(context.Background(), "s1")

		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("delegates to next on cache miss", func(t *testing.T) {
		cache := &repomock.MessageCache{}
		cache.On("GetRecentMessages", mock.Anything, "s1").Return(nil, nil)

		nextMsgs := []model.Message{{Role: model.RoleUser, Content: "from next"}}
		next := &stubLoader{load: func() ([]model.Message, bool, error) {
			return nextMsgs, true, nil
		}}

		loader := handler.NewRedisLoader(cache, next)
		got, found, err := loader.Load(context.Background(), "s1")

		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, nextMsgs, got)
	})

	t.Run("delegates to next on cache error", func(t *testing.T) {
		cache := &repomock.MessageCache{}
		cache.On("GetRecentMessages", mock.Anything, "s1").Return(nil, errors.New("redis down"))

		called := false
		next := &stubLoader{load: func() ([]model.Message, bool, error) {
			called = true
			return nil, false, nil
		}}

		loader := handler.NewRedisLoader(cache, next)
		_, _, _ = loader.Load(context.Background(), "s1")

		require.True(t, called)
	})
}

func TestPostgresLoader(t *testing.T) {
	t.Run("returns messages and warms redis on hit", func(t *testing.T) {
		msgs := []model.Message{{Role: model.RoleUser, Content: "hello"}}

		repo := &repomock.MessageRepository{}
		repo.On("GetRecentMessages", mock.Anything, "s1", mock.Anything).Return(msgs, nil)

		cache := &repomock.MessageCache{}
		cache.On("PushMessages", mock.Anything, "s1", msgs).Return(nil)

		loader := handler.NewPostgresLoader(repo, cache, nil)
		got, found, err := loader.Load(context.Background(), "s1")

		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, msgs, got)
		cache.AssertExpectations(t)
	})

	t.Run("returns not found when postgres miss and no next", func(t *testing.T) {
		repo := &repomock.MessageRepository{}
		repo.On("GetRecentMessages", mock.Anything, "s1", mock.Anything).Return(nil, nil)

		cache := &repomock.MessageCache{}

		loader := handler.NewPostgresLoader(repo, cache, nil)
		_, found, err := loader.Load(context.Background(), "s1")

		require.NoError(t, err)
		require.False(t, found)
		cache.AssertNotCalled(t, "PushMessages")
	})

	t.Run("delegates to next on postgres miss", func(t *testing.T) {
		repo := &repomock.MessageRepository{}
		repo.On("GetRecentMessages", mock.Anything, "s1", mock.Anything).Return(nil, nil)

		cache := &repomock.MessageCache{}

		called := false
		next := &stubLoader{load: func() ([]model.Message, bool, error) {
			called = true
			return nil, false, nil
		}}

		loader := handler.NewPostgresLoader(repo, cache, next)
		_, _, _ = loader.Load(context.Background(), "s1")

		require.True(t, called)
		cache.AssertNotCalled(t, "PushMessages")
	})

	t.Run("delegates to next on postgres error", func(t *testing.T) {
		repo := &repomock.MessageRepository{}
		repo.On("GetRecentMessages", mock.Anything, "s1", mock.Anything).Return(nil, errors.New("db down"))

		cache := &repomock.MessageCache{}

		called := false
		next := &stubLoader{load: func() ([]model.Message, bool, error) {
			called = true
			return nil, false, nil
		}}

		loader := handler.NewPostgresLoader(repo, cache, next)
		_, _, _ = loader.Load(context.Background(), "s1")

		require.True(t, called)
		cache.AssertNotCalled(t, "PushMessages")
	})
}

func TestRedisToPostgresChain(t *testing.T) {
	t.Run("redis miss falls through to postgres and warms cache", func(t *testing.T) {
		msgs := []model.Message{
			{Role: model.RoleUser, Content: "q"},
			{Role: model.RoleAssistant, Content: "a"},
		}

		cache := &repomock.MessageCache{}
		cache.On("GetRecentMessages", mock.Anything, "s1").Return(nil, nil)
		cache.On("PushMessages", mock.Anything, "s1", msgs).Return(nil)

		repo := &repomock.MessageRepository{}
		repo.On("GetRecentMessages", mock.Anything, "s1", mock.Anything).Return(msgs, nil)

		loader := handler.NewRedisLoader(cache, handler.NewPostgresLoader(repo, cache, nil))
		got, found, err := loader.Load(context.Background(), "s1")

		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, msgs, got)
		cache.AssertExpectations(t)
		repo.AssertExpectations(t)
	})

	t.Run("both miss returns not found", func(t *testing.T) {
		cache := &repomock.MessageCache{}
		cache.On("GetRecentMessages", mock.Anything, "s1").Return(nil, nil)

		repo := &repomock.MessageRepository{}
		repo.On("GetRecentMessages", mock.Anything, "s1", mock.Anything).Return(nil, nil)

		loader := handler.NewRedisLoader(cache, handler.NewPostgresLoader(repo, cache, nil))
		_, found, err := loader.Load(context.Background(), "s1")

		require.NoError(t, err)
		require.False(t, found)
		cache.AssertNotCalled(t, "PushMessages")
	})
}
