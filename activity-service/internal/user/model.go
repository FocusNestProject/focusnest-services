package user

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Profile represents a user profile
type Profile struct {
	ID              string     `json:"id"`
	UserID          string     `json:"user_id"`
	Bio             string     `json:"bio"`
	Birthdate       *time.Time `json:"birthdate"`
	BackgroundImage string     `json:"background_image"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	DeletedAt       *time.Time `json:"-"`
}

// UpdateInput captures the data required to update a user profile
type UpdateInput struct {
	UserID          string
	Bio             string
	Birthdate       *time.Time
	BackgroundImage string
}

// Validate ensures the input fields meet the domain constraints
func (i UpdateInput) Validate() error {
	var problems []string

	if i.UserID == "" {
		problems = append(problems, "user_id is required")
	}

	// Validate bio length
	if len(i.Bio) > 500 {
		problems = append(problems, "bio must be 500 characters or less")
	}

	// Validate birthdate is not in the future
	if i.Birthdate != nil && i.Birthdate.After(time.Now()) {
		problems = append(problems, "birthdate cannot be in the future")
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// Repository encapsulates persistence for user profiles
type Repository interface {
	GetByUserID(ctx context.Context, userID string) (Profile, error)
	Create(ctx context.Context, profile Profile) error
	Update(ctx context.Context, profile Profile) error
	Delete(ctx context.Context, userID string, deletedAt time.Time) error
}

// ErrNotFound indicates the requested profile does not exist
var ErrNotFound = errors.New("user profile not found")

// ErrConflict indicates a duplicate identifier collision
var ErrConflict = errors.New("user profile already exists")

// ErrInvalidInput indicates the provided data failed validation
var ErrInvalidInput = errors.New("invalid input")

// Clock delivers the current time; extracted for deterministic testing
type Clock interface {
	Now() time.Time
}

// IDGenerator produces unique identifiers for new entries
type IDGenerator interface {
	NewID() string
}

// Service orchestrates the domain operations for user profiles
type Service struct {
	repo  Repository
	clock Clock
	ids   IDGenerator
}

// NewService constructs a Service instance with the provided collaborators
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

// Get retrieves a user profile by user ID
func (s *Service) Get(ctx context.Context, userID string) (Profile, error) {
	if userID == "" {
		return Profile{}, ErrNotFound
	}
	return s.repo.GetByUserID(ctx, userID)
}

// Create creates a new user profile
func (s *Service) Create(ctx context.Context, userID string) (Profile, error) {
	if userID == "" {
		return Profile{}, ErrInvalidInput
	}

	now := s.clock.Now().UTC()
	profile := Profile{
		ID:        s.ids.NewID(),
		UserID:    userID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(ctx, profile); err != nil {
		return Profile{}, err
	}

	return profile, nil
}

// Update updates an existing user profile
func (s *Service) Update(ctx context.Context, input UpdateInput) (Profile, error) {
	if err := input.Validate(); err != nil {
		return Profile{}, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}

	// Get existing profile
	profile, err := s.repo.GetByUserID(ctx, input.UserID)
	if err != nil {
		return Profile{}, err
	}

	// Update fields
	profile.Bio = strings.TrimSpace(input.Bio)
	profile.Birthdate = input.Birthdate
	profile.BackgroundImage = strings.TrimSpace(input.BackgroundImage)
	profile.UpdatedAt = s.clock.Now().UTC()

	if err := s.repo.Update(ctx, profile); err != nil {
		return Profile{}, err
	}

	return profile, nil
}

// Delete removes a user profile
func (s *Service) Delete(ctx context.Context, userID string) error {
	if userID == "" {
		return ErrNotFound
	}
	return s.repo.Delete(ctx, userID, s.clock.Now().UTC())
}
