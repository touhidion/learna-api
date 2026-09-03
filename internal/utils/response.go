package utils

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ContextKey names the values middleware stashes on the gin context.
const (
	CtxRequestID = "request_id"
	CtxUserID    = "user_id"
	CtxUserRole  = "user_role"
	CtxLogger    = "logger"
)

// errorEnvelope matches the contract in the architecture doc:
//
//	{ "error": { "code": "...", "message": "..." } }
type errorEnvelope struct {
	Error *APIError `json:"error"`
}

// OK writes a 200 with the given body.
func OK(c *gin.Context, body any) { c.JSON(http.StatusOK, body) }

// Created writes a 201 with the given body.
func Created(c *gin.Context, body any) { c.JSON(http.StatusCreated, body) }

// NoContent writes a 204 with an empty body.
func NoContent(c *gin.Context) { c.Status(http.StatusNoContent) }

// Fail renders err as the error envelope and aborts the handler chain.
//
// Server-side failures (5xx) are logged with their cause; client errors are
// not, so a scanner cannot flood the logs. The error is also attached to the
// gin context so the access-log middleware can report it.
func Fail(c *gin.Context, err error) {
	apiErr := AsAPIError(err)

	if apiErr.Status >= http.StatusInternalServerError {
		LoggerFrom(c).Error("request failed",
			slog.String("code", string(apiErr.Code)),
			slog.String("error", apiErr.Error()),
		)
	}

	_ = c.Error(err) //nolint:errcheck // recorded for the access log only
	c.AbortWithStatusJSON(apiErr.Status, errorEnvelope{Error: apiErr})
}

// LoggerFrom returns the request-scoped logger, falling back to the default
// logger when middleware has not run (in tests, for example).
func LoggerFrom(c *gin.Context) *slog.Logger {
	if v, ok := c.Get(CtxLogger); ok {
		if l, ok := v.(*slog.Logger); ok {
			return l
		}
	}
	return slog.Default()
}

// RequestIDFrom returns the current request ID, or "" when unset.
func RequestIDFrom(c *gin.Context) string {
	if v, ok := c.Get(CtxRequestID); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
