package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/llm"
	llmmock "github.com/subipranuvem/desafio-chat-ia/internal/src/llm/mock"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
	repomock "github.com/subipranuvem/desafio-chat-ia/internal/src/repository/mock"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/server/handler"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/server/param"
)

func withSessionID(r *http.Request, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(param.SessionID, id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func newPostRequest(t *testing.T, sessionID string, body any) *http.Request {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/chat/session/"+sessionID, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	return withSessionID(req, sessionID)
}

func parseSSEEvents(body string) []map[string]any {
	var events []map[string]any
	for _, block := range strings.Split(body, "\n\n") {
		block = strings.TrimSpace(block)
		if !strings.HasPrefix(block, "data: ") {
			continue
		}
		var event map[string]any
		if json.Unmarshal([]byte(strings.TrimPrefix(block, "data: ")), &event) == nil {
			events = append(events, event)
		}
	}
	return events
}

func newRegistry(modelID string, client llm.LLMClient) *llm.Registry {
	r := llm.NewRegistry()
	r.Register(modelID, client)
	return r
}

// emptyHistory sets up both cache and repo to simulate a session with no prior history.
// Also covers the additional cache.GetRecentMessages call inside persist().
func emptyHistory(cache *repomock.MessageCache, repo *repomock.MessageRepository) {
	cache.On("GetRecentMessages", mock.Anything, mock.Anything).Return(nil, nil)
	repo.On("GetRecentMessages", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
}

func setupCountTokens(client *llmmock.LLMClient) {
	client.On("CountTokens", mock.Anything, mock.Anything).Return(int64(10), nil)
}

func TestChatHandler_PostMessage(t *testing.T) {
	t.Run("returns 400 when model not registered", func(t *testing.T) {
		repo := &repomock.MessageRepository{}
		cache := &repomock.MessageCache{}
		h := handler.NewChatHandler(llm.NewRegistry(), repo, cache, testWindowTokens)

		req := newPostRequest(t, "s1", map[string]any{"message": "hi", "model": "nonexistent"})
		w := httptest.NewRecorder()

		handler.Adapt(h.PostMessage)(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("uses default model when model field is absent", func(t *testing.T) {
		client := &llmmock.LLMClient{}
		setupCountTokens(client)
		client.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				onChunk := args.Get(2).(func(model.MessageChunk) error)
				onChunk(model.MessageChunk{Event: "done", Model: param.DefaultModel})
			}).Return(nil)

		repo := &repomock.MessageRepository{}
		repo.On("SaveMessages", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		cache := &repomock.MessageCache{}
		emptyHistory(cache, repo)
		cache.On("PushMessages", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		h := handler.NewChatHandler(newRegistry(param.DefaultModel, client), repo, cache, testWindowTokens)
		req := newPostRequest(t, "s1", map[string]any{"message": "hi"})
		w := httptest.NewRecorder()

		handler.Adapt(h.PostMessage)(w, req)

		client.AssertExpectations(t)
	})

	t.Run("streams chunk and done events in order", func(t *testing.T) {
		client := &llmmock.LLMClient{}
		setupCountTokens(client)
		client.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				onChunk := args.Get(2).(func(model.MessageChunk) error)
				onChunk(model.MessageChunk{Event: "chunk", Text: "hello"})
				onChunk(model.MessageChunk{Event: "chunk", Text: " world"})
				onChunk(model.MessageChunk{Event: "done", Model: "gemini-2.5-flash", InputTokens: 7, OutputTokens: 3})
			}).Return(nil)

		repo := &repomock.MessageRepository{}
		repo.On("SaveMessages", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		cache := &repomock.MessageCache{}
		emptyHistory(cache, repo)
		cache.On("PushMessages", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		h := handler.NewChatHandler(newRegistry("gemini-2.5-flash", client), repo, cache, testWindowTokens)
		req := newPostRequest(t, "s1", map[string]any{"message": "hi", "model": "gemini-2.5-flash"})
		w := httptest.NewRecorder()

		handler.Adapt(h.PostMessage)(w, req)

		events := parseSSEEvents(w.Body.String())
		require.Len(t, events, 3, "body: %s", w.Body.String())
		require.Equal(t, "chunk", events[0]["event"])
		require.Equal(t, "hello", events[0]["text"])
		require.Equal(t, "chunk", events[1]["event"])
		require.Equal(t, " world", events[1]["text"])
		require.Equal(t, "done", events[2]["event"])
	})

	t.Run("returns JSON error when SendMessage fails before first chunk", func(t *testing.T) {
		client := &llmmock.LLMClient{}
		setupCountTokens(client)
		client.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).
			Return(fmt.Errorf("upstream failure"))

		repo := &repomock.MessageRepository{}
		cache := &repomock.MessageCache{}
		emptyHistory(cache, repo)

		h := handler.NewChatHandler(newRegistry("gemini-2.5-flash", client), repo, cache, testWindowTokens)
		req := newPostRequest(t, "s1", map[string]any{"message": "hi", "model": "gemini-2.5-flash"})
		w := httptest.NewRecorder()

		handler.Adapt(h.PostMessage)(w, req)

		require.Equal(t, http.StatusBadGateway, w.Code)
		require.NotEqual(t, "text/event-stream", w.Header().Get("Content-Type"))
	})

	t.Run("streams SSE error event when SendMessage fails mid-stream", func(t *testing.T) {
		client := &llmmock.LLMClient{}
		setupCountTokens(client)
		client.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				onChunk := args.Get(2).(func(model.MessageChunk) error)
				onChunk(model.MessageChunk{Event: "chunk", Text: "partial"})
			}).
			Return(fmt.Errorf("connection reset"))

		repo := &repomock.MessageRepository{}
		cache := &repomock.MessageCache{}
		emptyHistory(cache, repo)

		h := handler.NewChatHandler(newRegistry("gemini-2.5-flash", client), repo, cache, testWindowTokens)
		req := newPostRequest(t, "s1", map[string]any{"message": "hi", "model": "gemini-2.5-flash"})
		w := httptest.NewRecorder()

		handler.Adapt(h.PostMessage)(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		events := parseSSEEvents(w.Body.String())
		require.Len(t, events, 2, "body: %s", w.Body.String())
		require.Equal(t, "chunk", events[0]["event"])
		require.Equal(t, "error", events[1]["event"])
	})

	t.Run("prepends system prompt before history and user message", func(t *testing.T) {
		var capturedChat model.Chat

		client := &llmmock.LLMClient{}
		setupCountTokens(client)
		client.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				capturedChat = args.Get(1).(model.Chat)
				onChunk := args.Get(2).(func(model.MessageChunk) error)
				onChunk(model.MessageChunk{Event: "done"})
			}).Return(nil)

		repo := &repomock.MessageRepository{}
		repo.On("SaveMessages", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		cache := &repomock.MessageCache{}
		emptyHistory(cache, repo)
		cache.On("PushMessages", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		h := handler.NewChatHandler(newRegistry("gemini-2.5-flash", client), repo, cache, testWindowTokens)
		req := newPostRequest(t, "s1", map[string]any{
			"message":       "hi",
			"model":         "gemini-2.5-flash",
			"system_prompt": "you are helpful",
		})
		w := httptest.NewRecorder()

		handler.Adapt(h.PostMessage)(w, req)

		require.GreaterOrEqual(t, len(capturedChat.Messages), 2)
		require.Equal(t, model.RoleSystem, capturedChat.Messages[0].Role)
		require.Equal(t, "you are helpful", capturedChat.Messages[0].Content)
	})

	t.Run("saves user and assistant messages with correct content", func(t *testing.T) {
		client := &llmmock.LLMClient{}
		setupCountTokens(client)
		client.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				onChunk := args.Get(2).(func(model.MessageChunk) error)
				onChunk(model.MessageChunk{Event: "chunk", Text: "response"})
				onChunk(model.MessageChunk{Event: "done", InputTokens: 3, OutputTokens: 2})
			}).Return(nil)

		var savedMessages []model.Message
		repo := &repomock.MessageRepository{}
		repo.On("SaveMessages", mock.Anything, "session-42", mock.Anything).
			Run(func(args mock.Arguments) {
				savedMessages = args.Get(2).([]model.Message)
			}).Return(nil)

		cache := &repomock.MessageCache{}
		emptyHistory(cache, repo)
		cache.On("PushMessages", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		h := handler.NewChatHandler(newRegistry("gemini-2.5-flash", client), repo, cache, testWindowTokens)
		req := newPostRequest(t, "session-42", map[string]any{"message": "user input", "model": "gemini-2.5-flash"})
		w := httptest.NewRecorder()

		handler.Adapt(h.PostMessage)(w, req)

		repo.AssertExpectations(t)
		require.Len(t, savedMessages, 2)
		require.Equal(t, model.RoleUser, savedMessages[0].Role)
		require.Equal(t, "user input", savedMessages[0].Content)
		require.Equal(t, model.RoleAssistant, savedMessages[1].Role)
		require.Equal(t, "response", savedMessages[1].Content)
	})

	t.Run("returns 400 when system prompt sent for existing conversation", func(t *testing.T) {
		client := &llmmock.LLMClient{}

		existingHistory := []model.Message{
			{Role: model.RoleSystem, Content: "original prompt"},
			{Role: model.RoleUser, Content: "prev question"},
			{Role: model.RoleAssistant, Content: "prev answer"},
		}
		cache := &repomock.MessageCache{}
		cache.On("GetRecentMessages", mock.Anything, mock.Anything).Return(existingHistory, nil)

		repo := &repomock.MessageRepository{}

		h := handler.NewChatHandler(newRegistry(param.DefaultModel, client), repo, cache, testWindowTokens)
		req := newPostRequest(t, "s1", map[string]any{
			"message":       "hi",
			"model":         param.DefaultModel,
			"system_prompt": "override attempt",
		})
		w := httptest.NewRecorder()

		handler.Adapt(h.PostMessage)(w, req)

		require.Equal(t, http.StatusBadRequest, w.Code)
		require.Contains(t, w.Body.String(), "cannot override system prompt")
		client.AssertNotCalled(t, "SendMessage")
	})

	t.Run("saves system message on first turn of new conversation", func(t *testing.T) {
		client := &llmmock.LLMClient{}
		setupCountTokens(client)
		client.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				onChunk := args.Get(2).(func(model.MessageChunk) error)
				onChunk(model.MessageChunk{Event: "done", Model: param.DefaultModel, InputTokens: 5, OutputTokens: 2})
			}).Return(nil)

		var savedMessages []model.Message
		repo := &repomock.MessageRepository{}
		repo.On("SaveMessages", mock.Anything, "s1", mock.Anything).
			Run(func(args mock.Arguments) {
				savedMessages = args.Get(2).([]model.Message)
			}).Return(nil)

		cache := &repomock.MessageCache{}
		emptyHistory(cache, repo)
		cache.On("PushMessages", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		h := handler.NewChatHandler(newRegistry(param.DefaultModel, client), repo, cache, testWindowTokens)
		req := newPostRequest(t, "s1", map[string]any{
			"message":       "hello",
			"model":         param.DefaultModel,
			"system_prompt": "you are a pirate",
		})
		w := httptest.NewRecorder()

		handler.Adapt(h.PostMessage)(w, req)

		require.Len(t, savedMessages, 3)
		require.Equal(t, model.RoleSystem, savedMessages[0].Role)
		require.Equal(t, "you are a pirate", savedMessages[0].Content)
		require.Equal(t, model.RoleUser, savedMessages[1].Role)
		require.Equal(t, model.RoleAssistant, savedMessages[2].Role)
	})

	t.Run("includes cached history in LLM context", func(t *testing.T) {
		var capturedChat model.Chat

		client := &llmmock.LLMClient{}
		setupCountTokens(client)
		client.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				capturedChat = args.Get(1).(model.Chat)
				onChunk := args.Get(2).(func(model.MessageChunk) error)
				onChunk(model.MessageChunk{Event: "done"})
			}).Return(nil)

		repo := &repomock.MessageRepository{}
		repo.On("SaveMessages", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		cachedHistory := []model.Message{
			{Role: model.RoleUser, Content: "previous question"},
			{Role: model.RoleAssistant, Content: "previous answer"},
		}
		cache := &repomock.MessageCache{}
		cache.On("GetRecentMessages", mock.Anything, "s1").Return(cachedHistory, nil)
		cache.On("PushMessages", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		h := handler.NewChatHandler(newRegistry("gemini-2.5-flash", client), repo, cache, testWindowTokens)
		req := newPostRequest(t, "s1", map[string]any{"message": "follow-up", "model": "gemini-2.5-flash"})
		w := httptest.NewRecorder()

		handler.Adapt(h.PostMessage)(w, req)

		require.Len(t, capturedChat.Messages, 3)
		require.Equal(t, model.RoleUser, capturedChat.Messages[0].Role)
		require.Equal(t, "previous question", capturedChat.Messages[0].Content)
		require.Equal(t, model.RoleAssistant, capturedChat.Messages[1].Role)
		require.Equal(t, model.RoleUser, capturedChat.Messages[2].Role)
		require.Equal(t, "follow-up", capturedChat.Messages[2].Content)
	})
}
