# Desafio Técnico — API de Chat com IA

Resolução do desafio técnico de back-end da SubiPraNuvem. API de chat assistido por IA com histórico gerenciado via **Sliding Window**, entrega em tempo real por **SSE** e suporte a múltiplos provedores de LLM.

## Stack

| Camada | Tecnologia |
|---|---|
| Linguagem | Go 1.26 |
| HTTP | Chi v5 |
| LLMs | Google Gemini · DeepSeek (via API OpenAI-compatível) |
| Banco relacional | PostgreSQL |
| Cache / sessão | Redis |
| Containerização | Docker · Docker Compose |


## Endpoints

| Método | Rota | Descrição |
|---|---|---|
| `POST` | `/chat/session/{session-id}` | Envia mensagem, retorna resposta em SSE |
| `GET` | `/chat/session/{session-id}` | Histórico paginado da sessão (PostgreSQL) |
| `GET` | `/chat/models` | Lista modelos disponíveis |

### POST `/chat/session/{session-id}`

```json
{
  "message": "Olá!",
  "model": "gemini-2.5-flash",
  "system_prompt": "Você é um assistente técnico.",
  "max_tokens": 4000
}
```

Resposta em SSE:

```
data: {"event": "chunk", "text": "Olá"}
data: {"event": "chunk", "text": "! Como posso ajudar?"}
data: {"event": "done", "metadata": {"tokens_used": 18, "model": "gemini-2.5-flash"}}
```

Erros retornam JSON (não SSE):

```json
{"status_code": 429, "message": "LLM API rate limit exceeded. Try again later.", "error": "..."}
```

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
| `REDIS_SESSION_TTL_IN_MILLIS` | `180000` | TTL das sessões no Redis; renovado a cada mensagem enviada |

Pelo menos uma API key de LLM deve estar configurada para o servidor processar mensagens.
