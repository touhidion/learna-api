// Package services holds the business logic. Handlers translate HTTP into
// service calls; services enforce the rules and call repositories.
//
// Services return *utils.APIError, so a handler can hand any error straight to
// utils.Fail without deciding on a status code itself.
package services

import (
	"github.com/learna/learna-api/internal/config"
	"github.com/learna/learna-api/internal/repository"
	"github.com/learna/learna-api/internal/utils"
	"github.com/learna/learna-api/pkg/cloudinary"
)

// Services bundles every service so wiring passes one value around.
type Services struct {
	Auth    *AuthService
	Users   *UserService
	Courses *CourseService
	Modules *ModuleService
	Lessons *LessonService
	Upload  *UploadService
}

// Deps is everything the service layer needs from the outside.
type Deps struct {
	Config     *config.Config
	Repos      *repository.Repositories
	Tokens     *utils.TokenManager
	Hasher     *utils.Hasher
	Cloudinary *cloudinary.Client
}

// New constructs the service layer.
func New(d Deps) *Services {
	return &Services{
		Auth:    NewAuthService(d),
		Users:   NewUserService(d),
		Courses: NewCourseService(d),
		Modules: NewModuleService(d),
		Lessons: NewLessonService(d),
		Upload:  NewUploadService(d),
	}
}
