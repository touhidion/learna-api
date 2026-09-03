package services

import (
	"context"

	"github.com/google/uuid"

	"github.com/learna/learna-api/internal/dto/response"
	"github.com/learna/learna-api/internal/repository"
	"github.com/learna/learna-api/internal/utils"
)

// AnalyticsService backs the admin dashboards — features AN1, AN2, ACA1.
type AnalyticsService struct {
	analytics *repository.AnalyticsRepository
	courses   *repository.CourseRepository
}

func NewAnalyticsService(d Deps) *AnalyticsService {
	return &AnalyticsService{analytics: d.Repos.Analytics, courses: d.Repos.Courses}
}

// Overview returns the portal-wide totals — feature AN1.
func (s *AnalyticsService) Overview(ctx context.Context) (*response.AnalyticsOverview, error) {
	o, err := s.analytics.GetOverview(ctx)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	return &response.AnalyticsOverview{
		TotalUsers:       o.TotalUsers,
		TotalLearners:    o.TotalLearners,
		TotalAdmins:      o.TotalAdmins,
		TotalCourses:     o.TotalCourses,
		PublishedCourses: o.PublishedCourses,
		DraftCourses:     o.DraftCourses,
		ArchivedCourses:  o.ArchivedCourses,
		TotalEnrollments: o.TotalEnrollments,
		TotalCompletions: o.TotalCompletions,
	}, nil
}

// CourseAnalytics returns per-course engagement — feature AN2.
func (s *AnalyticsService) CourseAnalytics(ctx context.Context, courseID uuid.UUID) (*response.CourseAnalytics, error) {
	stats, err := s.analytics.GetCourseStats(ctx, courseID)
	if err != nil {
		return nil, notFoundOr(err, "Course not found.")
	}

	// A course with no enrollments has a 0% completion rate, not a division by
	// zero.
	rate := 0.0
	if stats.EnrollmentCount > 0 {
		rate = float64(stats.CompletionCount) / float64(stats.EnrollmentCount) * 100
	}

	return &response.CourseAnalytics{
		CourseID:        courseID,
		CourseTitle:     stats.CourseTitle,
		EnrollmentCount: stats.EnrollmentCount,
		CompletionCount: stats.CompletionCount,
		CompletionRate:  round1(rate),
		AverageProgress: round1(stats.AverageProgress),
	}, nil
}

// CourseLearners lists each enrolled learner with their progress — ACA1.
func (s *AnalyticsService) CourseLearners(ctx context.Context, courseID uuid.UUID) ([]response.LearnerProgress, error) {
	rows, err := s.analytics.ListCourseLearners(ctx, courseID)
	if err != nil {
		return nil, utils.ErrInternal(err)
	}

	out := make([]response.LearnerProgress, 0, len(rows))
	for _, r := range rows {
		out = append(out, response.LearnerProgress{
			UserID:      r.UserID,
			Name:        r.Name,
			Email:       r.Email,
			Percentage:  round1(r.Percentage),
			IsCompleted: r.IsCompleted,
		})
	}
	return out, nil
}

// round1 keeps percentages to one decimal place, matching response.NewProgress.
func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}
