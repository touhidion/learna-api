package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/learna/learna-api/internal/dto/response"
	"github.com/learna/learna-api/internal/middleware"
	"github.com/learna/learna-api/internal/services"
	"github.com/learna/learna-api/internal/utils"
)

// LearningHandler serves enrollment and progress — features E1..E4, PR1..PR4.
type LearningHandler struct {
	learning *services.LearningService
}

// Enroll adds the caller to a course — feature E1.
//
// POST /api/v1/enrollments/:courseId
func (h *LearningHandler) Enroll(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		utils.Fail(c, err)
		return
	}

	courseID, err := utils.ParseUUIDParam(c, "courseId")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	enrollment, err := h.learning.Enroll(c.Request.Context(), userID, courseID)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.Created(c, enrollment)
}

// Unenroll removes the caller and their progress — feature E2.
//
// DELETE /api/v1/enrollments/:courseId
func (h *LearningHandler) Unenroll(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		utils.Fail(c, err)
		return
	}

	courseID, err := utils.ParseUUIDParam(c, "courseId")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	if err := h.learning.Unenroll(c.Request.Context(), userID, courseID); err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, response.Message{Message: "Unenrolled."})
}

// MyEnrollments lists the caller's courses — feature E3.
//
// GET /api/v1/me/enrollments
func (h *LearningHandler) MyEnrollments(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		utils.Fail(c, err)
		return
	}

	page, err := h.learning.MyEnrollments(c.Request.Context(), userID, utils.ParsePagination(c))
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, page)
}

// MarkComplete records a finished lesson — feature PR1.
//
// POST /api/v1/lessons/:lessonId/complete
func (h *LearningHandler) MarkComplete(c *gin.Context) {
	h.setLessonState(c, true)
}

// Uncomplete reverses it — feature PR2.
//
// DELETE /api/v1/lessons/:lessonId/complete
func (h *LearningHandler) Uncomplete(c *gin.Context) {
	h.setLessonState(c, false)
}

func (h *LearningHandler) setLessonState(c *gin.Context, complete bool) {
	userID, role, err := currentActor(c)
	if err != nil {
		utils.Fail(c, err)
		return
	}

	lessonID, err := utils.ParseUUIDParam(c, "lessonId")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	var progress *response.Progress
	if complete {
		progress, err = h.learning.MarkComplete(c.Request.Context(), userID, role, lessonID)
	} else {
		progress, err = h.learning.Unmark(c.Request.Context(), userID, role, lessonID)
	}
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, progress)
}

// CourseProgress reports the caller's percentage — feature PR3.
//
// GET /api/v1/learn/courses/:courseId/progress
func (h *LearningHandler) CourseProgress(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		utils.Fail(c, err)
		return
	}

	courseID, err := utils.ParseUUIDParam(c, "courseId")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	progress, err := h.learning.CourseProgress(c.Request.Context(), userID, courseID)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, progress)
}

// CertificateHandler serves certificates — features CT1..CT5.
type CertificateHandler struct {
	certificates *services.CertificateService
}

// Generate issues a certificate on completion — feature CT1.
//
// POST /api/v1/certificates/courses/:courseId
func (h *CertificateHandler) Generate(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		utils.Fail(c, err)
		return
	}

	courseID, err := utils.ParseUUIDParam(c, "courseId")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	cert, err := h.certificates.Generate(c.Request.Context(), userID, courseID)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.Created(c, cert)
}

// ListMine returns the caller's certificates — feature CT3.
//
// GET /api/v1/me/certificates
func (h *CertificateHandler) ListMine(c *gin.Context) {
	userID, err := middleware.CurrentUserID(c)
	if err != nil {
		utils.Fail(c, err)
		return
	}

	certs, err := h.certificates.ListMine(c.Request.Context(), userID)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, gin.H{"certificates": certs})
}

// Download streams the certificate PDF — features CT2, CT4.
//
// GET /api/v1/certificates/download/:id
//
// The PDF is rendered per request rather than stored, so it always reflects
// the current holder and course names.
func (h *CertificateHandler) Download(c *gin.Context) {
	userID, role, err := currentActor(c)
	if err != nil {
		utils.Fail(c, err)
		return
	}

	id, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	owner, err := h.certificates.OwnerOf(c.Request.Context(), id)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	// A certificate is personal. Admins may fetch one for support purposes.
	if owner != userID && !role.IsAdmin() {
		utils.Fail(c, utils.ErrForbidden("This certificate belongs to someone else."))
		return
	}

	pdf, filename, err := h.certificates.RenderPDF(c.Request.Context(), id)
	if err != nil {
		utils.Fail(c, err)
		return
	}

	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(http.StatusOK, "application/pdf", pdf)
}

// Verify checks a certificate number without auth — feature CT5.
//
// GET /api/v1/certificates/verify/:certNumber
func (h *CertificateHandler) Verify(c *gin.Context) {
	cert, err := h.certificates.Verify(c.Request.Context(), c.Param("certNumber"))
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, cert)
}

// AnalyticsHandler serves the admin dashboards — features AN1, AN2, ACA1.
type AnalyticsHandler struct {
	analytics *services.AnalyticsService
}

// Overview returns portal-wide totals — feature AN1.
//
// GET /api/v1/admin/analytics/overview
func (h *AnalyticsHandler) Overview(c *gin.Context) {
	overview, err := h.analytics.Overview(c.Request.Context())
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.OK(c, overview)
}

// Course returns per-course engagement and its learner table — AN2, ACA1.
//
// GET /api/v1/admin/courses/:id/analytics
func (h *AnalyticsHandler) Course(c *gin.Context) {
	id, err := utils.ParseUUIDParam(c, "id")
	if err != nil {
		utils.Fail(c, err)
		return
	}

	stats, err := h.analytics.CourseAnalytics(c.Request.Context(), id)
	if err != nil {
		utils.Fail(c, err)
		return
	}

	learners, err := h.analytics.CourseLearners(c.Request.Context(), id)
	if err != nil {
		utils.Fail(c, err)
		return
	}

	utils.OK(c, gin.H{"stats": stats, "learners": learners})
}
