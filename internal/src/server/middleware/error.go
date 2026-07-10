package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
)

type ErrorEntry struct {
	Substring  string
	StatusCode int
	Message    string
}

// DefaultErrors maps common error substrings to user-facing responses.
// Searched in order — first match wins.
var DefaultErrors = []ErrorEntry{
	{Substring: "failed to read request body", StatusCode: http.StatusInternalServerError, Message: "Failed to read request body."},
	{Substring: "invalid json", StatusCode: http.StatusBadRequest, Message: "Request body is not valid JSON."},
	{Substring: "failed to decode body", StatusCode: http.StatusBadRequest, Message: "Invalid request body."},
	{Substring: "jsonschema", StatusCode: http.StatusUnprocessableEntity, Message: "Request validation failed."},
	{Substring: "cannot override system prompt", StatusCode: http.StatusBadRequest, Message: "Cannot override the system prompt of an existing conversation."},
	{Substring: "not available", StatusCode: http.StatusBadRequest, Message: "Requested model is not available."},
	{Substring: "not registered", StatusCode: http.StatusBadRequest, Message: "Requested model is not registered."},
	{Substring: "streaming not supported", StatusCode: http.StatusInternalServerError, Message: "Server does not support streaming."},
	{Substring: "401", StatusCode: http.StatusUnauthorized, Message: "LLM API authentication failed. Check your API key."},
	{Substring: "Unauthorized", StatusCode: http.StatusUnauthorized, Message: "LLM API authentication failed. Check your API key."},
	{Substring: "403", StatusCode: http.StatusForbidden, Message: "LLM API access forbidden. Check your API key permissions."},
	{Substring: "429", StatusCode: http.StatusTooManyRequests, Message: "LLM API rate limit exceeded. Try again later."},
	{Substring: "Too Many Requests", StatusCode: http.StatusTooManyRequests, Message: "LLM API rate limit exceeded. Try again later."},
	{Substring: "rate limit", StatusCode: http.StatusTooManyRequests, Message: "LLM API rate limit exceeded. Try again later."},
}

type errorBody struct {
	StatusCode int    `json:"status_code"`
	Message    string `json:"message"`
	Error      string `json:"error"`
}

type errorRecorder struct {
	http.ResponseWriter
	status      int
	buf         bytes.Buffer
	passthrough bool
}

func (r *errorRecorder) WriteHeader(code int) {
	r.status = code
	if code < 400 {
		r.passthrough = true
		r.ResponseWriter.WriteHeader(code)
	}
}

func (r *errorRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
		r.passthrough = true
		r.ResponseWriter.WriteHeader(http.StatusOK)
	}
	if r.passthrough {
		return r.ResponseWriter.Write(b)
	}
	return r.buf.Write(b)
}

// Flush delegates to the underlying Flusher when in passthrough mode.
// Required so the SSE handler's http.Flusher type assertion succeeds.
func (r *errorRecorder) Flush() {
	if r.passthrough {
		if f, ok := r.ResponseWriter.(http.Flusher); ok {
			f.Flush()
		}
	}
}

func ErrorHandler(entries []ErrorEntry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := &errorRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)

			if rec.passthrough {
				return
			}

			errText := strings.TrimSpace(rec.buf.String())
			body := errorBody{
				StatusCode: rec.status,
				Message:    errText,
				Error:      errText,
			}

			for _, e := range entries {
				if strings.Contains(errText, e.Substring) {
					body.StatusCode = e.StatusCode
					body.Message = e.Message
					break
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(body.StatusCode)
			_ = json.NewEncoder(w).Encode(body) //nolint:errcheck,gosec
		})
	}
}
