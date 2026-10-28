package apperror

import (
	"errors"
	"net/http"
)

type Error struct {
	Status  int
	Code    string
	Message string
	Details map[string]string
	Err     error
}

func (e *Error) Error() string {
	if e.Err != nil {
		return e.Code + ": " + e.Message + ": " + e.Err.Error()
	}
	return e.Code + ": " + e.Message
}

func (e *Error) Unwrap() error {
	return e.Err
}

func (e *Error) WithCause(err error) *Error {
	c := *e
	c.Err = err
	return &c
}

func (e *Error) WithDetails(details map[string]string) *Error {
	c := *e
	c.Details = details
	return &c
}

func New(status int, code, message string) *Error {
	return &Error{Status: status, Code: code, Message: message}
}

func As(err error) (*Error, bool) {
	var ae *Error
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

func BadRequest(message string) *Error {
	return New(http.StatusBadRequest, "BAD_REQUEST", message)
}

func Validation(details map[string]string) *Error {
	return New(http.StatusBadRequest, "VALIDATION_FAILED", "request validation failed").WithDetails(details)
}

func Unauthorized(message string) *Error {
	return New(http.StatusUnauthorized, "UNAUTHORIZED", message)
}

func Forbidden(message string) *Error {
	return New(http.StatusForbidden, "FORBIDDEN", message)
}

func NotFound(message string) *Error {
	return New(http.StatusNotFound, "NOT_FOUND", message)
}

func Conflict(message string) *Error {
	return New(http.StatusConflict, "CONFLICT", message)
}

func Unavailable(message string) *Error {
	return New(http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", message)
}

func Internal() *Error {
	return New(http.StatusInternalServerError, "INTERNAL_ERROR", "internal server error")
}
