package services

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/learna/learna-api/internal/dto/request"
	"github.com/learna/learna-api/internal/dto/response"
	"github.com/learna/learna-api/internal/models"
	"github.com/learna/learna-api/internal/repository"
	"github.com/learna/learna-api/internal/utils"
)

// UserService implements admin user management — features U1..U6.
//
// Two rules run through everything here, and both exist to stop an admin
// escalating their own privileges:
//
//   - Only a super admin may set or change a role.
//   - Nobody may modify or delete their own account through these endpoints;
//     that is what /me is for.
type UserService struct {
	users  *repository.UserRepository
	tokens *repository.TokenRepository
	hasher *utils.Hasher
}

func NewUserService(d Deps) *UserService {
	return &UserService{
		users:  d.Repos.Users,
		tokens: d.Repos.Tokens,
		hasher: d.Hasher,
	}
}

// List returns a page of users — feature U1.
func (s *UserService) List(
	ctx context.Context,
	req request.ListUsers,
	page utils.Pagination,
) (*utils.Page[response.User], error) {
	filter := repository.UserFilter{
		Search: strings.TrimSpace(req.Search),
		Active: req.Active,
	}
	if req.Role != "" {
		filter.Role = models.Role(req.Role)
	}

	users, total, err := s.users.List(ctx, filter, page)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	result := utils.NewPage(response.NewUsers(users), total, page)
	return &result, nil
}

// Get returns one user — feature U3.
func (s *UserService) Get(ctx context.Context, id uuid.UUID) (*response.User, error) {
	user, err := s.users.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, utils.ErrNotFound("User not found.")
		}
		return nil, utils.ErrInternal(err)
	}
	out := response.NewUser(user)
	return &out, nil
}

// Create adds a user with an assigned role — feature U2.
//
// The role is validated against the caller's own: an admin may create learners
// only, so that an admin cannot mint a peer or a super admin.
func (s *UserService) Create(
	ctx context.Context,
	actorRole models.Role,
	req request.CreateUser,
) (*response.User, error) {
	role := models.Role(req.Role)
	if !role.Valid() {
		return nil, utils.ErrValidation("Unknown role %q.", req.Role)
	}
	if err := assertMayAssignRole(actorRole, role); err != nil {
		return nil, err
	}

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
		Role:         role,
		IsActive:     true,
	})
	if err != nil {
		if repository.IsDuplicate(err) {
			return nil, utils.ErrConflict("An account with this email already exists.")
		}
		return nil, utils.ErrInternal(err)
	}

	out := response.NewUser(user)
	return &out, nil
}

// Update changes name, role or active status — features U4 and U5.
//
// Deactivating revokes the user's refresh tokens: leaving them valid would let
// a deactivated account keep refreshing its way back in for a week.
func (s *UserService) Update(
	ctx context.Context,
	actor uuid.UUID,
	actorRole models.Role,
	id uuid.UUID,
	req request.UpdateUser,
) (*response.User, error) {
	if req.Name == nil && req.Role == nil && req.IsActive == nil {
		return nil, utils.ErrValidation("No fields to update.")
	}
	if actor == id {
		return nil, utils.ErrForbidden(
			"You cannot change your own role or status here. Use /api/v1/me instead.")
	}

	target, err := s.users.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, utils.ErrNotFound("User not found.")
		}
		return nil, utils.ErrInternal(err)
	}

	// An admin must not be able to edit a super admin.
	if err := assertMayManage(actorRole, target.Role); err != nil {
		return nil, err
	}

	update := repository.UserUpdate{IsActive: req.IsActive}

	if req.Name != nil {
		trimmed := strings.TrimSpace(*req.Name)
		update.Name = &trimmed
	}

	if req.Role != nil {
		role := models.Role(*req.Role)
		if !role.Valid() {
			return nil, utils.ErrValidation("Unknown role %q.", *req.Role)
		}
		if err := assertMayAssignRole(actorRole, role); err != nil {
			return nil, err
		}
		// Demoting the last super admin would lock everyone out of the portal.
		if target.Role == models.RoleSuperAdmin && role != models.RoleSuperAdmin {
			if err := s.assertNotLastSuperAdmin(ctx); err != nil {
				return nil, err
			}
		}
		update.Role = &role
	}

	// Same reasoning: deactivating the last super admin is a lockout.
	if req.IsActive != nil && !*req.IsActive && target.Role == models.RoleSuperAdmin {
		if err := s.assertNotLastSuperAdmin(ctx); err != nil {
			return nil, err
		}
	}

	user, err := s.users.Update(ctx, id, update)
	if err != nil {
		if repository.IsNotFound(err) {
			return nil, utils.ErrNotFound("User not found.")
		}
		return nil, utils.ErrInternal(err)
	}

	// A deactivated or demoted user should not keep a usable session.
	if (req.IsActive != nil && !*req.IsActive) || req.Role != nil {
		if err := s.tokens.RevokeAllForUser(ctx, id); err != nil {
			return nil, utils.ErrInternal(err)
		}
	}

	out := response.NewUser(user)
	return &out, nil
}

// Delete removes a user and everything cascading from them — feature U6.
func (s *UserService) Delete(
	ctx context.Context,
	actor uuid.UUID,
	actorRole models.Role,
	id uuid.UUID,
) error {
	if actor == id {
		return utils.ErrForbidden("You cannot delete your own account.")
	}

	target, err := s.users.GetByID(ctx, id)
	if err != nil {
		if repository.IsNotFound(err) {
			return utils.ErrNotFound("User not found.")
		}
		return utils.ErrInternal(err)
	}

	if err := assertMayManage(actorRole, target.Role); err != nil {
		return err
	}
	if target.Role == models.RoleSuperAdmin {
		if err := s.assertNotLastSuperAdmin(ctx); err != nil {
			return err
		}
	}

	if err := s.users.Delete(ctx, id); err != nil {
		if repository.IsNotFound(err) {
			return utils.ErrNotFound("User not found.")
		}
		return utils.ErrInternal(err)
	}
	return nil
}

// assertNotLastSuperAdmin refuses an operation that would leave the portal with
// no super admin, which nothing else could undo.
func (s *UserService) assertNotLastSuperAdmin(ctx context.Context) error {
	count, err := s.users.CountByRole(ctx, models.RoleSuperAdmin)
	if err != nil {
		return utils.ErrInternal(err)
	}
	if count <= 1 {
		return utils.ErrConflict(
			"This is the only super admin. Promote another user first.")
	}
	return nil
}

// assertMayAssignRole enforces that only super admins hand out privileged
// roles. An admin creating an admin would be self-escalation by proxy.
func assertMayAssignRole(actor, target models.Role) error {
	if actor == models.RoleSuperAdmin {
		return nil
	}
	if target == models.RoleLearner {
		return nil
	}
	return utils.ErrForbidden("Only a super admin can assign the %s role.", target)
}

// assertMayManage stops an admin from acting on a super admin's account.
func assertMayManage(actor, target models.Role) error {
	if target == models.RoleSuperAdmin && actor != models.RoleSuperAdmin {
		return utils.ErrForbidden("Only a super admin can manage another super admin.")
	}
	return nil
}
