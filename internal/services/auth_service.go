package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/learna/learna-api/internal/config"
	"github.com/learna/learna-api/internal/dto/request"
	"github.com/learna/learna-api/internal/dto/response"
	"github.com/learna/learna-api/internal/models"
	"github.com/learna/learna-api/internal/repository"
	"github.com/learna/learna-api/internal/utils"
)

// AuthService implements signup, login, token refresh, logout and the
// password flows.
type AuthService struct {
	cfg    *config.Config
	users  *repository.UserRepository
	tokens *repository.TokenRepository
	jwt    *utils.TokenManager
	hasher *utils.Hasher
}

func NewAuthService(d Deps) *AuthService {
	return &AuthService{
		cfg:    d.Config,
		users:  d.Repos.Users,
		tokens: d.Repos.Tokens,
		jwt:    d.Tokens,
		hasher: d.Hasher,
	}
}

// ClientInfo describes where a session was created from. Stored alongside a
// refresh token so a user can later be shown their active sessions.
type ClientInfo struct {
	UserAgent string
	IPAddress string
}

// Signup registers a self-service learner.
//
// Self-registration always produces a learner; admin accounts are created only
// through the admin API or the first-run seed. The role is not read from the
// request at all, so it cannot be escalated by adding a field to the JSON.
func (s *AuthService) Signup(ctx context.Context, req request.Signup, client ClientInfo) (*response.AuthResult, error) {
	email := normalizeEmail(req.Email)

	exists, err := s.users.ExistsByEmail(ctx, email)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}
	if exists {
		return nil, utils.ErrConflict("An account with this email already exists.")
	}

	hash, err := s.hasher.Hash(req.Password)
	if err != nil {
		return nil, utils.AsAPIError(err)
	}

	user, err := s.users.Create(ctx, &models.User{
		Email:        email,
		PasswordHash: hash,
		Name:         strings.TrimSpace(req.Name),
		Role:         models.RoleLearner,
		IsActive:     true,
	})
	if err != nil {
		// Lost the race against a concurrent signup with the same email.
		if repository.IsDuplicate(err) {
			return nil, utils.ErrConflict("An account with this email already exists.")
		}
		return nil, utils.ErrInternal(err)
	}

	return s.issueSession(ctx, user, client)
}

// Login verifies credentials and starts a session.
//
// Every failure path returns the same message. Distinguishing "no such user"
// from "wrong password" would turn the endpoint into an account enumerator.
func (s *AuthService) Login(ctx context.Context, req request.Login, client ClientInfo) (*response.AuthResult, error) {
	const invalid = "Invalid email or password."

	user, err := s.users.GetByEmail(ctx, req.Email)
	if err != nil {
		if repository.IsNotFound(err) {
			// Spend comparable time hashing so response latency does not
			// reveal whether the account exists.
			_, _ = s.hasher.Hash(req.Password)
			return nil, utils.ErrUnauthorized(invalid)
		}
		return nil, utils.ErrInternal(err)
	}

	if !s.hasher.Verify(user.PasswordHash, req.Password) {
		return nil, utils.ErrUnauthorized(invalid)
	}
	if !user.IsActive {
		return nil, utils.ErrForbidden("This account has been deactivated. Contact an administrator.")
	}

	return s.issueSession(ctx, user, client)
}

// Refresh exchanges a valid refresh token for a new token pair.
//
// The presented token is revoked and replaced (rotation), so a stolen token is
// usable at most once before the legitimate client's next refresh invalidates
// the thief's copy.
func (s *AuthService) Refresh(ctx context.Context, rawToken string, client ClientInfo) (*response.TokenPair, error) {
	const invalid = "Refresh token is invalid or has expired."

	stored, err := s.tokens.GetRefreshToken(ctx, utils.HashToken(rawToken))
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, utils.ErrUnauthorized(invalid)
		}
		return nil, utils.ErrInternal(err)
	}
	if !stored.IsUsable(time.Now()) {
		return nil, utils.ErrUnauthorized(invalid)
	}

	user, err := s.users.GetByID(ctx, stored.UserID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, utils.ErrUnauthorized(invalid)
		}
		return nil, utils.ErrInternal(err)
	}
	if !user.IsActive {
		return nil, utils.ErrForbidden("This account has been deactivated.")
	}

	if err := s.tokens.RevokeRefreshToken(ctx, stored.TokenHash); err != nil {
		return nil, utils.ErrInternal(err)
	}

	result, err := s.issueSession(ctx, user, client)
	if err != nil {
		return nil, err
	}
	return &result.Tokens, nil
}

// Logout revokes the presented refresh token. It succeeds even for an unknown
// token: logout is idempotent, and reporting "no such token" would leak which
// tokens exist.
func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	if err := s.tokens.RevokeRefreshToken(ctx, utils.HashToken(rawToken)); err != nil {
		return utils.ErrInternal(err)
	}
	return nil
}

// ForgotPassword issues a single-use reset token.
//
// It returns the token in the response because email delivery is Phase 3 —
// but only outside production, so an unauthenticated caller can never harvest
// reset tokens from a deployed instance. An unknown email is answered exactly
// like a known one.
func (s *AuthService) ForgotPassword(ctx context.Context, req request.ForgotPassword) (*response.ForgotPassword, error) {
	const ack = "If an account exists for that email, a password reset link has been sent."

	user, err := s.users.GetByEmail(ctx, req.Email)
	if err != nil {
		if repository.IsNotFound(err) {
			return &response.ForgotPassword{Message: ack}, nil
		}
		return nil, utils.ErrInternal(err)
	}

	raw, err := utils.GenerateOpaqueToken()
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	_, err = s.tokens.CreatePasswordResetToken(ctx, &models.PasswordResetToken{
		UserID:    user.ID,
		TokenHash: utils.HashToken(raw),
		ExpiresAt: time.Now().Add(s.jwt.ResetTTL()),
	})
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	out := &response.ForgotPassword{Message: ack}
	if !s.cfg.App.IsProduction() {
		out.Token = raw
	}
	return out, nil
}

// ResetPassword consumes a reset token and sets a new password. Every session
// for that user is revoked, on the assumption the reset was prompted by a
// compromise.
func (s *AuthService) ResetPassword(ctx context.Context, req request.ResetPassword) error {
	const invalid = "This password reset link is invalid or has expired."

	hash := utils.HashToken(req.Token)

	stored, err := s.tokens.GetPasswordResetToken(ctx, hash)
	if err != nil {
		if repository.IsNotFound(err) {
			return utils.ErrBadRequest(invalid)
		}
		return utils.ErrInternal(err)
	}
	if !stored.IsUsable(time.Now()) {
		return utils.ErrBadRequest(invalid)
	}

	newHash, err := s.hasher.Hash(req.Password)
	if err != nil {
		return utils.AsAPIError(err)
	}

	// Consume first: the conditional UPDATE is atomic, so two requests racing
	// on the same token cannot both go on to change the password.
	if err := s.tokens.ConsumePasswordResetToken(ctx, hash); err != nil {
		if repository.IsNotFound(err) {
			return utils.ErrBadRequest(invalid)
		}
		return utils.ErrInternal(err)
	}

	if err := s.users.UpdatePassword(ctx, stored.UserID, newHash); err != nil {
		return utils.ErrInternal(err)
	}
	if err := s.tokens.RevokeAllForUser(ctx, stored.UserID); err != nil {
		return utils.ErrInternal(err)
	}
	return nil
}

// ChangePassword updates the caller's own password after re-verifying the
// current one, then signs out every other session.
func (s *AuthService) ChangePassword(ctx context.Context, userID uuid.UUID, req request.ChangePassword) error {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if repository.IsNotFound(err) {
			return utils.ErrNotFound("User not found.")
		}
		return utils.ErrInternal(err)
	}

	if !s.hasher.Verify(user.PasswordHash, req.CurrentPassword) {
		return utils.ErrUnauthorized("Current password is incorrect.")
	}
	if req.CurrentPassword == req.NewPassword {
		return utils.ErrValidation("New password must differ from the current one.")
	}

	newHash, err := s.hasher.Hash(req.NewPassword)
	if err != nil {
		return utils.AsAPIError(err)
	}
	if err := s.users.UpdatePassword(ctx, userID, newHash); err != nil {
		return utils.ErrInternal(err)
	}
	if err := s.tokens.RevokeAllForUser(ctx, userID); err != nil {
		return utils.ErrInternal(err)
	}
	return nil
}

// Me returns the caller's own profile.
func (s *AuthService) Me(ctx context.Context, userID uuid.UUID) (*response.User, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, utils.ErrNotFound("User not found.")
		}
		return nil, utils.ErrInternal(err)
	}
	out := response.NewUser(user)
	return &out, nil
}

// UpdateProfile applies a partial update to the caller's own profile. Role and
// active status are deliberately not reachable here.
func (s *AuthService) UpdateProfile(ctx context.Context, userID uuid.UUID, req request.UpdateProfile) (*response.User, error) {
	if req.Name == nil && req.AvatarURL == nil {
		return nil, utils.ErrValidation("No fields to update.")
	}

	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		req.Name = &trimmed
	}

	user, err := s.users.Update(ctx, userID, repository.UserUpdate{
		Name:      req.Name,
		AvatarURL: req.AvatarURL,
	})
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, utils.ErrNotFound("User not found.")
		}
		return nil, utils.ErrInternal(err)
	}
	out := response.NewUser(user)
	return &out, nil
}

// EnsureSuperAdmin creates the configured super admin when none exists.
//
// Idempotent, and called on every boot. It only ever creates the first super
// admin: once one exists the seed is skipped, so changing the env vars later
// does not silently mint another privileged account.
func (s *AuthService) EnsureSuperAdmin(ctx context.Context) (created bool, err error) {
	if !s.cfg.SuperAdmin.Enabled() {
		return false, nil
	}

	count, err := s.users.CountByRole(ctx, models.RoleSuperAdmin)
	if err != nil {
		return false, fmt.Errorf("count super admins: %w", err)
	}
	if count > 0 {
		return false, nil
	}

	hash, err := s.hasher.Hash(s.cfg.SuperAdmin.Password)
	if err != nil {
		return false, fmt.Errorf("hash super admin password: %w", err)
	}

	_, err = s.users.Create(ctx, &models.User{
		Email:        normalizeEmail(s.cfg.SuperAdmin.Email),
		PasswordHash: hash,
		Name:         s.cfg.SuperAdmin.Name,
		Role:         models.RoleSuperAdmin,
		IsActive:     true,
	})
	if err != nil {
		// The email may already belong to a non-super-admin account.
		if repository.IsDuplicate(err) {
			return false, fmt.Errorf("cannot seed super admin: %q is already registered", s.cfg.SuperAdmin.Email)
		}
		return false, fmt.Errorf("create super admin: %w", err)
	}
	return true, nil
}

// issueSession mints an access token plus a persisted refresh token.
func (s *AuthService) issueSession(ctx context.Context, user *models.User, client ClientInfo) (*response.AuthResult, error) {
	accessToken, expiresAt, err := s.jwt.GenerateAccessToken(user)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	rawRefresh, err := utils.GenerateOpaqueToken()
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	_, err = s.tokens.CreateRefreshToken(ctx, &models.RefreshToken{
		UserID:    user.ID,
		TokenHash: utils.HashToken(rawRefresh),
		ExpiresAt: time.Now().Add(s.jwt.RefreshTTL()),
		UserAgent: nullable(client.UserAgent),
		IPAddress: nullable(client.IPAddress),
	})
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	return &response.AuthResult{
		User: response.NewUser(user),
		Tokens: response.TokenPair{
			AccessToken:  accessToken,
			RefreshToken: rawRefresh,
			TokenType:    "Bearer",
			ExpiresAt:    expiresAt,
			ExpiresIn:    int64(s.jwt.AccessTTL().Seconds()),
		},
	}, nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// nullable maps an empty string to a NULL column rather than storing "".
func nullable(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
