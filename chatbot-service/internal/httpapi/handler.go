package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/focusnest/chatbot-service/internal/chatbot"
)

const maxMessagesResponse = 200

// RegisterRoutes registers all chatbot routes
func RegisterRoutes(r chi.Router, service chatbot.Service, logger *slog.Logger) {
	r.Route("/v1/chatbot", func(r chi.Router) {
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)

		r.Get("/sessions", listSessions(service, logger))
		r.Get("/sessions/{sessionID}", getSession(service, logger))
		r.Patch("/sessions/{sessionID}", updateSessionTitle(service, logger))
		r.Delete("/sessions/{sessionID}", deleteSession(service, logger))
		r.Post("/ask", askQuestion(service, logger))
	})
}

func listSessions(service chatbot.Service, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := headerUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "missing X-User-ID header")
			return
		}

		sessions, err := service.GetSessions(userID)
		if err != nil {
			logServiceError(r.Context(), logger, "listSessions", userID, err)
			writeServiceError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"sessions": sessions})
	}
}

func getSession(service chatbot.Service, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := headerUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "missing X-User-ID header")
			return
		}
		sessionID := strings.TrimSpace(chi.URLParam(r, "sessionID"))
		if sessionID == "" {
			writeError(w, http.StatusBadRequest, "session ID required")
			return
		}

		session, err := service.GetSession(userID, sessionID)
		if err != nil {
			logServiceError(r.Context(), logger, "getSession", userID, err, sessionID)
			writeServiceError(w, err)
			return
		}
		messages, err := service.GetMessages(userID, sessionID)
		if err != nil {
			logServiceError(r.Context(), logger, "getMessages", userID, err, sessionID)
			writeServiceError(w, err)
			return
		}
		messages = truncateMessages(messages, maxMessagesResponse)

		writeJSON(w, http.StatusOK, map[string]any{
			"session":  session,
			"messages": messages,
		})
	}
}

func askQuestion(service chatbot.Service, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := headerUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "missing X-User-ID header")
			return
		}

		var req struct {
			SessionID string `json:"session_id"`
			Question  string `json:"question"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		message, sessionID, err := service.AskQuestion(r.Context(), userID, req.SessionID, req.Question)
		if err != nil {
			// Log detailed error for debugging assistant issues
			logger.Error("askQuestion failed",
				slog.String("userId", userID),
				slog.String("sessionId", req.SessionID),
				slog.String("question", req.Question),
				slog.Any("error", err),
			)
			logServiceError(r.Context(), logger, "askQuestion", userID, err, req.SessionID)
			writeServiceError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"session_id":        sessionID,
			"assistant_message": message,
		})
	}
}

func updateSessionTitle(service chatbot.Service, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := headerUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "missing X-User-ID header")
			return
		}
		sessionID := strings.TrimSpace(chi.URLParam(r, "sessionID"))
		if sessionID == "" {
			writeError(w, http.StatusBadRequest, "session ID required")
			return
		}

		var req struct {
			Title string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := service.UpdateSessionTitle(userID, sessionID, req.Title); err != nil {
			logServiceError(r.Context(), logger, "updateSessionTitle", userID, err, sessionID)
			writeServiceError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	}
}

func deleteSession(service chatbot.Service, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := headerUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "missing X-User-ID header")
			return
		}
		sessionID := strings.TrimSpace(chi.URLParam(r, "sessionID"))
		if sessionID == "" {
			writeError(w, http.StatusBadRequest, "session ID required")
			return
		}

		if err := service.DeleteSession(userID, sessionID); err != nil {
			logServiceError(r.Context(), logger, "deleteSession", userID, err, sessionID)
			writeServiceError(w, err)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func headerUserID(r *http.Request) string {
	if v := r.Header.Get("X-User-ID"); v != "" {
		return v
	}
	return r.Header.Get("x-user-id")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, chatbot.ErrSessionNotFound):
		writeError(w, http.StatusNotFound, "session not found")
	case errors.Is(err, chatbot.ErrUnauthorizedSessionAccess):
		writeError(w, http.StatusForbidden, "session does not belong to user")
	case errors.Is(err, chatbot.ErrEmptyQuestion):
		writeError(w, http.StatusBadRequest, "question is required")
	case errors.Is(err, chatbot.ErrEmptyTitle):
		writeError(w, http.StatusBadRequest, "title is required")
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func logServiceError(ctx context.Context, logger *slog.Logger, operation, userID string, err error, sessionID ...string) {
	if logger == nil || err == nil {
		return
	}
	attrs := []any{
		slog.String("operation", operation),
		slog.String("userId", userID),
		slog.Any("error", err),
	}
	if len(sessionID) > 0 && sessionID[0] != "" {
		attrs = append(attrs, slog.String("sessionId", sessionID[0]))
	}
	if reqID := middleware.GetReqID(ctx); reqID != "" {
		attrs = append(attrs, slog.String("requestId", reqID))
	}
	logger.Error("chatbot request failed", attrs...)
}

func truncateMessages(messages []*chatbot.ChatMessage, limit int) []*chatbot.ChatMessage {
	if limit <= 0 || len(messages) <= limit {
		return messages
	}
	return messages[len(messages)-limit:]
}
