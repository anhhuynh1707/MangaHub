package utils

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// AppError is a typed error that carries the HTTP status it should map to.
// Services return these for known/expected conditions (e.g. "already exists",
// "not found"); handlers pass them to RespondError to produce a consistent
// response without fragile string matching on err.Error().
type AppError struct {
	Code    int    // HTTP status code
	Message string // client-facing message (goes in the response "error" field)
	Err     error  // optional wrapped cause — logged, never exposed to the client
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

// Unwrap lets errors.Is / errors.As reach the wrapped cause.
func (e *AppError) Unwrap() error { return e.Err }

// Wrap attaches an underlying cause to an AppError (for logging).
func (e *AppError) Wrap(cause error) *AppError {
	e.Err = cause
	return e
}

// ── Constructors (one per status code we use) ────────────────────────

func NewAppError(code int, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func ErrBadRequest(message string) *AppError   { return NewAppError(http.StatusBadRequest, message) }
func ErrUnauthorized(message string) *AppError { return NewAppError(http.StatusUnauthorized, message) }
func ErrForbidden(message string) *AppError    { return NewAppError(http.StatusForbidden, message) }
func ErrNotFound(message string) *AppError     { return NewAppError(http.StatusNotFound, message) }
func ErrConflict(message string) *AppError     { return NewAppError(http.StatusConflict, message) }
func ErrInternal(message string) *AppError     { return NewAppError(http.StatusInternalServerError, message) }

// RespondError writes the correct HTTP error response for err. If err is (or
// wraps) an *AppError, its Code/Message are used; any other error is treated as
// an unexpected 500 and its details are NOT leaked to the client.
func RespondError(c *gin.Context, err error) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		ErrorResponse(c, appErr.Code, appErr.Message)
		return
	}
	ErrorResponse(c, http.StatusInternalServerError, "Internal server error")
}
