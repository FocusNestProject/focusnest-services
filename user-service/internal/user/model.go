package user

import (
	"context"
	"time"
)

// Profile represents the persisted profile document stored in Firestore.
type Profile struct {
	UserID    string     `json:"user_id" firestore:"user_id"`
	Bio       string     `json:"bio" firestore:"bio"`
	Birthdate *time.Time `json:"birthdate" firestore:"birthdate"`
	CreatedAt time.Time  `json:"created_at" firestore:"created_at"`
	UpdatedAt time.Time  `json:"updated_at" firestore:"updated_at"`
}

// ProfileMetadata captures derived counters that accompany a profile response.
type ProfileMetadata struct {
	LongestStreak       int `json:"longest_streak"`
	TotalProductivities int `json:"total_productivities"`
	TotalSessions       int `json:"total_sessions"`
}

// ProfileResponse combines persisted profile fields with derived metadata.
type ProfileResponse struct {
	UserID    string     `json:"user_id"`
	Bio       string     `json:"bio"`
	Birthdate *time.Time `json:"birthdate"`
	ProfileMetadata
	CreatedAt time.Time `json:"created_at,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// ProfileUpdateInput describes the allowed fields during a PATCH request.
type ProfileUpdateInput struct {
	Bio       *string
	Birthdate *BirthdatePatch
}

// BirthdatePatch differentiates between omitted and explicit null updates.
type BirthdatePatch struct {
	IsSet bool
	Value *time.Time
}

// Repository defines the interface for user data access.
type Repository interface {
	GetProfile(ctx context.Context, userID string) (*Profile, error)
	UpsertProfile(ctx context.Context, userID string, updates ProfileUpdateInput) (*Profile, error)
	GetProfileMetadata(ctx context.Context, userID string) (ProfileMetadata, error)
}

// Service defines the user service interface.
type Service interface {
	GetProfile(ctx context.Context, userID string) (*ProfileResponse, error)
	UpdateProfile(ctx context.Context, userID string, updates ProfileUpdateInput) (*ProfileResponse, error)
}
