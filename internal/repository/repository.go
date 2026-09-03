// Package repository is the only layer that speaks SQL. Services call these
// methods; handlers never do.
//
// Repositories return ErrNotFound / ErrDuplicate for the two outcomes callers
// routinely branch on, and wrapped driver errors for everything else. Mapping
// those onto HTTP status codes is the service layer's job.
package repository

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/learna/learna-api/internal/database"
)

var (
	// ErrNotFound means the query matched no rows.
	ErrNotFound = errors.New("record not found")
	// ErrDuplicate means a unique constraint rejected the write.
	ErrDuplicate = errors.New("record already exists")
)

// Postgres error codes worth naming.
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
)

// Repositories bundles every repository so wiring passes one value around.
//
// There is no AttachmentRepository: file upload (AT1-AT4, CL2, CL3) is out of
// scope for this phase, so the attachments table exists in the schema but has
// no code path.
type Repositories struct {
	Users        *UserRepository
	Tokens       *TokenRepository
	Courses      *CourseRepository
	Modules      *ModuleRepository
	Lessons      *LessonRepository
	Enrollments  *EnrollmentRepository
	Progress     *ProgressRepository
	Certificates *CertificateRepository
	Analytics    *AnalyticsRepository
}

// New constructs every repository over the same pool.
func New(db *database.DB) *Repositories {
	return &Repositories{
		Users:        &UserRepository{db: db},
		Tokens:       &TokenRepository{db: db},
		Courses:      &CourseRepository{db: db},
		Modules:      &ModuleRepository{db: db},
		Lessons:      &LessonRepository{db: db},
		Enrollments:  &EnrollmentRepository{db: db},
		Progress:     &ProgressRepository{db: db},
		Certificates: &CertificateRepository{db: db},
		Analytics:    &AnalyticsRepository{db: db},
	}
}

// translateError normalises the driver's errors into the package sentinels.
func translateError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation:
			return ErrDuplicate
		case pgForeignKeyViolation:
			return ErrNotFound
		}
	}
	return err
}

// IsNotFound and IsDuplicate keep call sites from importing errors just to ask.
func IsNotFound(err error) bool  { return errors.Is(err, ErrNotFound) }
func IsDuplicate(err error) bool { return errors.Is(err, ErrDuplicate) }
