package progress

import (
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

// Repository defines the interface for progress data access
type Repository interface {
	GetDailySummaries(userID string, startDate, endDate time.Time) ([]*DailySummary, error)
	GetProgressStats(userID string, startDate, endDate time.Time) (*ProgressStats, error)
}

// Service defines the progress service interface
type Service interface {
	GetProgress(userID string, startDate, endDate time.Time) (*ProgressStats, error)
}
