package llm

import (
	"context"
	"errors"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

type OpenAIClient struct {
	client  openai.Client
	modelID string
}

func NewOpenAIClient(apiKey, baseURL, modelID string) (*OpenAIClient, error) {
	if apiKey == "" {
		return nil, errors.New("openai: api key required")
	}
	if modelID == "" {
		return nil, errors.New("openai: model id required")
	}

	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if baseURL != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	return &OpenAIClient{
		client:  openai.NewClient(opts...),
		modelID: modelID,
	}, nil
}

func (c *OpenAIClient) SendMessage(ctx context.Context, chat model.Chat, onChunk func(model.MessageChunk) error) error {
	msgs := buildOpenAIMessages(chat)

	stream := c.client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model:    c.modelID,
		Messages: msgs,
	})
	defer stream.Close()

	acc := openai.ChatCompletionAccumulator{}

	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			if err := onChunk(model.MessageChunk{
				Event: "chunk",
				Text:  chunk.Choices[0].Delta.Content,
			}); err != nil {
				return err
			}
		}
	}

	if err := stream.Err(); err != nil {
		return err
	}

	return onChunk(model.MessageChunk{
		Event:      "done",
		Model:      c.modelID,
		TokensUsed: acc.Usage.TotalTokens,
	})
}

func buildOpenAIMessages(chat model.Chat) []openai.ChatCompletionMessageParamUnion {
	msgs := make([]openai.ChatCompletionMessageParamUnion, 0, len(chat.Messages))
	for _, m := range chat.Messages {
		switch m.Role {
		case model.RoleUser:
			msgs = append(msgs, openai.UserMessage(m.Content))
		case model.RoleAssistant:
			msgs = append(msgs, openai.AssistantMessage(m.Content))
		case model.RoleSystem:
			msgs = append(msgs, openai.SystemMessage(m.Content))
		}
	}
	return msgs
}
