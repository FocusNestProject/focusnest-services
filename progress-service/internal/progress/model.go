package progress

import (
	"context"
	"time"
)

// DailySummary represents a daily progress summary
type DailySummary struct {
	// Do not persist Firestore doc id as a field
	ID         string         `json:"id" firestore:"-"`
	UserID     string         `json:"user_id" firestore:"user_id"`
	Date       time.Time      `json:"date" firestore:"date"`
	TotalTime  int            `json:"total_time" firestore:"total_time"` // minutes
	Categories map[string]int `json:"categories" firestore:"categories"` // minutes per category
	Sessions   int            `json:"sessions" firestore:"sessions"`     // number of sessions that day
	CreatedAt  time.Time      `json:"created_at" firestore:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at" firestore:"updated_at"`
}

// ProgressStats represents progress statistics
type ProgressStats struct {
	TotalTime     int                    `json:"total_time"` // minutes
	TotalSessions int                    `json:"total_sessions"`
	Categories    map[string]int         `json:"categories"` // minutes per category
	Periods       map[string]interface{} `json:"periods"`    // convenience buckets
}

// StreakData represents streak information
type StreakData struct {
	TotalStreak   int         `json:"total_streak"`   // longest (all-time) consecutive active days
	CurrentStreak int         `json:"current_streak"` // consecutive active days ending today (or last completed day)
	Days          []DayStatus `json:"days"`
}

// SummaryRange represents the supported summary windows.
type SummaryRange string

const (
	SummaryRangeWeek    SummaryRange = "week"
	SummaryRangeMonth   SummaryRange = "month"
	SummaryRangeQuarter SummaryRange = "3months"
	SummaryRangeYear    SummaryRange = "year"
)

// SummaryInput captures query parameters for the summary endpoint.
type SummaryInput struct {
	Range         SummaryRange
	Category      string
	ReferenceDate time.Time
}

// SummaryBucket represents a distribution bucket.
type SummaryBucket struct {
	Label       string `json:"label"`
	TimeElapsed int    `json:"time_elapsed"`
}

// SummaryResponse is returned by the summary endpoint.
type SummaryResponse struct {
	Range                   SummaryRange    `json:"range"`
	ReferenceDate           time.Time       `json:"reference_date"`
	Category                string          `json:"category,omitempty"`
	TotalFilteredTime       int             `json:"total_filtered_time"`
	TimeDistribution        []SummaryBucket `json:"time_distribution"`
	TotalSessions           int             `json:"total_sessions"`
	TotalTimeFrame          int             `json:"total_time_frame"`
	MostProductiveHourStart *time.Time      `json:"most_productive_hour_start"`
	MostProductiveHourEnd   *time.Time      `json:"most_productive_hour_end"`
}

// ProductivityEntry represents a raw productivity session used for analytics.
type ProductivityEntry struct {
	StartTime   time.Time
	EndTime     time.Time
	TimeElapsed int
	Category    string
}

// MonthlyStreakData represents monthly streak data
type MonthlyStreakData struct {
	Month         int         `json:"month"`
	Year          int         `json:"year"`
	TotalStreak   int         `json:"total_streak"`
	CurrentStreak int         `json:"current_streak"`
	Days          []DayStatus `json:"days"`
}

// WeeklyStreakData represents weekly streak data
type WeeklyStreakData struct {
	Week          string      `json:"week"` // YYYY-WW format
	TotalStreak   int         `json:"total_streak"`
	CurrentStreak int         `json:"current_streak"`
	Days          []DayStatus `json:"days"`
}

// DayStatus represents the status of a single day
type DayStatus struct {
	Date   string `json:"date"`   // YYYY-MM-DD
	Day    string `json:"day"`    // Monday, Tuesday, ...
	Status string `json:"status"` // active, skipped, upcoming
}

// Repository defines the interface for progress data access
type Repository interface {
	GetDailySummaries(ctx context.Context, userID string, startDate, endDate time.Time) ([]*DailySummary, error)
	GetProgressStats(ctx context.Context, userID string, startDate, endDate time.Time) (*ProgressStats, error)
	ListProductivities(ctx context.Context, userID string, startDate, endDate time.Time) ([]ProductivityEntry, error)
}

// Service defines the progress service interface
type Service interface {
	GetProgress(ctx context.Context, userID string, startDate, endDate time.Time) (*ProgressStats, error)
	GetMonthlyStreak(ctx context.Context, userID string, month, year int) (*MonthlyStreakData, error)
	GetWeeklyStreak(ctx context.Context, userID string, targetDate time.Time) (*WeeklyStreakData, error)
	GetCurrentStreak(ctx context.Context, userID string) (*StreakData, error)
	GetSummary(ctx context.Context, userID string, input SummaryInput) (*SummaryResponse, error)
}
