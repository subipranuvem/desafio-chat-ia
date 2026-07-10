package handler

import (
	"encoding/json"
	"net/http"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

type ModelsHandler struct {
	models []model.ModelInfo
}

func NewModelsHandler(models []model.ModelInfo) *ModelsHandler {
	return &ModelsHandler{models: models}
}

func (h *ModelsHandler) GetModels(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string][]model.ModelInfo{"models": h.models})
}
