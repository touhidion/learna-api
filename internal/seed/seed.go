// Package seed populates the database with the rows the application needs to
// be usable at all — currently the first super admin (feature A8).
//
// Seeding is Go rather than SQL because the values come from the environment
// and the password must be bcrypt-hashed at the configured cost. A SQL file
// could do neither.
//
// Every seeder here must be idempotent: Run executes on every `-migrate=up`
// and on every server boot, so it has to be safe to repeat indefinitely.
package seed

import (
	"context"
	"fmt"

	"github.com/learna/learna-api/internal/config"
	"github.com/learna/learna-api/internal/models"
	"github.com/learna/learna-api/internal/repository"
	"github.com/learna/learna-api/internal/utils"
)

// Seeder writes the baseline rows.
type Seeder struct {
	cfg    *config.Config
	users  *repository.UserRepository
	tokens *repository.TokenRepository
	hasher *utils.Hasher
}

func New(cfg *config.Config, repos *repository.Repositories, hasher *utils.Hasher) *Seeder {
	return &Seeder{
		cfg:    cfg,
		users:  repos.Users,
		tokens: repos.Tokens,
		hasher: hasher,
	}
}

// Action is what a seeder did, for logging.
type Action string

const (
	// ActionCreated means the row did not exist and was inserted.
	ActionCreated Action = "created"
	// ActionUpdated means the row existed and was changed.
	ActionUpdated Action = "updated"
	// ActionSkipped means the row already existed and was left alone.
	ActionSkipped Action = "skipped"
	// ActionDisabled means the seeder was not configured to run.
	ActionDisabled Action = "disabled"
)

// Result reports the outcome of one seeder.
type Result struct {
	Name   string
	Action Action
	Detail string
}

// Run executes every seeder in order and returns one Result each.
//
// It stops at the first error: a later seeder may depend on an earlier one, so
// continuing past a failure would report misleading results.
func (s *Seeder) Run(ctx context.Context) ([]Result, error) {
	steps := []struct {
		name string
		fn   func(context.Context) (Result, error)
	}{
		{"super_admin", s.superAdmin},
	}

	results := make([]Result, 0, len(steps))
	for _, step := range steps {
		res, err := step.fn(ctx)
		if err != nil {
			return results, fmt.Errorf("seed %s: %w", step.name, err)
		}
		results = append(results, res)
	}
	return results, nil
}

// superAdmin creates the first super admin from SUPER_ADMIN_* (feature A8).
//
// The guard is "does any super admin exist", not "does this email exist", so
// once the portal has an owner the seed stops touching it. Changing
// SUPER_ADMIN_EMAIL later therefore does not mint a second privileged account
// — which is the point, not an oversight.
//
// The one exception is SUPER_ADMIN_RESET_PASSWORD=true, which re-syncs the
// existing account's password to match the environment. That is opt-in because
// on a deploy it would otherwise silently undo a password the owner rotated
// through the API.
func (s *Seeder) superAdmin(ctx context.Context) (Result, error) {
	const name = "super_admin"

	if !s.cfg.SuperAdmin.Enabled() {
		return Result{
			Name:   name,
			Action: ActionDisabled,
			Detail: "SUPER_ADMIN_EMAIL or SUPER_ADMIN_PASSWORD is not set",
		}, nil
	}

	email := s.cfg.SuperAdmin.Email

	count, err := s.users.CountByRole(ctx, models.RoleSuperAdmin)
	if err != nil {
		return Result{}, fmt.Errorf("count super admins: %w", err)
	}

	if count > 0 {
		if !s.cfg.SuperAdmin.ResetPassword {
			return Result{
				Name:   name,
				Action: ActionSkipped,
				Detail: fmt.Sprintf("%d super admin(s) already exist", count),
			}, nil
		}
		return s.resetSuperAdminPassword(ctx, email)
	}

	hash, err := s.hasher.Hash(s.cfg.SuperAdmin.Password)
	if err != nil {
		return Result{}, fmt.Errorf("hash password: %w", err)
	}

	if _, err := s.users.Create(ctx, &models.User{
		Email:        email,
		PasswordHash: hash,
		Name:         s.cfg.SuperAdmin.Name,
		Role:         models.RoleSuperAdmin,
		IsActive:     true,
	}); err != nil {
		// The address may already belong to a learner or admin account, in
		// which case the unique index rejects the insert. Say so plainly
		// rather than surfacing a raw constraint violation.
		if repository.IsDuplicate(err) {
			return Result{}, fmt.Errorf(
				"%q is already registered to a non-super-admin account; "+
					"use a different SUPER_ADMIN_EMAIL or promote that user instead", email)
		}
		return Result{}, fmt.Errorf("create user: %w", err)
	}

	return Result{Name: name, Action: ActionCreated, Detail: email}, nil
}

// resetSuperAdminPassword re-syncs the configured account's password and signs
// out its existing sessions, on the assumption a deliberate reset means the old
// credential should stop working everywhere.
func (s *Seeder) resetSuperAdminPassword(ctx context.Context, email string) (Result, error) {
	const name = "super_admin"

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if repository.IsNotFound(err) {
			// A super admin exists, but under a different address. Refuse
			// rather than guessing which account was meant.
			return Result{}, fmt.Errorf(
				"SUPER_ADMIN_RESET_PASSWORD is set but no account exists for %q; "+
					"a super admin under another address will not be touched", email)
		}
		return Result{}, fmt.Errorf("look up user: %w", err)
	}

	if user.Role != models.RoleSuperAdmin {
		return Result{}, fmt.Errorf(
			"%q exists but is a %s, not a super admin; refusing to reset its password",
			email, user.Role)
	}

	// Nothing to do when the stored hash already matches — this keeps the
	// common case from writing a new hash and revoking sessions on every sync.
	if s.hasher.Verify(user.PasswordHash, s.cfg.SuperAdmin.Password) {
		return Result{
			Name:   name,
			Action: ActionSkipped,
			Detail: "password already matches the environment",
		}, nil
	}

	hash, err := s.hasher.Hash(s.cfg.SuperAdmin.Password)
	if err != nil {
		return Result{}, fmt.Errorf("hash password: %w", err)
	}
	if err := s.users.UpdatePassword(ctx, user.ID, hash); err != nil {
		return Result{}, fmt.Errorf("update password: %w", err)
	}
	if err := s.tokens.RevokeAllForUser(ctx, user.ID); err != nil {
		return Result{}, fmt.Errorf("revoke sessions: %w", err)
	}

	return Result{
		Name:   name,
		Action: ActionUpdated,
		Detail: fmt.Sprintf("%s — password reset, existing sessions revoked", email),
	}, nil
}
