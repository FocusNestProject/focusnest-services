package chatbot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ChatEntry represents a chatbot conversation entry
type ChatEntry struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Title     string     `json:"title"`
	Messages  []Message  `json:"messages"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"-"`
}

// Message represents a single message in a chat conversation
type Message struct {
	ID        string    `json:"id"`
	Role      string    `json:"role"` // "user" or "assistant"
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// CreateInput captures the data required to create a new chat entry
type CreateInput struct {
	UserID   string
	Title    string
	Messages []Message
}

// AskInput captures the data required to ask the chatbot
type AskInput struct {
	UserID  string
	Message string
}

// AskResponse represents the response from asking the chatbot
type AskResponse struct {
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
}

// Validate ensures the input fields meet the domain constraints
func (i CreateInput) Validate() error {
	var problems []string

	if i.UserID == "" {
		problems = append(problems, "user_id is required")
	}
	if strings.TrimSpace(i.Title) == "" {
		problems = append(problems, "title is required")
	}
	if len(i.Messages) == 0 {
		problems = append(problems, "at least one message is required")
	}

	// Validate messages
	for i, msg := range i.Messages {
		if msg.Role != "user" && msg.Role != "assistant" {
			problems = append(problems, fmt.Sprintf("message %d: role must be 'user' or 'assistant'", i))
		}
		if strings.TrimSpace(msg.Content) == "" {
			problems = append(problems, fmt.Sprintf("message %d: content is required", i))
		}
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// Validate ensures the ask input meets domain constraints
func (i AskInput) Validate() error {
	var problems []string

	if i.UserID == "" {
		problems = append(problems, "user_id is required")
	}
	if strings.TrimSpace(i.Message) == "" {
		problems = append(problems, "message is required")
	}

	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

// Pagination describes paging preferences for list queries
type Pagination struct {
	Page     int
	PageSize int
}

// PageInfo summarizes pagination metadata for responses
type PageInfo struct {
	Page       int  `json:"page"`
	PageSize   int  `json:"pageSize"`
	TotalPages int  `json:"totalPages"`
	TotalItems int  `json:"totalItems"`
	HasNext    bool `json:"hasNext"`
}

// Repository encapsulates persistence for chatbot entries
type Repository interface {
	Create(ctx context.Context, entry ChatEntry) error
	GetByID(ctx context.Context, userID, entryID string) (ChatEntry, error)
	Delete(ctx context.Context, userID, entryID string, deletedAt time.Time) error
	ListByUser(ctx context.Context, userID string, pagination Pagination) ([]ChatEntry, PageInfo, error)
	Update(ctx context.Context, entry ChatEntry) error
}

// ErrNotFound indicates the requested entry does not exist for the user
var ErrNotFound = errors.New("chatbot entry not found")

// ErrConflict indicates a duplicate identifier collision
var ErrConflict = errors.New("chatbot entry already exists")

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

// Service orchestrates the domain operations for chatbot entries
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

// Create registers a new chatbot entry for the given user
func (s *Service) Create(ctx context.Context, input CreateInput) (ChatEntry, error) {
	if err := input.Validate(); err != nil {
		return ChatEntry{}, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}

	now := s.clock.Now().UTC()

	// Generate IDs for messages
	messages := make([]Message, len(input.Messages))
	for i, msg := range input.Messages {
		messages[i] = Message{
			ID:        s.ids.NewID(),
			Role:      msg.Role,
			Content:   strings.TrimSpace(msg.Content),
			Timestamp: now,
		}
	}

	entry := ChatEntry{
		ID:        s.ids.NewID(),
		UserID:    input.UserID,
		Title:     strings.TrimSpace(input.Title),
		Messages:  messages,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.repo.Create(ctx, entry); err != nil {
		return ChatEntry{}, err
	}

	return entry, nil
}

// Get retrieves a single chatbot entry by its ID for the provided user
func (s *Service) Get(ctx context.Context, userID, entryID string) (ChatEntry, error) {
	if userID == "" || entryID == "" {
		return ChatEntry{}, ErrNotFound
	}
	return s.repo.GetByID(ctx, userID, entryID)
}

// Delete removes a chatbot entry
func (s *Service) Delete(ctx context.Context, userID, entryID string) error {
	if userID == "" || entryID == "" {
		return ErrNotFound
	}
	return s.repo.Delete(ctx, userID, entryID, s.clock.Now().UTC())
}

// List returns chatbot entries for the user with pagination
func (s *Service) List(ctx context.Context, userID string, pagination Pagination) ([]ChatEntry, PageInfo, error) {
	if userID == "" {
		return nil, PageInfo{}, ErrNotFound
	}
	return s.repo.ListByUser(ctx, userID, pagination)
}

// Ask processes a user message and returns a response
func (s *Service) Ask(ctx context.Context, input AskInput) (AskResponse, error) {
	if err := input.Validate(); err != nil {
		return AskResponse{}, fmt.Errorf("%w: %s", ErrInvalidInput, err.Error())
	}

	// For now, return a simple echo response
	// In a real implementation, this would integrate with an AI service
	response := AskResponse{
		Message:   "I understand you said: " + input.Message,
		SessionID: s.ids.NewID(),
	}

	return response, nil
}
