package response

import (
	"time"

	"github.com/google/uuid"

	"github.com/learna/learna-api/internal/models"
)

// Course is the catalog-card shape: enough to render a listing, without the
// module tree.
type Course struct {
	ID           uuid.UUID           `json:"id"`
	Title        string              `json:"title"`
	Slug         string              `json:"slug"`
	Description  string              `json:"description"`
	ThumbnailURL *string             `json:"thumbnail_url"`
	Category     string              `json:"category"`
	Status       models.CourseStatus `json:"status"`
	LessonCount  int                 `json:"lesson_count"`
	DurationMin  int                 `json:"duration_min"`
	CreatedAt    time.Time           `json:"created_at"`
	UpdatedAt    time.Time           `json:"updated_at"`

	// Populated on admin listings only.
	EnrollmentCount *int `json:"enrollment_count,omitempty"`
	// Populated for authenticated callers on public listings.
	Progress *float64 `json:"progress,omitempty"`
}

// CourseDetail adds the module tree. On public routes the lessons carry titles
// only; Content is filled in solely for enrolled learners and admins.
type CourseDetail struct {
	Course
	Modules []Module `json:"modules"`
}

type Module struct {
	ID          uuid.UUID `json:"id"`
	CourseID    uuid.UUID `json:"course_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	SortOrder   int       `json:"sort_order"`
	Lessons     []Lesson  `json:"lessons"`
}

// Lesson doubles as the outline entry and the full lesson view. Content is
// omitted when empty, which is how the public outline stays content-free.
type Lesson struct {
	ID          uuid.UUID `json:"id"`
	ModuleID    uuid.UUID `json:"module_id"`
	Title       string    `json:"title"`
	Content     string    `json:"content,omitempty"`
	VideoURL    *string   `json:"video_url,omitempty"`
	DurationMin int       `json:"duration_min"`
	SortOrder   int       `json:"sort_order"`

	// Present for authenticated learners.
	Completed   *bool        `json:"completed,omitempty"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

type Attachment struct {
	ID        uuid.UUID `json:"id"`
	LessonID  uuid.UUID `json:"lesson_id"`
	FileName  string    `json:"file_name"`
	FileURL   string    `json:"file_url"`
	FileType  string    `json:"file_type"`
	FileSize  int64     `json:"file_size"`
	CreatedAt time.Time `json:"created_at"`
}

// Enrollment is one row of "my courses", course summary included so the
// dashboard renders from a single request.
type Enrollment struct {
	ID          uuid.UUID  `json:"id"`
	Course      Course     `json:"course"`
	EnrolledAt  time.Time  `json:"enrolled_at"`
	CompletedAt *time.Time `json:"completed_at"`
	Progress    Progress   `json:"progress"`
}

type Progress struct {
	TotalLessons     int     `json:"total_lessons"`
	CompletedLessons int     `json:"completed_lessons"`
	Percentage       float64 `json:"percentage"`
}

type Certificate struct {
	ID          uuid.UUID `json:"id"`
	CertNumber  string    `json:"cert_number"`
	CourseID    uuid.UUID `json:"course_id"`
	CourseTitle string    `json:"course_title"`
	UserName    string    `json:"user_name"`
	PDFURL      *string   `json:"pdf_url"`
	IssuedAt    time.Time `json:"issued_at"`
}

// Upload is what the Cloudinary-backed endpoints return.
type Upload struct {
	URL      string `json:"url"`
	PublicID string `json:"public_id"`
	FileName string `json:"file_name"`
	FileType string `json:"file_type"`
	FileSize int64  `json:"file_size"`
}

// AnalyticsOverview backs the admin dashboard cards.
type AnalyticsOverview struct {
	TotalUsers       int64 `json:"total_users"`
	TotalLearners    int64 `json:"total_learners"`
	TotalAdmins      int64 `json:"total_admins"`
	TotalCourses     int64 `json:"total_courses"`
	PublishedCourses int64 `json:"published_courses"`
	DraftCourses     int64 `json:"draft_courses"`
	ArchivedCourses  int64 `json:"archived_courses"`
	TotalEnrollments int64 `json:"total_enrollments"`
	TotalCompletions int64 `json:"total_completions"`
}

// CourseAnalytics backs the per-course admin stats page.
type CourseAnalytics struct {
	CourseID        uuid.UUID `json:"course_id"`
	CourseTitle     string    `json:"course_title"`
	EnrollmentCount int64     `json:"enrollment_count"`
	CompletionCount int64     `json:"completion_count"`
	CompletionRate  float64   `json:"completion_rate"`
	AverageProgress float64   `json:"average_progress"`
}

// NewCourse projects a model onto the catalog-card shape. Counts are supplied
// by the caller because they come from aggregate queries, not the course row.
func NewCourse(c *models.Course, lessonCount, durationMin int) Course {
	return Course{
		ID:           c.ID,
		Title:        c.Title,
		Slug:         c.Slug,
		Description:  c.Description,
		ThumbnailURL: c.ThumbnailURL,
		Category:     c.Category,
		Status:       c.Status,
		LessonCount:  lessonCount,
		DurationMin:  durationMin,
		CreatedAt:    c.CreatedAt,
		UpdatedAt:    c.UpdatedAt,
	}
}

// NewProgress computes the percentage, rounded to one decimal place, and
// treats a course with no lessons as 0% rather than dividing by zero.
func NewProgress(total, completed int) Progress {
	pct := 0.0
	if total > 0 {
		pct = float64(completed) / float64(total) * 100
		pct = float64(int(pct*10+0.5)) / 10
	}
	return Progress{TotalLessons: total, CompletedLessons: completed, Percentage: pct}
}
