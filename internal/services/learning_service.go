package services

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/learna/learna-api/internal/dto/response"
	"github.com/learna/learna-api/internal/models"
	"github.com/learna/learna-api/internal/repository"
	"github.com/learna/learna-api/internal/utils"
)

// LearningService covers enrollment and progress — features E1..E4, PR1..PR4.
//
// The two live together because every progress change has to re-evaluate the
// enrollment's completion state, and splitting them would mean one calling
// into the other for every mark and unmark.
type LearningService struct {
	courses     *repository.CourseRepository
	modules     *repository.ModuleRepository
	lessons     *repository.LessonRepository
	enrollments *repository.EnrollmentRepository
	progress    *repository.ProgressRepository
}

func NewLearningService(d Deps) *LearningService {
	return &LearningService{
		courses:     d.Repos.Courses,
		modules:     d.Repos.Modules,
		lessons:     d.Repos.Lessons,
		enrollments: d.Repos.Enrollments,
		progress:    d.Repos.Progress,
	}
}

// Enroll adds the caller to a course — feature E1.
//
// Only published courses accept enrollments: a draft is unfinished and an
// archived course is deliberately closed.
func (s *LearningService) Enroll(ctx context.Context, userID, courseID uuid.UUID) (*response.Enrollment, error) {
	course, err := s.courses.GetByID(ctx, courseID)
	if err != nil {
		return nil, notFoundOr(err, "Course not found.")
	}
	if course.Status != models.CourseStatusPublished {
		return nil, utils.ErrUnprocessable("This course is not open for enrollment.")
	}

	enrollment, err := s.enrollments.Create(ctx, userID, courseID)
	if err != nil {
		if repository.IsDuplicate(err) {
			return nil, utils.ErrConflict("You are already enrolled in this course.")
		}
		return nil, utils.ErrInternal(err)
	}

	completed, total, err := s.progress.CourseProgress(ctx, userID, courseID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	out := s.enrollmentResponse(enrollment, course, completed, total, 0)
	return &out, nil
}

// Unenroll removes the caller and their progress — feature E2.
func (s *LearningService) Unenroll(ctx context.Context, userID, courseID uuid.UUID) error {
	if err := s.enrollments.Delete(ctx, userID, courseID); err != nil {
		if repository.IsNotFound(err) {
			return utils.ErrNotFound("You are not enrolled in this course.")
		}
		return utils.ErrInternal(err)
	}
	return nil
}

// MyEnrollments lists the caller's courses with progress — feature E3.
func (s *LearningService) MyEnrollments(
	ctx context.Context,
	userID uuid.UUID,
	page utils.Pagination,
) (*utils.Page[response.Enrollment], error) {
	rows, total, err := s.enrollments.ListByUser(ctx, userID, page)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	items := make([]response.Enrollment, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		items = append(items, s.enrollmentResponse(
			&r.Enrollment, &r.Course, r.CompletedLessons, r.TotalLessons, r.DurationMin))
	}

	result := utils.NewPage(items, total, page)
	return &result, nil
}

// IsEnrolled backs the content guard — feature E4.
func (s *LearningService) IsEnrolled(ctx context.Context, userID, courseID uuid.UUID) (bool, error) {
	enrolled, err := s.enrollments.Exists(ctx, userID, courseID)
	if err != nil {
		return false, utils.ErrInternal(err)
	}
	return enrolled, nil
}

// RequireEnrollment refuses access to lesson content for a learner who has not
// enrolled — feature E4. Admins are exempt so they can review their own
// courses without enrolling in them.
func (s *LearningService) RequireEnrollment(
	ctx context.Context,
	userID uuid.UUID,
	role models.Role,
	courseID uuid.UUID,
) error {
	if role.IsAdmin() {
		return nil
	}

	enrolled, err := s.IsEnrolled(ctx, userID, courseID)
	if err != nil {
		return err
	}
	if !enrolled {
		return utils.ErrForbidden("Enrol in this course to access its lessons.")
	}
	return nil
}

// MarkComplete records a finished lesson — feature PR1.
func (s *LearningService) MarkComplete(
	ctx context.Context,
	userID uuid.UUID,
	role models.Role,
	lessonID uuid.UUID,
) (*response.Progress, error) {
	return s.setLessonState(ctx, userID, role, lessonID, true)
}

// Unmark reverses it — feature PR2.
func (s *LearningService) Unmark(
	ctx context.Context,
	userID uuid.UUID,
	role models.Role,
	lessonID uuid.UUID,
) (*response.Progress, error) {
	return s.setLessonState(ctx, userID, role, lessonID, false)
}

// setLessonState is the shared path for marking and unmarking, so completion
// bookkeeping cannot drift between the two.
func (s *LearningService) setLessonState(
	ctx context.Context,
	userID uuid.UUID,
	role models.Role,
	lessonID uuid.UUID,
	complete bool,
) (*response.Progress, error) {
	courseID, err := s.lessons.CourseIDFor(ctx, lessonID)
	if err != nil {
		return nil, notFoundOr(err, "Lesson not found.")
	}
	if err := s.RequireEnrollment(ctx, userID, role, courseID); err != nil {
		return nil, err
	}

	if complete {
		err = s.progress.MarkComplete(ctx, userID, lessonID)
	} else {
		err = s.progress.Unmark(ctx, userID, lessonID)
	}
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	return s.syncCompletion(ctx, userID, courseID)
}

// CourseProgress reports the caller's percentage — feature PR3.
func (s *LearningService) CourseProgress(
	ctx context.Context,
	userID, courseID uuid.UUID,
) (*response.Progress, error) {
	completed, total, err := s.progress.CourseProgress(ctx, userID, courseID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}
	out := response.NewProgress(total, completed)
	return &out, nil
}

// syncCompletion stamps or clears enrollments.completed_at — feature PR4.
//
// It runs after every change so the flag stays truthful in both directions: a
// learner who un-completes a lesson is no longer finished, and their
// certificate CTA disappears again.
func (s *LearningService) syncCompletion(
	ctx context.Context,
	userID, courseID uuid.UUID,
) (*response.Progress, error) {
	completed, total, err := s.progress.CourseProgress(ctx, userID, courseID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	enrollment, err := s.enrollments.Get(ctx, userID, courseID)
	if err != nil {
		// An admin previewing a course has no enrollment row; there is simply
		// nothing to stamp.
		if repository.IsNotFound(err) {
			out := response.NewProgress(total, completed)
			return &out, nil
		}
		return nil, utils.ErrInternal(err)
	}

	finished := total > 0 && completed >= total
	switch {
	case finished && enrollment.CompletedAt == nil:
		now := time.Now()
		if err := s.enrollments.SetCompletedAt(ctx, userID, courseID, &now); err != nil {
			return nil, utils.ErrInternal(err)
		}
	case !finished && enrollment.CompletedAt != nil:
		if err := s.enrollments.SetCompletedAt(ctx, userID, courseID, nil); err != nil {
			return nil, utils.ErrInternal(err)
		}
	}

	out := response.NewProgress(total, completed)
	return &out, nil
}

// CompletedLessons returns which lessons in a course the caller has finished,
// so the course tree can be marked up in one pass.
func (s *LearningService) CompletedLessons(
	ctx context.Context,
	userID, courseID uuid.UUID,
) (map[uuid.UUID]bool, error) {
	done, err := s.progress.CompletedLessonIDs(ctx, userID, courseID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}
	return done, nil
}

func (s *LearningService) enrollmentResponse(
	e *models.Enrollment,
	c *models.Course,
	completed, total, durationMin int,
) response.Enrollment {
	return response.Enrollment{
		ID:          e.ID,
		Course:      response.NewCourse(c, total, durationMin),
		EnrolledAt:  e.EnrolledAt,
		CompletedAt: e.CompletedAt,
		Progress:    response.NewProgress(total, completed),
	}
}
