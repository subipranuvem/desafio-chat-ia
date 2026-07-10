package llm

import (
	"context"
	"errors"

	"google.golang.org/genai"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

type GeminiClient struct {
	client  *genai.Client
	modelID string
}

func NewGeminiClient(ctx context.Context, apiKey, modelID string) (*GeminiClient, error) {
	if apiKey == "" {
		return nil, errors.New("gemini: api key required")
	}
	if modelID == "" {
		return nil, errors.New("gemini: model id required")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}
	return &GeminiClient{client: client, modelID: modelID}, nil
}

func (c *GeminiClient) CountTokens(ctx context.Context, message model.Message) (int64, error) {
	contents, _ := buildGeminiContents(model.Chat{Messages: []model.Message{message}})
	if len(contents) == 0 {
		return 0, nil
	}
	result, err := c.client.Models.CountTokens(ctx, c.modelID, contents, nil)
	if err != nil {
		return 0, err
	}
	return int64(result.TotalTokens), nil
}

func (c *GeminiClient) SendMessage(ctx context.Context, chat model.Chat, onChunk func(model.MessageChunk) error) error {
	contents, sysPrompt := buildGeminiContents(chat)

	config := &genai.GenerateContentConfig{}
	if sysPrompt != "" {
		config.SystemInstruction = genai.NewContentFromText(sysPrompt, genai.RoleUser)
	}

	var inputTokens, outputTokens int64

	for resp, err := range c.client.Models.GenerateContentStream(ctx, c.modelID, contents, config) {
		if err != nil {
			return err
		}

		// UsageMetadata with complete counts is only present on the last streaming chunk.
		if resp.UsageMetadata != nil {
			inputTokens = int64(resp.UsageMetadata.PromptTokenCount)
			outputTokens = int64(resp.UsageMetadata.CandidatesTokenCount)
		}

		for _, cand := range resp.Candidates {
			if cand.Content == nil {
				continue
			}
			for _, part := range cand.Content.Parts {
				if part.Text != "" {
					if err := onChunk(model.MessageChunk{Event: "chunk", Text: part.Text}); err != nil {
						return err
					}
				}
			}
		}
	}

	return onChunk(model.MessageChunk{
		Event:        "done",
		Model:        c.modelID,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
	})
}

func buildGeminiContents(chat model.Chat) ([]*genai.Content, string) {
	var contents []*genai.Content
	var sysPrompt string

	for _, m := range chat.Messages {
		switch m.Role {
		case model.RoleSystem:
			sysPrompt = m.Content
		case model.RoleUser:
			contents = append(contents, genai.NewContentFromText(m.Content, genai.RoleUser))
		case model.RoleAssistant:
			contents = append(contents, genai.NewContentFromText(m.Content, genai.RoleModel))
		}
	}

	return contents, sysPrompt
}
