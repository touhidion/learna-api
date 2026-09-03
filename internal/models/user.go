// Package models holds the domain entities as they are stored in PostgreSQL.
//
// These are persistence-shaped structs. Anything that crosses the HTTP
// boundary goes through internal/dto instead, so a field can never leak by
// accident (PasswordHash being the obvious one).
package models

import (
	"time"

	"github.com/google/uuid"
)

// Role is the user_role enum in Postgres.
type Role string

const (
	RoleSuperAdmin Role = "super_admin"
	RoleAdmin      Role = "admin"
	RoleLearner    Role = "learner"
)

// Valid reports whether r is one of the three known roles.
func (r Role) Valid() bool {
	switch r {
	case RoleSuperAdmin, RoleAdmin, RoleLearner:
		return true
	}
	return false
}

// IsAdmin reports whether the role may reach admin-only routes. Super admins
// can do everything an admin can, plus manage other admins.
func (r Role) IsAdmin() bool { return r == RoleAdmin || r == RoleSuperAdmin }

func (r Role) String() string { return string(r) }

type User struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	Name         string     `json:"name"`
	AvatarURL    *string    `json:"avatar_url"`
	Role         Role       `json:"role"`
	IsActive     bool       `json:"is_active"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// RefreshToken is a persisted, revocable refresh token. Only the SHA-256 hash
// of the token is stored.
type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	RevokedAt *time.Time
	UserAgent *string
	IPAddress *string
	CreatedAt time.Time
}

// IsUsable reports whether the token may still be exchanged for a new access
// token.
func (t *RefreshToken) IsUsable(now time.Time) bool {
	return t.RevokedAt == nil && now.Before(t.ExpiresAt)
}

// PasswordResetToken backs the forgot/reset password flow. Single use.
type PasswordResetToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

func (t *PasswordResetToken) IsUsable(now time.Time) bool {
	return t.UsedAt == nil && now.Before(t.ExpiresAt)
}
