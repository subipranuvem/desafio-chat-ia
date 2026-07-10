package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/aimodels"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/database"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/llm"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/repository"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/server"
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

	registry := llm.NewRegistry()
	availableModels, err := aimodels.Register(ctx, cfg, registry)
	if err != nil {
		slog.Error("failed to register llm clients", "error", err)
		os.Exit(1)
	}
	slog.Info("llm clients registered", "count", len(availableModels))

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
