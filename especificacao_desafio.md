
# Desafio Técnico Back-End: API de Chat com IA (Sliding Window & SSE)

## 1. Contexto do Projeto
O objetivo deste desafio é construir o motor de backend para um sistema de chat assistido por Inteligência Artificial (IA). O foco principal é criar uma API performática, escalável e resiliente, capaz de gerenciar o histórico de conversas utilizando a estratégia de **Janela Deslizante (Sliding Window)** para otimizar o uso de tokens das LLMs, entregando as respostas em tempo real via **Server-Sent Events (SSE)**.

---

## 2. Stack Tecnológica Obrigatória
* **Linguagem:** À sua escolha (Sugerido: Go, Node.js ou Python).
* **Banco de Dados Relacional:** PostgreSQL (Armazenamento histórico completo e persistência).
* **Banco de Dados em Memória / Cache:** Redis (Gerenciamento de contexto rápido e controle de sessão).
* **Containerização:** Docker e Docker Compose.

---

## 3. Requisitos Arquiteturais e de Negócio

### 3.1 Estratégia de Janela Deslizante (Sliding Window)
Para evitar estourar o limite de contexto da IA e controlar os custos por clique, a API não deve enviar o histórico completo de mensagens armazenadas no Postgres para a LLM. 

1. Ao receber uma nova mensagem, o sistema deve recuperar as mensagens recentes direto do **Redis**.
2. O sistema deve calcular o tamanho acumulado em tokens **de trás para frente** (da mensagem mais recente para a mais antiga).
3. Apenas as mensagens que couberem dentro do parâmetro `max_tokens` especificado na sessão devem ser enviadas no payload da IA.
4. **Regra Crítica:** O `system_prompt` (se configurado) deve **sempre** ser injetado na primeira posição do payload enviado à IA, independentemente de quantas mensagens a janela deslizante cortar.

### 3.2 Entrega em Tempo Real (SSE)
O endpoint de envio de mensagem deve obrigatoriamente responder utilizando o protocolo **Server-Sent Events (SSE)**. A resposta da IA deve ser transmitida em *chunks* conforme é gerada, garantindo baixa latência perceptível para o usuário final.

### 3.3 Persistência de Dados
* Toda mensagem enviada pelo usuário e a resposta completa gerada pela IA devem ser salvas de forma íntegra no **PostgreSQL** para fins de histórico e auditoria.
* O cache do **Redis** deve ser atualizado simultaneamente para manter a janela deslizante rápida nas próximas interações. Defina um TTL (Time-To-Live) de **2 horas** para sessões inativas no Redis.

---

## 4. Especificação dos Endpoints (Rotas da API)

### 4.1 `GET /chat/models`
Retorna a lista de modelos de IA disponíveis no sistema que o usuário pode escolher ao enviar uma mensagem.

#### Resposta Esperada (`200 OK`)
```json
{
  "models": [
    {
      "id": "gpt-4o-mini",
      "name": "GPT-4o Mini",
      "provider": "openai",
      "context_window": 128000,
      "description": "Modelo rápido e econômico para tarefas cotidianas."
    },
    {
      "id": "gpt-4o",
      "name": "GPT-4o",
      "provider": "openai",
      "context_window": 128000,
      "description": "Modelo de alta performance para raciocínio complexo."
    },
    {
      "id": "claude-3-5-sonnet",
      "name": "Claude 3.5 Sonnet",
      "provider": "anthropic",
      "context_window": 200000,
      "description": "Modelo estado-da-arte em geração de código e análise."
    }
  ]
}

```

---

### 4.2 `POST /chat/session/{session-id}`

Envia uma nova mensagem para uma sessão de chat existente ou inicializa uma nova sessão caso o `session-id` informado não exista no sistema.

* **Headers Obrigatórios:**
* `Content-Type: application/json`
* `Accept: text/event-stream`



#### JSON Schema de Entrada

```json
{
  "$schema": "[http://json-schema.org/draft-07/schema#](http://json-schema.org/draft-07/schema#)",
  "title": "ChatMessageRequest",
  "type": "object",
  "properties": {
    "message": {
      "type": "string",
      "minLength": 1,
      "description": "O conteúdo da mensagem enviada pelo usuário."
    },
    "model": {
      "type": "string",
      "default": "gpt-4o-mini",
      "description": "ID do modelo a ser utilizado (conforme listado no endpoint /chat/models)."
    },
    "system_prompt": {
      "type": "string",
      "description": "Prompt de sistema opcional para guiar o comportamento da IA nesta sessão."
    },
    "max_tokens": {
      "type": "integer",
      "minimum": 500,
      "default": 4000,
      "description": "Tamanho máximo (em tokens) que a janela deslizante pode ocupar nesta chamada."
    }
  },
  "required": ["message"]
}

```

#### Resposta Esperada (`200 OK` - Stream SSE)

```text
HTTP/1.1 200 OK
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive

data: {"event": "chunk", "text": "Olá"}

data: {"event": "chunk", "text": "!"}

data: {"event": "chunk", "text": " Como posso ajudar"}

data: {"event": "done", "metadata": {"tokens_used": 45, "model": "gpt-4o-mini"}}

```

---

### 4.3 `GET /chat/session/{session-id}`

Recupera o histórico completo e paginado de mensagens daquela sessão específica, buscando as informações direto do banco de dados relacional (PostgreSQL).

* **Query Parameters:**
* `limit` (integer, default: 20) — Quantidade de mensagens a retornar por página.
* `offset` (integer, default: 0) — Quantidade de registros a pular.



#### Resposta Esperada (`200 OK`)

```json
{
  "session_id": "sessao-xyz-123",
  "pagination": {
    "limit": 20,
    "offset": 0,
    "total_records": 142
  },
  "messages": [
    {
      "id": 1052,
      "role": "user",
      "content": "Qual a melhor estrutura para um backend de chat?",
      "tokens_count": 12,
      "created_at": "2026-06-25T20:00:00Z"
    },
    {
      "id": 1053,
      "role": "assistant",
      "content": "O ideal é utilizar uma estrutura de lista cronológica combinada com cache...",
      "tokens_count": 84,
      "created_at": "2026-06-25T20:00:05Z"
    }
  ]
}

```

---

## 5. Fluxo de Execução da Janela Deslizante

Para fins de implementação do algoritmo de corte de contexto, siga a árvore de decisão abaixo a cada nova requisição `POST`:

```
[Nova Mensagem do Usuário]
          │
          ▼
[Buscar Histórico Recente no Redis]
          │
          ▼
[Iterar Histórico do Fim para o Início] ◄──────────────────────┐
          │                                                    │
          ├─► (Soma de Tokens + Próxima Mensagem <= max_tokens) ─┘
          │
          └─► (Soma de Tokens Estourou o max_tokens)
                      │
                      ▼
            [Cortar Mensagens Antigas]
                      │
                      ▼
         [Injetar System Prompt no Topo]
                      │
                      ▼
         [Enviar Payload para a LLM]

```

*(Nota para o candidato: Pode utilizar uma aproximação matemática simples de contagem de tokens, ex: tamanho do texto dividido por 4, para focar estritamente na lógica arquitetural).*

---

## 6. Critérios de Avaliação

| Critério | O que será avaliado |
| --- | --- |
| **Modelagem do Banco** | Estruturação das tabelas no Postgres, uso correto de chaves (PK/FK) e criação de índices para otimizar queries por `session_id`. |
| **Gerenciamento de Cache** | Uso correto das estruturas de dados do Redis para leitura e gravação rápida do contexto. |
| **Lógica do Algoritmo** | Clareza, corretude e eficiência no algoritmo que varre o histórico de trás para frente aplicando a janela deslizante. |
| **Concorrência e SSE** | Tratamento correto de concorrência e fechamento apropriado das conexões HTTP em formato de stream (sem vazamento de memória). |
| **Orquestração** | Arquivos `Dockerfile` e `docker-compose.yml` configurados corretamente para rodar o ambiente completo com um único comando. |
