package user

import (
	"context"
	"sync"
	"time"
)

// memoryRepository implements Repository using in-memory storage
type memoryRepository struct {
	mu       sync.RWMutex
	profiles map[string]Profile
}

// NewMemoryRepository creates a new in-memory repository
func NewMemoryRepository() Repository {
	return &memoryRepository{
		profiles: make(map[string]Profile),
	}
}

func (r *memoryRepository) GetByUserID(ctx context.Context, userID string) (Profile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	profile, exists := r.profiles[userID]
	if !exists {
		return Profile{}, ErrNotFound
	}

	return profile, nil
}

func (r *memoryRepository) Create(ctx context.Context, profile Profile) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.profiles[profile.UserID]; exists {
		return ErrConflict
	}

	r.profiles[profile.UserID] = profile
	return nil
}

func (r *memoryRepository) Update(ctx context.Context, profile Profile) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.profiles[profile.UserID]; !exists {
		return ErrNotFound
	}

	r.profiles[profile.UserID] = profile
	return nil
}

func (r *memoryRepository) Delete(ctx context.Context, userID string, deletedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	profile, exists := r.profiles[userID]
	if !exists {
		return ErrNotFound
	}

	profile.DeletedAt = &deletedAt
	r.profiles[userID] = profile
	return nil
}
