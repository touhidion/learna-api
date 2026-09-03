package request

import "github.com/google/uuid"

// Pointer fields mean "absent" rather than "empty" on PATCH payloads: a nil
// field is left untouched, a non-nil one is written even when it is empty.

// --- users ------------------------------------------------------------------

type CreateUser struct {
	Name     string `json:"name" binding:"required,min=2,max=120"`
	Email    string `json:"email" binding:"required,email,max=255"`
	Password string `json:"password" binding:"required,min=8,max=72"`
	Role     string `json:"role" binding:"required,oneof=admin learner"`
}

type UpdateUser struct {
	Name     *string `json:"name" binding:"omitempty,min=2,max=120"`
	Role     *string `json:"role" binding:"omitempty,oneof=super_admin admin learner"`
	IsActive *bool   `json:"is_active"`
}

type ListUsers struct {
	Search string `form:"search" binding:"omitempty,max=255"`
	Role   string `form:"role" binding:"omitempty,oneof=super_admin admin learner"`
	Active *bool  `form:"active"`
}

// --- courses ----------------------------------------------------------------

type CreateCourse struct {
	Title        string  `json:"title" binding:"required,min=3,max=200"`
	Description  string  `json:"description" binding:"omitempty,max=10000"`
	Category     string  `json:"category" binding:"omitempty,max=100"`
	ThumbnailURL *string `json:"thumbnail_url" binding:"omitempty,url,max=1024"`
}

type UpdateCourse struct {
	Title        *string `json:"title" binding:"omitempty,min=3,max=200"`
	Description  *string `json:"description" binding:"omitempty,max=10000"`
	Category     *string `json:"category" binding:"omitempty,max=100"`
	ThumbnailURL *string `json:"thumbnail_url" binding:"omitempty,url,max=1024"`
}

type UpdateCourseStatus struct {
	Status string `json:"status" binding:"required,oneof=draft published archived"`
}

type ListCourses struct {
	Search   string `form:"search" binding:"omitempty,max=255"`
	Category string `form:"category" binding:"omitempty,max=100"`
	Status   string `form:"status" binding:"omitempty,oneof=draft published archived"`
}

// --- modules ----------------------------------------------------------------

type CreateModule struct {
	Title       string `json:"title" binding:"required,min=2,max=200"`
	Description string `json:"description" binding:"omitempty,max=5000"`
	SortOrder   *int   `json:"sort_order" binding:"omitempty,gte=0"`
}

type UpdateModule struct {
	Title       *string `json:"title" binding:"omitempty,min=2,max=200"`
	Description *string `json:"description" binding:"omitempty,max=5000"`
}

// --- lessons ----------------------------------------------------------------

type CreateLesson struct {
	Title       string  `json:"title" binding:"required,min=2,max=200"`
	Content     string  `json:"content" binding:"omitempty"`
	VideoURL    *string `json:"video_url" binding:"omitempty,url,max=1024"`
	DurationMin *int    `json:"duration_min" binding:"omitempty,gte=0,lte=10000"`
	SortOrder   *int    `json:"sort_order" binding:"omitempty,gte=0"`
}

type UpdateLesson struct {
	Title       *string `json:"title" binding:"omitempty,min=2,max=200"`
	Content     *string `json:"content"`
	VideoURL    *string `json:"video_url" binding:"omitempty,url,max=1024"`
	DurationMin *int    `json:"duration_min" binding:"omitempty,gte=0,lte=10000"`
}

// --- reordering -------------------------------------------------------------

// ReorderItem pairs an entity with its new position. Reorder endpoints take
// the whole list in one call so the new ordering is applied in a single
// transaction and never observed half-applied.
type ReorderItem struct {
	ID        uuid.UUID `json:"id" binding:"required"`
	SortOrder int       `json:"sort_order" binding:"gte=0"`
}

type Reorder struct {
	Items []ReorderItem `json:"items" binding:"required,min=1,max=500,dive"`
}
