package middleware

import (
	"github.com/gin-gonic/gin"

	"github.com/learna/learna-api/internal/models"
	"github.com/learna/learna-api/internal/utils"
)

// RequireRole admits only callers holding one of the listed roles. It must be
// chained after Auth.
func RequireRole(roles ...models.Role) gin.HandlerFunc {
	allowed := make(map[models.Role]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(c *gin.Context) {
		role, err := CurrentUserRole(c)
		if err != nil {
			utils.Fail(c, err)
			return
		}
		if _, ok := allowed[role]; !ok {
			utils.Fail(c, utils.ErrForbidden("Your role does not permit this action."))
			return
		}
		c.Next()
	}
}

// RequireAdmin admits admins and super admins — the guard for every
// /api/v1/admin route.
func RequireAdmin() gin.HandlerFunc {
	return RequireRole(models.RoleAdmin, models.RoleSuperAdmin)
}

// RequireSuperAdmin admits super admins only, for actions that manage other
// admins.
func RequireSuperAdmin() gin.HandlerFunc {
	return RequireRole(models.RoleSuperAdmin)
}
