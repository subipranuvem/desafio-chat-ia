package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// ValidateSchema returns a middleware that validates the request body against
// the provided JSON Schema string. Returns 422 if validation fails, 400 if
// the body is not valid JSON. Reinjects the body so the handler can read it.
func ValidateSchema(schemaJSON string) func(http.Handler) http.Handler {
	sch := mustCompileString(schemaJSON)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "failed to read request body", http.StatusInternalServerError)
				return
			}

			var decoded any
			if err := json.Unmarshal(body, &decoded); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}

			if err := sch.Validate(decoded); err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()}) //nolint:gosec
				return
			}

			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
		})
	}
}

func mustCompileString(schemaJSON string) *jsonschema.Schema {
	var doc any
	if err := json.Unmarshal([]byte(schemaJSON), &doc); err != nil {
		panic("json_schema middleware: invalid json: " + err.Error())
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("schema://inline", doc); err != nil {
		panic("json_schema middleware: failed to add resource: " + err.Error())
	}

	sch, err := compiler.Compile("schema://inline")
	if err != nil {
		panic("json_schema middleware: failed to compile schema: " + err.Error())
	}

	return sch
}
