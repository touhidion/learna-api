package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/learna/learna-api/internal/dto/request"
	"github.com/learna/learna-api/internal/dto/response"
	"github.com/learna/learna-api/internal/models"
	"github.com/learna/learna-api/internal/repository"
	"github.com/learna/learna-api/internal/utils"
)

// CourseService implements course management and the public catalog —
// features C1..C6 and PC1..PC2.
type CourseService struct {
	courses *repository.CourseRepository
	modules *repository.ModuleRepository
	lessons *repository.LessonRepository
}

func NewCourseService(d Deps) *CourseService {
	return &CourseService{
		courses: d.Repos.Courses,
		modules: d.Repos.Modules,
		lessons: d.Repos.Lessons,
	}
}

// maxSlugAttempts bounds the collision loop. Reaching it means something is
// badly wrong (a slug colliding 50 times), so failing loudly beats looping.
const maxSlugAttempts = 50

// Create adds a draft course — feature C1.
func (s *CourseService) Create(
	ctx context.Context,
	actor uuid.UUID,
	req request.CreateCourse,
) (*response.Course, error) {
	title := strings.TrimSpace(req.Title)

	slug, err := s.uniqueSlug(ctx, title, uuid.Nil)
	if err != nil {
		return nil, err
	}

	course, err := s.courses.Create(ctx, &models.Course{
		Title:        title,
		Slug:         slug,
		Description:  strings.TrimSpace(req.Description),
		Category:     strings.TrimSpace(req.Category),
		ThumbnailURL: req.ThumbnailURL,
		// Everything starts as a draft; publishing is a separate, deliberate
		// action so a half-built course cannot appear in the catalog.
		Status:    models.CourseStatusDraft,
		CreatedBy: &actor,
	})
	if err != nil {
		if repository.IsDuplicate(err) {
			return nil, utils.ErrConflict("A course with this title already exists.")
		}
		return nil, utils.ErrInternal(err)
	}

	out := response.NewCourse(course, 0, 0)
	return &out, nil
}

// List returns a page of courses for the admin tables — feature C2.
func (s *CourseService) List(
	ctx context.Context,
	req request.ListCourses,
	page utils.Pagination,
) (*utils.Page[response.Course], error) {
	filter := repository.CourseFilter{
		Search:   req.Search,
		Category: req.Category,
	}
	if req.Status != "" {
		filter.Status = models.CourseStatus(req.Status)
	}
	return s.list(ctx, filter, page, true)
}

// ListPublished returns the public catalog — feature PC1.
func (s *CourseService) ListPublished(
	ctx context.Context,
	req request.ListCourses,
	page utils.Pagination,
) (*utils.Page[response.Course], error) {
	filter := repository.CourseFilter{
		Search:        req.Search,
		Category:      req.Category,
		PublishedOnly: true,
	}
	return s.list(ctx, filter, page, false)
}

func (s *CourseService) list(
	ctx context.Context,
	filter repository.CourseFilter,
	page utils.Pagination,
	withCounts bool,
) (*utils.Page[response.Course], error) {
	rows, total, err := s.courses.List(ctx, filter, page)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	items := make([]response.Course, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		item := response.NewCourse(&row.Course, row.LessonCount, row.DurationMin)
		// Enrollment counts are an admin concern; the public catalog does not
		// advertise how few people are on a course.
		if withCounts {
			count := row.EnrollmentCount
			item.EnrollmentCount = &count
		}
		items = append(items, item)
	}

	result := utils.NewPage(items, total, page)
	return &result, nil
}

// Get returns one course by id, for admin editing.
func (s *CourseService) Get(ctx context.Context, id uuid.UUID) (*response.Course, error) {
	course, err := s.courses.GetByID(ctx, id)
	if err != nil {
		return nil, notFoundOr(err, "Course not found.")
	}

	lessons, duration, enrollments, err := s.courses.Stats(ctx, id)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	out := response.NewCourse(course, lessons, duration)
	out.EnrollmentCount = &enrollments
	return &out, nil
}

// GetBySlug returns a published course for the public catalog — feature PC2.
//
// A draft or archived course answers 404 rather than 403: revealing that a
// slug exists but is unpublished leaks the content pipeline.
func (s *CourseService) GetBySlug(ctx context.Context, slug string) (*response.Course, error) {
	course, err := s.courses.GetBySlug(ctx, slug)
	if err != nil {
		return nil, notFoundOr(err, "Course not found.")
	}
	if course.Status != models.CourseStatusPublished {
		return nil, utils.ErrNotFound("Course not found.")
	}

	lessons, duration, _, err := s.courses.Stats(ctx, course.ID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	out := response.NewCourse(course, lessons, duration)
	return &out, nil
}

// Update applies a partial update — feature C3.
//
// Changing the title regenerates the slug. That breaks any existing link to
// the old slug; Phase 1 accepts this, and a slug-history table is the fix when
// public URLs start to matter.
func (s *CourseService) Update(
	ctx context.Context,
	id uuid.UUID,
	req request.UpdateCourse,
) (*response.Course, error) {
	if req.Title == nil && req.Description == nil && req.Category == nil && req.ThumbnailURL == nil {
		return nil, utils.ErrValidation("No fields to update.")
	}

	current, err := s.courses.GetByID(ctx, id)
	if err != nil {
		return nil, notFoundOr(err, "Course not found.")
	}

	update := repository.CourseUpdate{
		Description:  trimmedPtr(req.Description),
		Category:     trimmedPtr(req.Category),
		ThumbnailURL: req.ThumbnailURL,
	}

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		update.Title = &title

		if title != current.Title {
			slug, err := s.uniqueSlug(ctx, title, id)
			if err != nil {
				return nil, err
			}
			update.Slug = &slug
		}
	}

	course, err := s.courses.Update(ctx, id, update)
	if err != nil {
		if repository.IsDuplicate(err) {
			return nil, utils.ErrConflict("A course with this title already exists.")
		}
		return nil, notFoundOr(err, "Course not found.")
	}

	lessons, duration, enrollments, err := s.courses.Stats(ctx, id)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	out := response.NewCourse(course, lessons, duration)
	out.EnrollmentCount = &enrollments
	return &out, nil
}

// SetStatus moves a course through the publish workflow — feature C5.
func (s *CourseService) SetStatus(
	ctx context.Context,
	id uuid.UUID,
	req request.UpdateCourseStatus,
) (*response.Course, error) {
	next := models.CourseStatus(req.Status)
	if !next.Valid() {
		return nil, utils.ErrValidation("Unknown status %q.", req.Status)
	}

	current, err := s.courses.GetByID(ctx, id)
	if err != nil {
		return nil, notFoundOr(err, "Course not found.")
	}

	if current.Status == next {
		return nil, utils.ErrConflict("Course is already %s.", next)
	}
	if !current.Status.CanTransitionTo(next) {
		return nil, utils.ErrUnprocessable(
			"Cannot move a course from %s to %s.", current.Status, next)
	}

	lessons, duration, enrollments, err := s.courses.Stats(ctx, id)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	// Publishing an empty course would put a dead entry in the catalog.
	if next == models.CourseStatusPublished && lessons == 0 {
		return nil, utils.ErrUnprocessable(
			"Add at least one lesson before publishing this course.")
	}

	course, err := s.courses.Update(ctx, id, repository.CourseUpdate{Status: &next})
	if err != nil {
		return nil, notFoundOr(err, "Course not found.")
	}

	out := response.NewCourse(course, lessons, duration)
	out.EnrollmentCount = &enrollments
	return &out, nil
}

// Delete removes a course and everything under it — feature C4.
func (s *CourseService) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.courses.Delete(ctx, id); err != nil {
		return notFoundOr(err, "Course not found.")
	}
	return nil
}

// Categories lists the categories in use — feature C6.
func (s *CourseService) Categories(ctx context.Context, publishedOnly bool) ([]string, error) {
	categories, err := s.courses.Categories(ctx, publishedOnly)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}
	return categories, nil
}

// uniqueSlug derives a slug from title and appends -2, -3, … until it is free.
//
// The check-then-insert is racy under concurrent creates; the unique index on
// courses.slug is the real guarantee, and the caller maps that violation to a
// 409.
func (s *CourseService) uniqueSlug(ctx context.Context, title string, excludeID uuid.UUID) (string, error) {
	base := utils.Slugify(title)
	if base == "" {
		// A title of only punctuation or non-Latin script folds to nothing.
		base = "course"
	}

	candidate := base
	for attempt := 2; attempt < maxSlugAttempts; attempt++ {
		taken, err := s.courses.SlugExists(ctx, candidate, excludeID)
		if err != nil {
			return "", utils.ErrInternal(err)
		}
		if !taken {
			return candidate, nil
		}
		candidate = utils.SlugWithSuffix(base, attempt)
	}
	return "", utils.ErrInternal(fmt.Errorf("could not find a free slug for %q", title))
}

// Detail returns a course with its full module and lesson tree.
//
// withContent decides whether lesson bodies travel with it. The public catalog
// passes false so an outline never carries the material itself (feature PC2);
// admins and enrolled learners pass true.
func (s *CourseService) Detail(
	ctx context.Context,
	course *models.Course,
	withContent bool,
) (*response.CourseDetail, error) {
	modules, err := s.modules.ListByCourse(ctx, course.ID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	lessons, err := s.lessons.ListByCourse(ctx, course.ID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	lessonCount := len(lessons)
	duration := 0
	for _, l := range lessons {
		duration += l.DurationMin
	}

	return &response.CourseDetail{
		Course:  response.NewCourse(course, lessonCount, duration),
		Modules: buildModuleTree(modules, lessons, withContent),
	}, nil
}

// DetailBySlug is the public course page — feature PC2.
func (s *CourseService) DetailBySlug(ctx context.Context, slug string) (*response.CourseDetail, error) {
	course, err := s.courses.GetBySlug(ctx, slug)
	if err != nil {
		return nil, notFoundOr(err, "Course not found.")
	}
	// An unpublished course is indistinguishable from a missing one, so the
	// content pipeline is not discoverable by guessing slugs.
	if course.Status != models.CourseStatusPublished {
		return nil, utils.ErrNotFound("Course not found.")
	}
	return s.Detail(ctx, course, false)
}

// DetailByID is the admin course page: any status, content included.
func (s *CourseService) DetailByID(ctx context.Context, id uuid.UUID) (*response.CourseDetail, error) {
	course, err := s.courses.GetByID(ctx, id)
	if err != nil {
		return nil, notFoundOr(err, "Course not found.")
	}

	detail, err := s.Detail(ctx, course, true)
	if err != nil {
		return nil, err
	}

	_, _, enrollments, err := s.courses.Stats(ctx, id)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}
	detail.EnrollmentCount = &enrollments
	return detail, nil
}

// notFoundOr maps a repository error onto the right APIError.
func notFoundOr(err error, message string) error {
	if repository.IsNotFound(err) {
		return utils.ErrNotFound("%s", message)
	}
	return utils.ErrInternal(err)
}

// trimmedPtr trims a pointed-to string, preserving nil as "field absent".
func trimmedPtr(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	return &t
}
