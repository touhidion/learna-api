// Package config loads all runtime configuration from environment variables.
//
// Every knob the service has lives here, so Load is the single place that
// decides what a missing or malformed value means. In development a .env file
// in the working directory is read first; in production the environment is
// expected to be populated by the orchestrator.
package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App        AppConfig
	Server     ServerConfig
	DB         DBConfig
	JWT        JWTConfig
	CORS       CORSConfig
	RateLimit  RateLimitConfig
	SuperAdmin SuperAdminConfig
	Cloudinary CloudinaryConfig
	Cert       CertConfig
}

type AppConfig struct {
	Env      string // development | production
	LogLevel string // debug | info | warn | error
}

func (a AppConfig) IsProduction() bool { return a.Env == "production" }

type ServerConfig struct {
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

func (s ServerConfig) Addr() string { return fmt.Sprintf(":%d", s.Port) }

type DBConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxConns        int32
	MinConns        int32
	MaxConnLifetime time.Duration
	AutoMigrate     bool
}

// DSN returns a URL-form connection string. User and password are escaped so
// that special characters cannot corrupt the DSN.
func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		url.QueryEscape(d.User),
		url.QueryEscape(d.Password),
		d.Host,
		d.Port,
		d.Name,
		d.SSLMode,
	)
}

type JWTConfig struct {
	Secret           string
	Issuer           string
	AccessTTL        time.Duration
	RefreshTTL       time.Duration
	PasswordResetTTL time.Duration
	BcryptCost       int
}

type CORSConfig struct {
	AllowedOrigins []string
}

// AllowAll reports whether the wildcard origin was configured. Intended for
// local development only; Load rejects it in production.
func (c CORSConfig) AllowAll() bool {
	for _, o := range c.AllowedOrigins {
		if o == "*" {
			return true
		}
	}
	return false
}

type RateLimitConfig struct {
	Enabled bool
	RPS     float64
	Burst   int
}

type SuperAdminConfig struct {
	Email    string
	Password string
	Name     string
}

// Enabled reports whether enough was configured to seed a super admin.
func (s SuperAdminConfig) Enabled() bool { return s.Email != "" && s.Password != "" }

type CloudinaryConfig struct {
	URL         string // cloudinary://key:secret@cloud-name
	Folder      string
	MaxFileSize int64
}

// Enabled reports whether uploads are configured. When false the upload
// endpoints answer 503 rather than the service failing at boot.
func (c CloudinaryConfig) Enabled() bool { return c.URL != "" }

type CertConfig struct {
	NumberPrefix string
	SiteName     string
}

// Load reads configuration from the environment, applying defaults for
// everything safe to default. It returns an error listing every problem it
// found rather than failing on the first, so a misconfigured deployment can be
// fixed in one pass.
func Load() (*Config, error) {
	// Best effort: a missing .env is normal in production.
	_ = godotenv.Load()

	var errs []string
	fail := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf(format, args...))
	}

	cfg := &Config{
		App: AppConfig{
			Env:      envString("APP_ENV", "development"),
			LogLevel: envString("LOG_LEVEL", "info"),
		},
		Server: ServerConfig{
			Port:            envInt("SERVER_PORT", 8080, fail),
			ReadTimeout:     envDuration("SERVER_READ_TIMEOUT", 15*time.Second, fail),
			WriteTimeout:    envDuration("SERVER_WRITE_TIMEOUT", 30*time.Second, fail),
			ShutdownTimeout: envDuration("SERVER_SHUTDOWN_TIMEOUT", 15*time.Second, fail),
		},
		DB: DBConfig{
			Host:            envString("DB_HOST", "localhost"),
			Port:            envInt("DB_PORT", 5432, fail),
			User:            envString("DB_USER", "learna"),
			Password:        envString("DB_PASSWORD", ""),
			Name:            envString("DB_NAME", "learna"),
			SSLMode:         envString("DB_SSLMODE", "disable"),
			MaxConns:        int32(envInt("DB_MAX_CONNS", 20, fail)),
			MinConns:        int32(envInt("DB_MIN_CONNS", 2, fail)),
			MaxConnLifetime: envDuration("DB_MAX_CONN_LIFETIME", time.Hour, fail),
			AutoMigrate:     envBool("DB_AUTO_MIGRATE", true, fail),
		},
		JWT: JWTConfig{
			Secret:           envString("JWT_SECRET", ""),
			Issuer:           envString("JWT_ISSUER", "learna-api"),
			AccessTTL:        envDuration("JWT_ACCESS_TTL", 15*time.Minute, fail),
			RefreshTTL:       envDuration("JWT_REFRESH_TTL", 168*time.Hour, fail),
			PasswordResetTTL: envDuration("PASSWORD_RESET_TTL", time.Hour, fail),
			BcryptCost:       envInt("BCRYPT_COST", 12, fail),
		},
		CORS: CORSConfig{
			AllowedOrigins: envCSV("CORS_ALLOWED_ORIGINS", []string{"http://localhost:3000"}),
		},
		RateLimit: RateLimitConfig{
			Enabled: envBool("RATE_LIMIT_ENABLED", true, fail),
			RPS:     envFloat("RATE_LIMIT_RPS", 5, fail),
			Burst:   envInt("RATE_LIMIT_BURST", 10, fail),
		},
		SuperAdmin: SuperAdminConfig{
			Email:    strings.ToLower(strings.TrimSpace(envString("SUPER_ADMIN_EMAIL", ""))),
			Password: envString("SUPER_ADMIN_PASSWORD", ""),
			Name:     envString("SUPER_ADMIN_NAME", "Super Admin"),
		},
		Cloudinary: CloudinaryConfig{
			URL:         envString("CLOUDINARY_URL", ""),
			Folder:      envString("CLOUDINARY_FOLDER", "learna"),
			MaxFileSize: envInt64("UPLOAD_MAX_FILE_SIZE", 25<<20, fail),
		},
		Cert: CertConfig{
			NumberPrefix: envString("CERT_NUMBER_PREFIX", "LEARNA"),
			SiteName:     envString("CERT_SITE_NAME", "Learna"),
		},
	}

	switch {
	case cfg.JWT.Secret == "":
		fail("JWT_SECRET is required")
	case cfg.App.IsProduction() && len(cfg.JWT.Secret) < 32:
		fail("JWT_SECRET must be at least 32 characters in production")
	}
	if cfg.DB.Password == "" && cfg.App.IsProduction() {
		fail("DB_PASSWORD is required in production")
	}
	if cfg.JWT.BcryptCost < 10 || cfg.JWT.BcryptCost > 14 {
		fail("BCRYPT_COST must be between 10 and 14, got %d", cfg.JWT.BcryptCost)
	}
	if cfg.App.IsProduction() && cfg.CORS.AllowAll() {
		fail("CORS_ALLOWED_ORIGINS may not be a wildcard in production")
	}

	if len(errs) > 0 {
		return nil, fmt.Errorf("invalid configuration:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return cfg, nil
}

// --- env helpers ------------------------------------------------------------
//
// Each helper falls back to def when the variable is unset or empty, and
// reports (rather than returns) a parse failure so Load can collect them all.

type failFn func(format string, args ...any)

func envString(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func envInt(key string, def int, fail failFn) int {
	return int(envInt64(key, int64(def), fail))
}

func envInt64(key string, def int64, fail failFn) int64 {
	raw := envString(key, "")
	if raw == "" {
		return def
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		fail("%s must be an integer, got %q", key, raw)
		return def
	}
	return v
}

func envFloat(key string, def float64, fail failFn) float64 {
	raw := envString(key, "")
	if raw == "" {
		return def
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		fail("%s must be a number, got %q", key, raw)
		return def
	}
	return v
}

func envBool(key string, def bool, fail failFn) bool {
	raw := envString(key, "")
	if raw == "" {
		return def
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		fail("%s must be a boolean, got %q", key, raw)
		return def
	}
	return v
}

func envDuration(key string, def time.Duration, fail failFn) time.Duration {
	raw := envString(key, "")
	if raw == "" {
		return def
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		fail("%s must be a duration such as 15m or 24h, got %q", key, raw)
		return def
	}
	return v
}

func envCSV(key string, def []string) []string {
	raw := envString(key, "")
	if raw == "" {
		return def
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return def
	}
	return out
}
