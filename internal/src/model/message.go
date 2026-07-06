package model

import "time"

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

type Message struct {
	ID          int64     `json:"id"`
	Role        Role      `json:"role"`
	Content     string    `json:"content"`
	InputToken  int64     `json:"input_tokens"`
	OutputToken int64     `json:"output_tokens"`
	CreatedAt   time.Time `json:"created_at"`
}

type Chat struct {
	SessionID     int64     `json:"session_id"`
	Messages      []Message `json:"messages"`
	LastModelUsed string    `json:"last_model_used"`
	TokensUsed    int64     `json:"tokens_used"`
}

type MessageChunk struct {
	Event        string `json:"event"`         // "chunk" | "done" | "error"
	Text         string `json:"text"`          // populated on "chunk"
	InputTokens  int64  `json:"input_tokens"`  // prompt tokens, populated on "done"
	OutputTokens int64  `json:"output_tokens"` // completion tokens, populated on "done"
	Model        string `json:"model"`         // populated on "done"
	Err          error  `json:"error"`         // non-nil signals stream error
}
