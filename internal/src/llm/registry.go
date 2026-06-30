package llm

import "fmt"

type Registry struct {
	clients map[string]LLMClient
}

func NewRegistry() *Registry {
	return &Registry{clients: make(map[string]LLMClient)}
}

func (r *Registry) Register(modelID string, client LLMClient) {
	r.clients[modelID] = client
}

func (r *Registry) For(modelID string) (LLMClient, error) {
	c, ok := r.clients[modelID]
	if !ok {
		return nil, fmt.Errorf("model %q not registered", modelID)
	}
	return c, nil
}
