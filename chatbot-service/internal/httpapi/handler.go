package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/focusnest/chatbot-service/internal/chatbot"
)

// RegisterRoutes registers all chatbot routes
func RegisterRoutes(r chi.Router, service chatbot.Service) {
	r.Route("/v1/chatbot", func(r chi.Router) {
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)

		r.Get("/history", getHistory(service))
		r.Post("/ask", askQuestion(service))
	})
}

func getHistory(service chatbot.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("x-user-id")
		if userID == "" {
			http.Error(w, "user ID required", http.StatusBadRequest)
			return
		}

		sessions, err := service.GetSessions(userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"sessions": sessions,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

func askQuestion(service chatbot.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("x-user-id")
		if userID == "" {
			http.Error(w, "user ID required", http.StatusBadRequest)
			return
		}

		var req struct {
			SessionID string `json:"session_id"`
			Question  string `json:"question"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.SessionID == "" {
			// Create new session
			session, err := service.CreateSession(userID, "New Chat")
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			req.SessionID = session.ID
		}

		response, err := service.AskQuestion(req.SessionID, req.Question)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
