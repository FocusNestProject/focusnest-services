package chatbot

import (
	"context"
	"errors"
	"time"
)

// ChatbotSession represents a chat session
type ChatbotSession struct {
	ID        string    `json:"id" firestore:"id"`
	UserID    string    `json:"user_id" firestore:"user_id"`
	Title     string    `json:"title" firestore:"title"`
	CreatedAt time.Time `json:"created_at" firestore:"created_at"`
	UpdatedAt time.Time `json:"updated_at" firestore:"updated_at"`
}

// ChatMessage represents a message in a chat session
type ChatMessage struct {
	ID        string    `json:"id" firestore:"id"`
	SessionID string    `json:"session_id" firestore:"session_id"`
	Role      string    `json:"role" firestore:"role"` // "user" or "assistant"
	Content   string    `json:"content" firestore:"content"`
	CreatedAt time.Time `json:"created_at" firestore:"created_at"`
}

// SessionHistory bundles a session and its ordered messages
type SessionHistory struct {
	Session  *ChatbotSession `json:"session"`
	Messages []*ChatMessage  `json:"messages"`
}

var (
	// ErrSessionNotFound signals that a session could not be located in storage
	ErrSessionNotFound = errors.New("chat session not found")
	// ErrUnauthorizedSessionAccess means a user tried to read or write another user's chat
	ErrUnauthorizedSessionAccess = errors.New("session does not belong to this user")
	// ErrEmptyQuestion is returned when the incoming prompt is blank
	ErrEmptyQuestion = errors.New("question is required")
	// ErrEmptyTitle is returned when attempting to save a blank title
	ErrEmptyTitle = errors.New("title is required")
)

// Repository defines the interface for chatbot data access
type Repository interface {
	CreateSession(session *ChatbotSession) error
	GetSessions(userID string) ([]*ChatbotSession, error)
	GetSession(sessionID string) (*ChatbotSession, error)
	CreateMessage(message *ChatMessage) error
	GetMessages(sessionID string) ([]*ChatMessage, error)
	UpdateSessionTimestamp(sessionID string, updatedAt time.Time) error
	UpdateSessionTitle(sessionID string, title string, updatedAt time.Time) error
	DeleteSession(sessionID string) error
	DeleteMessages(sessionID string) error
	GetRecentMessages(sessionID string, limit int) ([]*ChatMessage, error)
}

// Service defines the chatbot service interface
type Service interface {
	CreateSession(userID, title string) (*ChatbotSession, error)
	GetSessions(userID string) ([]*ChatbotSession, error)
	GetSession(userID, sessionID string) (*ChatbotSession, error)
	GetHistory(userID string) ([]*SessionHistory, error)
	GetMessages(userID, sessionID string) ([]*ChatMessage, error)
	UpdateSessionTitle(userID, sessionID, title string) error
	DeleteSession(userID, sessionID string) error
	AskQuestion(ctx context.Context, userID, sessionID, question string) (*ChatMessage, string, error)
}
