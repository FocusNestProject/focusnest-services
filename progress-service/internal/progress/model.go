package progress

import (
	"context"
	"time"
)

// DailySummary represents a daily progress summary
type DailySummary struct {
	ID         string         `json:"id" firestore:"id"`
	UserID     string         `json:"user_id" firestore:"user_id"`
	Date       time.Time      `json:"date" firestore:"date"`
	TotalTime  int            `json:"total_time" firestore:"total_time"` // in minutes
	Categories map[string]int `json:"categories" firestore:"categories"`
	CreatedAt  time.Time      `json:"created_at" firestore:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at" firestore:"updated_at"`
}

// ProgressStats represents progress statistics
type ProgressStats struct {
	TotalTime     int                    `json:"total_time"`
	TotalSessions int                    `json:"total_sessions"`
	Categories    map[string]int         `json:"categories"`
	Periods       map[string]interface{} `json:"periods"`
}

// StreakData represents streak information
type StreakData struct {
	TotalStreak   int         `json:"total_streak"`
	CurrentStreak int         `json:"current_streak"`
	Days          []DayStatus `json:"days"`
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
	Date   string `json:"date"`   // YYYY-MM-DD format
	Day    string `json:"day"`    // Monday, Tuesday, etc.
	Status string `json:"status"` // active, skipped, upcoming
}

// Repository defines the interface for progress data access
type Repository interface {
	GetDailySummaries(userID string, startDate, endDate time.Time) ([]*DailySummary, error)
	GetProgressStats(userID string, startDate, endDate time.Time) (*ProgressStats, error)
}

// Service defines the progress service interface
type Service interface {
	GetProgress(userID string, startDate, endDate time.Time) (*ProgressStats, error)
	GetMonthlyStreak(ctx context.Context, userID string, month, year int) (*MonthlyStreakData, error)
	GetWeeklyStreak(ctx context.Context, userID string, targetDate time.Time) (*WeeklyStreakData, error)
	GetCurrentStreak(ctx context.Context, userID string) (*StreakData, error)
}
