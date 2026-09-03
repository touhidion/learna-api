package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/learna/learna-api/internal/database"
	"github.com/learna/learna-api/internal/models"
	"github.com/learna/learna-api/internal/utils"
)

// EnrollmentRepository covers the enrollments table — features E1..E4.
type EnrollmentRepository struct{ db *database.DB }

const enrollmentColumns = `id, user_id, course_id, enrolled_at, completed_at`

func scanEnrollment(row pgx.Row) (*models.Enrollment, error) {
	var e models.Enrollment
	if err := row.Scan(&e.ID, &e.UserID, &e.CourseID, &e.EnrolledAt, &e.CompletedAt); err != nil {
		return nil, translateError(err)
	}
	return &e, nil
}

// Create enrolls a user — feature E1. A repeat enrollment hits the
// UNIQUE(user_id, course_id) constraint and surfaces as ErrDuplicate.
func (r *EnrollmentRepository) Create(ctx context.Context, userID, courseID uuid.UUID) (*models.Enrollment, error) {
	const q = `
		INSERT INTO enrollments (user_id, course_id)
		VALUES ($1, $2)
		RETURNING ` + enrollmentColumns

	return scanEnrollment(r.db.Pool.QueryRow(ctx, q, userID, courseID))
}

// Delete unenrolls a user. Progress rows are keyed by lesson, not enrollment,
// so they are removed explicitly in the same transaction — feature E2.
func (r *EnrollmentRepository) Delete(ctx context.Context, userID, courseID uuid.UUID) error {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return translateError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const clearProgress = `
		DELETE FROM lesson_progress
		WHERE user_id = $1 AND lesson_id IN (
			SELECT l.id FROM lessons l
			JOIN modules m ON m.id = l.module_id
			WHERE m.course_id = $2
		)`
	if _, err := tx.Exec(ctx, clearProgress, userID, courseID); err != nil {
		return translateError(err)
	}

	tag, err := tx.Exec(ctx,
		`DELETE FROM enrollments WHERE user_id = $1 AND course_id = $2`, userID, courseID)
	if err != nil {
		return translateError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}

	return translateError(tx.Commit(ctx))
}

// Get returns one enrollment, or ErrNotFound.
func (r *EnrollmentRepository) Get(ctx context.Context, userID, courseID uuid.UUID) (*models.Enrollment, error) {
	const q = `SELECT ` + enrollmentColumns + ` FROM enrollments WHERE user_id = $1 AND course_id = $2`
	return scanEnrollment(r.db.Pool.QueryRow(ctx, q, userID, courseID))
}

// Exists is the cheap check behind the lesson-content guard — feature E4.
func (r *EnrollmentRepository) Exists(ctx context.Context, userID, courseID uuid.UUID) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM enrollments WHERE user_id = $1 AND course_id = $2)`
	var exists bool
	if err := r.db.Pool.QueryRow(ctx, q, userID, courseID).Scan(&exists); err != nil {
		return false, translateError(err)
	}
	return exists, nil
}

// EnrollmentRow is an enrollment joined to its course and progress counts.
type EnrollmentRow struct {
	Enrollment       models.Enrollment
	Course           models.Course
	TotalLessons     int
	CompletedLessons int
	DurationMin      int
}

// ListByUser returns the caller's enrolled courses with progress — feature E3.
//
// Counts come from correlated subqueries: joining lessons and progress
// directly would multiply rows and make both counts wrong.
func (r *EnrollmentRepository) ListByUser(
	ctx context.Context,
	userID uuid.UUID,
	p utils.Pagination,
) ([]EnrollmentRow, int64, error) {
	var total int64
	if err := r.db.Pool.QueryRow(ctx,
		`SELECT count(*) FROM enrollments WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, translateError(err)
	}
	if total == 0 {
		return nil, 0, nil
	}

	const q = `
		SELECT
			e.id, e.user_id, e.course_id, e.enrolled_at, e.completed_at,
			c.id, c.title, c.slug, c.description, c.thumbnail_url, c.category,
			c.status, c.created_by, c.created_at, c.updated_at,
			COALESCE((
				SELECT count(*) FROM lessons l
				JOIN modules m ON m.id = l.module_id
				WHERE m.course_id = c.id
			), 0) AS total_lessons,
			COALESCE((
				SELECT count(*) FROM lesson_progress lp
				JOIN lessons l ON l.id = lp.lesson_id
				JOIN modules m ON m.id = l.module_id
				WHERE m.course_id = c.id AND lp.user_id = e.user_id
			), 0) AS completed_lessons,
			COALESCE((
				SELECT sum(l.duration_min) FROM lessons l
				JOIN modules m ON m.id = l.module_id
				WHERE m.course_id = c.id
			), 0) AS duration_min
		FROM enrollments e
		JOIN courses c ON c.id = e.course_id
		WHERE e.user_id = $1
		ORDER BY e.enrolled_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Pool.Query(ctx, q, userID, p.Limit(), p.Offset())
	if err != nil {
		return nil, 0, translateError(err)
	}
	defer rows.Close()

	var out []EnrollmentRow
	for rows.Next() {
		var row EnrollmentRow
		e, c := &row.Enrollment, &row.Course
		if err := rows.Scan(
			&e.ID, &e.UserID, &e.CourseID, &e.EnrolledAt, &e.CompletedAt,
			&c.ID, &c.Title, &c.Slug, &c.Description, &c.ThumbnailURL, &c.Category,
			&c.Status, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
			&row.TotalLessons, &row.CompletedLessons, &row.DurationMin,
		); err != nil {
			return nil, 0, translateError(err)
		}
		out = append(out, row)
	}
	return out, total, translateError(rows.Err())
}

// SetCompletedAt stamps or clears the completion time — feature PR4.
//
// Passing nil clears it, which is what happens when a learner un-completes a
// lesson after finishing a course.
func (r *EnrollmentRepository) SetCompletedAt(
	ctx context.Context,
	userID, courseID uuid.UUID,
	at *time.Time,
) error {
	const q = `UPDATE enrollments SET completed_at = $3 WHERE user_id = $1 AND course_id = $2`
	if _, err := r.db.Pool.Exec(ctx, q, userID, courseID, at); err != nil {
		return translateError(err)
	}
	return nil
}

// ProgressForCourses returns completed/total lesson counts for one user across
// several courses at once, so a catalog page can show progress without issuing
// a query per card.
func (r *EnrollmentRepository) ProgressForCourses(
	ctx context.Context,
	userID uuid.UUID,
	courseIDs []uuid.UUID,
) (map[uuid.UUID][2]int, error) {
	if len(courseIDs) == 0 {
		return map[uuid.UUID][2]int{}, nil
	}

	const q = `
		SELECT m.course_id,
			count(l.id) AS total,
			count(lp.id) AS completed
		FROM modules m
		JOIN lessons l ON l.module_id = m.id
		LEFT JOIN lesson_progress lp ON lp.lesson_id = l.id AND lp.user_id = $1
		WHERE m.course_id = ANY($2)
		GROUP BY m.course_id`

	rows, err := r.db.Pool.Query(ctx, q, userID, courseIDs)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	out := make(map[uuid.UUID][2]int, len(courseIDs))
	for rows.Next() {
		var id uuid.UUID
		var total, completed int
		if err := rows.Scan(&id, &total, &completed); err != nil {
			return nil, translateError(err)
		}
		out[id] = [2]int{completed, total}
	}
	return out, translateError(rows.Err())
}

// EnrolledCourseIDs returns the subset of the given courses the user is
// enrolled in, for badging a catalog listing.
func (r *EnrollmentRepository) EnrolledCourseIDs(
	ctx context.Context,
	userID uuid.UUID,
	courseIDs []uuid.UUID,
) (map[uuid.UUID]bool, error) {
	if len(courseIDs) == 0 {
		return map[uuid.UUID]bool{}, nil
	}

	const q = `SELECT course_id FROM enrollments WHERE user_id = $1 AND course_id = ANY($2)`
	rows, err := r.db.Pool.Query(ctx, q, userID, courseIDs)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	out := map[uuid.UUID]bool{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, translateError(err)
		}
		out[id] = true
	}
	return out, translateError(rows.Err())
}
