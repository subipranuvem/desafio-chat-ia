package handler_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
	repomock "github.com/subipranuvem/desafio-chat-ia/internal/src/repository/mock"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/server/handler"
)

const testWindowTokens = 8000

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
	t.Run("returns messages on hit", func(t *testing.T) {
		msgs := []model.Message{{Role: model.RoleUser, Content: "hello"}}

		repo := &repomock.MessageRepository{}
		repo.On("GetRecentMessages", mock.Anything, "s1", mock.Anything, mock.Anything).Return(msgs, nil)

		loader := handler.NewPostgresLoader(repo, nil, testWindowTokens)
		got, found, err := loader.Load(context.Background(), "s1")

		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, msgs, got)
	})

	t.Run("returns not found when postgres miss and no next", func(t *testing.T) {
		repo := &repomock.MessageRepository{}
		repo.On("GetRecentMessages", mock.Anything, "s1", mock.Anything, mock.Anything).Return(nil, nil)

		loader := handler.NewPostgresLoader(repo, nil, testWindowTokens)
		_, found, err := loader.Load(context.Background(), "s1")

		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("delegates to next on postgres miss", func(t *testing.T) {
		repo := &repomock.MessageRepository{}
		repo.On("GetRecentMessages", mock.Anything, "s1", mock.Anything, mock.Anything).Return(nil, nil)

		called := false
		next := &stubLoader{load: func() ([]model.Message, bool, error) {
			called = true
			return nil, false, nil
		}}

		loader := handler.NewPostgresLoader(repo, next, testWindowTokens)
		_, _, _ = loader.Load(context.Background(), "s1")

		require.True(t, called)
	})

	t.Run("delegates to next on postgres error", func(t *testing.T) {
		repo := &repomock.MessageRepository{}
		repo.On("GetRecentMessages", mock.Anything, "s1", mock.Anything, mock.Anything).Return(nil, errors.New("db down"))

		called := false
		next := &stubLoader{load: func() ([]model.Message, bool, error) {
			called = true
			return nil, false, nil
		}}

		loader := handler.NewPostgresLoader(repo, next, testWindowTokens)
		_, _, _ = loader.Load(context.Background(), "s1")

		require.True(t, called)
	})

	t.Run("paginates until page smaller than page size", func(t *testing.T) {
		page1 := make([]model.Message, 50)
		for i := range page1 {
			page1[i] = model.Message{Role: model.RoleUser, Content: "a"}
		}
		page2 := []model.Message{{Role: model.RoleUser, Content: "last"}}

		repo := &repomock.MessageRepository{}
		repo.On("GetRecentMessages", mock.Anything, "s1", 50, 0).Return(page1, nil)
		repo.On("GetRecentMessages", mock.Anything, "s1", 50, 50).Return(page2, nil)

		loader := handler.NewPostgresLoader(repo, nil, testWindowTokens)
		got, found, err := loader.Load(context.Background(), "s1")

		require.NoError(t, err)
		require.True(t, found)
		// page2 (older) prepended to page1 (newer)
		require.Len(t, got, 51)
		require.Equal(t, page2[0], got[0])
		repo.AssertExpectations(t)
	})

	t.Run("stops pagination when token budget satisfied", func(t *testing.T) {
		// Each message costs 200 tokens; budget is 300 → page 0 (200 tokens) not enough, page 1 (400 total) stops.
		largePage := make([]model.Message, 50)
		for i := range largePage {
			largePage[i] = model.Message{Role: model.RoleUser, InputToken: 4, OutputToken: 0}
		}

		repo := &repomock.MessageRepository{}
		repo.On("GetRecentMessages", mock.Anything, "s1", 50, 0).Return(largePage, nil)

		loader := handler.NewPostgresLoader(repo, nil, 100)
		got, found, err := loader.Load(context.Background(), "s1")

		require.NoError(t, err)
		require.True(t, found)
		require.Len(t, got, 50)
		// Only one page fetched — budget satisfied (50*4=200 >= 100)
		repo.AssertNumberOfCalls(t, "GetRecentMessages", 1)
	})
}

func TestCacheWarmingLoader(t *testing.T) {
	t.Run("warms cache on inner hit", func(t *testing.T) {
		msgs := []model.Message{{Role: model.RoleUser, Content: "hello"}}

		inner := &stubLoader{load: func() ([]model.Message, bool, error) {
			return msgs, true, nil
		}}
		cache := &repomock.MessageCache{}
		cache.On("PushMessages", mock.Anything, "s1", msgs).Return(nil)

		loader := handler.NewCacheWarmingLoader(inner, cache, testWindowTokens)
		got, found, err := loader.Load(context.Background(), "s1")

		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, msgs, got)
		cache.AssertExpectations(t)
	})

	t.Run("does not warm cache on inner miss", func(t *testing.T) {
		inner := &stubLoader{load: func() ([]model.Message, bool, error) {
			return nil, false, nil
		}}
		cache := &repomock.MessageCache{}

		loader := handler.NewCacheWarmingLoader(inner, cache, testWindowTokens)
		_, found, _ := loader.Load(context.Background(), "s1")

		require.False(t, found)
		cache.AssertNotCalled(t, "PushMessages")
	})

	t.Run("returns data even when cache warming fails", func(t *testing.T) {
		msgs := []model.Message{{Role: model.RoleUser, Content: "hello"}}

		inner := &stubLoader{load: func() ([]model.Message, bool, error) {
			return msgs, true, nil
		}}
		cache := &repomock.MessageCache{}
		cache.On("PushMessages", mock.Anything, "s1", msgs).Return(errors.New("redis down"))

		loader := handler.NewCacheWarmingLoader(inner, cache, testWindowTokens)
		got, found, err := loader.Load(context.Background(), "s1")

		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, msgs, got)
	})

	t.Run("applies buildWindow before warming cache", func(t *testing.T) {
		// Each message: 60 chars → 15 tokens (len/4); budget=20 → only newest fits
		longContent := strings.Repeat("x", 60)
		old := model.Message{Role: model.RoleUser, Content: longContent}
		recent := model.Message{Role: model.RoleAssistant, Content: longContent}
		msgs := []model.Message{old, recent}
		window := []model.Message{recent}

		inner := &stubLoader{load: func() ([]model.Message, bool, error) {
			return msgs, true, nil
		}}
		cache := &repomock.MessageCache{}
		cache.On("PushMessages", mock.Anything, "s1", window).Return(nil)

		loader := handler.NewCacheWarmingLoader(inner, cache, 20)
		got, found, err := loader.Load(context.Background(), "s1")

		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, window, got)
		cache.AssertExpectations(t)
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
		repo.On("GetRecentMessages", mock.Anything, "s1", mock.Anything, mock.Anything).Return(msgs, nil)

		loader := handler.NewRedisLoader(cache,
			handler.NewCacheWarmingLoader(handler.NewPostgresLoader(repo, nil, testWindowTokens), cache, testWindowTokens),
		)
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
		repo.On("GetRecentMessages", mock.Anything, "s1", mock.Anything, mock.Anything).Return(nil, nil)

		loader := handler.NewRedisLoader(cache,
			handler.NewCacheWarmingLoader(handler.NewPostgresLoader(repo, nil, testWindowTokens), cache, testWindowTokens),
		)
		_, found, err := loader.Load(context.Background(), "s1")

		require.NoError(t, err)
		require.False(t, found)
		cache.AssertNotCalled(t, "PushMessages")
	})
}
