// Command server is the learna-api entry point.
//
// Boot order matters: configuration, then logging, then the database and its
// migrations, then the object graph, and only then the listener. Nothing
// starts serving until every dependency it needs is proven healthy.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/learna/learna-api/internal/config"
	"github.com/learna/learna-api/internal/database"
	"github.com/learna/learna-api/internal/handlers"
	"github.com/learna/learna-api/internal/repository"
	"github.com/learna/learna-api/internal/router"
	"github.com/learna/learna-api/internal/services"
	"github.com/learna/learna-api/internal/utils"
	"github.com/learna/learna-api/pkg/cloudinary"
)

// version is stamped at build time by the Makefile:
//
//	go build -ldflags="-X main.version=$(git describe --tags --always)"
var version = "dev"

func main() {
	// A migration subcommand shares the same config loading as the server, so
	// `make migrate-up` and the API can never disagree about which database
	// they mean.
	migrateCmd := flag.String("migrate", "", "run migrations and exit: up | down | version | force")
	migrateN := flag.Int("n", 0, "number of steps for -migrate=down (0 means all), or the version for -migrate=force")
	flag.Parse()

	if err := run(*migrateCmd, *migrateN); err != nil {
		// slog may not be configured yet, so report to stderr directly.
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

func run(migrateCmd string, migrateN int) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	logger := newLogger(cfg.App)
	slog.SetDefault(logger)

	if migrateCmd != "" {
		return runMigrateCommand(cfg, logger, migrateCmd, migrateN)
	}

	// Bound startup so a wedged database fails the boot instead of hanging a
	// container in "starting" forever.
	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancelStartup()

	if cfg.DB.AutoMigrate {
		logger.Info("applying database migrations")
		if err := database.MigrateUp(cfg.DB); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	db, err := database.Connect(startupCtx, cfg.DB)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer db.Close()
	logger.Info("database connected",
		slog.String("host", cfg.DB.Host),
		slog.String("database", cfg.DB.Name),
	)

	cld, err := cloudinary.New(cfg.Cloudinary)
	if err != nil {
		return fmt.Errorf("init cloudinary: %w", err)
	}
	if !cld.Enabled() {
		logger.Warn("cloudinary is not configured; upload endpoints will return 503")
	}

	repos := repository.New(db)
	tokens := utils.NewTokenManager(cfg.JWT)
	hasher := utils.NewHasher(cfg.JWT.BcryptCost)

	svc := services.New(services.Deps{
		Config:     cfg,
		Repos:      repos,
		Tokens:     tokens,
		Hasher:     hasher,
		Cloudinary: cld,
	})

	// Feature A8: create the super admin on first run.
	created, err := svc.Auth.EnsureSuperAdmin(startupCtx)
	if err != nil {
		return fmt.Errorf("seed super admin: %w", err)
	}
	if created {
		logger.Info("seeded first super admin", slog.String("email", cfg.SuperAdmin.Email))
	}

	h := handlers.New(cfg, db, svc)
	engine := router.New(cfg, h, tokens, logger)

	srv := &http.Server{
		Addr:              cfg.Server.Addr(),
		Handler:           engine,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return serve(srv, cfg, logger)
}

// serve runs the HTTP server until SIGINT or SIGTERM, then drains in-flight
// requests before returning.
func serve(srv *http.Server, cfg *config.Config, logger *slog.Logger) error {
	// Buffered so a listener failure is never dropped if the signal lands at
	// the same moment.
	serverErr := make(chan error, 1)

	go func() {
		logger.Info("server listening",
			slog.String("addr", srv.Addr),
			slog.String("env", cfg.App.Env),
			slog.String("version", version),
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if err != nil {
			return fmt.Errorf("server: %w", err)
		}
		return nil

	case sig := <-quit:
		logger.Info("shutdown signal received", slog.String("signal", sig.String()))
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		// Requests still running past the grace period are cut off here.
		return fmt.Errorf("graceful shutdown timed out: %w", err)
	}

	logger.Info("server stopped cleanly")
	return nil
}

// runMigrateCommand handles the -migrate flag and exits without serving.
func runMigrateCommand(cfg *config.Config, logger *slog.Logger, cmd string, n int) error {
	switch cmd {
	case "up":
		if err := database.MigrateUp(cfg.DB); err != nil {
			return err
		}
		logger.Info("migrations applied")

	case "down":
		if err := database.MigrateDown(cfg.DB, n); err != nil {
			return err
		}
		logger.Info("migrations rolled back", slog.Int("steps", n))

	case "version":
		version, dirty, err := database.MigrateVersion(cfg.DB)
		if err != nil {
			return err
		}
		logger.Info("schema version", slog.Uint64("version", uint64(version)), slog.Bool("dirty", dirty))

	case "force":
		if err := database.MigrateForce(cfg.DB, n); err != nil {
			return err
		}
		logger.Info("schema version forced", slog.Int("version", n))

	default:
		return fmt.Errorf("unknown -migrate value %q: expected up, down, version or force", cmd)
	}
	return nil
}

// newLogger returns a JSON logger in production (feature I5) and a
// human-readable text logger in development.
func newLogger(cfg config.AppConfig) *slog.Logger {
	opts := &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}

	if cfg.IsProduction() {
		return slog.New(slog.NewJSONHandler(os.Stdout, opts))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, opts))
}

func parseLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
