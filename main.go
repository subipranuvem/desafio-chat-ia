package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/llm"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/server"
)

// noopRepo satisfies repository.MessageRepository until DB persistence is implemented.
type noopRepo struct{}

func (n *noopRepo) SaveMessages(_ context.Context, _ string, _ []model.Message) error {
	return nil
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := model.LoadConfig()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	registry := llm.NewRegistry()

	if cfg.GeminiAPIKey != "" {
		gemini, err := llm.NewGeminiClient(ctx, cfg.GeminiAPIKey, "gemini-2.5-flash")
		if err != nil {
			slog.Error("failed to create gemini client", "error", err)
			os.Exit(1)
		}
		models := []string{
			"gemini-2.5-flash",
			"gemini-3.5-flash",
			"gemini-3.1-flash-lite",
		}
		for _, model := range models {
			registry.Register(model, gemini)
			slog.Info("llm client registered", "model", model)
		}

		slog.Info("llm client registered", "model", "gemini-2.5-flash")
	}

	if cfg.DeepSeekAPIKey != "" {
		deepseek, err := llm.NewOpenAIClient(cfg.DeepSeekAPIKey, "https://api.deepseek.com/v1", "deepseek-chat")
		if err != nil {
			slog.Error("failed to create deepseek client", "error", err)
			os.Exit(1)
		}
		models := []string{
			"deepseek-v4-flash",
			"deepseek-v4-pro",
		}
		for _, model := range models {
			registry.Register(model, deepseek)
			slog.Info("llm client registered", "model", model)
		}
	}

	srv := server.New(server.Config{
		Addr:     ":8000",
		Registry: registry,
		Repo:     &noopRepo{},
	})

	slog.Info("server listening", "addr", ":8000")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
