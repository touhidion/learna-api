// Package router assembles the HTTP route table.
//
// Every Phase 1 endpoint from docs/learna-architecture.md is registered here,
// including the ones whose handlers are still stubs. Having the full table in
// one file makes the auth boundary auditable at a glance: anything under
// /admin sits behind RequireAdmin, and anything else that touches user data
// sits behind Auth.
package router

import (
	"log/slog"

	"github.com/gin-gonic/gin"

	"github.com/learna/learna-api/internal/config"
	"github.com/learna/learna-api/internal/handlers"
	"github.com/learna/learna-api/internal/middleware"
	"github.com/learna/learna-api/internal/utils"
)

// New builds the fully configured Gin engine.
func New(cfg *config.Config, h *handlers.Handlers, tokens *utils.TokenManager, logger *slog.Logger) *gin.Engine {
	if cfg.App.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	// gin.New, not gin.Default: the default logger and recovery are replaced
	// by the structured ones below.
	r := gin.New()

	// Client IPs arrive through the reverse proxy described in the deployment
	// diagram. Trusting only the loopback proxy means a client cannot spoof
	// its IP with an X-Forwarded-For header and slip past the rate limiter.
	// Widen this list if the API sits behind a proxy on another host.
	if err := r.SetTrustedProxies([]string{"127.0.0.1", "::1"}); err != nil {
		logger.Warn("could not set trusted proxies", slog.String("error", err.Error()))
	}

	utils.RegisterValidators()

	r.Use(
		middleware.RequestID(),
		middleware.Logger(logger),
		middleware.Recovery(),
		middleware.CORS(cfg.CORS),
	)

	r.NoRoute(middleware.NoRoute())
	r.NoMethod(middleware.NoMethod())

	// Probes live outside /api/v1 so they stay stable across API versions.
	r.GET("/health", h.Health.Health)
	r.GET("/live", h.Health.Live)

	var (
		authOnly     = middleware.Auth(tokens)
		optionalAuth = middleware.OptionalAuth(tokens)
		adminOnly    = middleware.RequireAdmin()
		superOnly    = middleware.RequireSuperAdmin()
		throttled    = middleware.RateLimit(cfg.RateLimit)
	)

	v1 := r.Group("/api/v1")

	registerAuthRoutes(v1, h, throttled)
	registerMeRoutes(v1, h, authOnly)
	registerPublicRoutes(v1, h, optionalAuth)
	registerLearnerRoutes(v1, h, authOnly)
	registerAdminRoutes(v1, h, authOnly, adminOnly, superOnly)

	return r
}

// registerAuthRoutes mounts /auth. Rate limited: these are the endpoints a
// credential-stuffing attempt would target — feature I4.
func registerAuthRoutes(v1 *gin.RouterGroup, h *handlers.Handlers, throttled gin.HandlerFunc) {
	auth := v1.Group("/auth", throttled)
	{
		auth.POST("/signup", h.Auth.Signup)
		auth.POST("/login", h.Auth.Login)
		auth.POST("/refresh", h.Auth.Refresh)
		auth.POST("/logout", h.Auth.Logout)
		auth.POST("/forgot-password", h.Auth.ForgotPassword)
		auth.POST("/reset-password", h.Auth.ResetPassword)
	}
}

// registerMeRoutes mounts the authenticated self-service routes.
func registerMeRoutes(v1 *gin.RouterGroup, h *handlers.Handlers, authOnly gin.HandlerFunc) {
	me := v1.Group("/me", authOnly)
	{
		me.GET("", h.Auth.Me)
		me.PATCH("", h.Auth.UpdateMe)
		me.PATCH("/password", h.Auth.ChangePassword)

		me.GET("/enrollments", h.Enrollment.Handle)  // E3
		me.GET("/certificates", h.Certificate.Handle) // CT3
	}
}

// registerPublicRoutes mounts the unauthenticated catalog and certificate
// verification. OptionalAuth lets a signed-in visitor see their own progress
// on the same endpoints, without ever requiring a token.
func registerPublicRoutes(v1 *gin.RouterGroup, h *handlers.Handlers, optionalAuth gin.HandlerFunc) {
	courses := v1.Group("/courses", optionalAuth)
	{
		courses.GET("", h.Course.Handle)       // PC1
		courses.GET("/:slug", h.Course.Handle) // PC2
	}

	// Certificate verification is deliberately open — feature CT5.
	v1.GET("/certificates/verify/:certNumber", h.Certificate.Handle)
}

// registerLearnerRoutes mounts the routes an enrolled learner uses.
//
// Note the :courseId / :moduleId path segments: Gin's router requires a single
// wildcard name per path position, so the public catalog uses /courses/:slug
// while the learner routes below live under distinct prefixes to avoid a
// conflict on that segment.
func registerLearnerRoutes(v1 *gin.RouterGroup, h *handlers.Handlers, authOnly gin.HandlerFunc) {
	enroll := v1.Group("/enrollments", authOnly)
	{
		enroll.POST("/:courseId", h.Enrollment.Handle)   // E1
		enroll.DELETE("/:courseId", h.Enrollment.Handle) // E2
	}

	learn := v1.Group("/learn", authOnly)
	{
		learn.GET("/courses/:courseId", h.Course.Handle)          // full course tree
		learn.GET("/courses/:courseId/progress", h.Progress.Handle) // PR3
		learn.GET("/modules/:moduleId/lessons", h.Lesson.Handle)
		learn.GET("/lessons/:lessonId", h.Lesson.Handle) // L5, full content
	}

	progress := v1.Group("/lessons/:lessonId", authOnly)
	{
		progress.POST("/complete", h.Progress.Handle)     // PR1
		progress.DELETE("/complete", h.Progress.Handle)   // PR2
	}

	// Every segment after /certificates is static before its wildcard.
	// Gin's tree cannot hold a static and a parameter segment at the same
	// position, so "/:id/download" would collide with the public
	// "/verify/:certNumber" registered above.
	certs := v1.Group("/certificates", authOnly)
	{
		certs.POST("/courses/:courseId", h.Certificate.Handle) // CT1
		certs.GET("/download/:id", h.Certificate.Handle)       // CT4
	}
}

// registerAdminRoutes mounts everything under /admin behind Auth + RequireAdmin.
// The two super-admin-only routes carry an extra guard.
func registerAdminRoutes(
	v1 *gin.RouterGroup,
	h *handlers.Handlers,
	authOnly, adminOnly, superOnly gin.HandlerFunc,
) {
	admin := v1.Group("/admin", authOnly, adminOnly)

	users := admin.Group("/users")
	{
		users.GET("", h.User.Handle)     // U1
		users.POST("", h.User.Handle)    // U2
		users.GET("/:id", h.User.Handle) // U3
		// Changing a role or deleting an account is super-admin territory:
		// an admin must not be able to promote themselves — feature U4/AUM6.
		users.PATCH("/:id", superOnly, h.User.Handle)
		users.DELETE("/:id", superOnly, h.User.Handle) // U6
	}

	courses := admin.Group("/courses")
	{
		courses.GET("", h.Course.Handle)            // C2
		courses.POST("", h.Course.Handle)           // C1
		courses.GET("/:id", h.Course.Handle)
		courses.PATCH("/:id", h.Course.Handle)       // C3
		courses.DELETE("/:id", h.Course.Handle)      // C4
		courses.PATCH("/:id/status", h.Course.Handle) // C5

		courses.GET("/:id/modules", h.Module.Handle)
		courses.POST("/:id/modules", h.Module.Handle)         // M1
		courses.PATCH("/:id/modules/reorder", h.Module.Handle) // M4
		courses.GET("/:id/analytics", h.Analytics.Handle)      // AN2
	}

	modules := admin.Group("/modules")
	{
		modules.PATCH("/:id", h.Module.Handle)  // M2
		modules.DELETE("/:id", h.Module.Handle) // M3

		modules.POST("/:id/lessons", h.Lesson.Handle)          // L1
		modules.PATCH("/:id/lessons/reorder", h.Lesson.Handle) // L4
	}

	lessons := admin.Group("/lessons")
	{
		lessons.PATCH("/:id", h.Lesson.Handle)  // L2
		lessons.DELETE("/:id", h.Lesson.Handle) // L3

		lessons.GET("/:id/attachments", h.Attachment.Handle)  // AT2
		lessons.POST("/:id/attachments", h.Attachment.Handle) // AT1
	}

	admin.DELETE("/attachments/:id", h.Attachment.Handle) // AT3

	admin.GET("/analytics/overview", h.Analytics.Handle) // AN1

	admin.POST("/upload/image", h.Upload.UploadImage) // CL1
}
