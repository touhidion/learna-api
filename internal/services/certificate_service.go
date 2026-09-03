package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/google/uuid"

	"github.com/learna/learna-api/internal/config"
	"github.com/learna/learna-api/internal/dto/response"
	"github.com/learna/learna-api/internal/models"
	"github.com/learna/learna-api/internal/repository"
	"github.com/learna/learna-api/internal/utils"
)

// CertificateService issues and verifies certificates — features CT1..CT5.
//
// The PDF is rendered on demand rather than stored: file upload is out of
// scope, and a certificate is fully reproducible from its number, holder and
// course. That also means a re-render always reflects the current data.
type CertificateService struct {
	cfg          config.CertConfig
	certificates *repository.CertificateRepository
	enrollments  *repository.EnrollmentRepository
	progress     *repository.ProgressRepository
	courses      *repository.CourseRepository
	users        *repository.UserRepository
}

func NewCertificateService(d Deps) *CertificateService {
	return &CertificateService{
		cfg:          d.Config.Cert,
		certificates: d.Repos.Certificates,
		enrollments:  d.Repos.Enrollments,
		progress:     d.Repos.Progress,
		courses:      d.Repos.Courses,
		users:        d.Repos.Users,
	}
}

// Generate issues a certificate once a course is fully complete — feature CT1.
//
// Idempotent: asking twice returns the existing certificate rather than
// issuing a second number for the same achievement.
func (s *CertificateService) Generate(
	ctx context.Context,
	userID, courseID uuid.UUID,
) (*response.Certificate, error) {
	if existing, err := s.certificates.GetByUserAndCourse(ctx, userID, courseID); err == nil {
		return s.toResponse(ctx, existing)
	} else if !repository.IsNotFound(err) {
		return nil, utils.ErrInternal(err)
	}

	if _, err := s.enrollments.Get(ctx, userID, courseID); err != nil {
		if repository.IsNotFound(err) {
			return nil, utils.ErrForbidden("You are not enrolled in this course.")
		}
		return nil, utils.ErrInternal(err)
	}

	completed, total, err := s.progress.CourseProgress(ctx, userID, courseID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}
	if total == 0 || completed < total {
		return nil, utils.ErrUnprocessable(
			"Finish every lesson first — %d of %d complete.", completed, total)
	}

	number, err := s.uniqueNumber(ctx)
	if err != nil {
		return nil, err
	}

	cert, err := s.certificates.Create(ctx, &models.Certificate{
		UserID:     userID,
		CourseID:   courseID,
		CertNumber: number,
	})
	if err != nil {
		// Lost a race with a concurrent request; return the winner's row.
		if repository.IsDuplicate(err) {
			existing, getErr := s.certificates.GetByUserAndCourse(ctx, userID, courseID)
			if getErr == nil {
				return s.toResponse(ctx, existing)
			}
		}
		return nil, utils.ErrInternal(err)
	}

	return s.toResponse(ctx, cert)
}

// ListMine returns the caller's certificates — feature CT3.
func (s *CertificateService) ListMine(ctx context.Context, userID uuid.UUID) ([]response.Certificate, error) {
	rows, err := s.certificates.ListByUser(ctx, userID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	out := make([]response.Certificate, 0, len(rows))
	for i := range rows {
		out = append(out, certificateFromRow(&rows[i]))
	}
	return out, nil
}

// Verify looks a certificate up by number, without auth — feature CT5.
func (s *CertificateService) Verify(ctx context.Context, number string) (*response.Certificate, error) {
	row, err := s.certificates.GetByNumber(ctx, number)
	if err != nil {
		return nil, notFoundOr(err, "No certificate found with that number.")
	}
	out := certificateFromRow(row)
	return &out, nil
}

// RenderPDF produces the certificate document — features CT2, CT4.
//
// Ownership is checked by the caller for the download route; verification by
// number is public and renders the same document.
func (s *CertificateService) RenderPDF(ctx context.Context, certID uuid.UUID) ([]byte, string, error) {
	row, err := s.certificates.GetRow(ctx, certID)
	if err != nil {
		return nil, "", notFoundOr(err, "Certificate not found.")
	}

	pdf, err := s.buildPDF(row)
	if err != nil {
		return nil, "", utils.ErrInternal(err)
	}
	return pdf, row.Certificate.CertNumber + ".pdf", nil
}

// OwnerOf reports which user a certificate belongs to, so the download route
// can refuse someone else's.
func (s *CertificateService) OwnerOf(ctx context.Context, certID uuid.UUID) (uuid.UUID, error) {
	cert, err := s.certificates.GetByID(ctx, certID)
	if err != nil {
		return uuid.Nil, notFoundOr(err, "Certificate not found.")
	}
	return cert.UserID, nil
}

// buildPDF lays out a landscape A4 certificate.
func (s *CertificateService) buildPDF(row *repository.CertificateRow) ([]byte, error) {
	const (
		pageW = 297.0 // A4 landscape, millimetres
		pageH = 210.0
	)

	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetTitle("Certificate "+row.Certificate.CertNumber, true)
	pdf.SetAuthor(s.cfg.SiteName, true)
	pdf.AddPage()

	// Double border.
	pdf.SetDrawColor(79, 70, 229) // the brand indigo
	pdf.SetLineWidth(1.5)
	pdf.Rect(10, 10, pageW-20, pageH-20, "D")
	pdf.SetLineWidth(0.4)
	pdf.Rect(14, 14, pageW-28, pageH-28, "D")

	center := func(y float64, h float64, text string) {
		pdf.SetY(y)
		pdf.CellFormat(pageW, h, text, "", 1, "C", false, 0, "")
	}

	pdf.SetTextColor(79, 70, 229)
	pdf.SetFont("Helvetica", "B", 15)
	center(30, 8, s.cfg.SiteName)

	pdf.SetTextColor(40, 40, 40)
	pdf.SetFont("Helvetica", "", 13)
	center(48, 8, "CERTIFICATE OF COMPLETION")

	pdf.SetFont("Helvetica", "", 11)
	pdf.SetTextColor(110, 110, 110)
	center(66, 7, "This is to certify that")

	pdf.SetFont("Helvetica", "B", 26)
	pdf.SetTextColor(20, 20, 20)
	center(78, 14, row.UserName)

	pdf.SetFont("Helvetica", "", 11)
	pdf.SetTextColor(110, 110, 110)
	center(98, 7, "has successfully completed the course")

	// A long title would overflow a single centred cell, so it wraps.
	pdf.SetFont("Helvetica", "B", 17)
	pdf.SetTextColor(20, 20, 20)
	pdf.SetY(110)
	pdf.SetX(30)
	pdf.MultiCell(pageW-60, 9, row.CourseTitle, "", "C", false)

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(110, 110, 110)
	center(142, 6, "Issued on "+row.Certificate.IssuedAt.Format("2 January 2006"))

	// The number is the verification handle, so it is given prominence.
	pdf.SetFont("Courier", "B", 12)
	pdf.SetTextColor(79, 70, 229)
	center(154, 7, row.Certificate.CertNumber)

	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(150, 150, 150)
	center(168, 5, "Verify this certificate at /verify/"+row.Certificate.CertNumber)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("render certificate pdf: %w", err)
	}
	return buf.Bytes(), nil
}

// uniqueNumber generates LEARNA-YYYY-XXXXXX with a crypto-random suffix.
//
// Random rather than sequential: a sequential number would leak how many
// certificates the portal has issued, and let anyone enumerate them through
// the public verification endpoint.
func (s *CertificateService) uniqueNumber(ctx context.Context) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no I/O/0/1
	const attempts = 10

	prefix := s.cfg.NumberPrefix
	if prefix == "" {
		prefix = "LEARNA"
	}

	for range attempts {
		suffix := make([]byte, 6)
		for i := range suffix {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
			if err != nil {
				return "", utils.ErrInternal(err)
			}
			suffix[i] = alphabet[n.Int64()]
		}

		number := fmt.Sprintf("%s-%d-%s", prefix, time.Now().Year(), suffix)

		taken, err := s.certificates.NumberExists(ctx, number)
		if err != nil {
			return "", utils.ErrInternal(err)
		}
		if !taken {
			return number, nil
		}
	}
	return "", utils.ErrInternal(fmt.Errorf("could not generate a unique certificate number"))
}

func (s *CertificateService) toResponse(ctx context.Context, cert *models.Certificate) (*response.Certificate, error) {
	row, err := s.certificates.GetRow(ctx, cert.ID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}
	out := certificateFromRow(row)
	return &out, nil
}

func certificateFromRow(row *repository.CertificateRow) response.Certificate {
	c := &row.Certificate
	return response.Certificate{
		ID:          c.ID,
		CertNumber:  c.CertNumber,
		CourseID:    c.CourseID,
		CourseTitle: row.CourseTitle,
		UserName:    row.UserName,
		PDFURL:      c.PDFURL,
		IssuedAt:    c.IssuedAt,
	}
}
