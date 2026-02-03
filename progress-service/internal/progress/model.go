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

// StreakStatus is the user's streak state for recovery/grace logic.
const (
	StreakStatusActive  = "active"
	StreakStatusGrace   = "grace"
	StreakStatusExpired = "expired"
)

// StreakData represents streak information
type StreakData struct {
	TotalStreak            int         `json:"total_streak"`              // longest (all-time) consecutive active days
	CurrentStreak          int         `json:"current_streak"`             // consecutive active days ending today (or last completed day)
	Days                   []DayStatus `json:"days"`
	Status                 string      `json:"status"`                     // active | grace | expired
	GraceEndsAt            string      `json:"grace_ends_at,omitempty"`   // YYYY-MM-DD; only when status=grace (last day user can recover)
	ExpiredAt              string      `json:"expired_at,omitempty"`       // YYYY-MM-DD; when streak expired
	RecoveryUsedThisMonth  int         `json:"recovery_used_this_month"`  // premium: recoveries used in current month
	RecoveryQuotaPerMonth  int         `json:"recovery_quota_per_month"`  // premium: 5
}

// StreakState is persisted per user for expired/grace and recovery override.
type StreakState struct {
	UserID                 string `firestore:"user_id"`
	ExpiredAt              string `firestore:"expired_at"`               // YYYY-MM-DD when streak expired
	StreakValueBeforeExpired int   `firestore:"streak_value_before_expired"`
	OverrideStreakValue    int    `firestore:"override_streak_value"`    // after recovery, show this until next activity
	UpdatedAt              time.Time `firestore:"updated_at"`
}

// RecoveryQuota is recovery count per user per month (reset on 1st).
type RecoveryQuota struct {
	UserID    string    `firestore:"user_id"`
	YearMonth string    `firestore:"year_month"` // e.g. "2026-02"
	Count     int       `firestore:"count"`
	UpdatedAt time.Time `firestore:"updated_at"`
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
	GetStreakState(ctx context.Context, userID string) (*StreakState, error)
	SetStreakState(ctx context.Context, userID string, state *StreakState) error
	GetRecoveryQuota(ctx context.Context, userID string, yearMonth string) (int, error)
	IncrementRecoveryQuota(ctx context.Context, userID string, yearMonth string) (int, error)
}

// Service defines the progress service interface
type Service interface {
	GetProgress(ctx context.Context, userID string, startDate, endDate time.Time) (*ProgressStats, error)
	GetMonthlyStreak(ctx context.Context, userID string, month, year int) (*MonthlyStreakData, error)
	GetWeeklyStreak(ctx context.Context, userID string, targetDate time.Time) (*WeeklyStreakData, error)
	GetCurrentStreak(ctx context.Context, userID string, timezone string) (*StreakData, error)
	RecoverStreak(ctx context.Context, userID string, isPremium bool, timezone string) (*StreakData, error)
	GetSummary(ctx context.Context, userID string, input SummaryInput) (*SummaryResponse, error)
}
