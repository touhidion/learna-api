package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/learna/learna-api/internal/config"
	"github.com/learna/learna-api/internal/database"
)

// HealthHandler serves the liveness and readiness probes.
type HealthHandler struct {
	cfg *config.Config
	db  *database.DB
}

type healthResponse struct {
	Status   string            `json:"status"` // ok | degraded
	Env      string            `json:"env"`
	Time     time.Time         `json:"time"`
	Services map[string]string `json:"services"`
}

// Health reports whether the API can serve traffic — feature I8.
//
// It returns 503 when the database is unreachable so an orchestrator can pull
// the instance out of rotation rather than sending it requests that will fail.
func (h *HealthHandler) Health(c *gin.Context) {
	// Bounded independently of the request: a probe must answer quickly even
	// when the database is hanging.
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	body := healthResponse{
		Status:   "ok",
		Env:      h.cfg.App.Env,
		Time:     time.Now().UTC(),
		Services: map[string]string{},
	}

	status := http.StatusOK
	if err := h.db.Ping(ctx); err != nil {
		body.Status = "degraded"
		body.Services["database"] = "unreachable"
		status = http.StatusServiceUnavailable
	} else {
		body.Services["database"] = "ok"
	}

	c.JSON(status, body)
}

// Live is the liveness probe: the process is up. It never touches the
// database, so a database outage does not get the container restarted.
func (h *HealthHandler) Live(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
