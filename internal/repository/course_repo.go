package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/learna/learna-api/internal/database"
	"github.com/learna/learna-api/internal/models"
	"github.com/learna/learna-api/internal/utils"
)

// CourseRepository covers the courses table — features C1..C6, PC1..PC2.
type CourseRepository struct{ db *database.DB }

const courseColumns = `id, title, slug, description, thumbnail_url, category, status, created_by, created_at, updated_at`

func scanCourse(row pgx.Row) (*models.Course, error) {
	var c models.Course
	err := row.Scan(
		&c.ID, &c.Title, &c.Slug, &c.Description, &c.ThumbnailURL,
		&c.Category, &c.Status, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, translateError(err)
	}
	return &c, nil
}

func (r *CourseRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Course, error) {
	const q = `SELECT ` + courseColumns + ` FROM courses WHERE id = $1`
	return scanCourse(r.db.Pool.QueryRow(ctx, q, id))
}

func (r *CourseRepository) GetBySlug(ctx context.Context, slug string) (*models.Course, error) {
	const q = `SELECT ` + courseColumns + ` FROM courses WHERE slug = $1`
	return scanCourse(r.db.Pool.QueryRow(ctx, q, strings.TrimSpace(slug)))
}

// SlugExists reports whether a slug is taken, optionally ignoring one course so
// that renaming a course does not collide with itself.
func (r *CourseRepository) SlugExists(ctx context.Context, slug string, excludeID uuid.UUID) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM courses WHERE slug = $1 AND ($2::uuid IS NULL OR id <> $2))`

	var exclude *uuid.UUID
	if excludeID != uuid.Nil {
		exclude = &excludeID
	}

	var exists bool
	if err := r.db.Pool.QueryRow(ctx, q, slug, exclude).Scan(&exists); err != nil {
		return false, translateError(err)
	}
	return exists, nil
}

// Create inserts a course — feature C1.
func (r *CourseRepository) Create(ctx context.Context, c *models.Course) (*models.Course, error) {
	const q = `
		INSERT INTO courses (title, slug, description, thumbnail_url, category, status, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING ` + courseColumns

	return scanCourse(r.db.Pool.QueryRow(ctx, q,
		c.Title, c.Slug, c.Description, c.ThumbnailURL, c.Category, c.Status, c.CreatedBy,
	))
}

// CourseUpdate carries a partial update; nil fields are left untouched.
type CourseUpdate struct {
	Title        *string
	Slug         *string
	Description  *string
	Category     *string
	ThumbnailURL *string
	Status       *models.CourseStatus
}

// Update applies a partial update — features C3 and C5.
func (r *CourseRepository) Update(ctx context.Context, id uuid.UUID, u CourseUpdate) (*models.Course, error) {
	const q = `
		UPDATE courses SET
			title         = COALESCE($2, title),
			slug          = COALESCE($3, slug),
			description   = COALESCE($4, description),
			category      = COALESCE($5, category),
			thumbnail_url = COALESCE($6, thumbnail_url),
			status        = COALESCE($7, status)
		WHERE id = $1
		RETURNING ` + courseColumns

	return scanCourse(r.db.Pool.QueryRow(ctx, q,
		id, u.Title, u.Slug, u.Description, u.Category, u.ThumbnailURL, u.Status,
	))
}

// Delete removes a course. Modules, lessons, attachments, enrollments and
// progress go with it via ON DELETE CASCADE — feature C4.
func (r *CourseRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM courses WHERE id = $1`, id)
	if err != nil {
		return translateError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CourseFilter narrows a listing. Zero values mean "no filter".
type CourseFilter struct {
	Search   string
	Category string
	Status   models.CourseStatus
	// PublishedOnly forces status = 'published' regardless of Status, for the
	// public catalog where a draft must never leak.
	PublishedOnly bool
}

// CourseRow is a course plus the aggregates a listing needs. Counting lessons
// and enrollments in the same query avoids the N+1 that a per-course count
// would produce.
type CourseRow struct {
	Course          models.Course
	LessonCount     int
	DurationMin     int
	EnrollmentCount int
}

// List returns one page of courses with their aggregates — features C2, PC1.
func (r *CourseRepository) List(
	ctx context.Context,
	f CourseFilter,
	p utils.Pagination,
) ([]CourseRow, int64, error) {
	var (
		where []string
		args  []any
	)
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}

	if f.PublishedOnly {
		where = append(where, `c.status = 'published'`)
	} else if f.Status != "" {
		add(`c.status = $%d`, f.Status)
	}
	if s := strings.TrimSpace(f.Search); s != "" {
		add(`(c.title ILIKE '%%' || $%d || '%%' OR c.description ILIKE '%%' || $%[1]d || '%%')`, s)
	}
	if cat := strings.TrimSpace(f.Category); cat != "" {
		add(`c.category = $%d`, cat)
	}

	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := r.db.Pool.QueryRow(ctx, `SELECT count(*) FROM courses c`+clause, args...).Scan(&total); err != nil {
		return nil, 0, translateError(err)
	}
	if total == 0 {
		return nil, 0, nil
	}

	// Aggregates come from correlated subqueries rather than JOIN + GROUP BY:
	// joining three one-to-many tables at once multiplies rows and would make
	// every count wrong.
	q := `
		SELECT ` + courseColumns2("c") + `,
			COALESCE((
				SELECT count(*) FROM lessons l
				JOIN modules m ON m.id = l.module_id
				WHERE m.course_id = c.id
			), 0) AS lesson_count,
			COALESCE((
				SELECT sum(l.duration_min) FROM lessons l
				JOIN modules m ON m.id = l.module_id
				WHERE m.course_id = c.id
			), 0) AS duration_min,
			COALESCE((
				SELECT count(*) FROM enrollments e WHERE e.course_id = c.id
			), 0) AS enrollment_count
		FROM courses c` + clause +
		fmt.Sprintf(` ORDER BY c.created_at DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)

	args = append(args, p.Limit(), p.Offset())

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, translateError(err)
	}
	defer rows.Close()

	var out []CourseRow
	for rows.Next() {
		var row CourseRow
		c := &row.Course
		if err := rows.Scan(
			&c.ID, &c.Title, &c.Slug, &c.Description, &c.ThumbnailURL,
			&c.Category, &c.Status, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
			&row.LessonCount, &row.DurationMin, &row.EnrollmentCount,
		); err != nil {
			return nil, 0, translateError(err)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, translateError(err)
	}
	return out, total, nil
}

// Stats returns the aggregates for a single course.
func (r *CourseRepository) Stats(ctx context.Context, id uuid.UUID) (lessonCount, durationMin, enrollmentCount int, err error) {
	const q = `
		SELECT
			COALESCE((SELECT count(*) FROM lessons l JOIN modules m ON m.id = l.module_id WHERE m.course_id = $1), 0),
			COALESCE((SELECT sum(l.duration_min) FROM lessons l JOIN modules m ON m.id = l.module_id WHERE m.course_id = $1), 0),
			COALESCE((SELECT count(*) FROM enrollments e WHERE e.course_id = $1), 0)`

	if err := r.db.Pool.QueryRow(ctx, q, id).Scan(&lessonCount, &durationMin, &enrollmentCount); err != nil {
		return 0, 0, 0, translateError(err)
	}
	return lessonCount, durationMin, enrollmentCount, nil
}

// Categories lists the distinct non-empty categories in use — feature C6.
func (r *CourseRepository) Categories(ctx context.Context, publishedOnly bool) ([]string, error) {
	q := `SELECT DISTINCT category FROM courses WHERE category <> ''`
	if publishedOnly {
		q += ` AND status = 'published'`
	}
	q += ` ORDER BY category`

	rows, err := r.db.Pool.Query(ctx, q)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	categories := []string{}
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, translateError(err)
		}
		categories = append(categories, c)
	}
	return categories, translateError(rows.Err())
}

// courseColumns2 qualifies the projection with a table alias.
func courseColumns2(alias string) string {
	parts := strings.Split(courseColumns, ", ")
	for i, p := range parts {
		parts[i] = alias + "." + p
	}
	return strings.Join(parts, ", ")
}
