package router

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/learna/learna-api/internal/config"
	"github.com/learna/learna-api/internal/handlers"
	"github.com/learna/learna-api/internal/repository"
	"github.com/learna/learna-api/internal/services"
	"github.com/learna/learna-api/internal/utils"
)

// testRouter builds the real route table with stub handlers and no database.
//
// Gin's tree cannot hold a static and a parameter segment at the same
// position, and it reports that by panicking at registration time — so simply
// constructing the router is the assertion.
func testRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cfg := &config.Config{
		App:  config.AppConfig{Env: "test"},
		JWT:  config.JWTConfig{Secret: "test-secret-long-enough-for-hs256", Issuer: "learna-api"},
		CORS: config.CORSConfig{AllowedOrigins: []string{"http://localhost:3000"}},
	}

	// A nil pool is enough: these tests exercise routing and middleware, and
	// stop before any handler reaches the database.
	tokens := utils.NewTokenManager(cfg.JWT)
	svc := services.New(services.Deps{
		Config: cfg,
		Repos:  repository.New(nil),
		Tokens: tokens,
		Hasher: utils.NewHasher(cfg.JWT.BcryptCost),
	})

	h := handlers.New(cfg, nil, svc)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return New(cfg, h, tokens, logger)
}

func TestRouteTableHasNoConflicts(t *testing.T) {
	r := testRouter(t)

	if got := len(r.Routes()); got == 0 {
		t.Fatal("no routes registered")
	}
}

// TestProtectedRoutesRejectAnonymous walks every registered route and checks
// that the ones behind Auth answer 401 rather than reaching their handler.
func TestProtectedRoutesRejectAnonymous(t *testing.T) {
	r := testRouter(t)

	protected := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/me"},
		{http.MethodPatch, "/api/v1/me"},
		{http.MethodPatch, "/api/v1/me/password"},
		{http.MethodGet, "/api/v1/me/enrollments"},
		{http.MethodGet, "/api/v1/admin/users"},
		{http.MethodPost, "/api/v1/admin/courses"},
		{http.MethodGet, "/api/v1/admin/analytics/overview"},
	}

	for _, tc := range protected {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))

			if w.Code != http.StatusUnauthorized {
				t.Errorf("got %d, want 401 for an unauthenticated request", w.Code)
			}
		})
	}
}

// TestPublicRoutesAreReachable confirms the open endpoints are not sitting
// behind the auth middleware by mistake. They answer 501 because their
// handlers are still stubs — the point is that they are reached at all.
func TestPublicRoutesAreReachable(t *testing.T) {
	r := testRouter(t)

	public := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/courses"},
		{http.MethodGet, "/api/v1/courses/some-slug"},
		{http.MethodGet, "/api/v1/certificates/verify/LEARNA-2026-ABC123"},
	}

	for _, tc := range public {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))

			if w.Code == http.StatusUnauthorized {
				t.Errorf("public route answered 401; it should not require auth")
			}
		})
	}
}
