package chatbot

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

type service struct {
	repo          Repository
	assistant     Assistant
	contextWindow int
}

// NewService wires the chatbot service with persistence and responder.
func NewService(repo Repository, assistant Assistant, contextWindow int) (Service, error) {
	if repo == nil {
		return nil, errors.New("repository is required")
	}
	if assistant == nil {
		assistant = NewTemplateAssistant()
	}
	if contextWindow <= 0 {
		contextWindow = 16
	}
	return &service{repo: repo, assistant: assistant, contextWindow: contextWindow}, nil
}

func (s *service) CreateSession(userID, title string) (*ChatbotSession, error) {
	trimmedTitle := strings.TrimSpace(title)
	if trimmedTitle == "" {
		trimmedTitle = deriveChatTitle("")
	}
	now := time.Now().UTC()
	session := &ChatbotSession{
		ID:        uuid.New().String(),
		UserID:    userID,
		Title:     trimmedTitle,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.CreateSession(session); err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return session, nil
}

func (s *service) GetSessions(userID string) ([]*ChatbotSession, error) {
	return s.repo.GetSessions(userID)
}

func (s *service) GetSession(userID, sessionID string) (*ChatbotSession, error) {
	return s.ensureSessionOwnership(userID, sessionID)
}

func (s *service) GetMessages(userID, sessionID string) ([]*ChatMessage, error) {
	if _, err := s.ensureSessionOwnership(userID, sessionID); err != nil {
		return nil, err
	}
	return s.repo.GetMessages(sessionID)
}

func (s *service) UpdateSessionTitle(userID, sessionID, title string) error {
	if _, err := s.ensureSessionOwnership(userID, sessionID); err != nil {
		return err
	}
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return ErrEmptyTitle
	}
	return s.repo.UpdateSessionTitle(sessionID, trimmed, time.Now().UTC())
}

func (s *service) DeleteSession(userID, sessionID string) error {
	if _, err := s.ensureSessionOwnership(userID, sessionID); err != nil {
		return err
	}
	if err := s.repo.DeleteMessages(sessionID); err != nil {
		return fmt.Errorf("delete messages: %w", err)
	}
	if err := s.repo.DeleteSession(sessionID); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

func (s *service) AskQuestion(ctx context.Context, userID, sessionID, question string) (*ChatMessage, string, error) {
	trimmed := strings.TrimSpace(question)
	if trimmed == "" {
		return nil, "", ErrEmptyQuestion
	}

	session, err := s.ensureSessionForPrompt(userID, sessionID, trimmed)
	if err != nil {
		return nil, "", err
	}

	userMessage := &ChatMessage{
		ID:        uuid.New().String(),
		SessionID: session.ID,
		Role:      "user",
		Content:   trimmed,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.CreateMessage(userMessage); err != nil {
		return nil, "", fmt.Errorf("create user message: %w", err)
	}

	contextMessages, err := s.repo.GetRecentMessages(session.ID, s.contextWindow)
	if err != nil {
		return nil, "", fmt.Errorf("load context: %w", err)
	}

	lang := detectLanguage(trimmed, contextMessages)
	var responseText string
	if !isProductivityContext(trimmed, contextMessages) {
		responseText = boundaryMessage(lang)
	} else {
		responseText, err = s.assistant.Respond(ctx, lang, trimmed, contextMessages)
		if err != nil {
			responseText = buildProductivityResponse(trimmed, contextMessages, lang)
		}
	}

	assistantMessage := &ChatMessage{
		ID:        uuid.New().String(),
		SessionID: session.ID,
		Role:      "assistant",
		Content:   responseText,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.CreateMessage(assistantMessage); err != nil {
		return nil, "", fmt.Errorf("create assistant message: %w", err)
	}
	if err := s.repo.UpdateSessionTimestamp(session.ID, assistantMessage.CreatedAt); err != nil {
		return nil, "", fmt.Errorf("update session timestamp: %w", err)
	}

	return assistantMessage, session.ID, nil
}

func (s *service) ensureSessionOwnership(userID, sessionID string) (*ChatbotSession, error) {
	session, err := s.repo.GetSession(sessionID)
	if err != nil {
		return nil, err
	}
	if session.UserID != userID {
		return nil, ErrUnauthorizedSessionAccess
	}
	return session, nil
}

func (s *service) ensureSessionForPrompt(userID, sessionID, prompt string) (*ChatbotSession, error) {
	if strings.TrimSpace(sessionID) == "" {
		title := deriveChatTitle(prompt)
		return s.CreateSession(userID, title)
	}
	return s.ensureSessionOwnership(userID, sessionID)
}

func deriveChatTitle(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return fmt.Sprintf("Focus Chat %s", time.Now().Format("Jan 02 15:04"))
	}
	words := strings.Fields(trimmed)
	if len(words) > 6 {
		words = words[:6]
	}
	title := strings.Join(words, " ")
	return cases.Title(language.English).String(title)
}
