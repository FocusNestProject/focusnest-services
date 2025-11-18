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
	ID           string     `json:"id"`
	UserID       string     `json:"user_id"`
	ActivityName string     `json:"activity_name"`
	TimeElapsed  int        `json:"time_elapsed"`
	NumCycle     int        `json:"num_cycle"`
	TimeMode     string     `json:"time_mode"`
	Category     string     `json:"category"`
	Description  string     `json:"description,omitempty"`
	Mood         string     `json:"mood,omitempty"`
	Image        string     `json:"image,omitempty"`
	StartTime    time.Time  `json:"start_time"`
	EndTime      time.Time  `json:"end_time"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeletedAt    *time.Time `json:"-"`
}

// ValidCategories defines the allowed productivity categories.
var ValidCategories = []string{
	"Work",
	"Study",
	"Read",
	"Journal",
	"Cook",
	"Workout",
	"Music",
	"Other",
}

// ValidTimeModes defines the allowed time modes.
var ValidTimeModes = []string{
	"Pomodoro",
	"Deep Work",
	"Quick Focus",
	"Free Timer",
	"Other",
}

// ValidMoods defines the allowed mood options.
var ValidMoods = []string{
	"Fokus",
	"Semangat",
	"Biasa Aja",
	"Capek",
	"Burn Out",
	"Mengantuk",
}

// CreateInput captures the data required to create a new entry.
type CreateInput struct {
	UserID       string
	ActivityName string
	TimeElapsed  int
	NumCycle     int
	TimeMode     string
	Category     string
	Description  string
	Mood         string
	Image        string
	StartTime    time.Time
	EndTime      time.Time
}

// PatchInput captures partial updates for an entry.
type PatchInput struct {
	ActivityName *string
	TimeElapsed  *int
	NumCycle     *int
	TimeMode     *string
	Category     *string
	Description  *string
	Mood         *string
	Image        *string
	StartTime    *time.Time
	EndTime      *time.Time
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
	Date                string `json:"date"`   // YYYY-MM-DD format
	Status              string `json:"status"` // active, skipped, today, upcoming
	TotalElapsedSeconds int    `json:"total_elapsed_seconds"`
	Sessions            int    `json:"sessions"`
}

// MonthHistoryResponse represents the response for monthly history.
type MonthHistoryResponse struct {
	Month int         `json:"month"`
	Year  int         `json:"year"`
	Days  []DayStatus `json:"days"`
}

// ListItem is a lightweight projection returned by the list endpoint.
type ListItem struct {
	ID          string    `json:"id"`
	Image       string    `json:"image"`
	Category    string    `json:"category"`
	TimeElapsed int       `json:"time_elapsed"`
	NumCycle    int       `json:"num_cycle"`
	TimeMode    string    `json:"time_mode"`
	StartTime   time.Time `json:"start_time"`
}

// ListResponse represents a paginated list response.
type ListResponse struct {
	Items    []ListItem `json:"items"`
	PageInfo PageInfo   `json:"pageInfo"`
}

// Validate ensures the input fields meet the domain constraints.
func (i CreateInput) Validate() error {
	var problems []string

	if i.UserID == "" {
		problems = append(problems, "user_id is required")
	}
	if strings.TrimSpace(i.ActivityName) == "" {
		problems = append(problems, "activity_name is required")
	}
	if i.TimeElapsed <= 0 {
		problems = append(problems, "time_elapsed must be greater than 0")
	}
	if i.NumCycle <= 0 {
		problems = append(problems, "num_cycle must be greater than 0")
	}
	if i.StartTime.IsZero() {
		problems = append(problems, "start_time is required")
	}
	if i.EndTime.IsZero() {
		problems = append(problems, "end_time is required")
	}
	if !i.EndTime.IsZero() && i.EndTime.Before(i.StartTime) {
		problems = append(problems, "end_time must be on or after start_time")
	}

	// Validate category
	trimmedCategory := strings.TrimSpace(i.Category)
	if trimmedCategory == "" {
		problems = append(problems, "category is required")
	} else {
		validCategory := false
		for _, cat := range ValidCategories {
			if cat == trimmedCategory {
				validCategory = true
				break
			}
		}
		if !validCategory {
			problems = append(problems, fmt.Sprintf("category must be one of: %s", strings.Join(ValidCategories, ", ")))
		}
	}

	// Validate time mode
	trimmedMode := strings.TrimSpace(i.TimeMode)
	if trimmedMode == "" {
		problems = append(problems, "time_mode is required")
	} else {
		validMode := false
		for _, mode := range ValidTimeModes {
			if mode == trimmedMode {
				validMode = true
				break
			}
		}
		if !validMode {
			problems = append(problems, fmt.Sprintf("time_mode must be one of: %s", strings.Join(ValidTimeModes, ", ")))
		}
	}

	// Validate mood
	if trimmed := strings.TrimSpace(i.Mood); trimmed != "" {
		validMood := false
		for _, mood := range ValidMoods {
			if mood == trimmed {
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

// Apply validates and mutates an entry using a patch input. Service layer uses this
// to reuse validation logic.
func (p PatchInput) Apply(e Entry) (Entry, error) {
	if p.ActivityName != nil {
		e.ActivityName = strings.TrimSpace(*p.ActivityName)
	}
	if p.TimeElapsed != nil {
		e.TimeElapsed = *p.TimeElapsed
	}
	if p.NumCycle != nil {
		e.NumCycle = *p.NumCycle
	}
	if p.TimeMode != nil {
		e.TimeMode = strings.TrimSpace(*p.TimeMode)
	}
	if p.Category != nil {
		e.Category = strings.TrimSpace(*p.Category)
	}
	if p.Description != nil {
		e.Description = strings.TrimSpace(*p.Description)
	}
	if p.Mood != nil {
		e.Mood = strings.TrimSpace(*p.Mood)
	}
	if p.Image != nil {
		e.Image = strings.TrimSpace(*p.Image)
	}
	if p.StartTime != nil {
		e.StartTime = p.StartTime.UTC()
	}
	if p.EndTime != nil {
		e.EndTime = p.EndTime.UTC()
	}
	ci := CreateInput{
		UserID:       e.UserID,
		ActivityName: e.ActivityName,
		TimeElapsed:  e.TimeElapsed,
		NumCycle:     e.NumCycle,
		TimeMode:     e.TimeMode,
		Category:     e.Category,
		Description:  e.Description,
		Mood:         e.Mood,
		Image:        e.Image,
		StartTime:    e.StartTime,
		EndTime:      e.EndTime,
	}
	if err := ci.Validate(); err != nil {
		return Entry{}, err
	}
	return e, nil
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
	Update(ctx context.Context, entry Entry) error
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
	entry := Entry{
		ID:           s.ids.NewID(),
		UserID:       input.UserID,
		ActivityName: strings.TrimSpace(input.ActivityName),
		TimeElapsed:  input.TimeElapsed,
		NumCycle:     input.NumCycle,
		TimeMode:     strings.TrimSpace(input.TimeMode),
		Category:     strings.TrimSpace(input.Category),
		Description:  strings.TrimSpace(input.Description),
		Mood:         strings.TrimSpace(input.Mood),
		Image:        strings.TrimSpace(input.Image),
		StartTime:    input.StartTime.UTC(),
		EndTime:      input.EndTime.UTC(),
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	if err := s.repo.Create(ctx, entry); err != nil {
		return Entry{}, err
	}

	return entry, nil
}

// Update applies partial modifications to an existing productivity entry.
func (s *Service) Update(ctx context.Context, userID, entryID string, patch PatchInput) (Entry, error) {
	if userID == "" || entryID == "" {
		return Entry{}, ErrNotFound
	}
	current, err := s.repo.GetByID(ctx, userID, entryID)
	if err != nil {
		return Entry{}, err
	}
	updated, err := patch.Apply(current)
	if err != nil {
		return Entry{}, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}
	updated.UpdatedAt = s.clock.Now().UTC()
	if err := s.repo.Update(ctx, updated); err != nil {
		return Entry{}, err
	}
	return updated, nil
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

	items := make([]ListItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, ListItem{
			ID:          e.ID,
			Image:       e.Image,
			Category:    e.Category,
			TimeElapsed: e.TimeElapsed,
			NumCycle:    e.NumCycle,
			TimeMode:    e.TimeMode,
			StartTime:   e.StartTime,
		})
	}

	return ListResponse{
		Items:    items,
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
			Date:                dateStr,
			Status:              status,
			TotalElapsedSeconds: 0,
			Sessions:            0,
		}
	}

	// Aggregate entries by day
	for _, entry := range entries {
		dayStr := entry.StartTime.Format("2006-01-02")
		if dayStatus, exists := dayMap[dayStr]; exists {
			dayStatus.TotalElapsedSeconds += entry.TimeElapsed
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
