// Package handlers is the HTTP layer. A handler parses and validates the
// request, calls exactly one service method, and renders the result. Business
// rules and SQL belong further down.
package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/learna/learna-api/internal/config"
	"github.com/learna/learna-api/internal/database"
	"github.com/learna/learna-api/internal/services"
	"github.com/learna/learna-api/internal/utils"
)

// Handlers bundles every handler group for route registration.
type Handlers struct {
	Health *HealthHandler
	Auth   *AuthHandler
	Upload *UploadHandler

	// Groups whose endpoints are registered but not yet implemented. They
	// share one implementation that answers 501 with the feature ID from
	// docs/learna-features.md, so the route table is complete and honest from
	// the start.
	Course      *StubHandler
	Module      *StubHandler
	Lesson      *StubHandler
	Attachment  *StubHandler
	Enrollment  *StubHandler
	Progress    *StubHandler
	Certificate *StubHandler
	User        *StubHandler
	Analytics   *StubHandler
}

// New wires the handler layer.
func New(cfg *config.Config, db *database.DB, svc *services.Services) *Handlers {
	return &Handlers{
		Health: &HealthHandler{cfg: cfg, db: db},
		Auth:   &AuthHandler{auth: svc.Auth},
		Upload: &UploadHandler{upload: svc.Upload},

		Course:      &StubHandler{module: "courses", features: "C1-C6, PC1-PC2"},
		Module:      &StubHandler{module: "modules", features: "M1-M4"},
		Lesson:      &StubHandler{module: "lessons", features: "L1-L5"},
		Attachment:  &StubHandler{module: "attachments", features: "AT1-AT4"},
		Enrollment:  &StubHandler{module: "enrollments", features: "E1-E4"},
		Progress:    &StubHandler{module: "progress", features: "PR1-PR4"},
		Certificate: &StubHandler{module: "certificates", features: "CT1-CT5"},
		User:        &StubHandler{module: "users", features: "U1-U6"},
		Analytics:   &StubHandler{module: "analytics", features: "AN1-AN2"},
	}
}

// StubHandler answers every route in a not-yet-built module with a 501 that
// names the module and the features it will cover.
type StubHandler struct {
	module   string
	features string
}

// Handle is the gin.HandlerFunc to register for an unimplemented endpoint.
func (h *StubHandler) Handle(c *gin.Context) {
	utils.Fail(c, utils.ErrNotImplemented(
		"The %s module is not implemented yet (features %s).", h.module, h.features))
}
