package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/learna/learna-api/internal/dto/request"
	"github.com/learna/learna-api/internal/dto/response"
	"github.com/learna/learna-api/internal/middleware"
	"github.com/learna/learna-api/internal/services"
	"github.com/learna/learna-api/internal/utils"
)

// CourseHandler serves both the admin course routes (C1..C6) and the public
// catalog (PC1..PC2). They share a service; the split is which filter is
// applied, and that decision stays in the route table rather than in a flag.
type CourseHandler struct {
	courses *services.CourseService
}

// ListPublic returns the public catalog — feature PC1.
//
// GET /api/v1/courses?search=&category=&page=&page_size=
func (h *CourseHandler) ListPublic(c *gin.Context) {
	var req request.ListCourses
	if err := utils.BindQuery(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	page, err := h.courses.ListPublished(c.Request.Context(), req, utils.ParsePagination(c))
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, page)
}

// GetPublic returns one published course by slug — feature PC2.
//
// GET /api/v1/courses/:slug
func (h *CourseHandler) GetPublic(c *gin.Context) {
	course, err := h.courses.GetBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, course)
}

// PublicCategories lists categories that have at least one published course.
//
// GET /api/v1/courses/categories
func (h *CourseHandler) PublicCategories(c *gin.Context) {
	categories, err := h.courses.Categories(c.Request.Context(), true)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, gin.H{"categories": categories})
}

// List returns all courses regardless of status — feature C2.
//
// GET /api/v1/admin/courses?search=&category=&status=
func (h *CourseHandler) List(c *gin.Context) {
	var req request.ListCourses
	if err := utils.BindQuery(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	page, err := h.courses.List(c.Request.Context(), req, utils.ParsePagination(c))
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, page)
}

// Get returns one course by id — admin view, any status.
//
// GET /api/v1/admin/courses/:id
func (h *CourseHandler) Get(c *gin.Context) {
	id, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	course, err := h.courses.Get(c.Request.Context(), id)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, course)
}

// Create adds a draft course — feature C1.
//
// POST /api/v1/admin/courses
func (h *CourseHandler) Create(c *gin.Context) {
	actor, err := middleware.CurrentUserID(c)
	if err != nil {
		utils.Fail(c, err)
		return
	}

	var req request.CreateCourse
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	course, err := h.courses.Create(c.Request.Context(), actor, req)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.Created(c, course)
}

// Update edits a course — feature C3.
//
// PATCH /api/v1/admin/courses/:id
func (h *CourseHandler) Update(c *gin.Context) {
	id, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	var req request.UpdateCourse
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	course, err := h.courses.Update(c.Request.Context(), id, req)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, course)
}

// SetStatus publishes, unpublishes or archives — feature C5.
//
// PATCH /api/v1/admin/courses/:id/status
func (h *CourseHandler) SetStatus(c *gin.Context) {
	id, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	var req request.UpdateCourseStatus
	if err := utils.BindJSON(c, &req); err != nil {
		utils.Fail(c, err)
		return
	}

	course, err := h.courses.SetStatus(c.Request.Context(), id, req)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, course)
}

// Delete removes a course and its whole tree — feature C4.
//
// DELETE /api/v1/admin/courses/:id
func (h *CourseHandler) Delete(c *gin.Context) {
	id, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	if err := h.courses.Delete(c.Request.Context(), id); err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, response.Message{Message: "Course deleted."})
}

// Categories lists every category in use — feature C6.
//
// GET /api/v1/admin/courses/categories
func (h *CourseHandler) Categories(c *gin.Context) {
	categories, err := h.courses.Categories(c.Request.Context(), false)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, gin.H{"categories": categories})
}
