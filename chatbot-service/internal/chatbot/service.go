package chatbot

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type service struct {
	repo Repository
}

// NewService creates a new chatbot service
func NewService(repo Repository) (Service, error) {
	return &service{repo: repo}, nil
}

func (s *service) CreateSession(userID, title string) (*ChatbotSession, error) {
	session := &ChatbotSession{
		ID:        uuid.New().String(),
		UserID:    userID,
		Title:     title,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.CreateSession(session); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}

	return session, nil
}

func (s *service) GetSessions(userID string) ([]*ChatbotSession, error) {
	return s.repo.GetSessions(userID)
}

func (s *service) AskQuestion(sessionID, question string) (*ChatMessage, error) {
	// Create user message
	userMessage := &ChatMessage{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Role:      "user",
		Content:   question,
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateMessage(userMessage); err != nil {
		return nil, fmt.Errorf("create user message: %w", err)
	}

	// Generate AI response (placeholder)
	response := "I'm here to help you with your productivity goals! This is a placeholder response. In a real implementation, this would integrate with an AI service like Gemini."

	// Create assistant message
	assistantMessage := &ChatMessage{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		Role:      "assistant",
		Content:   response,
		CreatedAt: time.Now(),
	}

	if err := s.repo.CreateMessage(assistantMessage); err != nil {
		return nil, fmt.Errorf("create assistant message: %w", err)
	}

	return assistantMessage, nil
}
