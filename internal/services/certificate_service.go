package services

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"math"
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

// buildPDF lays out a modern, luxury landscape A4 certificate with golden accents.
func (s *CertificateService) buildPDF(row *repository.CertificateRow) ([]byte, error) {
	const (
		pageW = 297.0 // A4 landscape, millimetres
		pageH = 210.0
	)

	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetTitle("Certificate "+row.Certificate.CertNumber, true)
	pdf.SetAuthor(s.cfg.SiteName, true)
	pdf.AddPage()

	// Gold palette
	goldPrimary := []int{197, 145, 34}   // #C59122 Rich Gold
	goldLight := []int{232, 194, 98}     // #E8C262 Champagne Gold
	goldDark := []int{142, 98, 14}       // #8E620E Bronze Gold
	goldBg := []int{254, 252, 246}       // Warm subtle ivory tint
	darkNavy := []int{15, 23, 42}        // #0F172A Deep Slate Navy
	charcoal := []int{51, 65, 85}        // #334155 Slate Grey
	mutedGrey := []int{100, 116, 139}    // #64748B Muted Slate

	// Background subtle warm tint
	pdf.SetFillColor(goldBg[0], goldBg[1], goldBg[2])
	pdf.Rect(0, 0, pageW, pageH, "F")

	// Outer primary gold border
	pdf.SetDrawColor(goldPrimary[0], goldPrimary[1], goldPrimary[2])
	pdf.SetLineWidth(1.8)
	pdf.Rect(9, 9, pageW-18, pageH-18, "D")

	// Middle thin bronze border
	pdf.SetDrawColor(goldDark[0], goldDark[1], goldDark[2])
	pdf.SetLineWidth(0.4)
	pdf.Rect(11.5, 11.5, pageW-23, pageH-23, "D")

	// Inner delicate champagne border
	pdf.SetDrawColor(goldLight[0], goldLight[1], goldLight[2])
	pdf.SetLineWidth(0.8)
	pdf.Rect(14, 14, pageW-28, pageH-28, "D")

	// Helper for drawing decorative corner brackets / ornaments
	drawCorner := func(x, y float64, flipX, flipY bool) {
		pdf.SetDrawColor(goldPrimary[0], goldPrimary[1], goldPrimary[2])
		pdf.SetLineWidth(0.9)
		dx := 1.0
		if flipX {
			dx = -1.0
		}
		dy := 1.0
		if flipY {
			dy = -1.0
		}

		// Corner L-line
		pdf.Line(x, y+dy*10, x, y)
		pdf.Line(x, y, x+dx*10, y)

		// Corner diagonal accent
		pdf.Line(x+dx*2, y+dy*8, x+dx*8, y+dy*2)

		// Small decorative gold diamond in the corner
		pdf.SetFillColor(goldDark[0], goldDark[1], goldDark[2])
		pdf.Polygon([]fpdf.PointType{
			{X: x + dx*3.5, Y: y + dy*1.5},
			{X: x + dx*5.5, Y: y + dy*3.5},
			{X: x + dx*3.5, Y: y + dy*5.5},
			{X: x + dx*1.5, Y: y + dy*3.5},
		}, "F")
	}

	drawCorner(17, 17, false, false)              // Top-Left
	drawCorner(pageW-17, 17, true, false)         // Top-Right
	drawCorner(17, pageH-17, false, true)         // Bottom-Left
	drawCorner(pageW-17, pageH-17, true, true)    // Bottom-Right

	center := func(y float64, h float64, text string) {
		pdf.SetY(y)
		pdf.CellFormat(pageW, h, text, "", 1, "C", false, 0, "")
	}

	// 1. Institution / Brand Header
	brand := s.cfg.SiteName
	if brand == "" {
		brand = "LEARNA ACADEMY"
	}
	pdf.SetFont("Helvetica", "B", 13)
	pdf.SetTextColor(goldDark[0], goldDark[1], goldDark[2])
	center(24, 6, brand)

	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(mutedGrey[0], mutedGrey[1], mutedGrey[2])
	center(30, 4, "OFFICIAL VERIFIED CERTIFICATION")

	// Subtle gold divider beneath brand
	pdf.SetDrawColor(goldLight[0], goldLight[1], goldLight[2])
	pdf.SetLineWidth(0.4)
	pdf.Line(pageW/2-40, 36, pageW/2+40, 36)
	pdf.SetFillColor(goldPrimary[0], goldPrimary[1], goldPrimary[2])
	pdf.Polygon([]fpdf.PointType{
		{X: pageW / 2, Y: 35},
		{X: pageW/2 + 1.8, Y: 36},
		{X: pageW / 2, Y: 37},
		{X: pageW/2 - 1.8, Y: 36},
	}, "F")

	// 2. Main Title
	pdf.SetFont("Helvetica", "B", 24)
	pdf.SetTextColor(darkNavy[0], darkNavy[1], darkNavy[2])
	center(42, 10, "CERTIFICATE OF ACHIEVEMENT")

	// 3. Presentation line
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(mutedGrey[0], mutedGrey[1], mutedGrey[2])
	center(56, 6, "THIS IS PROUDLY PRESENTED TO")

	// 4. Recipient Name
	pdf.SetFont("Helvetica", "B", 28)
	pdf.SetTextColor(darkNavy[0], darkNavy[1], darkNavy[2])
	center(65, 14, row.UserName)

	// Gold underline for recipient name
	nameW := pdf.GetStringWidth(row.UserName)
	if nameW < 60 {
		nameW = 60
	}
	pdf.SetDrawColor(goldPrimary[0], goldPrimary[1], goldPrimary[2])
	pdf.SetLineWidth(0.8)
	pdf.Line(pageW/2-nameW/2-10, 81, pageW/2+nameW/2+10, 81)

	// Center diamond accent on the line
	pdf.SetFillColor(goldDark[0], goldDark[1], goldDark[2])
	pdf.Polygon([]fpdf.PointType{
		{X: pageW / 2, Y: 79.8},
		{X: pageW/2 + 2, Y: 81},
		{X: pageW / 2, Y: 82.2},
		{X: pageW/2 - 2, Y: 81},
	}, "F")

	// 5. Completion Narrative
	pdf.SetFont("Helvetica", "", 10.5)
	pdf.SetTextColor(charcoal[0], charcoal[1], charcoal[2])
	center(87, 6, "for successfully mastering and completing all curriculum requirements for")

	// 6. Course Title
	pdf.SetFont("Helvetica", "B", 17)
	pdf.SetTextColor(darkNavy[0], darkNavy[1], darkNavy[2])
	pdf.SetY(96)
	pdf.SetX(35)
	pdf.MultiCell(pageW-70, 7.5, row.CourseTitle, "", "C", false)

	// 7. Authentic Luxury Gold Official Seal (Center bottom)
	sealX := pageW / 2
	sealY := 142.0

	// A. Official Medallion Ribbon Tails draped beneath seal
	pdf.SetDrawColor(goldDark[0], goldDark[1], goldDark[2])
	pdf.SetLineWidth(0.5)

	// Left Ribbon with Swallowtail Notch
	pdf.SetFillColor(goldPrimary[0], goldPrimary[1], goldPrimary[2])
	pdf.Polygon([]fpdf.PointType{
		{X: sealX - 4, Y: sealY + 7},
		{X: sealX - 11, Y: sealY + 25},
		{X: sealX - 6, Y: sealY + 22},
		{X: sealX - 1, Y: sealY + 25},
		{X: sealX - 1, Y: sealY + 7},
	}, "FD")

	// Right Ribbon with Swallowtail Notch
	pdf.Polygon([]fpdf.PointType{
		{X: sealX + 1, Y: sealY + 7},
		{X: sealX + 1, Y: sealY + 25},
		{X: sealX + 6, Y: sealY + 22},
		{X: sealX + 11, Y: sealY + 25},
		{X: sealX + 4, Y: sealY + 7},
	}, "FD")

	// B. 36-Point Scalloped Starburst Rosette Edge
	numPoints := 36
	outerR := 16.5
	innerR := 14.8
	rosettePoints := make([]fpdf.PointType, 0, numPoints*2)
	for i := 0; i < numPoints*2; i++ {
		angle := (float64(i) * math.Pi / float64(numPoints)) - math.Pi/2
		r := outerR
		if i%2 != 0 {
			r = innerR
		}
		rosettePoints = append(rosettePoints, fpdf.PointType{
			X: sealX + r*math.Cos(angle),
			Y: sealY + r*math.Sin(angle),
		})
	}
	pdf.SetFillColor(goldLight[0], goldLight[1], goldLight[2])
	pdf.SetDrawColor(goldDark[0], goldDark[1], goldDark[2])
	pdf.SetLineWidth(0.6)
	pdf.Polygon(rosettePoints, "FD")

	// C. Concentric Layered Rings
	pdf.SetDrawColor(goldDark[0], goldDark[1], goldDark[2])
	pdf.SetLineWidth(0.6)
	pdf.Circle(sealX, sealY, 14.0, "D")

	pdf.SetDrawColor(goldPrimary[0], goldPrimary[1], goldPrimary[2])
	pdf.SetLineWidth(1.0)
	pdf.Circle(sealX, sealY, 13.2, "D")

	// Beaded Dots Ring (24 gold beads)
	pdf.SetFillColor(goldDark[0], goldDark[1], goldDark[2])
	numDots := 24
	for i := 0; i < numDots; i++ {
		angle := float64(i) * 2 * math.Pi / float64(numDots)
		bx := sealX + 11.8*math.Cos(angle)
		by := sealY + 11.8*math.Sin(angle)
		pdf.Circle(bx, by, 0.3, "F")
	}

	// Inner Parchment Disc
	pdf.SetFillColor(goldBg[0], goldBg[1], goldBg[2])
	pdf.SetDrawColor(goldDark[0], goldDark[1], goldDark[2])
	pdf.SetLineWidth(0.6)
	pdf.Circle(sealX, sealY, 10.5, "FD")

	// D. Star and Emblem inside the Seal
	pdf.SetFont("Helvetica", "B", 7.5)
	pdf.SetTextColor(goldDark[0], goldDark[1], goldDark[2])
	pdf.SetY(sealY - 6.0)
	pdf.SetX(sealX - 12)
	pdf.CellFormat(24, 3.5, "*  *  *", "", 1, "C", false, 0, "")

	pdf.SetFont("Helvetica", "B", 5.8)
	pdf.SetTextColor(darkNavy[0], darkNavy[1], darkNavy[2])
	pdf.SetY(sealY - 2.0)
	pdf.SetX(sealX - 12)
	pdf.CellFormat(24, 3.2, "OFFICIAL SEAL", "", 1, "C", false, 0, "")

	pdf.SetDrawColor(goldPrimary[0], goldPrimary[1], goldPrimary[2])
	pdf.SetLineWidth(0.35)
	pdf.Line(sealX-5.5, sealY+1.8, sealX+5.5, sealY+1.8)

	pdf.SetFont("Helvetica", "B", 5.0)
	pdf.SetTextColor(goldDark[0], goldDark[1], goldDark[2])
	pdf.SetY(sealY + 2.5)
	pdf.SetX(sealX - 12)
	pdf.CellFormat(24, 2.5, "VERIFIED", "", 1, "C", false, 0, "")

	// 8. Left Column: Issued Date
	pdf.SetDrawColor(mutedGrey[0], mutedGrey[1], mutedGrey[2])
	pdf.SetLineWidth(0.4)
	pdf.Line(42, 147, 98, 147)

	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(darkNavy[0], darkNavy[1], darkNavy[2])
	pdf.SetY(149)
	pdf.SetX(42)
	pdf.CellFormat(56, 4.5, row.Certificate.IssuedAt.Format("January 2, 2006"), "", 1, "C", false, 0, "")

	pdf.SetFont("Helvetica", "", 7.5)
	pdf.SetTextColor(mutedGrey[0], mutedGrey[1], mutedGrey[2])
	pdf.SetY(154)
	pdf.SetX(42)
	pdf.CellFormat(56, 4, "DATE OF ISSUANCE", "", 1, "C", false, 0, "")

	// 9. Right Column: Verification & Certificate ID
	pdf.SetDrawColor(mutedGrey[0], mutedGrey[1], mutedGrey[2])
	pdf.SetLineWidth(0.4)
	pdf.Line(pageW-98, 147, pageW-42, 147)

	pdf.SetFont("Courier", "B", 9)
	pdf.SetTextColor(goldDark[0], goldDark[1], goldDark[2])
	pdf.SetY(149)
	pdf.SetX(pageW - 98)
	pdf.CellFormat(56, 4.5, row.Certificate.CertNumber, "", 1, "C", false, 0, "")

	pdf.SetFont("Helvetica", "", 7.5)
	pdf.SetTextColor(mutedGrey[0], mutedGrey[1], mutedGrey[2])
	pdf.SetY(154)
	pdf.SetX(pageW - 98)
	pdf.CellFormat(56, 4, "CERTIFICATE ID", "", 1, "C", false, 0, "")

	// 10. Footer Security & Verification Link
	pdf.SetFont("Helvetica", "", 7)
	pdf.SetTextColor(mutedGrey[0], mutedGrey[1], mutedGrey[2])
	center(175, 4, "This credential is tamper-evident and recorded in the learner repository.")
	pdf.SetTextColor(goldDark[0], goldDark[1], goldDark[2])
	center(179, 4, "Verify authenticity: /verify/"+row.Certificate.CertNumber)

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
