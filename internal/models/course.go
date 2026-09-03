package models

import (
	"time"

	"github.com/google/uuid"
)

// CourseStatus is the course_status enum in Postgres. Only Published courses
// appear in the public catalog.
type CourseStatus string

const (
	CourseStatusDraft     CourseStatus = "draft"
	CourseStatusPublished CourseStatus = "published"
	CourseStatusArchived  CourseStatus = "archived"
)

func (s CourseStatus) Valid() bool {
	switch s {
	case CourseStatusDraft, CourseStatusPublished, CourseStatusArchived:
		return true
	}
	return false
}

func (s CourseStatus) String() string { return string(s) }

// CanTransitionTo encodes the Phase 1 status machine:
//
//	draft <-> published -> archived -> draft
func (s CourseStatus) CanTransitionTo(next CourseStatus) bool {
	switch s {
	case CourseStatusDraft:
		return next == CourseStatusPublished || next == CourseStatusArchived
	case CourseStatusPublished:
		return next == CourseStatusDraft || next == CourseStatusArchived
	case CourseStatusArchived:
		return next == CourseStatusDraft
	}
	return false
}

type Course struct {
	ID           uuid.UUID    `json:"id"`
	Title        string       `json:"title"`
	Slug         string       `json:"slug"`
	Description  string       `json:"description"`
	ThumbnailURL *string      `json:"thumbnail_url"`
	Category     string       `json:"category"`
	Status       CourseStatus `json:"status"`
	CreatedBy    *uuid.UUID   `json:"created_by"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}

type Module struct {
	ID          uuid.UUID `json:"id"`
	CourseID    uuid.UUID `json:"course_id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Lesson struct {
	ID          uuid.UUID `json:"id"`
	ModuleID    uuid.UUID `json:"module_id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"` // raw markdown; the UI renders it
	VideoURL    *string   `json:"video_url"`
	DurationMin int       `json:"duration_min"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Attachment struct {
	ID        uuid.UUID `json:"id"`
	LessonID  uuid.UUID `json:"lesson_id"`
	FileName  string    `json:"file_name"`
	FileURL   string    `json:"file_url"`
	PublicID  *string   `json:"-"` // Cloudinary handle, never sent to clients
	FileType  string    `json:"file_type"`
	FileSize  int64     `json:"file_size"`
	CreatedAt time.Time `json:"created_at"`
}
