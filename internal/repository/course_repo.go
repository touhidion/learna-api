package repository

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/learna/learna-api/internal/database"
	"github.com/learna/learna-api/internal/models"
)

// CourseRepository covers the courses table.
//
// Implemented so far: the slug helpers, which the course service needs from
// its first line. The CRUD and listing methods land with the course module —
// see docs/learna-features.md items C1..C6.
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

// SlugExists reports whether a slug is taken, optionally ignoring one course
// so that renaming a course does not collide with itself.
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
