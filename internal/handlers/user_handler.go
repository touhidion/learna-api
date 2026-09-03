package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/learna/learna-api/internal/dto/request"
	"github.com/learna/learna-api/internal/dto/response"
	"github.com/learna/learna-api/internal/middleware"
	"github.com/learna/learna-api/internal/services"
	"github.com/learna/learna-api/internal/utils"
)

// UserHandler serves /api/v1/admin/users — features U1..U6.
type UserHandler struct {
	users *services.UserService
}

// List returns a page of users — feature U1.
//
// GET /api/v1/admin/users?search=&role=&active=&page=&page_size=
func (h *UserHandler) List(c *gin.Context) {
	var req request.ListUsers
	if err := utils.BindQuery(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	page, err := h.users.List(c.Request.Context(), req, utils.ParsePagination(c))
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, page)
}

// Get returns one user — feature U3.
//
// GET /api/v1/admin/users/:id
func (h *UserHandler) Get(c *gin.Context) {
	id, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	user, err := h.users.Get(c.Request.Context(), id)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, user)
}

// Create adds a user with an assigned role — feature U2.
//
// POST /api/v1/admin/users
func (h *UserHandler) Create(c *gin.Context) {
	role, err := middleware.CurrentUserRole(c)
	if err != nil {
		utils.Fail(c, err)
		return
	}

	var req request.CreateUser
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	user, err := h.users.Create(c.Request.Context(), role, req)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.Created(c, user)
}

// Update changes name, role or active status — features U4, U5.
//
// PATCH /api/v1/admin/users/:id
func (h *UserHandler) Update(c *gin.Context) {
	actor, role, err := currentActor(c)
	if err != nil {
		utils.Fail(c, err)
		return
	}

	id, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	var req request.UpdateUser
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	user, err := h.users.Update(c.Request.Context(), actor, role, id, req)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, user)
}

// Delete removes a user — feature U6.
//
// DELETE /api/v1/admin/users/:id
func (h *UserHandler) Delete(c *gin.Context) {
	actor, role, err := currentActor(c)
	if err != nil {
		utils.Fail(c, err)
		return
	}

	id, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	if err := h.users.Delete(c.Request.Context(), actor, role, id); err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, response.Message{Message: "User deleted."})
}
