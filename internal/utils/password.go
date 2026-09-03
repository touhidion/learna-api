package utils

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// Hasher wraps bcrypt at the configured cost.
type Hasher struct{ cost int }

func NewHasher(cost int) *Hasher {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		cost = bcrypt.DefaultCost
	}
	return &Hasher{cost: cost}
}

// Hash returns the bcrypt hash of a plaintext password.
//
// bcrypt silently truncates input beyond 72 bytes, so anything longer is
// rejected rather than accepted with only its prefix checked.
func (h *Hasher) Hash(password string) (string, error) {
	if len(password) > 72 {
		return "", ErrValidation("Password must be at most 72 bytes.")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(hash), nil
}

// Verify reports whether password matches hash. It is deliberately boolean:
// callers must not distinguish "no such user" from "wrong password" in what
// they return to the client.
func (h *Hasher) Verify(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
