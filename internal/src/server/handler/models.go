package handler

import (
	"encoding/json"
	"net/http"
)

type ModelInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Provider      string `json:"provider"`
	ContextWindow int    `json:"context_window"`
	Description   string `json:"description"`
}

type ModelsHandler struct {
	models []ModelInfo
}

func NewModelsHandler(models []ModelInfo) *ModelsHandler {
	return &ModelsHandler{models: models}
}

func (h *ModelsHandler) GetModels(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string][]ModelInfo{"models": h.models})
}
