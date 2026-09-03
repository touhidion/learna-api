package database

import (
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	// Registers the "pgx5" database driver with golang-migrate.
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/learna/learna-api/internal/config"
)

// migrationsFS embeds the SQL files into the binary so a deployed image needs
// no migration directory alongside it.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// newMigrator builds a migrate instance over the embedded files. The caller
// must Close it; the two returned errors from migrate.Close are joined.
func newMigrator(cfg config.DBConfig) (*migrate.Migrate, error) {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, fmt.Errorf("open embedded migrations: %w", err)
	}

	// golang-migrate selects its driver from the URL scheme, and registers the
	// pgx/v5 driver as "pgx5".
	dbURL := strings.Replace(cfg.DSN(), "postgres://", "pgx5://", 1)

	m, err := migrate.NewWithSourceInstance("iofs", src, dbURL)
	if err != nil {
		return nil, fmt.Errorf("init migrator: %w", err)
	}
	return m, nil
}

// closeMigrator releases the migrator, folding its pair of errors into one.
func closeMigrator(m *migrate.Migrate) error {
	srcErr, dbErr := m.Close()
	return errors.Join(srcErr, dbErr)
}

// MigrateUp applies every pending migration. It is a no-op when the schema is
// already current, so it is safe to call on every boot.
func MigrateUp(cfg config.DBConfig) error {
	m, err := newMigrator(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = closeMigrator(m) }()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// MigrateDown rolls back n migrations, or all of them when n <= 0.
// Destructive: exposed through `make migrate-down` for development only.
func MigrateDown(cfg config.DBConfig, n int) error {
	m, err := newMigrator(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = closeMigrator(m) }()

	if n <= 0 {
		err = m.Down()
	} else {
		err = m.Steps(-n)
	}
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("roll back migrations: %w", err)
	}
	return nil
}

// MigrateVersion reports the current schema version and whether the last
// migration left the schema dirty (a failed migration that needs manual
// attention before anything else will run).
func MigrateVersion(cfg config.DBConfig) (version uint, dirty bool, err error) {
	m, err := newMigrator(cfg)
	if err != nil {
		return 0, false, err
	}
	defer func() { _ = closeMigrator(m) }()

	version, dirty, err = m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, nil
	}
	return version, dirty, err
}

// MigrateForce pins the schema version and clears the dirty flag. Use only to
// recover from a partially applied migration, after fixing the schema by hand.
func MigrateForce(cfg config.DBConfig, version int) error {
	m, err := newMigrator(cfg)
	if err != nil {
		return err
	}
	defer func() { _ = closeMigrator(m) }()

	if err := m.Force(version); err != nil {
		return fmt.Errorf("force migration version %d: %w", version, err)
	}
	return nil
}
