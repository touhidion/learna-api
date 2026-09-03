package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/learna/learna-api/internal/database"
	"github.com/learna/learna-api/internal/models"
)

// LessonRepository covers the lessons table — features L1..L5.
type LessonRepository struct{ db *database.DB }

const lessonColumns = `id, module_id, title, content, video_url, duration_min, sort_order, created_at, updated_at`

func scanLesson(row pgx.Row) (*models.Lesson, error) {
	var l models.Lesson
	err := row.Scan(
		&l.ID, &l.ModuleID, &l.Title, &l.Content, &l.VideoURL,
		&l.DurationMin, &l.SortOrder, &l.CreatedAt, &l.UpdatedAt,
	)
	if err != nil {
		return nil, translateError(err)
	}
	return &l, nil
}

// Create inserts a lesson — feature L1.
func (r *LessonRepository) Create(ctx context.Context, l *models.Lesson) (*models.Lesson, error) {
	const q = `
		INSERT INTO lessons (module_id, title, content, video_url, duration_min, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING ` + lessonColumns

	return scanLesson(r.db.Pool.QueryRow(ctx, q,
		l.ModuleID, l.Title, l.Content, l.VideoURL, l.DurationMin, l.SortOrder,
	))
}

func (r *LessonRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Lesson, error) {
	const q = `SELECT ` + lessonColumns + ` FROM lessons WHERE id = $1`
	return scanLesson(r.db.Pool.QueryRow(ctx, q, id))
}

// ListByModule returns a module's lessons in display order.
func (r *LessonRepository) ListByModule(ctx context.Context, moduleID uuid.UUID) ([]*models.Lesson, error) {
	const q = `SELECT ` + lessonColumns + `
		FROM lessons WHERE module_id = $1
		ORDER BY sort_order, created_at`

	return r.queryLessons(ctx, q, moduleID)
}

// ListByCourse returns every lesson in a course, ordered by module then
// position.
//
// One query for the whole tree: fetching lessons per module would be an N+1,
// and a course with twenty modules would issue twenty-one round trips to Neon.
func (r *LessonRepository) ListByCourse(ctx context.Context, courseID uuid.UUID) ([]*models.Lesson, error) {
	const q = `
		SELECT ` + lessonColumnsQualified + `
		FROM lessons l
		JOIN modules m ON m.id = l.module_id
		WHERE m.course_id = $1
		ORDER BY m.sort_order, m.created_at, l.sort_order, l.created_at`

	return r.queryLessons(ctx, q, courseID)
}

func (r *LessonRepository) queryLessons(ctx context.Context, q string, arg any) ([]*models.Lesson, error) {
	rows, err := r.db.Pool.Query(ctx, q, arg)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	lessons := []*models.Lesson{}
	for rows.Next() {
		l, err := scanLesson(rows)
		if err != nil {
			return nil, err
		}
		lessons = append(lessons, l)
	}
	return lessons, translateError(rows.Err())
}

// LessonUpdate carries a partial update; nil fields are left untouched.
type LessonUpdate struct {
	Title       *string
	Content     *string
	VideoURL    *string
	DurationMin *int
}

// Update applies a partial update — feature L2.
func (r *LessonRepository) Update(ctx context.Context, id uuid.UUID, u LessonUpdate) (*models.Lesson, error) {
	const q = `
		UPDATE lessons SET
			title        = COALESCE($2, title),
			content      = COALESCE($3, content),
			video_url    = COALESCE($4, video_url),
			duration_min = COALESCE($5, duration_min)
		WHERE id = $1
		RETURNING ` + lessonColumns

	return scanLesson(r.db.Pool.QueryRow(ctx, q, id, u.Title, u.Content, u.VideoURL, u.DurationMin))
}

// Delete removes a lesson; attachments and progress cascade — feature L3.
func (r *LessonRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM lessons WHERE id = $1`, id)
	if err != nil {
		return translateError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// NextSortOrder returns the position a new lesson should take.
func (r *LessonRepository) NextSortOrder(ctx context.Context, moduleID uuid.UUID) (int, error) {
	const q = `SELECT COALESCE(max(sort_order) + 1, 0) FROM lessons WHERE module_id = $1`
	var next int
	if err := r.db.Pool.QueryRow(ctx, q, moduleID).Scan(&next); err != nil {
		return 0, translateError(err)
	}
	return next, nil
}

// Reorder rewrites sort_order within one module, in a transaction — feature L4.
//
// Scoped to moduleID for the same reason as ModuleRepository.Reorder: an id
// from another module matches no row, so the whole call is rejected rather than
// silently reordering a neighbour's lessons.
func (r *LessonRepository) Reorder(ctx context.Context, moduleID uuid.UUID, order map[uuid.UUID]int) error {
	if len(order) == 0 {
		return nil
	}

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return translateError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const q = `UPDATE lessons SET sort_order = $3 WHERE id = $1 AND module_id = $2`
	for id, position := range order {
		tag, err := tx.Exec(ctx, q, id, moduleID, position)
		if err != nil {
			return translateError(err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
	}

	return translateError(tx.Commit(ctx))
}

// CourseIDFor resolves the course a lesson belongs to, for authorisation checks
// that start from a lesson id.
func (r *LessonRepository) CourseIDFor(ctx context.Context, lessonID uuid.UUID) (uuid.UUID, error) {
	const q = `
		SELECT m.course_id FROM lessons l
		JOIN modules m ON m.id = l.module_id
		WHERE l.id = $1`

	var courseID uuid.UUID
	if err := r.db.Pool.QueryRow(ctx, q, lessonID).Scan(&courseID); err != nil {
		return uuid.Nil, translateError(err)
	}
	return courseID, nil
}

// lessonColumnsQualified is the projection with the `l` alias, for the joined
// queries above.
const lessonColumnsQualified = `l.id, l.module_id, l.title, l.content, l.video_url, ` +
	`l.duration_min, l.sort_order, l.created_at, l.updated_at`
