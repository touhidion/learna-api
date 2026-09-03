// Package handlers is the HTTP layer. A handler parses and validates the
// request, calls exactly one service method, and renders the result. Business
// rules and SQL belong further down.
package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/learna/learna-api/internal/config"
	"github.com/learna/learna-api/internal/database"
	"github.com/learna/learna-api/internal/middleware"
	"github.com/learna/learna-api/internal/models"
	"github.com/learna/learna-api/internal/services"
)

// Handlers bundles every handler group for route registration.
type Handlers struct {
	Health       *HealthHandler
	Auth         *AuthHandler
	Users        *UserHandler
	Courses      *CourseHandler
	Modules      *ModuleHandler
	Lessons      *LessonHandler
	Learning     *LearningHandler
	Certificates *CertificateHandler
	Analytics    *AnalyticsHandler
	Upload       *UploadHandler
}

// New wires the handler layer.
func New(cfg *config.Config, db *database.DB, svc *services.Services) *Handlers {
	return &Handlers{
		Health:       &HealthHandler{cfg: cfg, db: db},
		Auth:         &AuthHandler{auth: svc.Auth},
		Users:        &UserHandler{users: svc.Users},
		Courses:      &CourseHandler{courses: svc.Courses},
		Modules:      &ModuleHandler{modules: svc.Modules},
		Lessons:      &LessonHandler{lessons: svc.Lessons},
		Learning:     &LearningHandler{learning: svc.Learning},
		Certificates: &CertificateHandler{certificates: svc.Certificates},
		Analytics:    &AnalyticsHandler{analytics: svc.Analytics},
		Upload:       &UploadHandler{upload: svc.Upload},
	}
}

// currentActor returns the caller's id and role together, which every admin
// handler needs in order to enforce "you cannot act on yourself" and the
// super-admin-only rules.
func currentActor(c *gin.Context) (uuid.UUID, models.Role, error) {
	id, err := middleware.CurrentUserID(c)
	if err != nil {
		return uuid.Nil, "", err
	}
	role, err := middleware.CurrentUserRole(c)
	if err != nil {
		return uuid.Nil, "", err
	}
	return id, role, nil
}
