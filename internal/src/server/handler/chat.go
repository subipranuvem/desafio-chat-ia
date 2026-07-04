package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/llm"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/repository"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/server/param"
)

const chatPostSchema = `{
	"type": "object",
	"required": ["message"],
	"properties": {
		"message":       {"type": "string", "minLength": 1},
		"model":         {"type": "string", "enum": [
                "deepseek-v4-flash",
                "deepseek-v4-pro",
                "gemini-3.5-flash",
                "gemini-3.1-flash-lite",
                "gemini-2.5-flash"
            ]},
		"system_prompt": {"type": "string"},
		"max_tokens":    {"type": "integer", "minimum": 500}
	}
}`

type chatRequest struct {
	Message      string `json:"message"`
	Model        string `json:"model"`
	SystemPrompt string `json:"system_prompt"`
	MaxTokens    int    `json:"max_tokens"`
}

type ChatHandler struct {
	registry *llm.Registry
	repo     repository.MessageRepository
	cache    repository.MessageCache
}

func NewChatHandler(registry *llm.Registry, repo repository.MessageRepository, cache repository.MessageCache) *ChatHandler {
	return &ChatHandler{registry: registry, repo: repo, cache: cache}
}

func (h *ChatHandler) Schema() string {
	return chatPostSchema
}

func (h *ChatHandler) PostMessage(w http.ResponseWriter, r *http.Request) error {
	sessionID := chi.URLParam(r, param.SessionID)

	req := chatRequest{
		Model:     param.DefaultModel,
		MaxTokens: param.DefaultMaxTokens,
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return HTTPError{Code: http.StatusBadRequest, Message: "failed to decode body"}
	}

	client, err := h.registry.For(req.Model)
	if err != nil {
		slog.Warn("model not available", "session_id", sessionID, "model", req.Model)
		return HTTPError{Code: http.StatusBadRequest, Message: fmt.Sprintf("model %q not available", req.Model)}
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		return HTTPError{Code: http.StatusInternalServerError, Message: "streaming not supported"}
	}

	history := h.loadHistory(r, sessionID)

	slog.Info("chat request", "session_id", sessionID, "model", req.Model, "history_len", len(history))

	userMessage := model.Message{
		Role:      model.RoleUser,
		Content:   req.Message,
		CreatedAt: time.Now(),
	}

	messages := make([]model.Message, 0, len(history)+2)
	if req.SystemPrompt != "" {
		messages = append(messages, model.Message{Role: model.RoleSystem, Content: req.SystemPrompt})
	}
	messages = append(messages, history...)
	messages = append(messages, userMessage)

	chat := model.Chat{Messages: messages}

	var fullResponse strings.Builder
	var doneChunk model.MessageChunk
	var sseStarted bool

	// SSE headers are sent lazily on the first chunk so that pre-stream errors
	// (auth failure, rate limit) can still be returned as plain JSON.
	writeSSE := func(data string) {
		if !sseStarted {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.WriteHeader(http.StatusOK)
			sseStarted = true
		}
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	streamErr := client.SendMessage(r.Context(), chat, func(chunk model.MessageChunk) error {
		switch chunk.Event {
		case "chunk":
			fullResponse.WriteString(chunk.Text)
			payload, _ := json.Marshal(map[string]string{"event": "chunk", "text": chunk.Text})
			writeSSE(string(payload))

		case "done":
			doneChunk = chunk
			payload, _ := json.Marshal(map[string]any{
				"event": "done",
				"metadata": map[string]any{
					"tokens_used": chunk.TokensUsed,
					"model":       chunk.Model,
				},
			})
			writeSSE(string(payload))
		}
		return nil
	})

	if streamErr != nil {
		if !sseStarted {
			slog.Error("stream failed before first chunk", "session_id", sessionID, "model", req.Model, "error", streamErr)
			return HTTPError{Code: http.StatusBadGateway, Message: streamErr.Error()}
		}
		slog.Error("stream error mid-flight", "session_id", sessionID, "model", req.Model, "error", streamErr)
		payload, _ := json.Marshal(map[string]string{"event": "error", "message": streamErr.Error()})
		writeSSE(string(payload))
		return nil
	}

	slog.Info("stream complete", "session_id", sessionID, "model", doneChunk.Model, "tokens", doneChunk.TokensUsed)

	assistantMessage := model.Message{
		Role:        model.RoleAssistant,
		Content:     fullResponse.String(),
		InputToken:  doneChunk.TokensUsed,
		OutputToken: doneChunk.TokensUsed,
		CreatedAt:   time.Now(),
	}

	h.persist(r, sessionID, userMessage, assistantMessage)

	return nil
}

// loadHistory fetches the sliding window context from Redis, falling back to Postgres on miss or error.
func (h *ChatHandler) loadHistory(r *http.Request, sessionID string) []model.Message {
	msgs, err := h.cache.GetRecentMessages(r.Context(), sessionID)
	if err != nil {
		slog.Warn("redis cache miss, falling back to postgres", "session_id", sessionID, "error", err)
	}
	if len(msgs) > 0 {
		return msgs
	}

	msgs, err = h.repo.GetRecentMessages(r.Context(), sessionID, param.DefaultWindowSize)
	if err != nil {
		slog.Warn("failed to load history from postgres", "session_id", sessionID, "error", err)
		return nil
	}
	return msgs
}

// persist saves messages to Postgres and updates the Redis sliding window.
func (h *ChatHandler) persist(r *http.Request, sessionID string, msgs ...model.Message) {
	if err := h.repo.SaveMessages(r.Context(), sessionID, msgs); err != nil {
		slog.Error("failed to save messages to postgres", "session_id", sessionID, "error", err)
	}
	if err := h.cache.PushMessages(r.Context(), sessionID, msgs); err != nil {
		slog.Error("failed to push messages to redis", "session_id", sessionID, "error", err)
	}
}
