// Package cloudinary wraps the Cloudinary Go SDK behind the narrow surface
// Learna actually uses: upload a file, delete a file.
//
// The wrapper exists so that the rest of the codebase never imports the SDK
// directly — swapping Cloudinary for S3 or local disk later means replacing
// this file, not touching every call site.
package cloudinary

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"

	// Aliased: this package is also called "cloudinary", and the bare name
	// would leave every call site ambiguous to a reader.
	sdk "github.com/cloudinary/cloudinary-go/v2"
	"github.com/cloudinary/cloudinary-go/v2/api/uploader"

	"github.com/learna/learna-api/internal/config"
)

// Folder names mirror the layout in the features doc (CL4).
const (
	FolderThumbnails  = "thumbnails"
	FolderAvatars     = "avatars"
	FolderAttachments = "attachments"
	FolderCertificates = "certificates"
)

// Client is a thin facade over the Cloudinary SDK. A nil *Client means uploads
// were not configured; every method reports ErrNotConfigured, so callers can
// degrade gracefully instead of panicking.
type Client struct {
	cld    *sdk.Cloudinary
	folder string
}

// ErrNotConfigured is returned by every method when CLOUDINARY_URL is unset.
var ErrNotConfigured = fmt.Errorf("cloudinary is not configured")

// New builds a client from config. It returns (nil, nil) when uploads are
// disabled, which is a valid state for local development.
func New(cfg config.CloudinaryConfig) (*Client, error) {
	if !cfg.Enabled() {
		return nil, nil
	}

	cld, err := sdk.NewFromURL(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("init cloudinary: %w", err)
	}
	// Cloudinary serves over HTTPS only in production-grade setups.
	cld.Config.URL.Secure = true

	return &Client{cld: cld, folder: strings.Trim(cfg.Folder, "/")}, nil
}

// Enabled reports whether uploads can actually be performed.
func (c *Client) Enabled() bool { return c != nil && c.cld != nil }

// UploadResult is what a successful upload yields.
type UploadResult struct {
	URL      string
	PublicID string
	Format   string
	Bytes    int64
}

// UploadParams describes one upload.
type UploadParams struct {
	// Folder is one of the Folder* constants; it is nested under the
	// configured root folder.
	Folder string
	// FileName seeds the public ID. Cloudinary appends a random suffix, so
	// two uploads of "notes.pdf" never collide.
	FileName string
	// ResourceType is "image", "video", "raw", or "auto" to let Cloudinary
	// decide. Non-media attachments must use "raw".
	ResourceType string
}

// Upload streams r to Cloudinary and returns the CDN URL to persist.
func (c *Client) Upload(ctx context.Context, r io.Reader, p UploadParams) (*UploadResult, error) {
	if !c.Enabled() {
		return nil, ErrNotConfigured
	}

	resourceType := p.ResourceType
	if resourceType == "" {
		resourceType = "auto"
	}

	// Strip the extension: Cloudinary appends the format itself, and leaving
	// it in produces public IDs such as "notes.pdf.pdf".
	base := strings.TrimSuffix(p.FileName, path.Ext(p.FileName))

	res, err := c.cld.Upload.Upload(ctx, r, uploader.UploadParams{
		Folder:         c.folderPath(p.Folder),
		PublicID:       sanitizePublicID(base),
		ResourceType:   resourceType,
		UniqueFilename: boolPtr(true),
		Overwrite:      boolPtr(false),
	})
	if err != nil {
		return nil, fmt.Errorf("upload to cloudinary: %w", err)
	}
	if res.Error.Message != "" {
		return nil, fmt.Errorf("cloudinary rejected the upload: %s", res.Error.Message)
	}

	return &UploadResult{
		URL:      res.SecureURL,
		PublicID: res.PublicID,
		Format:   res.Format,
		Bytes:    int64(res.Bytes),
	}, nil
}

// Delete removes an asset by its public ID.
//
// resourceType must match what the asset was uploaded as; Cloudinary keys its
// namespaces separately, and a mismatch silently deletes nothing.
func (c *Client) Delete(ctx context.Context, publicID, resourceType string) error {
	if !c.Enabled() {
		return ErrNotConfigured
	}
	if publicID == "" {
		return nil
	}
	if resourceType == "" {
		resourceType = "image"
	}

	res, err := c.cld.Upload.Destroy(ctx, uploader.DestroyParams{
		PublicID:     publicID,
		ResourceType: resourceType,
		Invalidate:   boolPtr(true),
	})
	if err != nil {
		return fmt.Errorf("delete from cloudinary: %w", err)
	}
	// "not found" is treated as success: the goal is that the asset is gone.
	if res.Result != "ok" && res.Result != "not found" {
		return fmt.Errorf("cloudinary delete returned %q", res.Result)
	}
	return nil
}

// folderPath nests a sub-folder under the configured root.
func (c *Client) folderPath(sub string) string {
	sub = strings.Trim(sub, "/")
	switch {
	case c.folder == "":
		return sub
	case sub == "":
		return c.folder
	default:
		return c.folder + "/" + sub
	}
}

// sanitizePublicID keeps public IDs to characters that survive a URL path.
func sanitizePublicID(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_':
			b.WriteRune(r)
		case r == ' ', r == '.':
			b.WriteRune('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "file"
	}
	if len(out) > 100 {
		out = strings.Trim(out[:100], "-")
	}
	return out
}

func boolPtr(b bool) *bool { return &b }
