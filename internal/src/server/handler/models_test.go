package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/server/handler"
)

func TestModelsHandler_GetModels(t *testing.T) {
	t.Run("returns registered models", func(t *testing.T) {
		models := []model.ModelInfo{
			{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Provider: "google", ContextWindow: 1000000, Description: "Fast model"},
			{ID: "deepseek-v4-flash", Name: "DeepSeek V4 Flash", Provider: "deepseek", ContextWindow: 64000, Description: "Economic model"},
		}
		h := handler.NewModelsHandler(models)
		req := httptest.NewRequest(http.MethodGet, "/chat/models", nil)
		w := httptest.NewRecorder()

		handler.Adapt(h.GetModels)(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var resp map[string][]model.ModelInfo
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		require.Len(t, resp["models"], 2)
		require.Equal(t, "gemini-2.5-flash", resp["models"][0].ID)
		require.Equal(t, "google", resp["models"][0].Provider)
		require.Equal(t, "deepseek-v4-flash", resp["models"][1].ID)
	})

	t.Run("returns empty array when no models registered", func(t *testing.T) {
		h := handler.NewModelsHandler([]model.ModelInfo{})
		req := httptest.NewRequest(http.MethodGet, "/chat/models", nil)
		w := httptest.NewRecorder()

		handler.Adapt(h.GetModels)(w, req)

		require.Equal(t, http.StatusOK, w.Code)

		var resp map[string][]model.ModelInfo
		require.NoError(t, json.NewDecoder(w.Body).Decode(&resp))
		require.Empty(t, resp["models"])
	})
}
