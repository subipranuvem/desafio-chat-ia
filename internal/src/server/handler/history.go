package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/server/param"
)

type historyResponse struct {
	Data   any   `json:"data"`
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

func (h *ChatHandler) GetHistory(w http.ResponseWriter, r *http.Request) error {
	sessionID := chi.URLParam(r, param.SessionID)

	limit := parseQueryInt(r, param.QueryLimit, 20)
	if limit > 100 {
		limit = 100
	}
	offset := parseQueryInt(r, param.QueryOffset, 0)

	messages, total, err := h.repo.GetMessages(r.Context(), sessionID, limit, offset)
	if err != nil {
		return HTTPError{Code: http.StatusInternalServerError, Message: "failed to fetch messages"}
	}

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(historyResponse{
		Data:   messages,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

func parseQueryInt(r *http.Request, key string, defaultVal int) int {
	raw := r.URL.Query().Get(key)
	if raw == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return defaultVal
	}
	return n
}
