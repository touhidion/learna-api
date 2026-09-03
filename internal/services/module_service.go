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

// ModuleService implements module management — features M1..M4.
type ModuleService struct {
	courses *repository.CourseRepository
	modules *repository.ModuleRepository
	lessons *repository.LessonRepository
}

func NewModuleService(d Deps) *ModuleService {
	return &ModuleService{
		courses: d.Repos.Courses,
		modules: d.Repos.Modules,
		lessons: d.Repos.Lessons,
	}
}

// List returns a course's modules with their lessons.
func (s *ModuleService) List(ctx context.Context, courseID uuid.UUID) ([]response.Module, error) {
	if _, err := s.courses.GetByID(ctx, courseID); err != nil {
		return nil, notFoundOr(err, "Course not found.")
	}

	modules, err := s.modules.ListByCourse(ctx, courseID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	lessons, err := s.lessons.ListByCourse(ctx, courseID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	// Admin view: lesson content is included, no completion state.
	return buildModuleTree(modules, lessons, true, nil), nil
}

// Create adds a module, appended to the end — feature M1.
func (s *ModuleService) Create(
	ctx context.Context,
	courseID uuid.UUID,
	req request.CreateModule,
) (*response.Module, error) {
	if _, err := s.courses.GetByID(ctx, courseID); err != nil {
		return nil, notFoundOr(err, "Course not found.")
	}

	sortOrder := 0
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	} else {
		next, err := s.modules.NextSortOrder(ctx, courseID)
		if err != nil {
			return nil, utils.ErrInternal(err)
		}
		sortOrder = next
	}

	module, err := s.modules.Create(ctx, &models.Module{
		CourseID:    courseID,
		Title:       strings.TrimSpace(req.Title),
		Description: strings.TrimSpace(req.Description),
		SortOrder:   sortOrder,
	})
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	out := newModuleResponse(module, nil, true, nil)
	return &out, nil
}

// Update edits a module — feature M2.
func (s *ModuleService) Update(
	ctx context.Context,
	id uuid.UUID,
	req request.UpdateModule,
) (*response.Module, error) {
	if req.Title == nil && req.Description == nil {
		return nil, utils.ErrValidation("No fields to update.")
	}

	update := repository.ModuleUpdate{
		Title:       trimmedPtr(req.Title),
		Description: trimmedPtr(req.Description),
	}

	module, err := s.modules.Update(ctx, id, update)
	if err != nil {
		return nil, notFoundOr(err, "Module not found.")
	}

	out := newModuleResponse(module, nil, true, nil)
	return &out, nil
}

// Delete removes a module and its lessons — feature M3.
func (s *ModuleService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.modules.Delete(ctx, id); err != nil {
		return notFoundOr(err, "Module not found.")
	}
	return nil
}

// Reorder rewrites the positions of a course's modules — feature M4.
//
// The payload must list every module in the course. A partial list would leave
// the omitted ones on stale positions, producing duplicate or gapped orders
// that the UI would render unpredictably.
func (s *ModuleService) Reorder(
	ctx context.Context,
	courseID uuid.UUID,
	req request.Reorder,
) ([]response.Module, error) {
	if _, err := s.courses.GetByID(ctx, courseID); err != nil {
		return nil, notFoundOr(err, "Course not found.")
	}

	order, err := reorderMap(req)
	if err != nil {
		return nil, err
	}

	total, err := s.modules.CountByCourse(ctx, courseID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}
	if len(order) != total {
		return nil, utils.ErrValidation(
			"Send every module in the course: got %d, the course has %d.", len(order), total)
	}

	if err := s.modules.Reorder(ctx, courseID, order); err != nil {
		if repository.IsNotFound(err) {
			return nil, utils.ErrValidation("One or more modules do not belong to this course.")
		}
		return nil, utils.ErrInternal(err)
	}

	return s.List(ctx, courseID)
}

// reorderMap converts the request into id -> position, rejecting duplicates.
//
// A duplicated id would mean the last occurrence silently wins, so it is an
// error rather than something to resolve arbitrarily.
func reorderMap(req request.Reorder) (map[uuid.UUID]int, error) {
	order := make(map[uuid.UUID]int, len(req.Items))
	for _, item := range req.Items {
		if _, seen := order[item.ID]; seen {
			return nil, utils.ErrValidation("Item %s appears more than once.", item.ID)
		}
		order[item.ID] = item.SortOrder
	}
	return order, nil
}

// buildModuleTree nests lessons under their modules in one pass.
//
// completed, when non-nil, marks each lesson the caller has finished. It is
// nil for admin views, where completion is not a meaningful concept.
func buildModuleTree(
	modules []*models.Module,
	lessons []*models.Lesson,
	withContent bool,
	completed map[uuid.UUID]bool,
) []response.Module {
	byModule := make(map[uuid.UUID][]*models.Lesson, len(modules))
	for _, l := range lessons {
		byModule[l.ModuleID] = append(byModule[l.ModuleID], l)
	}

	out := make([]response.Module, 0, len(modules))
	for _, m := range modules {
		out = append(out, newModuleResponse(m, byModule[m.ID], withContent, completed))
	}
	return out
}

func newModuleResponse(
	m *models.Module,
	lessons []*models.Lesson,
	withContent bool,
	completed map[uuid.UUID]bool,
) response.Module {
	items := make([]response.Lesson, 0, len(lessons))
	for _, l := range lessons {
		item := newLessonResponse(l, withContent)
		if completed != nil {
			done := completed[l.ID]
			item.Completed = &done
		}
		items = append(items, item)
	}

	return response.Module{
		ID:          m.ID,
		CourseID:    m.CourseID,
		Title:       m.Title,
		Description: m.Description,
		SortOrder:   m.SortOrder,
		Lessons:     items,
	}
}

// newLessonResponse projects a lesson, omitting the body when the caller is
// not entitled to it — that omission is what keeps the public outline free of
// paid content (feature PC2).
func newLessonResponse(l *models.Lesson, withContent bool) response.Lesson {
	out := response.Lesson{
		ID:          l.ID,
		ModuleID:    l.ModuleID,
		Title:       l.Title,
		DurationMin: l.DurationMin,
		SortOrder:   l.SortOrder,
	}
	if withContent {
		out.Content = l.Content
		out.VideoURL = l.VideoURL
	}
	return out
}
