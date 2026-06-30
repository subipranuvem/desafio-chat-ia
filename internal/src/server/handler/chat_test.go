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

func TestChatHandler_PostMessage(t *testing.T) {
	t.Run("returns 400 when model not registered", func(t *testing.T) {
		repo := &repomock.MessageRepository{}
		h := handler.NewChatHandler(llm.NewRegistry(), repo)

		req := newPostRequest(t, "s1", map[string]any{"message": "hi", "model": "nonexistent"})
		w := httptest.NewRecorder()

		h.PostMessage(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", w.Code)
		}
	})

	t.Run("uses default model when model field is absent", func(t *testing.T) {
		client := &llmmock.LLMClient{}
		client.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				onChunk := args.Get(2).(func(model.MessageChunk) error)
				onChunk(model.MessageChunk{Event: "done", Model: param.DefaultModel})
			}).Return(nil)

		repo := &repomock.MessageRepository{}
		repo.On("SaveMessages", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		h := handler.NewChatHandler(newRegistry(param.DefaultModel, client), repo)
		req := newPostRequest(t, "s1", map[string]any{"message": "hi"})
		w := httptest.NewRecorder()

		h.PostMessage(w, req)

		client.AssertExpectations(t)
	})

	t.Run("streams chunk and done events in order", func(t *testing.T) {
		client := &llmmock.LLMClient{}
		client.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				onChunk := args.Get(2).(func(model.MessageChunk) error)
				onChunk(model.MessageChunk{Event: "chunk", Text: "hello"})
				onChunk(model.MessageChunk{Event: "chunk", Text: " world"})
				onChunk(model.MessageChunk{Event: "done", Model: "gemini-2.5-flash", TokensUsed: 10})
			}).Return(nil)

		repo := &repomock.MessageRepository{}
		repo.On("SaveMessages", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		h := handler.NewChatHandler(newRegistry("gemini-2.5-flash", client), repo)
		req := newPostRequest(t, "s1", map[string]any{"message": "hi", "model": "gemini-2.5-flash"})
		w := httptest.NewRecorder()

		h.PostMessage(w, req)

		events := parseSSEEvents(w.Body.String())
		if len(events) != 3 {
			t.Fatalf("expected 3 SSE events, got %d: %s", len(events), w.Body.String())
		}
		if events[0]["event"] != "chunk" || events[0]["text"] != "hello" {
			t.Fatalf("unexpected first event: %v", events[0])
		}
		if events[1]["event"] != "chunk" || events[1]["text"] != " world" {
			t.Fatalf("unexpected second event: %v", events[1])
		}
		if events[2]["event"] != "done" {
			t.Fatalf("expected done event, got: %v", events[2])
		}
	})

	t.Run("returns JSON error when SendMessage fails before first chunk", func(t *testing.T) {
		client := &llmmock.LLMClient{}
		client.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).
			Return(fmt.Errorf("upstream failure"))

		repo := &repomock.MessageRepository{}

		h := handler.NewChatHandler(newRegistry("gemini-2.5-flash", client), repo)
		req := newPostRequest(t, "s1", map[string]any{"message": "hi", "model": "gemini-2.5-flash"})
		w := httptest.NewRecorder()

		h.PostMessage(w, req)

		if w.Code != http.StatusBadGateway {
			t.Fatalf("expected 502, got %d", w.Code)
		}
		if w.Header().Get("Content-Type") == "text/event-stream" {
			t.Fatal("expected plain response, got SSE content-type")
		}
	})

	t.Run("streams SSE error event when SendMessage fails mid-stream", func(t *testing.T) {
		client := &llmmock.LLMClient{}
		client.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				onChunk := args.Get(2).(func(model.MessageChunk) error)
				onChunk(model.MessageChunk{Event: "chunk", Text: "partial"})
			}).
			Return(fmt.Errorf("connection reset"))

		repo := &repomock.MessageRepository{}

		h := handler.NewChatHandler(newRegistry("gemini-2.5-flash", client), repo)
		req := newPostRequest(t, "s1", map[string]any{"message": "hi", "model": "gemini-2.5-flash"})
		w := httptest.NewRecorder()

		h.PostMessage(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200 (SSE already started), got %d", w.Code)
		}
		events := parseSSEEvents(w.Body.String())
		if len(events) != 2 {
			t.Fatalf("expected 2 SSE events (chunk + error), got %d: %s", len(events), w.Body.String())
		}
		if events[0]["event"] != "chunk" {
			t.Fatalf("expected first event to be chunk, got: %v", events[0])
		}
		if events[1]["event"] != "error" {
			t.Fatalf("expected second event to be error, got: %v", events[1])
		}
	})

	t.Run("prepends system prompt as first message in chat", func(t *testing.T) {
		var capturedChat model.Chat

		client := &llmmock.LLMClient{}
		client.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				capturedChat = args.Get(1).(model.Chat)
				onChunk := args.Get(2).(func(model.MessageChunk) error)
				onChunk(model.MessageChunk{Event: "done"})
			}).Return(nil)

		repo := &repomock.MessageRepository{}
		repo.On("SaveMessages", mock.Anything, mock.Anything, mock.Anything).Return(nil)

		h := handler.NewChatHandler(newRegistry("gemini-2.5-flash", client), repo)
		req := newPostRequest(t, "s1", map[string]any{
			"message":       "hi",
			"model":         "gemini-2.5-flash",
			"system_prompt": "you are helpful",
		})
		w := httptest.NewRecorder()

		h.PostMessage(w, req)

		if len(capturedChat.Messages) < 2 {
			t.Fatalf("expected at least 2 messages in chat, got %d", len(capturedChat.Messages))
		}
		if capturedChat.Messages[0].Role != model.RoleSystem {
			t.Fatalf("expected first message role %q, got %q", model.RoleSystem, capturedChat.Messages[0].Role)
		}
		if capturedChat.Messages[0].Content != "you are helpful" {
			t.Fatalf("unexpected system prompt content: %q", capturedChat.Messages[0].Content)
		}
	})

	t.Run("saves user and assistant messages with correct content", func(t *testing.T) {
		client := &llmmock.LLMClient{}
		client.On("SendMessage", mock.Anything, mock.Anything, mock.Anything).
			Run(func(args mock.Arguments) {
				onChunk := args.Get(2).(func(model.MessageChunk) error)
				onChunk(model.MessageChunk{Event: "chunk", Text: "response"})
				onChunk(model.MessageChunk{Event: "done", TokensUsed: 5})
			}).Return(nil)

		var savedMessages []model.Message
		repo := &repomock.MessageRepository{}
		repo.On("SaveMessages", mock.Anything, "session-42", mock.Anything).
			Run(func(args mock.Arguments) {
				savedMessages = args.Get(2).([]model.Message)
			}).Return(nil)

		h := handler.NewChatHandler(newRegistry("gemini-2.5-flash", client), repo)
		req := newPostRequest(t, "session-42", map[string]any{"message": "user input", "model": "gemini-2.5-flash"})
		w := httptest.NewRecorder()

		h.PostMessage(w, req)

		repo.AssertExpectations(t)
		if len(savedMessages) != 2 {
			t.Fatalf("expected 2 saved messages, got %d", len(savedMessages))
		}
		if savedMessages[0].Role != model.RoleUser || savedMessages[0].Content != "user input" {
			t.Fatalf("unexpected user message: %+v", savedMessages[0])
		}
		if savedMessages[1].Role != model.RoleAssistant || savedMessages[1].Content != "response" {
			t.Fatalf("unexpected assistant message: %+v", savedMessages[1])
		}
	})
}
