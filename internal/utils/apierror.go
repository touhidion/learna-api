// Package utils holds the small cross-cutting helpers shared by handlers,
// services and repositories: the error type, response envelopes, JWT signing,
// password hashing, slugs, pagination and validation.
package utils

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrorCode is the stable, machine-readable half of an error response. Clients
// branch on these; the message is for humans and may change.
type ErrorCode string

const (
	CodeBadRequest     ErrorCode = "BAD_REQUEST"
	CodeValidation     ErrorCode = "VALIDATION_ERROR"
	CodeUnauthorized   ErrorCode = "UNAUTHORIZED"
	CodeForbidden      ErrorCode = "FORBIDDEN"
	CodeNotFound       ErrorCode = "NOT_FOUND"
	CodeConflict       ErrorCode = "CONFLICT"
	CodeUnprocessable  ErrorCode = "UNPROCESSABLE"
	CodeTooManyReqs    ErrorCode = "TOO_MANY_REQUESTS"
	CodePayloadTooLarge ErrorCode = "PAYLOAD_TOO_LARGE"
	CodeInternal       ErrorCode = "INTERNAL_ERROR"
	CodeUnavailable    ErrorCode = "SERVICE_UNAVAILABLE"
	CodeNotImplemented ErrorCode = "NOT_IMPLEMENTED"
)

// APIError is the error type that flows from services up to handlers. It
// carries everything needed to render a response, plus an optional wrapped
// cause that is logged but never sent to the client.
type APIError struct {
	Status  int          `json:"-"`
	Code    ErrorCode    `json:"code"`
	Message string       `json:"message"`
	Fields  []FieldError `json:"fields,omitempty"`

	cause error
}

// FieldError describes one failed field in a validation error.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *APIError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap exposes the underlying cause to errors.Is and errors.As.
func (e *APIError) Unwrap() error { return e.cause }

// WithCause attaches an internal error for logging. The cause is never
// serialised into the response body.
func (e *APIError) WithCause(err error) *APIError {
	clone := *e
	clone.cause = err
	return &clone
}

// WithFields attaches per-field validation detail.
func (e *APIError) WithFields(fields ...FieldError) *APIError {
	clone := *e
	clone.Fields = append(append([]FieldError(nil), clone.Fields...), fields...)
	return &clone
}

func newAPIError(status int, code ErrorCode, format string, args ...any) *APIError {
	return &APIError{Status: status, Code: code, Message: fmt.Sprintf(format, args...)}
}

// --- constructors -----------------------------------------------------------

func ErrBadRequest(format string, args ...any) *APIError {
	return newAPIError(http.StatusBadRequest, CodeBadRequest, format, args...)
}

func ErrValidation(format string, args ...any) *APIError {
	return newAPIError(http.StatusUnprocessableEntity, CodeValidation, format, args...)
}

func ErrUnauthorized(format string, args ...any) *APIError {
	return newAPIError(http.StatusUnauthorized, CodeUnauthorized, format, args...)
}

func ErrForbidden(format string, args ...any) *APIError {
	return newAPIError(http.StatusForbidden, CodeForbidden, format, args...)
}

func ErrNotFound(format string, args ...any) *APIError {
	return newAPIError(http.StatusNotFound, CodeNotFound, format, args...)
}

func ErrConflict(format string, args ...any) *APIError {
	return newAPIError(http.StatusConflict, CodeConflict, format, args...)
}

func ErrUnprocessable(format string, args ...any) *APIError {
	return newAPIError(http.StatusUnprocessableEntity, CodeUnprocessable, format, args...)
}

func ErrTooManyRequests(format string, args ...any) *APIError {
	return newAPIError(http.StatusTooManyRequests, CodeTooManyReqs, format, args...)
}

func ErrPayloadTooLarge(format string, args ...any) *APIError {
	return newAPIError(http.StatusRequestEntityTooLarge, CodePayloadTooLarge, format, args...)
}

// ErrInternal is deliberately vague in its message: internal failures are
// logged with their cause, never described to the caller.
func ErrInternal(err error) *APIError {
	return (&APIError{
		Status:  http.StatusInternalServerError,
		Code:    CodeInternal,
		Message: "An unexpected error occurred.",
	}).WithCause(err)
}

func ErrUnavailable(format string, args ...any) *APIError {
	return newAPIError(http.StatusServiceUnavailable, CodeUnavailable, format, args...)
}

func ErrNotImplemented(format string, args ...any) *APIError {
	return newAPIError(http.StatusNotImplemented, CodeNotImplemented, format, args...)
}

// AsAPIError converts any error into an *APIError. Anything that is not
// already one becomes an opaque 500 with the original error kept as the cause.
func AsAPIError(err error) *APIError {
	if err == nil {
		return nil
	}
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}
	return ErrInternal(err)
}
