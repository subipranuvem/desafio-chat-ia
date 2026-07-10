package handler

import (
	"errors"
	"log/slog"
	"net/http"
)

type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

type HTTPError struct {
	Code       int
	Message    string
	LogMessage string
}

func NewHTTPError(code int, message string) HTTPError {
	return HTTPError{Code: code, Message: message}
}

func (e HTTPError) WithLogMessage(msg string) HTTPError {
	e.LogMessage = msg
	return e
}

func (e HTTPError) Error() string { return e.Message }

func Adapt(h HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			if httpErr, ok := errors.AsType[HTTPError](err); ok {
				logMsg := httpErr.LogMessage
				if logMsg == "" {
					logMsg = httpErr.Message
				}
				if httpErr.Code >= 500 {
					slog.Error(logMsg, "status", httpErr.Code)
				} else {
					slog.Warn(logMsg, "status", httpErr.Code)
				}
				http.Error(w, httpErr.Message, httpErr.Code)
				return
			}
			slog.Error(err.Error(), "status", http.StatusInternalServerError)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
