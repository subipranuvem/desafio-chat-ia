# Desafio Técnico — API de Chat com IA

Resolução do desafio técnico de back-end da SubiPraNuvem. API de chat assistido por IA com histórico gerenciado via **Sliding Window**, entrega em tempo real por **SSE** e suporte a múltiplos provedores de LLM.

## Como rodar

### Pré-requisitos

- Go 1.26+
- Docker e Docker Compose

### 1. Configure as variáveis de ambiente

```bash
cp .env.example .env
# Edite .env com suas API keys
```

### 2. Suba o PostgreSQL e Redis

```bash
docker compose up -d
```

### 3. Execute a API

```bash
go run main.go
```

API disponível em `http://localhost:8000`.

## Testes

```bash
go test ./...
```

## Variáveis de ambiente

| Variável | Padrão | Descrição |
|---|---|---|
| `POSTGRES_DSN` | — | Connection string do PostgreSQL (obrigatória) |
| `REDIS_DSN` | — | Connection string do Redis (obrigatória) |
| `GEMINI_API_KEY` | — | API key do Google Gemini |
| `DEEPSEEK_API_KEY` | — | API key do DeepSeek |
| `PING_DATABASE_INTERVAL_IN_MILLIS` | `60000` | Intervalo entre health checks de Postgres e Redis |
| `REDIS_SESSION_TTL_IN_MILLIS` | `7200000` | TTL das sessões no Redis; renovado a cada mensagem enviada |
| `CONTEXT_WINDOW_TOKENS` | `8000` | Tamanho máximo da janela de contexto enviada ao LLM (em tokens) |

Pelo menos uma API key de LLM deve estar configurada para o servidor processar mensagens.
