package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/learna/learna-api/internal/dto/request"
	"github.com/learna/learna-api/internal/dto/response"
	"github.com/learna/learna-api/internal/services"
	"github.com/learna/learna-api/internal/utils"
)

// ModuleHandler serves the module routes — features M1..M4.
type ModuleHandler struct {
	modules *services.ModuleService
}

// List returns a course's modules with their lessons.
//
// GET /api/v1/admin/courses/:id/modules
func (h *ModuleHandler) List(c *gin.Context) {
	courseID, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	modules, err := h.modules.List(c.Request.Context(), courseID)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, gin.H{"modules": modules})
}

// Create adds a module to a course — feature M1.
//
// POST /api/v1/admin/courses/:id/modules
func (h *ModuleHandler) Create(c *gin.Context) {
	courseID, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	var req request.CreateModule
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	module, err := h.modules.Create(c.Request.Context(), courseID, req)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.Created(c, module)
}

// Reorder rewrites the positions of a course's modules — feature M4.
//
// PATCH /api/v1/admin/courses/:id/modules/reorder
func (h *ModuleHandler) Reorder(c *gin.Context) {
	courseID, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	var req request.Reorder
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	modules, err := h.modules.Reorder(c.Request.Context(), courseID, req)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, gin.H{"modules": modules})
}

// Update edits a module — feature M2.
//
// PATCH /api/v1/admin/modules/:id
func (h *ModuleHandler) Update(c *gin.Context) {
	id, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	var req request.UpdateModule
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	module, err := h.modules.Update(c.Request.Context(), id, req)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, module)
}

// Delete removes a module and its lessons — feature M3.
//
// DELETE /api/v1/admin/modules/:id
func (h *ModuleHandler) Delete(c *gin.Context) {
	id, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	if err := h.modules.Delete(c.Request.Context(), id); err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, response.Message{Message: "Module deleted."})
}
