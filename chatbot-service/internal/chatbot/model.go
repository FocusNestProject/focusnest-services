package chatbot

import (
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

// Repository defines the interface for chatbot data access
type Repository interface {
	CreateSession(session *ChatbotSession) error
	GetSessions(userID string) ([]*ChatbotSession, error)
	CreateMessage(message *ChatMessage) error
	GetMessages(sessionID string) ([]*ChatMessage, error)
}

// Service defines the chatbot service interface
type Service interface {
	CreateSession(userID, title string) (*ChatbotSession, error)
	GetSessions(userID string) ([]*ChatbotSession, error)
	AskQuestion(sessionID, question string) (*ChatMessage, error)
}
