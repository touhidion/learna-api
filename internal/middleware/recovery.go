package middleware

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"runtime/debug"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/learna/learna-api/internal/utils"
)

// Recovery turns a panic into a logged 500 in the standard error envelope,
// instead of Gin's default plain-text dump.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}

			// A client that hangs up mid-write surfaces as a panic on a broken
			// pipe. Nothing can be written back, so close the connection
			// quietly rather than logging it as a server fault.
			if isBrokenPipe(rec) {
				utils.LoggerFrom(c).Warn("client disconnected", slog.Any("error", rec))
				c.Abort()
				return
			}

			utils.LoggerFrom(c).Error("panic recovered",
				slog.Any("panic", rec),
				slog.String("stack", string(debug.Stack())),
			)
			utils.Fail(c, utils.ErrInternal(fmt.Errorf("panic: %v", rec)))
		}()

		c.Next()
	}
}

// isBrokenPipe reports whether the recovered value is a write to a connection
// the peer already closed.
func isBrokenPipe(rec any) bool {
	ne, ok := rec.(*net.OpError)
	if !ok {
		return false
	}
	var se *os.SyscallError
	if !errors.As(ne.Err, &se) {
		return false
	}
	msg := strings.ToLower(se.Error())
	return strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset by peer") ||
		strings.Contains(msg, "an established connection was aborted")
}

// NoRoute answers unknown paths in the same envelope as every other error.
func NoRoute() gin.HandlerFunc {
	return func(c *gin.Context) {
		utils.Fail(c, utils.ErrNotFound("No route matches %s %s.", c.Request.Method, c.Request.URL.Path))
	}
}

// NoMethod answers a known path with the wrong verb.
func NoMethod() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.AbortWithStatusJSON(http.StatusMethodNotAllowed, gin.H{
			"error": gin.H{
				"code":    utils.CodeBadRequest,
				"message": fmt.Sprintf("Method %s is not allowed on %s.", c.Request.Method, c.Request.URL.Path),
			},
		})
	}
}
