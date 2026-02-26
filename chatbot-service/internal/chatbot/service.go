package chatbot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	logger        *slog.Logger
	enrichment    EnrichmentProvider
}

// NewService wires the chatbot service with persistence and responder.
func NewService(repo Repository, assistant Assistant, contextWindow int, opts ...func(*service)) (Service, error) {
	if repo == nil {
		return nil, errors.New("repository is required")
	}
	if assistant == nil {
		assistant = NewTemplateAssistant()
	}
	if contextWindow <= 0 {
		contextWindow = 32
	}
	s := &service{repo: repo, assistant: assistant, contextWindow: contextWindow, logger: slog.Default()}
	for _, o := range opts {
		o(s)
	}
	return s, nil
}

// WithLogger sets a custom logger for the service.
func WithLogger(l *slog.Logger) func(*service) {
	return func(s *service) { s.logger = l }
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
		Pinned:    false,
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

func (s *service) UpdateSessionPinned(userID, sessionID string, pinned bool) error {
	if _, err := s.ensureSessionOwnership(userID, sessionID); err != nil {
		return err
	}
	return s.repo.UpdateSessionPinned(sessionID, pinned, time.Now().UTC())
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

	// Get context before creating the new user message to avoid duplicates
	contextMessages, err := s.repo.GetRecentMessages(session.ID, s.contextWindow)
	if err != nil {
		return nil, "", fmt.Errorf("load context: %w", err)
	}

	lang := detectLanguage(trimmed, contextMessages)

	// Fetch user productivity data for enriched context
	var enrichmentText string
	if s.enrichment != nil {
		userCtx, err := s.enrichment.GetUserContext(ctx, userID)
		if err != nil {
			s.logger.Warn("enrichment fetch failed, proceeding without it",
				slog.String("userId", userID),
				slog.Any("error", err),
			)
		} else {
			enrichmentText = FormatEnrichmentPrompt(userCtx, lang)
		}
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

	// Try to get AI-generated response, with retry on failure
	responseText, err := s.assistant.Respond(ctx, lang, trimmed, contextMessages, enrichmentText)
	if err != nil {
		s.logger.Error("gemini respond failed (attempt 1)",
			slog.String("sessionId", session.ID),
			slog.Int("contextSize", len(contextMessages)),
			slog.Any("error", err),
		)
		// Retry once with a simpler context if first attempt fails
		// Use only the last few messages for retry
		simplifiedContext := contextMessages
		if len(simplifiedContext) > 4 {
			simplifiedContext = simplifiedContext[len(simplifiedContext)-4:]
		}
		responseText, err = s.assistant.Respond(ctx, lang, trimmed, simplifiedContext, enrichmentText)
		if err != nil {
			s.logger.Error("gemini respond failed (attempt 2 - giving up)",
				slog.String("sessionId", session.ID),
				slog.Int("contextSize", len(simplifiedContext)),
				slog.Any("error", err),
			)
			// Last resort: return a simple, context-aware message without templates
			if lang == languageIndonesian {
				responseText = "Maaf, ada masalah teknis. Bisa coba lagi? Aku di sini untuk membantu dengan produktivitas dan fokus kamu."
			} else {
				responseText = "Sorry, I'm having a technical issue. Could you try again? I'm here to help with your productivity and focus."
			}
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
