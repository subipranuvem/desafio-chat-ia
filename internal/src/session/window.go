package session

import (
	"slices"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

// BuildWindow returns the newest messages that fit within the token budget.
// Uses len/4 as a byte-to-token approximation.
func BuildWindow(msgs []model.Message, contextWindowTokens int) []model.Message {
	var total int64
	for i, msg := range slices.Backward(msgs) {
		cost := int64(len(msg.Content) / 4)
		if total+cost > int64(contextWindowTokens) {
			return msgs[i+1:]
		}
		total += cost
	}
	return msgs
}
