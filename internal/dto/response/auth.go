// Package response holds the outbound payload shapes. Handlers convert models
// into these, which is what keeps PasswordHash and Cloudinary public IDs off
// the wire.
package response

import (
	"time"

	"github.com/google/uuid"

	"github.com/learna/learna-api/internal/models"
)

// User is the public view of a user.
type User struct {
	ID        uuid.UUID   `json:"id"`
	Name      string      `json:"name"`
	Email     string      `json:"email"`
	AvatarURL *string     `json:"avatar_url"`
	Role      models.Role `json:"role"`
	IsActive  bool        `json:"is_active"`
	CreatedAt time.Time   `json:"created_at"`
}

// NewUser projects a model onto the public shape.
func NewUser(u *models.User) User {
	return User{
		ID:        u.ID,
		Name:      u.Name,
		Email:     u.Email,
		AvatarURL: u.AvatarURL,
		Role:      u.Role,
		IsActive:  u.IsActive,
		CreatedAt: u.CreatedAt,
	}
}

// NewUsers projects a slice, normalising nil to an empty slice.
func NewUsers(us []*models.User) []User {
	out := make([]User, 0, len(us))
	for _, u := range us {
		out = append(out, NewUser(u))
	}
	return out
}

// TokenPair is returned by login and refresh.
//
// The refresh token is opaque and single-purpose; the client sends it only to
// /auth/refresh and /auth/logout.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"` // always "Bearer"
	ExpiresAt    time.Time `json:"expires_at"` // access token expiry
	ExpiresIn    int64     `json:"expires_in"` // access token lifetime, seconds
}

// AuthResult is the login and signup payload: who you are, plus your tokens.
type AuthResult struct {
	User   User      `json:"user"`
	Tokens TokenPair `json:"tokens"`
}

// Message is the envelope for endpoints whose only answer is an
// acknowledgement.
type Message struct {
	Message string `json:"message"`
}

// ForgotPassword returns the reset token directly until email delivery lands
// in Phase 3. Token is omitted in production builds so it never leaks over an
// unauthenticated endpoint.
type ForgotPassword struct {
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
}
