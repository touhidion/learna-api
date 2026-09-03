package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/learna/learna-api/internal/database"
)

// ProgressRepository covers lesson_progress and the derived course percentage
// — features PR1..PR4.
type ProgressRepository struct{ db *database.DB }

// MarkComplete records that a lesson is finished — feature PR1.
//
// ON CONFLICT DO NOTHING makes it idempotent: a double-click, or two tabs
// marking the same lesson, must not error or move the original timestamp.
func (r *ProgressRepository) MarkComplete(ctx context.Context, userID, lessonID uuid.UUID) error {
	const q = `
		INSERT INTO lesson_progress (user_id, lesson_id, completed)
		VALUES ($1, $2, TRUE)
		ON CONFLICT (user_id, lesson_id) DO NOTHING`

	if _, err := r.db.Pool.Exec(ctx, q, userID, lessonID); err != nil {
		return translateError(err)
	}
	return nil
}

// Unmark removes the record — feature PR2.
//
// Deleting rather than flipping a flag keeps the table a simple set of what is
// done, so a count is the progress figure with no filtering.
func (r *ProgressRepository) Unmark(ctx context.Context, userID, lessonID uuid.UUID) error {
	const q = `DELETE FROM lesson_progress WHERE user_id = $1 AND lesson_id = $2`
	if _, err := r.db.Pool.Exec(ctx, q, userID, lessonID); err != nil {
		return translateError(err)
	}
	return nil
}

// CourseProgress returns completed and total lesson counts — feature PR3.
func (r *ProgressRepository) CourseProgress(
	ctx context.Context,
	userID, courseID uuid.UUID,
) (completed, total int, err error) {
	const q = `
		SELECT
			count(l.id) AS total,
			count(lp.id) AS completed
		FROM modules m
		JOIN lessons l ON l.module_id = m.id
		LEFT JOIN lesson_progress lp ON lp.lesson_id = l.id AND lp.user_id = $1
		WHERE m.course_id = $2`

	if err := r.db.Pool.QueryRow(ctx, q, userID, courseID).Scan(&total, &completed); err != nil {
		return 0, 0, translateError(err)
	}
	return completed, total, nil
}

// CompletedLessonIDs returns which of a course's lessons the user has
// finished, so the whole tree can be marked up in one query.
func (r *ProgressRepository) CompletedLessonIDs(
	ctx context.Context,
	userID, courseID uuid.UUID,
) (map[uuid.UUID]bool, error) {
	const q = `
		SELECT lp.lesson_id
		FROM lesson_progress lp
		JOIN lessons l ON l.id = lp.lesson_id
		JOIN modules m ON m.id = l.module_id
		WHERE lp.user_id = $1 AND m.course_id = $2`

	rows, err := r.db.Pool.Query(ctx, q, userID, courseID)
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
