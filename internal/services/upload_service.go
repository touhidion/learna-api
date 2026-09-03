package services

import (
	"context"
	"mime/multipart"
	"path"
	"strings"

	"github.com/learna/learna-api/internal/config"
	"github.com/learna/learna-api/internal/dto/response"
	"github.com/learna/learna-api/internal/utils"
	"github.com/learna/learna-api/pkg/cloudinary"
)

// allowedImageExt gates thumbnail and avatar uploads.
var allowedImageExt = map[string]struct{}{
	".png": {}, ".jpg": {}, ".jpeg": {}, ".webp": {}, ".gif": {},
}

// allowedAttachmentExt gates lesson attachments — feature AT4.
var allowedAttachmentExt = map[string]struct{}{
	".pdf": {}, ".doc": {}, ".docx": {}, ".ppt": {}, ".pptx": {},
	".xls": {}, ".xlsx": {}, ".txt": {}, ".csv": {}, ".zip": {},
	".png": {}, ".jpg": {}, ".jpeg": {}, ".webp": {}, ".gif": {},
}

// UploadService fronts Cloudinary with Learna's own validation rules.
type UploadService struct {
	cfg *config.CloudinaryConfig
	cld *cloudinary.Client
}

func NewUploadService(d Deps) *UploadService {
	return &UploadService{cfg: &d.Config.Cloudinary, cld: d.Cloudinary}
}

// Available reports whether uploads are configured, so handlers can answer 503
// rather than failing mid-request.
func (s *UploadService) Available() bool { return s.cld.Enabled() }

// UploadImage stores a thumbnail or avatar.
func (s *UploadService) UploadImage(ctx context.Context, fh *multipart.FileHeader, folder string) (*response.Upload, error) {
	return s.upload(ctx, fh, folder, "image", allowedImageExt)
}

// UploadAttachment stores a lesson attachment.
//
// The resource type is "raw" for anything that is not an image: Cloudinary
// would otherwise try to decode a PDF or ZIP as media and reject it.
func (s *UploadService) UploadAttachment(ctx context.Context, fh *multipart.FileHeader) (*response.Upload, error) {
	ext := strings.ToLower(path.Ext(fh.Filename))
	resourceType := "raw"
	if _, isImage := allowedImageExt[ext]; isImage {
		resourceType = "image"
	}
	return s.upload(ctx, fh, cloudinary.FolderAttachments, resourceType, allowedAttachmentExt)
}

// upload runs the shared validate-then-stream path.
func (s *UploadService) upload(
	ctx context.Context,
	fh *multipart.FileHeader,
	folder string,
	resourceType string,
	allowed map[string]struct{},
) (*response.Upload, error) {
	if !s.Available() {
		return nil, utils.ErrUnavailable("File uploads are not configured on this server.")
	}
	if fh == nil {
		return nil, utils.ErrBadRequest("A file is required.")
	}
	if fh.Size <= 0 {
		return nil, utils.ErrBadRequest("The uploaded file is empty.")
	}
	if fh.Size > s.cfg.MaxFileSize {
		return nil, utils.ErrPayloadTooLarge(
			"File is %d bytes; the limit is %d bytes.", fh.Size, s.cfg.MaxFileSize)
	}

	ext := strings.ToLower(path.Ext(fh.Filename))
	if _, ok := allowed[ext]; !ok {
		return nil, utils.ErrValidation("File type %q is not allowed.", ext)
	}

	file, err := fh.Open()
	if err != nil {
		return nil, utils.ErrBadRequest("Could not read the uploaded file.").WithCause(err)
	}
	defer func() { _ = file.Close() }()

	res, err := s.cld.Upload(ctx, file, cloudinary.UploadParams{
		Folder:       folder,
		FileName:     fh.Filename,
		ResourceType: resourceType,
	})
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	return &response.Upload{
		URL:      res.URL,
		PublicID: res.PublicID,
		FileName: fh.Filename,
		FileType: fh.Header.Get("Content-Type"),
		FileSize: fh.Size,
	}, nil
}

// Delete removes a previously uploaded asset.
func (s *UploadService) Delete(ctx context.Context, publicID, resourceType string) error {
	if !s.Available() {
		return utils.ErrUnavailable("File uploads are not configured on this server.")
	}
	if err := s.cld.Delete(ctx, publicID, resourceType); err != nil {
		return utils.ErrInternal(err)
	}
	return nil
}
