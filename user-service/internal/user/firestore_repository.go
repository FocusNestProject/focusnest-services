package user

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
)

type firestoreRepository struct {
	client *firestore.Client
}

// NewFirestoreRepository creates a new Firestore repository
func NewFirestoreRepository(client *firestore.Client) Repository {
	return &firestoreRepository{client: client}
}

func (r *firestoreRepository) GetProfile(userID string) (*UserProfile, error) {
	ctx := context.Background()
	doc, err := r.client.Collection("users").Doc(userID).Get(ctx)
	if err != nil {
		return nil, err
	}

	var profile UserProfile
	if err := doc.DataTo(&profile); err != nil {
		return nil, fmt.Errorf("unmarshal profile: %w", err)
	}
	profile.ID = doc.Ref.ID
	profile.UserID = userID

	return &profile, nil
}

func (r *firestoreRepository) UpdateProfile(profile *UserProfile) error {
	ctx := context.Background()
	profile.UpdatedAt = time.Now()
	_, err := r.client.Collection("users").Doc(profile.UserID).Set(ctx, profile)
	return err
}

func (r *firestoreRepository) GetStats(userID string) (*UserStats, error) {
	// This would typically aggregate data from activities
	// Return mock data for development
	return &UserStats{
		TotalSessions: 42,
		TotalTime:     1200, // 20 hours
		Streak:        7,
	}, nil
}

func (r *firestoreRepository) GetStreaks(userID string) (*UserStreaks, error) {
	// This would typically calculate streaks from activity data
	// Return mock data for development
	return &UserStreaks{
		CurrentStreak: 7,
		LongestStreak: 15,
		LastActivity:  time.Now().Add(-24 * time.Hour),
	}, nil
}
