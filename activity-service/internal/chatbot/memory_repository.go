package chatbot

import (
	"context"
	"sync"
	"time"
)

// memoryRepository implements Repository using in-memory storage
type memoryRepository struct {
	mu      sync.RWMutex
	entries map[string]ChatEntry
}

// NewMemoryRepository creates a new in-memory repository
func NewMemoryRepository() Repository {
	return &memoryRepository{
		entries: make(map[string]ChatEntry),
	}
}

func (r *memoryRepository) Create(ctx context.Context, entry ChatEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.entries[entry.ID]; exists {
		return ErrConflict
	}

	r.entries[entry.ID] = entry
	return nil
}

func (r *memoryRepository) GetByID(ctx context.Context, userID, entryID string) (ChatEntry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.entries[entryID]
	if !exists || entry.UserID != userID {
		return ChatEntry{}, ErrNotFound
	}

	return entry, nil
}

func (r *memoryRepository) Delete(ctx context.Context, userID, entryID string, deletedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.entries[entryID]
	if !exists || entry.UserID != userID {
		return ErrNotFound
	}

	entry.DeletedAt = &deletedAt
	r.entries[entryID] = entry
	return nil
}

func (r *memoryRepository) ListByUser(ctx context.Context, userID string, pagination Pagination) ([]ChatEntry, PageInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var userEntries []ChatEntry
	for _, entry := range r.entries {
		if entry.UserID == userID && entry.DeletedAt == nil {
			userEntries = append(userEntries, entry)
		}
	}

	// Simple pagination
	start := (pagination.Page - 1) * pagination.PageSize
	end := start + pagination.PageSize

	if start >= len(userEntries) {
		return []ChatEntry{}, PageInfo{
			Page:       pagination.Page,
			PageSize:   pagination.PageSize,
			TotalPages: 0,
			TotalItems: len(userEntries),
			HasNext:    false,
		}, nil
	}

	if end > len(userEntries) {
		end = len(userEntries)
	}

	pageEntries := userEntries[start:end]
	totalPages := (len(userEntries) + pagination.PageSize - 1) / pagination.PageSize

	return pageEntries, PageInfo{
		Page:       pagination.Page,
		PageSize:   pagination.PageSize,
		TotalPages: totalPages,
		TotalItems: len(userEntries),
		HasNext:    pagination.Page < totalPages,
	}, nil
}

func (r *memoryRepository) Update(ctx context.Context, entry ChatEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.entries[entry.ID]; !exists {
		return ErrNotFound
	}

	r.entries[entry.ID] = entry
	return nil
}
