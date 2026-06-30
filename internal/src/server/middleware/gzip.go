package middleware

import (
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func Gzip() func(http.Handler) http.Handler {
	c := chimiddleware.NewCompressor(5, "application/json")
	return c.Handler
}
