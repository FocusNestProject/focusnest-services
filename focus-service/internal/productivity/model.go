package productivity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Entry struct {
	ID          string     `json:"id"`
	UserID      string     `json:"user_id"`
	Category    string     `json:"category"`
	TimeMode    string     `json:"time_mode"`
	Description string     `json:"description,omitempty"`
	Mood        string     `json:"mood,omitempty"`
	Cycles      int        `json:"cycles"`
	ElapsedMs   int        `json:"elapsed_ms"`
	StartAt     time.Time  `json:"start_at"`
	EndAt       time.Time  `json:"end_at"`
	Image       *ImageInfo `json:"image,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   *time.Time `json:"-"`
}

type ImageInfo struct {
	OriginalPath string `json:"original_path"`
	OverviewPath string `json:"overview_path"`
	OriginalURL  string `json:"original_url"`
	OverviewURL  string `json:"overview_url"`
}

// ValidCategories defines the allowed productivity categories
var ValidCategories = []string{
	"Work",
	"Study",
	"Reading",
	"Journaling",
	"Cooking",
	"Workout",
	"Other",
}

// ValidTimeModes defines the allowed time modes
var ValidTimeModes = []string{
	"Pomodoro",    // 25 min work, 5 min break
	"QuickFocus",  // 15 min work, 5 min break
	"FreeTimer",   // custom timer, can be stopped anytime
	"CustomTimer", // user-defined timer
}

// ValidMoods defines the allowed mood options
var ValidMoods = []string{
	"excited",
	"focused",
	"calm",
	"energetic",
	"tired",
	"motivated",
	"stressed",
	"relaxed",
}

// CreateInput captures the data required to create a new entry.
type CreateInput struct {
	UserID      string
	Category    string
	TimeMode    string
	Description string
	Mood        string
	Cycles      int
	ElapsedMs   int
	StartAt     *time.Time
	EndAt       *time.Time
	Image       *ImageInfo
}

// Validate ensures the input fields meet the domain constraints.
func (i CreateInput) Validate() error {
	var problems []string

	if i.UserID == "" {
		problems = append(problems, "user_id is required")
	}
	if strings.TrimSpace(i.Category) == "" {
		problems = append(problems, "category is required")
	}
	if i.ElapsedMs <= 0 {
		problems = append(problems, "elapsed_ms must be greater than 0")
	}
	if i.Cycles < 0 {
		problems = append(problems, "cycles must be non-negative")
	}
	if i.StartAt != nil && i.EndAt != nil && i.EndAt.Before(*i.StartAt) {
		problems = append(problems, "end_at must be on or after start_at")
	}

	// Validate category
	if i.Category != "" {
		validCategory := false
		for _, cat := range ValidCategories {
			if cat == i.Category {
				validCategory = true
				break
			}
		}
		if !validCategory {
			problems = append(problems, fmt.Sprintf("category must be one of: %s", strings.Join(ValidCategories, ", ")))
		}
	}

	// Validate time mode
	if i.TimeMode != "" {
		validMode := false
		for _, mode := range ValidTimeModes {
			if mode == i.TimeMode {
				validMode = true
				break
			}
		}
		if !validMode {
			problems = append(problems, fmt.Sprintf("time_mode must be one of: %s", strings.Join(ValidTimeModes, ", ")))
		}
	}

	// Validate mood
	if i.Mood != "" {
		validMood := false
		for _, mood := range ValidMoods {
			if mood == i.Mood {
				validMood = true
				break
			}
		}
		if !validMood {
			problems = append(problems, fmt.Sprintf("mood must be one of: %s", strings.Join(ValidMoods, ", ")))
		}
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// Pagination describes paging preferences for list queries.
type Pagination struct {
	Page     int
	PageSize int
}

// PageInfo summarizes pagination metadata for responses.
type PageInfo struct {
	Page       int  `json:"page"`
	PageSize   int  `json:"pageSize"`
	TotalPages int  `json:"totalPages"`
	TotalItems int  `json:"totalItems"`
	HasNext    bool `json:"hasNext"`
}

// Repository encapsulates persistence for productivity entries.
type Repository interface {
	Create(ctx context.Context, entry Entry) error
	GetByID(ctx context.Context, userID, entryID string) (Entry, error)
	Delete(ctx context.Context, userID, entryID string, deletedAt time.Time) error
	ListByRange(ctx context.Context, userID string, startInclusive, endExclusive time.Time, pagination Pagination) ([]Entry, PageInfo, error)
}

// ErrNotFound indicates the requested entry does not exist for the user.
var ErrNotFound = errors.New("productivity entry not found")

// ErrConflict indicates a duplicate identifier collision.
var ErrConflict = errors.New("productivity entry already exists")

// ErrInvalidInput indicates the provided data failed validation.
var ErrInvalidInput = errors.New("invalid input")

// Clock delivers the current time; extracted for deterministic testing.
type Clock interface {
	Now() time.Time
}

// IDGenerator produces unique identifiers for new entries.
type IDGenerator interface {
	NewID() string
}

// Service orchestrates the domain operations for productivity entries.
type Service struct {
	repo  Repository
	clock Clock
	ids   IDGenerator
}

// NewService constructs a Service instance with the provided collaborators.
func NewService(repo Repository, clock Clock, ids IDGenerator) (*Service, error) {
	if repo == nil {
		return nil, errors.New("repo is required")
	}
	if clock == nil {
		return nil, errors.New("clock is required")
	}
	if ids == nil {
		return nil, errors.New("id generator is required")
	}
	return &Service{repo: repo, clock: clock, ids: ids}, nil
}

// Create registers a new productivity entry for the given user.
func (s *Service) Create(ctx context.Context, input CreateInput) (Entry, error) {
	if err := input.Validate(); err != nil {
		return Entry{}, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}

	now := s.clock.Now().UTC()
	startAt := now
	if input.StartAt != nil {
		startAt = input.StartAt.UTC()
	}
	endAt := startAt.Add(time.Duration(input.ElapsedMs) * time.Millisecond)
	if input.EndAt != nil {
		endAt = input.EndAt.UTC()
	}

	entry := Entry{
		ID:          s.ids.NewID(),
		UserID:      input.UserID,
		Category:    strings.TrimSpace(input.Category),
		TimeMode:    strings.TrimSpace(input.TimeMode),
		Description: strings.TrimSpace(input.Description),
		Mood:        strings.TrimSpace(input.Mood),
		Cycles:      input.Cycles,
		ElapsedMs:   input.ElapsedMs,
		StartAt:     startAt,
		EndAt:       endAt,
		Image:       input.Image,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := s.repo.Create(ctx, entry); err != nil {
		return Entry{}, err
	}

	return entry, nil
}

// Get retrieves a single productivity entry by its ID for the provided user.
func (s *Service) Get(ctx context.Context, userID, entryID string) (Entry, error) {
	if userID == "" || entryID == "" {
		return Entry{}, ErrNotFound
	}
	return s.repo.GetByID(ctx, userID, entryID)
}

// Delete removes a productivity entry.
func (s *Service) Delete(ctx context.Context, userID, entryID string) error {
	if userID == "" || entryID == "" {
		return ErrNotFound
	}
	return s.repo.Delete(ctx, userID, entryID, s.clock.Now().UTC())
}

// ListMonth returns entries for the month containing the provided anchor time.
func (s *Service) ListMonth(ctx context.Context, userID string, anchor time.Time, pagination Pagination) ([]Entry, PageInfo, error) {
	if userID == "" {
		return nil, PageInfo{}, ErrNotFound
	}

	monthStart := time.Date(anchor.Year(), anchor.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)

	return s.repo.ListByRange(ctx, userID, monthStart, monthEnd, pagination)
}
