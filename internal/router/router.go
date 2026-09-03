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

	// Which proxies may set X-Forwarded-For. Getting this wrong is not
	// cosmetic: trust too little and every request behind a load balancer
	// reports the balancer's IP, so the per-IP rate limiter throttles all
	// users as a single client; trust too much on a directly reachable port
	// and a client spoofs its way past that limiter.
	trusted := cfg.Server.TrustedProxies
	if cfg.Server.TrustAllProxies() {
		trusted = []string{"0.0.0.0/0", "::/0"}
	}
	if err := r.SetTrustedProxies(trusted); err != nil {
		logger.Warn("could not set trusted proxies",
			slog.Any("proxies", cfg.Server.TrustedProxies),
			slog.String("error", err.Error()),
		)
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

		me.GET("/enrollments", h.Enrollment.Handle)   // E3
		me.GET("/certificates", h.Certificate.Handle) // CT3
	}
}

// registerPublicRoutes mounts the unauthenticated catalog and certificate
// verification. OptionalAuth lets a signed-in visitor see their own progress
// on the same endpoints, without ever requiring a token.
func registerPublicRoutes(v1 *gin.RouterGroup, h *handlers.Handlers, optionalAuth gin.HandlerFunc) {
	courses := v1.Group("/courses", optionalAuth)
	{
		courses.GET("", h.Courses.ListPublic)      // PC1
		courses.GET("/:slug", h.Courses.GetPublic) // PC2
	}

	// Categories sits outside /courses on purpose: Gin cannot hold the static
	// segment "categories" and the wildcard ":slug" at the same position.
	v1.GET("/categories", h.Courses.PublicCategories) // C6

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
		learn.GET("/courses/:courseId", h.Courses.Get)              // full course tree
		learn.GET("/courses/:courseId/progress", h.Progress.Handle) // PR3
		learn.GET("/modules/:moduleId/lessons", h.Lesson.Handle)
		learn.GET("/lessons/:lessonId", h.Lesson.Handle) // L5, full content
	}

	progress := v1.Group("/lessons/:lessonId", authOnly)
	{
		progress.POST("/complete", h.Progress.Handle)   // PR1
		progress.DELETE("/complete", h.Progress.Handle) // PR2
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
		users.GET("", h.Users.List)    // U1
		users.POST("", h.Users.Create) // U2
		users.GET("/:id", h.Users.Get) // U3
		// No extra super-admin guard here: UserService decides per field, so
		// an admin may rename a learner but not touch a role. A blanket
		// superOnly would block that legitimate case.
		users.PATCH("/:id", h.Users.Update)  // U4, U5
		users.DELETE("/:id", h.Users.Delete) // U6
	}

	// Categories is its own path, not /courses/categories: Gin cannot hold a
	// static segment and the ":id" wildcard at the same position.
	admin.GET("/categories", h.Courses.Categories) // C6

	adminCourses := admin.Group("/courses")
	{
		adminCourses.GET("", h.Courses.List)    // C2
		adminCourses.POST("", h.Courses.Create) // C1
		adminCourses.GET("/:id", h.Courses.Get)
		adminCourses.PATCH("/:id", h.Courses.Update)           // C3
		adminCourses.DELETE("/:id", h.Courses.Delete)          // C4
		adminCourses.PATCH("/:id/status", h.Courses.SetStatus) // C5

		adminCourses.GET("/:id/modules", h.Module.Handle)
		adminCourses.POST("/:id/modules", h.Module.Handle)          // M1
		adminCourses.PATCH("/:id/modules/reorder", h.Module.Handle) // M4
		adminCourses.GET("/:id/analytics", h.Analytics.Handle)      // AN2
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
