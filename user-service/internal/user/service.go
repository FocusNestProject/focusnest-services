package user

import (
	"fmt"
	"time"
)

type service struct {
	repo Repository
}

// NewService creates a new user service
func NewService(repo Repository) Service {
	return &service{repo: repo}
}

func (s *service) GetProfile(userID string) (*UserProfile, error) {
	return s.repo.GetProfile(userID)
}

func (s *service) UpdateProfile(userID string, updates map[string]interface{}) (*UserProfile, error) {
	profile, err := s.repo.GetProfile(userID)
	if err != nil {
		return nil, fmt.Errorf("get profile: %w", err)
	}

	// Apply updates
	if name, ok := updates["name"].(string); ok {
		profile.Name = name
	}
	if email, ok := updates["email"].(string); ok {
		profile.Email = email
	}
	if avatar, ok := updates["avatar"].(string); ok {
		profile.Avatar = avatar
	}

	profile.UpdatedAt = time.Now()

	if err := s.repo.UpdateProfile(profile); err != nil {
		return nil, fmt.Errorf("update profile: %w", err)
	}

	return profile, nil
}

func (s *service) GetStats(userID string) (*UserStats, error) {
	return s.repo.GetStats(userID)
}

func (s *service) GetStreaks(userID string) (*UserStreaks, error) {
	return s.repo.GetStreaks(userID)
}
