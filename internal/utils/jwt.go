package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/learna/learna-api/internal/config"
	"github.com/learna/learna-api/internal/models"
)

// Access tokens are the only JWTs the system issues. Refresh and password
// reset tokens are opaque random strings, stored hashed in Postgres so they
// can be revoked — a stateless JWT could not be.

// AccessClaims is the payload of an access token.
type AccessClaims struct {
	UserID uuid.UUID   `json:"uid"`
	Email  string      `json:"email"`
	Role   models.Role `json:"role"`
	jwt.RegisteredClaims
}

// TokenManager signs and verifies access tokens, and mints the opaque tokens.
type TokenManager struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
	resetTTL   time.Duration
}

func NewTokenManager(cfg config.JWTConfig) *TokenManager {
	return &TokenManager{
		secret:     []byte(cfg.Secret),
		issuer:     cfg.Issuer,
		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,
		resetTTL:   cfg.PasswordResetTTL,
	}
}

func (m *TokenManager) AccessTTL() time.Duration  { return m.accessTTL }
func (m *TokenManager) RefreshTTL() time.Duration { return m.refreshTTL }
func (m *TokenManager) ResetTTL() time.Duration   { return m.resetTTL }

// GenerateAccessToken signs a short-lived HS256 token for u.
func (m *TokenManager) GenerateAccessToken(u *models.User) (string, time.Time, error) {
	now := time.Now()
	expiresAt := now.Add(m.accessTTL)

	claims := AccessClaims{
		UserID: u.ID,
		Email:  u.Email,
		Role:   u.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID.String(),
			Issuer:    m.issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			ID:        uuid.NewString(),
		},
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

// ParseAccessToken verifies the signature, algorithm and issuer, then returns
// the claims. Expired or malformed tokens come back as an APIError so the
// caller can pass them straight to Fail.
func (m *TokenManager) ParseAccessToken(raw string) (*AccessClaims, error) {
	claims := &AccessClaims{}

	_, err := jwt.ParseWithClaims(raw, claims,
		func(t *jwt.Token) (any, error) {
			// Pin the algorithm: without this an attacker could present a
			// token signed with "none" or an asymmetric algorithm.
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method %q", t.Header["alg"])
			}
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(m.issuer),
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrUnauthorized("Access token has expired.").WithCause(err)
		}
		return nil, ErrUnauthorized("Invalid access token.").WithCause(err)
	}

	if claims.UserID == uuid.Nil {
		return nil, ErrUnauthorized("Access token is missing a subject.")
	}
	return claims, nil
}

// GenerateOpaqueToken returns a 256-bit URL-safe random token. Used for
// refresh and password-reset tokens, which are stored only as hashes.
func GenerateOpaqueToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate random token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// HashToken returns the hex SHA-256 of an opaque token. SHA-256 (not bcrypt)
// is the right tool here: the input is already high-entropy random, so the
// lookup needs to be a constant-time-comparable, indexable value.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
