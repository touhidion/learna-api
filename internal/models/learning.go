package models

import (
	"time"

	"github.com/google/uuid"
)

// Enrollment links a learner to a course. CompletedAt is stamped when course
// progress first reaches 100%.
type Enrollment struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	CourseID    uuid.UUID  `json:"course_id"`
	EnrolledAt  time.Time  `json:"enrolled_at"`
	CompletedAt *time.Time `json:"completed_at"`
}

// LessonProgress records that a learner finished one lesson. Unmarking a
// lesson deletes the row rather than flipping Completed, so the table stays a
// simple set of what is done.
type LessonProgress struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	LessonID    uuid.UUID `json:"lesson_id"`
	Completed   bool      `json:"completed"`
	CompletedAt time.Time `json:"completed_at"`
}

// CourseProgress is a computed view, not a table.
type CourseProgress struct {
	CourseID         uuid.UUID  `json:"course_id"`
	TotalLessons     int        `json:"total_lessons"`
	CompletedLessons int        `json:"completed_lessons"`
	Percentage       float64    `json:"percentage"`
	CompletedAt      *time.Time `json:"completed_at"`
}

// Certificate is issued once per (user, course) on 100% completion.
type Certificate struct {
	ID         uuid.UUID `json:"id"`
	UserID     uuid.UUID `json:"user_id"`
	CourseID   uuid.UUID `json:"course_id"`
	CertNumber string    `json:"cert_number"` // LEARNA-YYYY-XXXXXX
	PDFURL     *string   `json:"pdf_url"`
	PublicID   *string   `json:"-"` // Cloudinary handle, never sent to clients
	IssuedAt   time.Time `json:"issued_at"`
}
