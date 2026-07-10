package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/llm"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/repository"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/server/handler"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/server/middleware"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/server/param"
)

type Config struct {
	Addr                string
	Registry            *llm.Registry
	Repo                repository.MessageRepository
	Cache               repository.MessageCache
	ContextWindowTokens int
	Models              []model.ModelInfo
}

func New(cfg Config) *http.Server {
	r := chi.NewRouter()

	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(middleware.Gzip())
	r.Use(middleware.ErrorHandler(middleware.DefaultErrors))

	chat := handler.NewChatHandler(cfg.Registry, cfg.Repo, cfg.Cache, cfg.ContextWindowTokens)
	models := handler.NewModelsHandler(cfg.Models)

	r.Route("/chat", func(r chi.Router) {
		r.Get("/models", handler.Adapt(models.GetModels))

		r.With(middleware.ValidateSchema(chat.Schema())).
			Post("/session/{"+param.SessionID+"}", handler.Adapt(chat.PostMessage))

		r.Get("/session/{"+param.SessionID+"}", handler.Adapt(chat.GetHistory))
	})

	return &http.Server{
		Addr:    cfg.Addr,
		Handler: r,
	}
}
