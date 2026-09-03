package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/learna/learna-api/internal/models"
	"github.com/learna/learna-api/internal/utils"
)

// Auth requires a valid access token and stores the caller's identity on the
// context for downstream handlers.
func Auth(tokens *utils.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := bearerToken(c)
		if err != nil {
			utils.Fail(c, err)
			return
		}

		claims, err := tokens.ParseAccessToken(raw)
		if err != nil {
			utils.Fail(c, err)
			return
		}

		c.Set(utils.CtxUserID, claims.UserID)
		c.Set(utils.CtxUserRole, claims.Role)
		c.Next()
	}
}

// OptionalAuth populates the caller's identity when a valid token is present
// but never rejects the request. Used by public endpoints that render
// differently for signed-in users, such as the course catalog showing
// "Continue" instead of "Enroll".
func OptionalAuth(tokens *utils.TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, err := bearerToken(c)
		if err == nil {
			if claims, err := tokens.ParseAccessToken(raw); err == nil {
				c.Set(utils.CtxUserID, claims.UserID)
				c.Set(utils.CtxUserRole, claims.Role)
			}
		}
		c.Next()
	}
}

// bearerToken extracts the credential from the Authorization header.
func bearerToken(c *gin.Context) (string, error) {
	header := c.GetHeader("Authorization")
	if header == "" {
		return "", utils.ErrUnauthorized("Authorization header is required.")
	}

	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || strings.TrimSpace(token) == "" {
		return "", utils.ErrUnauthorized("Authorization header must be in the form: Bearer <token>.")
	}
	return strings.TrimSpace(token), nil
}

// CurrentUserID returns the authenticated caller's ID. It returns an error
// rather than a zero UUID when auth middleware has not run, so a route
// misconfiguration fails loudly instead of acting as an anonymous user.
func CurrentUserID(c *gin.Context) (uuid.UUID, error) {
	v, ok := c.Get(utils.CtxUserID)
	if !ok {
		return uuid.Nil, utils.ErrUnauthorized("Authentication is required.")
	}
	id, ok := v.(uuid.UUID)
	if !ok {
		return uuid.Nil, utils.ErrInternal(nil)
	}
	return id, nil
}

// CurrentUserRole returns the authenticated caller's role.
func CurrentUserRole(c *gin.Context) (models.Role, error) {
	v, ok := c.Get(utils.CtxUserRole)
	if !ok {
		return "", utils.ErrUnauthorized("Authentication is required.")
	}
	role, ok := v.(models.Role)
	if !ok {
		return "", utils.ErrInternal(nil)
	}
	return role, nil
}

// IsAuthenticated reports whether a caller was identified. Meant for handlers
// behind OptionalAuth.
func IsAuthenticated(c *gin.Context) bool {
	_, ok := c.Get(utils.CtxUserID)
	return ok
}
