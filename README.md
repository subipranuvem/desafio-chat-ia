# Technical Challenge — AI Chat API

Back-end challenge solution for SubiPraNuvem. AI-assisted chat API with history managed via **Sliding Window**, real-time delivery via **SSE**, and support for multiple LLM providers.

## Running

### Prerequisites

- Go 1.26+
- Docker and Docker Compose

### 1. Set up environment variables

```bash
cp .env.example .env
# Edit .env with your API keys
```

### 2. Start PostgreSQL and Redis

```bash
docker compose up -d
```

### 3. Run the API

```bash
go run main.go
```

API available at `http://localhost:8000`.

## Makefile targets

```bash
make run          # run locally
make test         # run tests
make test-race    # run tests with race detector
make build        # build binary to bin/
make docker-build # build Docker image
make docker-run   # run Docker container (reads from .env)
make gosec        # static security analysis
make trivy        # filesystem vulnerability scan (via Docker)
make sec-check    # run gosec + trivy
```

## Environment variables

| Variable | Default | Description |
|---|---|---|
| `POSTGRES_DSN` | — | PostgreSQL connection string (required) |
| `REDIS_DSN` | — | Redis connection string (required) |
| `GEMINI_API_KEY` | — | Google Gemini API key |
| `DEEPSEEK_API_KEY` | — | DeepSeek API key |
| `PING_DATABASE_INTERVAL_IN_MILLIS` | `60000` | Interval between Postgres and Redis health checks (ms) |
| `REDIS_SESSION_TTL_IN_MILLIS` | `7200000` | Session TTL in Redis; refreshed on every message (ms) |
| `CONTEXT_WINDOW_TOKENS` | `8000` | Maximum context window sent to the LLM (in tokens) |

At least one LLM API key must be set for the server to process messages.
