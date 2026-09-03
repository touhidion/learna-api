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

type UserRepository struct{ db *database.DB }

// userColumns is the projection every scanUser expects. Keeping it in one
// constant means adding a column is a two-line change, not a hunt.
const userColumns = `id, email, password_hash, name, avatar_url, role, is_active, created_at, updated_at`

// scanUser reads one row in userColumns order.
func scanUser(row pgx.Row) (*models.User, error) {
	var u models.User
	err := row.Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name,
		&u.AvatarURL, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, translateError(err)
	}
	return &u, nil
}

// Create inserts a user and returns the stored row, so generated columns
// (id, timestamps) come back populated.
func (r *UserRepository) Create(ctx context.Context, u *models.User) (*models.User, error) {
	const q = `
		INSERT INTO users (email, password_hash, name, avatar_url, role, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING ` + userColumns

	return scanUser(r.db.Pool.QueryRow(ctx, q,
		strings.ToLower(strings.TrimSpace(u.Email)),
		u.PasswordHash, u.Name, u.AvatarURL, u.Role, u.IsActive,
	))
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	const q = `SELECT ` + userColumns + ` FROM users WHERE id = $1`
	return scanUser(r.db.Pool.QueryRow(ctx, q, id))
}

// GetByEmail matches case-insensitively, using the lower(email) unique index.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	const q = `SELECT ` + userColumns + ` FROM users WHERE lower(email) = lower($1)`
	return scanUser(r.db.Pool.QueryRow(ctx, q, strings.TrimSpace(email)))
}

// ExistsByEmail is the cheap check for signup, avoiding a full row fetch.
func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM users WHERE lower(email) = lower($1))`
	var exists bool
	if err := r.db.Pool.QueryRow(ctx, q, strings.TrimSpace(email)).Scan(&exists); err != nil {
		return false, translateError(err)
	}
	return exists, nil
}

// CountByRole reports how many users hold a role. Used by the first-run seed
// to decide whether a super admin already exists.
func (r *UserRepository) CountByRole(ctx context.Context, role models.Role) (int64, error) {
	const q = `SELECT count(*) FROM users WHERE role = $1`
	var n int64
	if err := r.db.Pool.QueryRow(ctx, q, role).Scan(&n); err != nil {
		return 0, translateError(err)
	}
	return n, nil
}

// UserFilter narrows a listing. Zero values mean "no filter".
type UserFilter struct {
	Search string
	Role   models.Role
	Active *bool
}

// List returns one page of users plus the total matching the filter.
//
// Filters are appended as parameterised clauses; nothing is ever concatenated
// from user input into the SQL text.
func (r *UserRepository) List(ctx context.Context, f UserFilter, p utils.Pagination) ([]*models.User, int64, error) {
	var (
		where []string
		args  []any
	)
	add := func(clause string, value any) {
		args = append(args, value)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}

	if s := strings.TrimSpace(f.Search); s != "" {
		add("(name ILIKE '%%' || $%d || '%%' OR email ILIKE '%%' || $%[1]d || '%%')", s)
	}
	if f.Role != "" {
		add("role = $%d", f.Role)
	}
	if f.Active != nil {
		add("is_active = $%d", *f.Active)
	}

	clause := ""
	if len(where) > 0 {
		clause = " WHERE " + strings.Join(where, " AND ")
	}

	var total int64
	if err := r.db.Pool.QueryRow(ctx, `SELECT count(*) FROM users`+clause, args...).Scan(&total); err != nil {
		return nil, 0, translateError(err)
	}
	if total == 0 {
		return nil, 0, nil
	}

	q := `SELECT ` + userColumns + ` FROM users` + clause +
		fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	args = append(args, p.Limit(), p.Offset())

	rows, err := r.db.Pool.Query(ctx, q, args...)
	if err != nil {
		return nil, 0, translateError(err)
	}
	defer rows.Close()

	var users []*models.User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, translateError(err)
	}
	return users, total, nil
}

// UserUpdate carries a partial update; nil fields are left untouched.
type UserUpdate struct {
	Name      *string
	AvatarURL *string
	Role      *models.Role
	IsActive  *bool
}

// Update applies a partial update. COALESCE keeps the existing value when a
// parameter is NULL, so one statement handles every combination of fields.
func (r *UserRepository) Update(ctx context.Context, id uuid.UUID, u UserUpdate) (*models.User, error) {
	const q = `
		UPDATE users SET
			name       = COALESCE($2, name),
			avatar_url = COALESCE($3, avatar_url),
			role       = COALESCE($4, role),
			is_active  = COALESCE($5, is_active)
		WHERE id = $1
		RETURNING ` + userColumns

	return scanUser(r.db.Pool.QueryRow(ctx, q, id, u.Name, u.AvatarURL, u.Role, u.IsActive))
}

// UpdatePassword sets a new hash. Callers are expected to revoke the user's
// refresh tokens afterwards.
func (r *UserRepository) UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error {
	const q = `UPDATE users SET password_hash = $2 WHERE id = $1`
	tag, err := r.db.Pool.Exec(ctx, q, id, hash)
	if err != nil {
		return translateError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a user; enrollments, progress and certificates cascade.
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if err != nil {
		return translateError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
