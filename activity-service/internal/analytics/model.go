package analytics

import (
	"context"
	"time"
)

// ProgressStats represents aggregated productivity statistics
type ProgressStats struct {
	TimeConsumedMinutes int                    `json:"time_consumed_minutes"`
	TotalSessions       int                    `json:"total_sessions"`
	TotalHours          float64                `json:"total_hours"`
	MostProductiveHours []int                  `json:"most_productive_hours"` // hours of day (0-23)
	Streak              StreakInfo             `json:"streak"`
	ByCategory          map[string]int         `json:"by_category"` // category -> minutes
	ByPeriod            map[string]PeriodStats `json:"by_period"`   // period -> stats
}

// StreakInfo represents streak information
type StreakInfo struct {
	Current    int       `json:"current"`
	Longest    int       `json:"longest"`
	LastActive time.Time `json:"last_active"`
}

// PeriodStats represents statistics for a specific time period
type PeriodStats struct {
	TimeConsumedMinutes int            `json:"time_consumed_minutes"`
	TotalSessions       int            `json:"total_sessions"`
	TotalHours          float64        `json:"total_hours"`
	ByCategory          map[string]int `json:"by_category"`
}

// TimeRange represents a time range for analytics queries
type TimeRange struct {
	Start time.Time
	End   time.Time
}

// PeriodType represents different time periods for analytics
type PeriodType string

const (
	PeriodWeek    PeriodType = "week"
	PeriodMonth   PeriodType = "month"
	PeriodQuarter PeriodType = "quarter"
	PeriodYear    PeriodType = "year"
	PeriodAll     PeriodType = "all"
)

// AnalyticsRequest represents a request for analytics data
type AnalyticsRequest struct {
	UserID    string
	Period    PeriodType
	Category  string // optional filter by category
	StartDate *time.Time
	EndDate   *time.Time
}

// AnalyticsResponse represents the response from analytics queries
type AnalyticsResponse struct {
	Period      string        `json:"period"`
	Range       TimeRange     `json:"range"`
	Stats       ProgressStats `json:"stats"`
	GeneratedAt time.Time     `json:"generated_at"`
}

// Repository encapsulates analytics data access
type Repository interface {
	GetProgressStats(ctx context.Context, userID string, start, end time.Time, category string) (ProgressStats, error)
	GetStreakInfo(ctx context.Context, userID string) (StreakInfo, error)
	GetMostProductiveHours(ctx context.Context, userID string, start, end time.Time) ([]int, error)
	GetCategoryBreakdown(ctx context.Context, userID string, start, end time.Time) (map[string]int, error)
}

// Service orchestrates analytics operations
type Service struct {
	repo Repository
}

// NewService constructs a Service instance
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// GetProgress retrieves progress analytics for a user
func (s *Service) GetProgress(ctx context.Context, req AnalyticsRequest) (AnalyticsResponse, error) {
	start, end := s.calculateTimeRange(req)

	stats, err := s.repo.GetProgressStats(ctx, req.UserID, start, end, req.Category)
	if err != nil {
		return AnalyticsResponse{}, err
	}

	return AnalyticsResponse{
		Period:      string(req.Period),
		Range:       TimeRange{Start: start, End: end},
		Stats:       stats,
		GeneratedAt: time.Now().UTC(),
	}, nil
}

// calculateTimeRange determines the time range based on the period type
func (s *Service) calculateTimeRange(req AnalyticsRequest) (time.Time, time.Time) {
	now := time.Now().UTC()

	if req.StartDate != nil && req.EndDate != nil {
		return *req.StartDate, *req.EndDate
	}

	switch req.Period {
	case PeriodWeek:
		// Last 7 days
		start := now.AddDate(0, 0, -7)
		return start, now
	case PeriodMonth:
		// Last 4 weeks (28 days)
		start := now.AddDate(0, 0, -28)
		return start, now
	case PeriodQuarter:
		// Last 3 months
		start := now.AddDate(0, -3, 0)
		return start, now
	case PeriodYear:
		// Last 12 months
		start := now.AddDate(-1, 0, 0)
		return start, now
	case PeriodAll:
		// All time (last 5 years as reasonable limit)
		start := now.AddDate(-5, 0, 0)
		return start, now
	default:
		// Default to last month
		start := now.AddDate(0, 0, -28)
		return start, now
	}
}
