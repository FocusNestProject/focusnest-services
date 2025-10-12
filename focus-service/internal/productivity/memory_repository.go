package productivity

import (
	"context"
	"sort"
	"sync"
	"time"
)

type memoryRepository struct {
	mu    sync.RWMutex
	store map[string]map[string]Entry // userID -> entryID -> Entry
}

// NewMemoryRepository returns an in-memory repository intended for local development and tests.
func NewMemoryRepository() Repository {
	return &memoryRepository{
		store: make(map[string]map[string]Entry),
	}
}

func (r *memoryRepository) Create(_ context.Context, entry Entry) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	userStore, ok := r.store[entry.UserID]
	if !ok {
		userStore = make(map[string]Entry)
		r.store[entry.UserID] = userStore
	}

	if _, exists := userStore[entry.ID]; exists {
		return ErrConflict
	}

	userStore[entry.ID] = entry
	return nil
}

func (r *memoryRepository) GetByID(_ context.Context, userID, entryID string) (Entry, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	userStore, ok := r.store[userID]
	if !ok {
		return Entry{}, ErrNotFound
	}

	entry, ok := userStore[entryID]
	if !ok || entry.DeletedAt != nil {
		return Entry{}, ErrNotFound
	}

	return entry, nil
}

func (r *memoryRepository) Delete(_ context.Context, userID, entryID string, deletedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	userStore, ok := r.store[userID]
	if !ok {
		return ErrNotFound
	}

	entry, ok := userStore[entryID]
	if !ok || entry.DeletedAt != nil {
		return ErrNotFound
	}

	entry.DeletedAt = &deletedAt
	entry.UpdatedAt = deletedAt
	userStore[entryID] = entry

	return nil
}

func (r *memoryRepository) ListByRange(_ context.Context, userID string, startInclusive, endExclusive time.Time, pagination Pagination) ([]Entry, PageInfo, error) {
	r.mu.RLock()
	snapshot := make([]Entry, 0)

	if userStore, ok := r.store[userID]; ok {
		for _, entry := range userStore {
			if entry.DeletedAt != nil {
				continue
			}

			anchor := entry.StartAt
			if anchor.IsZero() {
				anchor = entry.CreatedAt
			}

			if (anchor.Equal(startInclusive) || anchor.After(startInclusive)) && anchor.Before(endExclusive) {
				snapshot = append(snapshot, entry)
			}
		}
	}
	r.mu.RUnlock()

	sort.Slice(snapshot, func(i, j int) bool {
		return snapshot[i].StartAt.After(snapshot[j].StartAt)
	})

	pageSize := pagination.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	totalItems := len(snapshot)
	totalPages := totalItems / pageSize
	if totalItems%pageSize != 0 {
		totalPages++
	}
	if totalPages == 0 {
		totalPages = 1
	}

	// For simplicity in memory implementation, treat empty token as first page
	start := 0
	if pagination.Token != "" {
		// In a real implementation, decode token to get offset
		// Return empty if token is provided (simplified pagination)
		return []Entry{}, PageInfo{
			PageSize:   pageSize,
			TotalPages: totalPages,
			TotalItems: totalItems,
			HasNext:    false,
			NextToken:  "",
		}, nil
	}

	end := start + pageSize
	if end > totalItems {
		end = totalItems
	}

	items := make([]Entry, end-start)
	copy(items, snapshot[start:end])

	hasNext := end < totalItems
	nextToken := ""
	if hasNext {
		// In a real implementation, encode cursor position
		nextToken = "next-page-token"
	}

	return items, PageInfo{
		PageSize:   pageSize,
		TotalPages: totalPages,
		TotalItems: totalItems,
		HasNext:    hasNext,
		NextToken:  nextToken,
	}, nil
}
