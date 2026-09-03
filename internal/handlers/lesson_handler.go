package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/learna/learna-api/internal/dto/request"
	"github.com/learna/learna-api/internal/dto/response"
	"github.com/learna/learna-api/internal/services"
	"github.com/learna/learna-api/internal/utils"
)

// LessonHandler serves the lesson routes — features L1..L5.
type LessonHandler struct {
	lessons *services.LessonService
}

// ListByModule returns a module's lessons.
//
// GET /api/v1/learn/modules/:moduleId/lessons
func (h *LessonHandler) ListByModule(c *gin.Context) {
	moduleID, err := utils.ParseUUIDParam(c, "moduleId")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	lessons, err := h.lessons.ListByModule(c.Request.Context(), moduleID)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, gin.H{"lessons": lessons})
}

// Get returns one lesson with its markdown body — feature L5.
//
// GET /api/v1/learn/lessons/:lessonId
//
// Enrollment is not enforced yet; that guard arrives with feature E4.
func (h *LessonHandler) Get(c *gin.Context) {
	id, err := utils.ParseUUIDParam(c, "lessonId")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	lesson, err := h.lessons.Get(c.Request.Context(), id)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, lesson)
}

// Create adds a lesson to a module — feature L1.
//
// POST /api/v1/admin/modules/:id/lessons
func (h *LessonHandler) Create(c *gin.Context) {
	moduleID, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	var req request.CreateLesson
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	lesson, err := h.lessons.Create(c.Request.Context(), moduleID, req)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.Created(c, lesson)
}

// Reorder rewrites the positions of a module's lessons — feature L4.
//
// PATCH /api/v1/admin/modules/:id/lessons/reorder
func (h *LessonHandler) Reorder(c *gin.Context) {
	moduleID, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	var req request.Reorder
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	lessons, err := h.lessons.Reorder(c.Request.Context(), moduleID, req)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, gin.H{"lessons": lessons})
}

// AdminList returns a module's lessons for the editor.
//
// GET /api/v1/admin/modules/:id/lessons
func (h *LessonHandler) AdminList(c *gin.Context) {
	moduleID, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	lessons, err := h.lessons.ListByModule(c.Request.Context(), moduleID)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, gin.H{"lessons": lessons})
}

// Update edits a lesson — feature L2.
//
// PATCH /api/v1/admin/lessons/:id
func (h *LessonHandler) Update(c *gin.Context) {
	id, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	var req request.UpdateLesson
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	lesson, err := h.lessons.Update(c.Request.Context(), id, req)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, lesson)
}

// Delete removes a lesson — feature L3.
//
// DELETE /api/v1/admin/lessons/:id
func (h *LessonHandler) Delete(c *gin.Context) {
	id, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	if err := h.lessons.Delete(c.Request.Context(), id); err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, response.Message{Message: "Lesson deleted."})
}

// AdminGet returns one lesson for the editor.
//
// GET /api/v1/admin/lessons/:id
func (h *LessonHandler) AdminGet(c *gin.Context) {
	id, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	lesson, err := h.lessons.Get(c.Request.Context(), id)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, lesson)
}
