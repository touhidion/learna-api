package services

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/learna/learna-api/internal/dto/request"
	"github.com/learna/learna-api/internal/dto/response"
	"github.com/learna/learna-api/internal/models"
	"github.com/learna/learna-api/internal/repository"
	"github.com/learna/learna-api/internal/utils"
)

// LessonService implements lesson management — features L1..L5.
//
// Lesson bodies are stored as raw markdown and never rendered here (L5). The
// UI renders them, which keeps the API free of an HTML sanitiser and means the
// same content can be rendered differently by different clients.
type LessonService struct {
	modules *repository.ModuleRepository
	lessons *repository.LessonRepository
}

func NewLessonService(d Deps) *LessonService {
	return &LessonService{
		modules: d.Repos.Modules,
		lessons: d.Repos.Lessons,
	}
}

// ListByModule returns a module's lessons.
func (s *LessonService) ListByModule(ctx context.Context, moduleID uuid.UUID) ([]response.Lesson, error) {
	if _, err := s.modules.GetByID(ctx, moduleID); err != nil {
		return nil, notFoundOr(err, "Module not found.")
	}

	lessons, err := s.lessons.ListByModule(ctx, moduleID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	out := make([]response.Lesson, 0, len(lessons))
	for _, l := range lessons {
		out = append(out, newLessonResponse(l, true))
	}
	return out, nil
}

// Get returns one lesson with its full content.
func (s *LessonService) Get(ctx context.Context, id uuid.UUID) (*response.Lesson, error) {
	lesson, err := s.lessons.GetByID(ctx, id)
	if err != nil {
		return nil, notFoundOr(err, "Lesson not found.")
	}
	out := newLessonResponse(lesson, true)
	return &out, nil
}

// Create adds a lesson, appended to the end of its module — feature L1.
func (s *LessonService) Create(
	ctx context.Context,
	moduleID uuid.UUID,
	req request.CreateLesson,
) (*response.Lesson, error) {
	if _, err := s.modules.GetByID(ctx, moduleID); err != nil {
		return nil, notFoundOr(err, "Module not found.")
	}

	sortOrder := 0
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	} else {
		next, err := s.lessons.NextSortOrder(ctx, moduleID)
		if err != nil {
			return nil, utils.ErrInternal(err)
		}
		sortOrder = next
	}

	duration := 0
	if req.DurationMin != nil {
		duration = *req.DurationMin
	}

	lesson, err := s.lessons.Create(ctx, &models.Lesson{
		ModuleID:    moduleID,
		Title:       strings.TrimSpace(req.Title),
		Content:     req.Content,
		VideoURL:    emptyToNil(req.VideoURL),
		DurationMin: duration,
		SortOrder:   sortOrder,
	})
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	out := newLessonResponse(lesson, true)
	return &out, nil
}

// Update edits a lesson — feature L2.
func (s *LessonService) Update(
	ctx context.Context,
	id uuid.UUID,
	req request.UpdateLesson,
) (*response.Lesson, error) {
	if req.Title == nil && req.Content == nil && req.VideoURL == nil && req.DurationMin == nil {
		return nil, utils.ErrValidation("No fields to update.")
	}

	update := repository.LessonUpdate{
		Title:       trimmedPtr(req.Title),
		Content:     req.Content, // markdown is stored verbatim, whitespace included
		VideoURL:    req.VideoURL,
		DurationMin: req.DurationMin,
	}

	lesson, err := s.lessons.Update(ctx, id, update)
	if err != nil {
		return nil, notFoundOr(err, "Lesson not found.")
	}

	out := newLessonResponse(lesson, true)
	return &out, nil
}

// Delete removes a lesson, its attachments and its progress rows — feature L3.
func (s *LessonService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.lessons.Delete(ctx, id); err != nil {
		return notFoundOr(err, "Lesson not found.")
	}
	return nil
}

// Reorder rewrites the positions of a module's lessons — feature L4.
func (s *LessonService) Reorder(
	ctx context.Context,
	moduleID uuid.UUID,
	req request.Reorder,
) ([]response.Lesson, error) {
	if _, err := s.modules.GetByID(ctx, moduleID); err != nil {
		return nil, notFoundOr(err, "Module not found.")
	}

	order, err := reorderMap(req)
	if err != nil {
		return nil, err
	}

	existing, err := s.lessons.ListByModule(ctx, moduleID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}
	if len(order) != len(existing) {
		return nil, utils.ErrValidation(
			"Send every lesson in the module: got %d, the module has %d.",
			len(order), len(existing))
	}

	if err := s.lessons.Reorder(ctx, moduleID, order); err != nil {
		if repository.IsNotFound(err) {
			return nil, utils.ErrValidation("One or more lessons do not belong to this module.")
		}
		return nil, utils.ErrInternal(err)
	}

	return s.ListByModule(ctx, moduleID)
}

// emptyToNil stores an empty optional URL as NULL rather than "".
func emptyToNil(s *string) *string {
	if s == nil {
		return nil
	}
	if strings.TrimSpace(*s) == "" {
		return nil
	}
	return s
}
