package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/learna/learna-api/internal/database"
	"github.com/learna/learna-api/internal/models"
)

// TokenRepository stores refresh and password-reset tokens. Both are held as
// SHA-256 hashes; a database leak therefore yields no usable credential.
type TokenRepository struct{ db *database.DB }

// --- refresh tokens ---------------------------------------------------------

const refreshColumns = `id, user_id, token_hash, expires_at, revoked_at, user_agent, ip_address, created_at`

func scanRefreshToken(row pgx.Row) (*models.RefreshToken, error) {
	var t models.RefreshToken
	err := row.Scan(
		&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt,
		&t.RevokedAt, &t.UserAgent, &t.IPAddress, &t.CreatedAt,
	)
	if err != nil {
		return nil, translateError(err)
	}
	return &t, nil
}

func (r *TokenRepository) CreateRefreshToken(ctx context.Context, t *models.RefreshToken) (*models.RefreshToken, error) {
	const q = `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at, user_agent, ip_address)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING ` + refreshColumns

	return scanRefreshToken(r.db.Pool.QueryRow(ctx, q,
		t.UserID, t.TokenHash, t.ExpiresAt, t.UserAgent, t.IPAddress,
	))
}

// GetRefreshToken looks a token up by its hash. Expiry and revocation are left
// for the caller to judge via RefreshToken.IsUsable, so the service can tell
// "unknown token" apart from "expired token" when deciding what to log.
func (r *TokenRepository) GetRefreshToken(ctx context.Context, tokenHash string) (*models.RefreshToken, error) {
	const q = `SELECT ` + refreshColumns + ` FROM refresh_tokens WHERE token_hash = $1`
	return scanRefreshToken(r.db.Pool.QueryRow(ctx, q, tokenHash))
}

// RevokeRefreshToken marks a single token unusable. Revoking an
// already-revoked token is a no-op, which keeps logout idempotent.
func (r *TokenRepository) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	const q = `UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`
	if _, err := r.db.Pool.Exec(ctx, q, tokenHash); err != nil {
		return translateError(err)
	}
	return nil
}

// RevokeAllForUser signs a user out everywhere. Called on password change and
// on deactivation.
func (r *TokenRepository) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	const q = `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`
	if _, err := r.db.Pool.Exec(ctx, q, userID); err != nil {
		return translateError(err)
	}
	return nil
}

// --- password reset tokens --------------------------------------------------

const resetColumns = `id, user_id, token_hash, expires_at, used_at, created_at`

func scanResetToken(row pgx.Row) (*models.PasswordResetToken, error) {
	var t models.PasswordResetToken
	err := row.Scan(&t.ID, &t.UserID, &t.TokenHash, &t.ExpiresAt, &t.UsedAt, &t.CreatedAt)
	if err != nil {
		return nil, translateError(err)
	}
	return &t, nil
}

// CreatePasswordResetToken invalidates any outstanding token for the user
// before issuing a new one, so only the most recent reset link works.
func (r *TokenRepository) CreatePasswordResetToken(ctx context.Context, t *models.PasswordResetToken) (*models.PasswordResetToken, error) {
	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		return nil, translateError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }() // no-op once committed

	const invalidate = `UPDATE password_reset_tokens SET used_at = now() WHERE user_id = $1 AND used_at IS NULL`
	if _, err := tx.Exec(ctx, invalidate, t.UserID); err != nil {
		return nil, translateError(err)
	}

	const insert = `
		INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
		RETURNING ` + resetColumns

	created, err := scanResetToken(tx.QueryRow(ctx, insert, t.UserID, t.TokenHash, t.ExpiresAt))
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, translateError(err)
	}
	return created, nil
}

func (r *TokenRepository) GetPasswordResetToken(ctx context.Context, tokenHash string) (*models.PasswordResetToken, error) {
	const q = `SELECT ` + resetColumns + ` FROM password_reset_tokens WHERE token_hash = $1`
	return scanResetToken(r.db.Pool.QueryRow(ctx, q, tokenHash))
}

// ConsumePasswordResetToken marks a token used, and reports ErrNotFound when
// it was already spent. The conditional UPDATE makes this atomic, so two
// concurrent resets cannot both succeed.
func (r *TokenRepository) ConsumePasswordResetToken(ctx context.Context, tokenHash string) error {
	const q = `UPDATE password_reset_tokens SET used_at = now() WHERE token_hash = $1 AND used_at IS NULL`
	tag, err := r.db.Pool.Exec(ctx, q, tokenHash)
	if err != nil {
		return translateError(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// --- housekeeping -----------------------------------------------------------

// DeleteExpired clears out tokens that expired before cutoff. Wire it to a
// periodic job; nothing depends on it for correctness.
func (r *TokenRepository) DeleteExpired(ctx context.Context, cutoff time.Time) (int64, error) {
	var deleted int64

	tag, err := r.db.Pool.Exec(ctx, `DELETE FROM refresh_tokens WHERE expires_at < $1`, cutoff)
	if err != nil {
		return 0, translateError(err)
	}
	deleted += tag.RowsAffected()

	tag, err = r.db.Pool.Exec(ctx, `DELETE FROM password_reset_tokens WHERE expires_at < $1`, cutoff)
	if err != nil {
		return deleted, translateError(err)
	}
	return deleted + tag.RowsAffected(), nil
}
