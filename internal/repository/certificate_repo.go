package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/learna/learna-api/internal/database"
	"github.com/learna/learna-api/internal/models"
)

// CertificateRepository covers the certificates table — features CT1..CT5.
type CertificateRepository struct{ db *database.DB }

const certColumns = `id, user_id, course_id, cert_number, pdf_url, public_id, issued_at`

func scanCertificate(row pgx.Row) (*models.Certificate, error) {
	var c models.Certificate
	err := row.Scan(&c.ID, &c.UserID, &c.CourseID, &c.CertNumber,
		&c.PDFURL, &c.PublicID, &c.IssuedAt)
	if err != nil {
		return nil, translateError(err)
	}
	return &c, nil
}

// Create issues a certificate — feature CT1. UNIQUE(user_id, course_id) means
// a second attempt surfaces as ErrDuplicate rather than a duplicate row.
func (r *CertificateRepository) Create(ctx context.Context, c *models.Certificate) (*models.Certificate, error) {
	const q = `
		INSERT INTO certificates (user_id, course_id, cert_number)
		VALUES ($1, $2, $3)
		RETURNING ` + certColumns

	return scanCertificate(r.db.Pool.QueryRow(ctx, q, c.UserID, c.CourseID, c.CertNumber))
}

func (r *CertificateRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Certificate, error) {
	const q = `SELECT ` + certColumns + ` FROM certificates WHERE id = $1`
	return scanCertificate(r.db.Pool.QueryRow(ctx, q, id))
}

func (r *CertificateRepository) GetByUserAndCourse(ctx context.Context, userID, courseID uuid.UUID) (*models.Certificate, error) {
	const q = `SELECT ` + certColumns + ` FROM certificates WHERE user_id = $1 AND course_id = $2`
	return scanCertificate(r.db.Pool.QueryRow(ctx, q, userID, courseID))
}

// CertificateRow joins a certificate to the names shown on it and in listings.
type CertificateRow struct {
	Certificate models.Certificate
	CourseTitle string
	UserName    string
}

const certJoinQuery = `
	SELECT c.id, c.user_id, c.course_id, c.cert_number, c.pdf_url, c.public_id, c.issued_at,
		co.title, u.name
	FROM certificates c
	JOIN courses co ON co.id = c.course_id
	JOIN users u ON u.id = c.user_id`

func scanCertificateRow(row pgx.Row) (*CertificateRow, error) {
	var out CertificateRow
	c := &out.Certificate
	err := row.Scan(&c.ID, &c.UserID, &c.CourseID, &c.CertNumber,
		&c.PDFURL, &c.PublicID, &c.IssuedAt, &out.CourseTitle, &out.UserName)
	if err != nil {
		return nil, translateError(err)
	}
	return &out, nil
}

// GetByNumber backs the public verification endpoint — feature CT5.
func (r *CertificateRepository) GetByNumber(ctx context.Context, number string) (*CertificateRow, error) {
	return scanCertificateRow(r.db.Pool.QueryRow(ctx, certJoinQuery+` WHERE c.cert_number = $1`, number))
}

// GetRow returns one certificate with its joined names.
func (r *CertificateRepository) GetRow(ctx context.Context, id uuid.UUID) (*CertificateRow, error) {
	return scanCertificateRow(r.db.Pool.QueryRow(ctx, certJoinQuery+` WHERE c.id = $1`, id))
}

// ListByUser returns the caller's certificates — feature CT3.
func (r *CertificateRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]CertificateRow, error) {
	rows, err := r.db.Pool.Query(ctx, certJoinQuery+` WHERE c.user_id = $1 ORDER BY c.issued_at DESC`, userID)
	if err != nil {
		return nil, translateError(err)
	}
	defer rows.Close()

	out := []CertificateRow{}
	for rows.Next() {
		row, err := scanCertificateRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *row)
	}
	return out, translateError(rows.Err())
}

// NumberExists guards against a collision in the generated cert number.
func (r *CertificateRepository) NumberExists(ctx context.Context, number string) (bool, error) {
	var exists bool
	err := r.db.Pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM certificates WHERE cert_number = $1)`, number).Scan(&exists)
	if err != nil {
		return false, translateError(err)
	}
	return exists, nil
}
