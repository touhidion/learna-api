package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/learna/learna-api/internal/config"
)

// corsMaxAge caps how long a browser may cache a preflight response.
const corsMaxAge = 12 * 60 * 60 // 12 hours, in seconds

var (
	corsAllowedMethods = strings.Join([]string{
		http.MethodGet, http.MethodPost, http.MethodPatch,
		http.MethodPut, http.MethodDelete, http.MethodOptions,
	}, ", ")

	corsAllowedHeaders = strings.Join([]string{
		"Origin", "Content-Type", "Content-Length", "Accept",
		"Authorization", "X-Request-ID",
	}, ", ")

	corsExposedHeaders = strings.Join([]string{"X-Request-ID"}, ", ")
)

// CORS answers preflights and stamps the response headers for allowed origins.
//
// Origins are matched exactly against the configured list. When the wildcard
// is configured (development only — Load rejects it in production) the
// request's own origin is echoed back rather than "*", so credentialed
// requests still work.
func CORS(cfg config.CORSConfig) gin.HandlerFunc {
	allowAll := cfg.AllowAll()

	allowed := make(map[string]struct{}, len(cfg.AllowedOrigins))
	for _, o := range cfg.AllowedOrigins {
		allowed[strings.TrimRight(o, "/")] = struct{}{}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			// Not a browser cross-origin request; nothing to negotiate.
			c.Next()
			return
		}

		_, ok := allowed[strings.TrimRight(origin, "/")]
		if ok || allowAll {
			h := c.Writer.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Allow-Credentials", "true")
			h.Set("Access-Control-Allow-Methods", corsAllowedMethods)
			h.Set("Access-Control-Allow-Headers", corsAllowedHeaders)
			h.Set("Access-Control-Expose-Headers", corsExposedHeaders)
			h.Set("Access-Control-Max-Age", strconv.Itoa(corsMaxAge))
			// The response body varies by origin; without this a shared cache
			// could serve one origin's headers to another.
			h.Add("Vary", "Origin")
		}

		if c.Request.Method == http.MethodOptions {
			// Disallowed origins reach here without the headers above, which
			// is what makes the browser block the real request.
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
