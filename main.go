package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

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

	interval := time.Duration(cfg.PingDatabaseIntervalInMillis) * time.Millisecond
	database.PingPostgresEventually(ctx, pg, interval)
	database.PingRedisEventually(ctx, rdb, interval)

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

	addr := fmt.Sprintf(":%d", cfg.Port)
	srv := server.New(server.Config{
		Addr:                addr,
		Registry:            registry,
		Repo:                repo,
		Cache:               cache,
		ContextWindowTokens: cfg.ContextWindowTokens,
		Models:              availableModels,
	})

	go func() {
		slog.Info("server listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	stop() // release signal handler so a second signal kills immediately

	slog.Info("shutdown signal received, draining connections")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
	slog.Info("shutdown complete")
}
