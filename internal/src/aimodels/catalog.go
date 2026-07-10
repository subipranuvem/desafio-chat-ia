package aimodels

import (
	"context"
	"fmt"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/llm"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

type modelDef struct {
	id            string
	provider      string
	info          model.ModelInfo
	clientFactory func() (llm.LLMClient, error)
}

func catalog(ctx context.Context, cfg model.Config) []modelDef {
	return []modelDef{
		{
			id:       "gemini-2.5-flash",
			provider: "google",
			info: model.ModelInfo{
				ID:            "gemini-2.5-flash",
				Name:          "Gemini 2.5 Flash",
				Provider:      "google",
				ContextWindow: 1000000,
				Description:   "Fast and capable multimodal model with a 1M token context window.",
			},
			clientFactory: func() (llm.LLMClient, error) {
				return llm.NewGeminiClient(ctx, cfg.GeminiAPIKey, "gemini-2.5-flash")
			},
		},
		{
			id:       "gemini-3.5-flash",
			provider: "google",
			info: model.ModelInfo{
				ID:            "gemini-3.5-flash",
				Name:          "Gemini 3.5 Flash",
				Provider:      "google",
				ContextWindow: 1000000,
				Description:   "Next-generation Flash model optimized for speed and efficiency.",
			},
			clientFactory: func() (llm.LLMClient, error) {
				return llm.NewGeminiClient(ctx, cfg.GeminiAPIKey, "gemini-3.5-flash")
			},
		},
		{
			id:       "gemini-3.1-flash-lite",
			provider: "google",
			info: model.ModelInfo{
				ID:            "gemini-3.1-flash-lite",
				Name:          "Gemini 3.1 Flash Lite",
				Provider:      "google",
				ContextWindow: 1000000,
				Description:   "Lightweight model for low-latency, cost-sensitive tasks.",
			},
			clientFactory: func() (llm.LLMClient, error) {
				return llm.NewGeminiClient(ctx, cfg.GeminiAPIKey, "gemini-3.1-flash-lite")
			},
		},
		{
			id:       "deepseek-v4-flash",
			provider: "deepseek",
			info: model.ModelInfo{
				ID:            "deepseek-v4-flash",
				Name:          "DeepSeek V4 Flash",
				Provider:      "deepseek",
				ContextWindow: 64000,
				Description:   "Fast and economic model for everyday reasoning tasks.",
			},
			clientFactory: func() (llm.LLMClient, error) {
				return llm.NewOpenAIClient(cfg.DeepSeekAPIKey, "https://api.deepseek.com/v1", "deepseek-v4-flash")
			},
		},
		{
			id:       "deepseek-v4-pro",
			provider: "deepseek",
			info: model.ModelInfo{
				ID:            "deepseek-v4-pro",
				Name:          "DeepSeek V4 Pro",
				Provider:      "deepseek",
				ContextWindow: 64000,
				Description:   "High-performance model for complex reasoning and code generation.",
			},
			clientFactory: func() (llm.LLMClient, error) {
				return llm.NewOpenAIClient(cfg.DeepSeekAPIKey, "https://api.deepseek.com/v1", "deepseek-v4-pro")
			},
		},
	}
}

// Register creates LLM clients for all models whose API key is present,
// registers them in the registry, and returns the available model infos.
func Register(ctx context.Context, cfg model.Config, registry *llm.Registry) ([]model.ModelInfo, error) {
	missingKey := map[string]bool{
		"google":   cfg.GeminiAPIKey == "",
		"deepseek": cfg.DeepSeekAPIKey == "",
	}

	var available []model.ModelInfo
	for _, m := range catalog(ctx, cfg) {
		if missingKey[m.provider] {
			continue
		}
		client, err := m.clientFactory()
		if err != nil {
			return nil, fmt.Errorf("create client %s: %w", m.id, err)
		}
		registry.Register(m.id, client)
		available = append(available, m.info)
	}
	return available, nil
}
