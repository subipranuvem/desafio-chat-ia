package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/database"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/llm"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/repository"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/server"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/server/handler"
	"github.com/subipranuvem/desafio-chat-ia/migrations"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))

	cfg, err := model.LoadConfig()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	pg := database.NewPostgresDB()
	if err := pg.Connect(ctx, cfg.PostgresDSN); err != nil {
		slog.Error("postgres connect failed", "error", err)
		os.Exit(1)
	}
	defer pg.Close()
	if err := pg.Ping(ctx); err != nil {
		slog.Error("postgres ping failed", "error", err)
		os.Exit(1)
	}
	slog.Info("postgres connected")

	if err := database.RunMigrations(pg.Pool(), migrations.FS); err != nil {
		slog.Error("migrations failed", "error", err)
		os.Exit(1)
	}
	slog.Info("migrations applied")

	rdb := database.NewRedisDB()
	if err := rdb.Connect(ctx, cfg.RedisDSN); err != nil {
		slog.Error("redis connect failed", "error", err)
		os.Exit(1)
	}
	defer rdb.Close()
	if err := rdb.Ping(ctx); err != nil {
		slog.Error("redis ping failed", "error", err)
		os.Exit(1)
	}
	slog.Info("redis connected")

	go func() {
		ticker := time.NewTicker(time.Duration(cfg.PingDatabaseIntervalInMillis) * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if err := pg.Ping(ctx); err != nil {
				slog.Error("postgres ping failed", "error", err)
				continue
			}
			s := pg.Stats()
			slog.Info("postgres pool stats",
				"total_conns", s.TotalConns,
				"acquired_conns", s.AcquiredConns,
				"idle_conns", s.IdleConns,
				"max_conns", s.MaxConns,
			)
		}
	}()

	go func() {
		ticker := time.NewTicker(time.Duration(cfg.PingDatabaseIntervalInMillis) * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			if err := rdb.Ping(ctx); err != nil {
				slog.Error("redis ping failed", "error", err)
				continue
			}
			s := rdb.Stats()
			slog.Info("redis pool stats",
				"total_conns", s.TotalConns,
				"idle_conns", s.IdleConns,
				"hits", s.Hits,
				"misses", s.Misses,
				"timeouts", s.Timeouts,
			)
		}
	}()

	type modelDef struct {
		id            string
		info          handler.ModelInfo
		clientFactory func() (llm.LLMClient, error)
	}

	allModels := []modelDef{
		{
			id: "gemini-2.5-flash",
			info: handler.ModelInfo{
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
			id: "gemini-3.5-flash",
			info: handler.ModelInfo{
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
			id: "gemini-3.1-flash-lite",
			info: handler.ModelInfo{
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
			id: "deepseek-v4-flash",
			info: handler.ModelInfo{
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
			id: "deepseek-v4-pro",
			info: handler.ModelInfo{
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

	registry := llm.NewRegistry()
	var availableModels []handler.ModelInfo

	for _, m := range allModels {
		needsGemini := m.info.Provider == "google" && cfg.GeminiAPIKey == ""
		needsDeepSeek := m.info.Provider == "deepseek" && cfg.DeepSeekAPIKey == ""
		if needsGemini || needsDeepSeek {
			continue
		}
		client, err := m.clientFactory()
		if err != nil {
			slog.Error("failed to create llm client", "model", m.id, "error", err)
			os.Exit(1)
		}
		registry.Register(m.id, client)
		availableModels = append(availableModels, m.info)
		slog.Info("llm client registered", "model", m.id)
	}

	repo := repository.NewPostgresMessageRepository(pg)
	sessionTTL := time.Duration(cfg.RedisSessionTTLInMillis) * time.Millisecond
	cache := repository.NewRedisMessageCache(rdb, sessionTTL)

	srv := server.New(server.Config{
		Addr:                ":8000",
		Registry:            registry,
		Repo:                repo,
		Cache:               cache,
		ContextWindowTokens: cfg.ContextWindowTokens,
		Models:              availableModels,
	})

	slog.Info("server listening", "addr", ":8000")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}
