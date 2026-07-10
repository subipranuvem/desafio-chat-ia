package handler

import (
	"context"
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
	registry            *llm.Registry
	repo                repository.MessageRepository
	cache               repository.MessageCache
	loader              ConversationLoader
	contextWindowTokens int
}

type countResult struct {
	tokens int64
	err    error
}

func NewChatHandler(registry *llm.Registry, repo repository.MessageRepository, cache repository.MessageCache, contextWindowTokens int) *ChatHandler {
	warmingLoader := NewCacheWarmingLoader(NewPostgresLoader(repo, nil, contextWindowTokens), cache, contextWindowTokens)
	redisLoader := NewRedisLoader(cache, warmingLoader)
	return &ChatHandler{registry: registry, repo: repo, cache: cache, loader: redisLoader, contextWindowTokens: contextWindowTokens}
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
		return NewHTTPError(http.StatusBadRequest, "failed to decode body")
	}

	client, err := h.registry.For(req.Model)
	if err != nil {
		return NewHTTPError(http.StatusBadRequest, fmt.Sprintf("model %q not available", req.Model)).
			WithLogMessage(fmt.Sprintf("model not available session_id=%s model=%s", sessionID, req.Model))
	}

	history, found, err := h.loader.Load(r.Context(), sessionID)
	if err != nil {
		return NewHTTPError(http.StatusInternalServerError, "failed to load conversation history").
			WithLogMessage(fmt.Sprintf("failed to load conversation history session_id=%s error=%s", sessionID, err))
	}

	if found && req.SystemPrompt != "" {
		return NewHTTPError(http.StatusBadRequest, "cannot override system prompt of existing conversation")
	}

	history = buildWindow(history, h.contextWindowTokens)

	sse, ok := newSSEWriter(w)
	if !ok {
		return NewHTTPError(http.StatusInternalServerError, "streaming not supported")
	}

	slog.Info("chat request", "session_id", sessionID, "model", req.Model, "history_len", len(history))

	userMessage := model.Message{
		Role:      model.RoleUser,
		Content:   req.Message,
		CreatedAt: time.Now(),
	}

	tokenCh := make(chan countResult, 1)
	go func() {
		countCtx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		tokens, err := client.CountTokens(countCtx, userMessage)
		tokenCh <- countResult{tokens, err}
	}()

	var systemMsg *model.Message
	if !found && req.SystemPrompt != "" {
		msg := model.Message{Role: model.RoleSystem, Content: req.SystemPrompt, CreatedAt: time.Now()}
		systemMsg = &msg
	}

	messagesLen := len(history) + 2
	if systemMsg != nil {
		messagesLen++
	}

	messages := make([]model.Message, 0, messagesLen)
	if systemMsg != nil {
		messages = append(messages, *systemMsg)
	}
	messages = append(messages, history...)
	messages = append(messages, userMessage)

	chat := model.Chat{Messages: messages, LastModelUsed: req.Model}

	var fullResponse strings.Builder
	var doneChunk model.MessageChunk

	streamErr := client.SendMessage(r.Context(), chat, func(chunk model.MessageChunk) error {
		switch chunk.Event {
		case sseEventChunk:
			fullResponse.WriteString(chunk.Text)
			sse.WriteChunk(chunk.Text)
		case sseEventDone:
			doneChunk = chunk
			sse.WriteDone(chunk)
		}
		return nil
	})

	if streamErr != nil {
		if !sse.Started() {
			return NewHTTPError(http.StatusBadGateway, streamErr.Error()).
				WithLogMessage(fmt.Sprintf("stream failed before first chunk session_id=%s model=%s error=%s", sessionID, req.Model, streamErr))
		}
		slog.Error("stream error mid-flight", "session_id", sessionID, "model", req.Model, "error", streamErr)
		sse.WriteError(streamErr)
		return nil
	}

	slog.Info("stream complete", "session_id", sessionID, "model", doneChunk.Model,
		"input_tokens", doneChunk.InputTokens, "output_tokens", doneChunk.OutputTokens)

	cr := <-tokenCh
	if cr.err != nil {
		slog.Warn("token count failed, falling back to estimation", "session_id", sessionID, "error", cr.err)
		userMessage.InputToken = int64(len(req.Message) / 4)
	} else {
		userMessage.InputToken = cr.tokens
	}

	assistantMessage := model.Message{
		Role:        model.RoleAssistant,
		Content:     fullResponse.String(),
		OutputToken: doneChunk.OutputTokens,
		CreatedAt:   time.Now(),
	}

	persistMsgs := make([]model.Message, 0, 3)
	if systemMsg != nil {
		persistMsgs = append(persistMsgs, *systemMsg)
	}
	persistMsgs = append(persistMsgs, userMessage, assistantMessage)

	// Persist runs synchronously before the handler returns, keeping the SSE connection
	// open for a few extra milliseconds. The client already received the "done" event and
	// is not waiting on anything useful — the overhead is negligible for typical DB writes.
	// Alternatives (detached goroutine, in-process queue) close the connection faster but
	// risk message loss if the process dies between the return and the write completing.
	h.persist(r, sessionID, persistMsgs...)

	return nil
}

// persist saves messages to Postgres and replaces the Redis window.
// Reads the current Redis state, appends new messages, applies buildWindow, then stores the result.
func (h *ChatHandler) persist(r *http.Request, sessionID string, msgs ...model.Message) {
	if err := h.repo.SaveMessages(r.Context(), sessionID, msgs); err != nil {
		slog.Error("failed to save messages to postgres", "session_id", sessionID, "error", err)
	}

	existing, err := h.cache.GetRecentMessages(r.Context(), sessionID)
	if err != nil {
		slog.Error("failed to get recent messages from redis", "session_id", sessionID, "error", err)
		return
	}

	combined := append(existing, msgs...)
	window := buildWindow(combined, h.contextWindowTokens)
	if err := h.cache.PushMessages(r.Context(), sessionID, window); err != nil {
		slog.Error("failed to push messages to redis", "session_id", sessionID, "error", err)
	}
}
