package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/subipranuvem/desafio-chat-ia/internal/src/model"
)

const (
	sseEventChunk = "chunk"
	sseEventDone  = "done"
	sseEventError = "error"
)

type chunkPayload struct {
	Event string `json:"event"`
	Text  string `json:"text"`
}

type doneMetadata struct {
	TokensUsed int64  `json:"tokens_used"`
	Model      string `json:"model"`
}

type donePayload struct {
	Event    string       `json:"event"`
	Metadata doneMetadata `json:"metadata"`
}

type errorPayload struct {
	Event   string `json:"event"`
	Message string `json:"message"`
}

// sseWriter handles lazy SSE header sending and event framing.
// Headers are flushed on the first write so that pre-stream errors can still
// be returned as plain JSON responses.
type sseWriter struct {
	w       http.ResponseWriter
	flusher http.Flusher
	started bool
}

func newSSEWriter(w http.ResponseWriter) (*sseWriter, bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, false
	}
	return &sseWriter{w: w, flusher: flusher}, true
}

func (s *sseWriter) Started() bool { return s.started }

func (s *sseWriter) write(payload any) {
	if !s.started {
		s.w.Header().Set("Content-Type", "text/event-stream")
		s.w.Header().Set("Cache-Control", "no-cache")
		s.w.Header().Set("Connection", "keep-alive")
		s.w.WriteHeader(http.StatusOK)
		s.started = true
	}
	data, _ := json.Marshal(payload)
	fmt.Fprintf(s.w, "data: %s\n\n", data)
	s.flusher.Flush()
}

func (s *sseWriter) WriteChunk(text string) {
	s.write(chunkPayload{Event: sseEventChunk, Text: text})
}

func (s *sseWriter) WriteDone(chunk model.MessageChunk) {
	s.write(donePayload{
		Event: sseEventDone,
		Metadata: doneMetadata{
			TokensUsed: chunk.InputTokens + chunk.OutputTokens,
			Model:      chunk.Model,
		},
	})
}

func (s *sseWriter) WriteError(err error) {
	s.write(errorPayload{Event: sseEventError, Message: err.Error()})
}
