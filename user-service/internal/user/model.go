package user

import (
	"time"
)

// UserProfile represents a user profile
type UserProfile struct {
	ID        string    `json:"id" firestore:"id"`
	UserID    string    `json:"user_id" firestore:"user_id"`
	Email     string    `json:"email" firestore:"email"`
	Name      string    `json:"name" firestore:"name"`
	Avatar    string    `json:"avatar" firestore:"avatar"`
	CreatedAt time.Time `json:"created_at" firestore:"created_at"`
	UpdatedAt time.Time `json:"updated_at" firestore:"updated_at"`
}

// UserStats represents user statistics
type UserStats struct {
	TotalSessions int `json:"total_sessions"`
	TotalTime     int `json:"total_time"` // in minutes
	Streak        int `json:"streak"`
}

// UserStreaks represents user streak information
type UserStreaks struct {
	CurrentStreak int       `json:"current_streak"`
	LongestStreak int       `json:"longest_streak"`
	LastActivity  time.Time `json:"last_activity"`
}

// Repository defines the interface for user data access
type Repository interface {
	GetProfile(userID string) (*UserProfile, error)
	UpdateProfile(profile *UserProfile) error
	GetStats(userID string) (*UserStats, error)
	GetStreaks(userID string) (*UserStreaks, error)
}

// Service defines the user service interface
type Service interface {
	GetProfile(userID string) (*UserProfile, error)
	UpdateProfile(userID string, updates map[string]interface{}) (*UserProfile, error)
	GetStats(userID string) (*UserStats, error)
	GetStreaks(userID string) (*UserStreaks, error)
}
