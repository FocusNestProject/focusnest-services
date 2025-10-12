package productivity

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Entry is a single productivity event captured by the user.
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

// ImageInfo stores optional image metadata for an entry.
type ImageInfo struct {
	OriginalURL string `json:"original_url"`
	OverviewURL string `json:"overview_url"`
}

// ValidCategories defines the allowed productivity categories.
var ValidCategories = []string{
	"Work",
	"Study",
	"Reading",
	"Journaling",
	"Cooking",
	"Workout",
	"Other",
}

// ValidTimeModes defines the allowed time modes.
var ValidTimeModes = []string{
	"Pomodoro",    // 25 min work, 5 min break
	"QuickFocus",  // 15 min work, 5 min break
	"FreeTimer",   // custom timer, can be stopped anytime
	"CustomTimer", // user-defined timer
}

// ValidMoods defines the allowed mood options.
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

// ListInput captures query parameters for listing entries.
type ListInput struct {
	UserID    string
	PageSize  int
	PageToken string
	Month     *int
	Year      *int
}

// MonthHistoryInput captures parameters for monthly history.
type MonthHistoryInput struct {
	UserID string
	Month  int
	Year   int
}

// DayStatus represents the status of a single day in monthly history.
type DayStatus struct {
	Date           string `json:"date"`   // YYYY-MM-DD format
	Status         string `json:"status"` // active, skipped, today, upcoming
	TotalElapsedMs int    `json:"total_elapsed_ms"`
	Sessions       int    `json:"sessions"`
}

// MonthHistoryResponse represents the response for monthly history.
type MonthHistoryResponse struct {
	Month int         `json:"month"`
	Year  int         `json:"year"`
	Days  []DayStatus `json:"days"`
}

// ListResponse represents a paginated list response.
type ListResponse struct {
	Data     []Entry  `json:"data"`
	PageInfo PageInfo `json:"pageInfo"`
}

// ImageUploadResponse represents the response for image upload.
type ImageUploadResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
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

// Pagination describes cursor-based paging preferences for list queries.
// Page numbers are dropped in favor of an opaque page token.
type Pagination struct {
	// PageSize is the maximum number of items to return.
	PageSize int `json:"pageSize"`

	// Token is an opaque cursor produced by a previous response's PageInfo.NextToken.
	// Leave empty for the first page.
	Token string `json:"pageToken,omitempty"`
}

// PageInfo summarizes pagination metadata for responses (cursor-based).
type PageInfo struct {
	// PageSize echoes the requested/used page size for this page.
	PageSize int `json:"pageSize"`

	// TotalPages is computed from the total item count and page size.
	// (Useful for UI even with cursors; computed via server-side COUNT aggregation.)
	TotalPages int `json:"totalPages"`

	// TotalItems is the total number of items matching the query.
	TotalItems int `json:"totalItems"`

	// HasNext indicates if there is at least one more page.
	HasNext bool `json:"hasNext"`

	// NextToken is the opaque cursor for fetching the next page.
	// Empty when there are no more results.
	NextToken string `json:"nextToken,omitempty"`
}

// Repository encapsulates persistence for productivity entries.
type Repository interface {
	Create(ctx context.Context, entry Entry) error
	GetByID(ctx context.Context, userID, entryID string) (Entry, error)
	Delete(ctx context.Context, userID, entryID string, deletedAt time.Time) error
	ListByRange(ctx context.Context, userID string, startInclusive, endExclusive time.Time, pagination Pagination) ([]Entry, PageInfo, error)
}

// Domain errors.
var (
	// ErrNotFound indicates the requested entry does not exist for the user.
	ErrNotFound = errors.New("productivity entry not found")

	// ErrConflict indicates a duplicate identifier collision.
	ErrConflict = errors.New("productivity entry already exists")

	// ErrInvalidInput indicates the provided data failed validation.
	ErrInvalidInput = errors.New("invalid input")
)

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

// Delete removes a productivity entry (soft delete in repository).
func (s *Service) Delete(ctx context.Context, userID, entryID string) error {
	if userID == "" || entryID == "" {
		return ErrNotFound
	}
	return s.repo.Delete(ctx, userID, entryID, s.clock.Now().UTC())
}

// ListMonth returns entries for the month containing the provided anchor time.
// The repository applies time-window filtering on a canonical "anchor" field.
func (s *Service) ListMonth(ctx context.Context, userID string, anchor time.Time, pagination Pagination) ([]Entry, PageInfo, error) {
	if userID == "" {
		return nil, PageInfo{}, ErrNotFound
	}

	// Month boundaries in UTC (repository uses them on "anchor").
	monthStart := time.Date(anchor.Year(), anchor.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)

	return s.repo.ListByRange(ctx, userID, monthStart, monthEnd, pagination)
}

// List returns entries based on the provided list input.
func (s *Service) List(ctx context.Context, input ListInput) (ListResponse, error) {
	if input.UserID == "" {
		return ListResponse{}, ErrNotFound
	}

	// Set defaults
	if input.PageSize <= 0 {
		input.PageSize = 20
	}
	if input.PageSize > 100 {
		input.PageSize = 100
	}

	pagination := Pagination{
		PageSize: input.PageSize,
		Token:    input.PageToken,
	}

	// Handle month/year filtering
	var startTime, endTime time.Time
	if input.Month != nil && input.Year != nil {
		startTime = time.Date(*input.Year, time.Month(*input.Month), 1, 0, 0, 0, 0, time.UTC)
		endTime = startTime.AddDate(0, 1, 0)
	} else {
		// Default to all time
		startTime = time.Time{}
		endTime = s.clock.Now().UTC().Add(24 * time.Hour)
	}

	entries, pageInfo, err := s.repo.ListByRange(ctx, input.UserID, startTime, endTime, pagination)
	if err != nil {
		return ListResponse{}, err
	}

	return ListResponse{
		Data:     entries,
		PageInfo: pageInfo,
	}, nil
}

// GetMonthHistory returns daily productivity summary for the specified month.
func (s *Service) GetMonthHistory(ctx context.Context, input MonthHistoryInput) (MonthHistoryResponse, error) {
	if input.UserID == "" {
		return MonthHistoryResponse{}, ErrNotFound
	}

	// Set defaults to current month/year if not provided
	now := s.clock.Now().UTC()
	if input.Month == 0 {
		input.Month = int(now.Month())
	}
	if input.Year == 0 {
		input.Year = now.Year()
	}

	// Get all entries for the month
	monthStart := time.Date(input.Year, time.Month(input.Month), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)

	entries, _, err := s.repo.ListByRange(ctx, input.UserID, monthStart, monthEnd, Pagination{PageSize: 1000})
	if err != nil {
		return MonthHistoryResponse{}, err
	}

	// Group entries by day
	dayMap := make(map[string]*DayStatus)

	// Initialize all days in the month
	daysInMonth := time.Date(input.Year, time.Month(input.Month+1), 0, 0, 0, 0, 0, time.UTC).Day()
	for day := 1; day <= daysInMonth; day++ {
		date := time.Date(input.Year, time.Month(input.Month), day, 0, 0, 0, 0, time.UTC)
		dateStr := date.Format("2006-01-02")

		status := "upcoming"
		if date.Before(now.Truncate(24 * time.Hour)) {
			status = "active"
		} else if date.Equal(now.Truncate(24 * time.Hour)) {
			status = "today"
		}

		dayMap[dateStr] = &DayStatus{
			Date:           dateStr,
			Status:         status,
			TotalElapsedMs: 0,
			Sessions:       0,
		}
	}

	// Aggregate entries by day
	for _, entry := range entries {
		dayStr := entry.StartAt.Format("2006-01-02")
		if dayStatus, exists := dayMap[dayStr]; exists {
			dayStatus.TotalElapsedMs += entry.ElapsedMs
			dayStatus.Sessions++
		}
	}

	// Convert to slice
	days := make([]DayStatus, 0, len(dayMap))
	for day := 1; day <= daysInMonth; day++ {
		date := time.Date(input.Year, time.Month(input.Month), day, 0, 0, 0, 0, time.UTC)
		dateStr := date.Format("2006-01-02")
		if dayStatus, exists := dayMap[dateStr]; exists {
			days = append(days, *dayStatus)
		}
	}

	return MonthHistoryResponse{
		Month: input.Month,
		Year:  input.Year,
		Days:  days,
	}, nil
}

// UploadImage handles image upload for an existing entry.
func (s *Service) UploadImage(ctx context.Context, userID, entryID string, imageData []byte, filename string) (ImageUploadResponse, error) {
	if userID == "" || entryID == "" {
		return ImageUploadResponse{}, ErrNotFound
	}

	// Verify entry exists
	_, err := s.repo.GetByID(ctx, userID, entryID)
	if err != nil {
		return ImageUploadResponse{}, err
	}

	// Image upload is handled by the storage service
	return ImageUploadResponse{
		Success: true,
		Message: "Image uploaded, overview generation queued.",
	}, nil
}

// RetryImageOverview triggers overview regeneration for an entry.
func (s *Service) RetryImageOverview(ctx context.Context, userID, entryID string) (ImageUploadResponse, error) {
	if userID == "" || entryID == "" {
		return ImageUploadResponse{}, ErrNotFound
	}

	// Verify entry exists
	_, err := s.repo.GetByID(ctx, userID, entryID)
	if err != nil {
		return ImageUploadResponse{}, err
	}

	// Overview generation will be triggered by the media worker service
	return ImageUploadResponse{
		Success: true,
		Message: "Overview generation re-triggered.",
	}, nil
}
