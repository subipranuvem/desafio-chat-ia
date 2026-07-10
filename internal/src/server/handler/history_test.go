package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/llm"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
	repomock "github.com/subipranuvem/desafio-chat-ia/internal/src/repository/mock"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/server/handler"
)

func newGetRequest(t *testing.T, sessionID, query string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/chat/session/%s/history?%s", sessionID, query), nil)
	return withSessionID(req, sessionID)
}

func newHistoryHandler(repo *repomock.MessageRepository) *handler.ChatHandler {
	cache := &repomock.MessageCache{}
	return handler.NewChatHandler(llm.NewRegistry(), repo, cache, testWindowTokens)
}

func TestChatHandler_GetHistory(t *testing.T) {
	t.Run("returns messages with default pagination", func(t *testing.T) {
		msgs := []model.Message{
			{Role: model.RoleUser, Content: "hello"},
			{Role: model.RoleAssistant, Content: "hi there"},
		}
		page := model.MessageQuery{Messages: msgs, Total: 2}

		repo := &repomock.MessageRepository{}
		repo.On("GetMessages", mock.Anything, "s1", 20, 0).Return(page, nil)

		h := newHistoryHandler(repo)
		req := newGetRequest(t, "s1", "")
		w := httptest.NewRecorder()

		handler.Adapt(h.GetHistory)(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var resp map[string]any
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		require.Equal(t, "s1", resp["session_id"])
		pg := resp["pagination"].(map[string]any)
		require.InDelta(t, 20.0, pg["limit"], 0)
		require.InDelta(t, 0.0, pg["offset"], 0)
		require.InDelta(t, 2.0, pg["total_records"], 0)
		messages := resp["messages"].([]any)
		require.Len(t, messages, 2)

		repo.AssertExpectations(t)
	})

	t.Run("returns messages with custom limit and offset", func(t *testing.T) {
		page := model.MessageQuery{
			Messages: []model.Message{{Role: model.RoleUser, Content: "q"}},
			Total:    50,
		}

		repo := &repomock.MessageRepository{}
		repo.On("GetMessages", mock.Anything, "s1", 10, 30).Return(page, nil)

		h := newHistoryHandler(repo)
		req := newGetRequest(t, "s1", "limit=10&offset=30")
		w := httptest.NewRecorder()

		handler.Adapt(h.GetHistory)(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string]any
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		pg := resp["pagination"].(map[string]any)
		require.InDelta(t, 10.0, pg["limit"], 0)
		require.InDelta(t, 30.0, pg["offset"], 0)
		require.InDelta(t, 50.0, pg["total_records"], 0)

		repo.AssertExpectations(t)
	})

	t.Run("caps limit at 100 when exceeded", func(t *testing.T) {
		repo := &repomock.MessageRepository{}
		repo.On("GetMessages", mock.Anything, "s1", 100, 0).Return(model.MessageQuery{}, nil)

		h := newHistoryHandler(repo)
		req := newGetRequest(t, "s1", "limit=500")
		w := httptest.NewRecorder()

		handler.Adapt(h.GetHistory)(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		repo.AssertExpectations(t)
	})

	t.Run("returns 500 when repo fails", func(t *testing.T) {
		repo := &repomock.MessageRepository{}
		repo.On("GetMessages", mock.Anything, "s1", 20, 0).
			Return(model.MessageQuery{}, fmt.Errorf("db down"))

		h := newHistoryHandler(repo)
		req := newGetRequest(t, "s1", "")
		w := httptest.NewRecorder()

		handler.Adapt(h.GetHistory)(w, req)

		require.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("uses default limit when value is invalid", func(t *testing.T) {
		repo := &repomock.MessageRepository{}
		repo.On("GetMessages", mock.Anything, "s1", 20, 0).Return(model.MessageQuery{}, nil)

		h := newHistoryHandler(repo)
		req := newGetRequest(t, "s1", "limit=abc")
		w := httptest.NewRecorder()

		handler.Adapt(h.GetHistory)(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		repo.AssertExpectations(t)
	})

	t.Run("uses default offset when value is negative", func(t *testing.T) {
		repo := &repomock.MessageRepository{}
		repo.On("GetMessages", mock.Anything, "s1", 20, 0).Return(model.MessageQuery{}, nil)

		h := newHistoryHandler(repo)
		req := newGetRequest(t, "s1", "offset=-5")
		w := httptest.NewRecorder()

		handler.Adapt(h.GetHistory)(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		repo.AssertExpectations(t)
	})
}
