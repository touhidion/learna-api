package repository

import "github.com/learna/learna-api/internal/database"

// The repositories below are declared so that wiring, route registration and
// the service layer all compile against their final shape. Their queries land
// with the module each one belongs to.
//
// Every table they target already exists in migration 000001, and the pattern
// to follow is user_repo.go: a `<entity>Columns` constant, a `scan<Entity>`
// helper, and methods returning ErrNotFound / ErrDuplicate via translateError.

// ModuleRepository covers modules — features M1..M4. Reorder must update every
// row in one transaction so no client sees a half-applied ordering.
type ModuleRepository struct{ db *database.DB }

// LessonRepository covers lessons — features L1..L5.
type LessonRepository struct{ db *database.DB }

// AttachmentRepository covers attachments — features AT1..AT4. Deleting a row
// must also delete the Cloudinary asset named by its public_id.
type AttachmentRepository struct{ db *database.DB }

// EnrollmentRepository covers enrollments — features E1..E4.
type EnrollmentRepository struct{ db *database.DB }

// ProgressRepository covers lesson_progress and the derived course percentage
// — features PR1..PR4.
type ProgressRepository struct{ db *database.DB }

// CertificateRepository covers certificates — features CT1..CT5, including
// lookup by cert_number for the public verification endpoint.
type CertificateRepository struct{ db *database.DB }
