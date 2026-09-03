package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/learna/learna-api/internal/database"
)

// AnalyticsRepository serves the admin dashboards — features AN1, AN2.
type AnalyticsRepository struct{ db *database.DB }

// Overview holds the portal-wide totals.
type Overview struct {
	TotalUsers       int64
	TotalLearners    int64
	TotalAdmins      int64
	TotalCourses     int64
	PublishedCourses int64
	DraftCourses     int64
	ArchivedCourses  int64
	TotalEnrollments int64
	TotalCompletions int64
}

// GetOverview collects every dashboard figure in a single round trip —
// feature AN1. Nine separate COUNT queries would be nine trips to Neon for a
// page that renders them together.
func (r *AnalyticsRepository) GetOverview(ctx context.Context) (*Overview, error) {
	const q = `
		SELECT
			(SELECT count(*) FROM users),
			(SELECT count(*) FROM users WHERE role = 'learner'),
			(SELECT count(*) FROM users WHERE role IN ('admin', 'super_admin')),
			(SELECT count(*) FROM courses),
			(SELECT count(*) FROM courses WHERE status = 'published'),
			(SELECT count(*) FROM courses WHERE status = 'draft'),
			(SELECT count(*) FROM courses WHERE status = 'archived'),
			(SELECT count(*) FROM enrollments),
			(SELECT count(*) FROM enrollments WHERE completed_at IS NOT NULL)`

	var o Overview
	err := r.db.Pool.QueryRow(ctx, q).Scan(
		&o.TotalUsers, &o.TotalLearners, &o.TotalAdmins,
		&o.TotalCourses, &o.PublishedCourses, &o.DraftCourses, &o.ArchivedCourses,
		&o.TotalEnrollments, &o.TotalCompletions,
	)
	if err != nil {
		return nil, translateError(err)
	}
	return &o, nil
}

// CourseStats holds per-course engagement figures.
type CourseStats struct {
	CourseTitle     string
	EnrollmentCount int64
	CompletionCount int64
	AverageProgress float64
}

// GetCourseStats computes engagement for one course — feature AN2.
//
// Average progress is the mean over enrolled learners, so a course nobody has
// enrolled in reports 0 rather than dividing by zero.
func (r *AnalyticsRepository) GetCourseStats(ctx context.Context, courseID uuid.UUID) (*CourseStats, error) {
	const q = `
		WITH lesson_total AS (
			SELECT count(*)::float AS n
			FROM lessons l JOIN modules m ON m.id = l.module_id
			WHERE m.course_id = $1
		),
		per_user AS (
			SELECT e.user_id,
				(SELECT count(*) FROM lesson_progress lp
				 JOIN lessons l ON l.id = lp.lesson_id
				 JOIN modules m ON m.id = l.module_id
				 WHERE m.course_id = $1 AND lp.user_id = e.user_id)::float AS done
			FROM enrollments e
			WHERE e.course_id = $1
		)
		SELECT
			c.title,
			(SELECT count(*) FROM enrollments WHERE course_id = $1),
			(SELECT count(*) FROM enrollments WHERE course_id = $1 AND completed_at IS NOT NULL),
			COALESCE((
				SELECT avg(CASE WHEN lt.n = 0 THEN 0 ELSE pu.done / lt.n * 100 END)
				FROM per_user pu CROSS JOIN lesson_total lt
			), 0)
		FROM courses c
		WHERE c.id = $1`

	var s CourseStats
	err := r.db.Pool.QueryRow(ctx, q, courseID).Scan(
		&s.CourseTitle, &s.EnrollmentCount, &s.CompletionCount, &s.AverageProgress,
	)
	if err != nil {
		return nil, translateError(err)
	}
	return &s, nil
}

// LearnerProgress is one row of the per-course learner table — feature ACA1.
type LearnerProgress struct {
	UserID      uuid.UUID
	Name        string
	Email       string
	Percentage  float64
	IsCompleted bool
}

// ListCourseLearners returns each enrolled learner with their progress.
func (r *AnalyticsRepository) ListCourseLearners(ctx context.Context, courseID uuid.UUID) ([]LearnerProgress, error) {
	const q = `
		WITH lesson_total AS (
			SELECT count(*)::float AS n
			FROM lessons l JOIN modules m ON m.id = l.module_id
			WHERE m.course_id = $1
		)
		SELECT u.id, u.name, u.email,
			CASE WHEN lt.n = 0 THEN 0 ELSE (
				SELECT count(*) FROM lesson_progress lp
				JOIN lessons l ON l.id = lp.lesson_id
				JOIN modules m ON m.id = l.module_id
				WHERE m.course_id = $1 AND lp.user_id = u.id
			)::float / lt.n * 100 END AS percentage,
			e.completed_at IS NOT NULL
		FROM enrollments e
		JOIN users u ON u.id = e.user_id
		CROSS JOIN lesson_total lt
		WHERE e.course_id = $1
		ORDER BY percentage DESC, u.name`

	rows, err := r.db.Pool.Query(ctx, q, courseID)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	out := []LearnerProgress{}
	for rows.Next() {
		var lp LearnerProgress
		if err := rows.Scan(&lp.UserID, &lp.Name, &lp.Email, &lp.Percentage, &lp.IsCompleted); err != nil {
			return nil, translateError(err)
		}
		out = append(out, lp)
	}
	return out, translateError(rows.Err())
}
