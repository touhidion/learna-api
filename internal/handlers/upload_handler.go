package handlers

import (
	"github.com/gin-gonic/gin"

	"github.com/learna/learna-api/internal/services"
	"github.com/learna/learna-api/internal/utils"
	"github.com/learna/learna-api/pkg/cloudinary"
)

// UploadHandler serves the generic admin upload endpoints — features CL1, CL2.
type UploadHandler struct {
	upload *services.UploadService
}

// UploadImage stores a thumbnail or avatar and returns its CDN URL.
//
// POST /api/v1/admin/upload/image  (multipart/form-data: file, folder?)
func (h *UploadHandler) UploadImage(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		utils.Fail(c, utils.ErrBadRequest("A multipart field named \"file\" is required.").WithCause(err))
		return
	}

	// Callers may target the avatars folder; anything else falls back to
	// thumbnails rather than letting a client write to an arbitrary path.
	folder := cloudinary.FolderThumbnails
	if c.PostForm("folder") == cloudinary.FolderAvatars {
		folder = cloudinary.FolderAvatars
	}

	result, err := h.upload.UploadImage(c.Request.Context(), fh, folder)
	if err != nil {
		utils.Fail(c, err)
		return
	}
	utils.Created(c, result)
}
