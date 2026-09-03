package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/learna/learna-api/internal/database"
	"github.com/learna/learna-api/internal/models"
)

// ModuleRepository covers the modules table — features M1..M4.
type ModuleRepository struct{ db *database.DB }

const moduleColumns = `id, course_id, title, description, sort_order, created_at, updated_at`

func scanModule(row pgx.Row) (*models.Module, error) {
	var m models.Module
	err := row.Scan(
		&m.ID, &m.CourseID, &m.Title, &m.Description,
		&m.SortOrder, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, translateError(err)
	}
	return &m, nil
}

// Create inserts a module — feature M1.
func (r *ModuleRepository) Create(ctx context.Context, m *models.Module) (*models.Module, error) {
	const q = `
		INSERT INTO modules (course_id, title, description, sort_order)
		VALUES ($1, $2, $3, $4)
		RETURNING ` + moduleColumns

	return scanModule(r.db.Pool.QueryRow(ctx, q, m.CourseID, m.Title, m.Description, m.SortOrder))
}

func (r *ModuleRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Module, error) {
	const q = `SELECT ` + moduleColumns + ` FROM modules WHERE id = $1`
	return scanModule(r.db.Pool.QueryRow(ctx, q, id))
}

// ListByCourse returns a course's modules in display order.
func (r *ModuleRepository) ListByCourse(ctx context.Context, courseID uuid.UUID) ([]*models.Module, error) {
	const q = `SELECT ` + moduleColumns + `
		FROM modules WHERE course_id = $1
		ORDER BY sort_order, created_at`

	rows, err := r.db.Pool.Query(ctx, q, courseID)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	modules := []*models.Module{}
	for rows.Next() {
		m, err := scanModule(rows)
		if err != nil {
			return nil, err
		}
		modules = append(modules, m)
	}
	return modules, translateError(rows.Err())
}

// ModuleUpdate carries a partial update; nil fields are left untouched.
type ModuleUpdate struct {
	Title       *string
	Description *string
}

// Update applies a partial update — feature M2.
func (r *ModuleRepository) Update(ctx context.Context, id uuid.UUID, u ModuleUpdate) (*models.Module, error) {
	const q = `
		UPDATE modules SET
			title       = COALESCE($2, title),
			description = COALESCE($3, description)
		WHERE id = $1
		RETURNING ` + moduleColumns

	return scanModule(r.db.Pool.QueryRow(ctx, q, id, u.Title, u.Description))
}

// Delete removes a module; lessons and their attachments cascade — feature M3.
func (r *ModuleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM modules WHERE id = $1`, id)
	if err != nil {
		return translateError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// NextSortOrder returns the position a new module should take, so callers can
// append without having to count first.
func (r *ModuleRepository) NextSortOrder(ctx context.Context, courseID uuid.UUID) (int, error) {
	const q = `SELECT COALESCE(max(sort_order) + 1, 0) FROM modules WHERE course_id = $1`
	var next int
	if err := r.db.Pool.QueryRow(ctx, q, courseID).Scan(&next); err != nil {
		return 0, translateError(err)
	}
	return next, nil
}

// Reorder rewrites sort_order for a course's modules in one transaction —
// feature M4.
//
// Every UPDATE is scoped to courseID, so an id belonging to another course
// matches no row. Requiring every id to match is what stops a caller reordering
// modules they were not editing, and the transaction means no client ever
// observes a half-applied ordering.
func (r *ModuleRepository) Reorder(ctx context.Context, courseID uuid.UUID, order map[uuid.UUID]int) error {
	if len(order) == 0 {
		return nil
	}

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return translateError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op once committed

	const q = `UPDATE modules SET sort_order = $3 WHERE id = $1 AND course_id = $2`
	for id, position := range order {
		tag, err := tx.Exec(ctx, q, id, courseID, position)
		if err != nil {
			return translateError(err)
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
	}

	return translateError(tx.Commit(ctx))
}

// CountByCourse reports how many modules a course has.
func (r *ModuleRepository) CountByCourse(ctx context.Context, courseID uuid.UUID) (int, error) {
	const q = `SELECT count(*) FROM modules WHERE course_id = $1`
	var n int
	if err := r.db.Pool.QueryRow(ctx, q, courseID).Scan(&n); err != nil {
		return 0, translateError(err)
	}
	return n, nil
}
