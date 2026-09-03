// Package middleware holds the cross-cutting Gin handlers: request ID,
// structured logging, panic recovery, CORS, JWT auth, role guards and rate
// limiting.
package middleware

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/learna/learna-api/internal/utils"
)

// headerRequestID is echoed back so a client can quote it in a bug report.
const headerRequestID = "X-Request-ID"

// RequestID assigns each request an ID, trusting an inbound X-Request-ID when
// one is present so a trace survives across services. The ID is stored on the
// context and mirrored in the response header.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.GetHeader(headerRequestID)
		if id == "" {
			id = uuid.NewString()
		}
		c.Set(utils.CtxRequestID, id)
		c.Header(headerRequestID, id)
		c.Next()
	}
}

// Logger attaches a request-scoped slog.Logger to the context and writes one
// structured access-log line per request when it completes.
func Logger(base *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		reqLogger := base.With(
			slog.String("request_id", utils.RequestIDFrom(c)),
			slog.String("method", c.Request.Method),
			slog.String("path", c.Request.URL.Path),
		)
		c.Set(utils.CtxLogger, reqLogger)

		c.Next()

		status := c.Writer.Status()
		attrs := []any{
			slog.Int("status", status),
			slog.Duration("duration", time.Since(start)),
			slog.Int("bytes", c.Writer.Size()),
			slog.String("client_ip", c.ClientIP()),
		}
		if raw := c.Request.URL.RawQuery; raw != "" {
			attrs = append(attrs, slog.String("query", raw))
		}
		if uid, ok := c.Get(utils.CtxUserID); ok {
			attrs = append(attrs, slog.Any("user_id", uid))
		}
		// Errors recorded via c.Error, including the one Fail attaches.
		if errs := c.Errors.Errors(); len(errs) > 0 {
			attrs = append(attrs, slog.Any("errors", errs))
		}

		switch {
		case status >= 500:
			reqLogger.Error("request", attrs...)
		case status >= 400:
			reqLogger.Warn("request", attrs...)
		default:
			reqLogger.Info("request", attrs...)
		}
	}
}
