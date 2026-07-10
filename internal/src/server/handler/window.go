package handler

import (
	"slices"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

// buildWindow returns the newest messages whose combined estimated token cost fits within contextWindowTokens.
// Iterates newest→oldest; stops when adding the next message would exceed the limit.
// Token cost is estimated as len(Content)/4 (1 token ≈ 4 chars).
// Returns messages in chronological (ASC) order.
func buildWindow(msgs []model.Message, contextWindowTokens int) []model.Message {
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
