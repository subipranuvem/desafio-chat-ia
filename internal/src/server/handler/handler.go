package handler

import (
	"errors"
	"net/http"
)

type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

type HTTPError struct {
	Code    int
	Message string
}

func (e HTTPError) Error() string { return e.Message }

func Adapt(h HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			var httpErr HTTPError
			if errors.As(err, &httpErr) {
				http.Error(w, httpErr.Message, httpErr.Code)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
