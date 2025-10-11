package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/focusnest/user-service/internal/user"
)

// RegisterRoutes registers all user routes
func RegisterRoutes(r chi.Router, service user.Service) {
	r.Route("/v1/me", func(r chi.Router) {
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)

		r.Get("/", getProfile(service))
		r.Patch("/", updateProfile(service))
		r.Get("/streaks", getStreaks(service))
	})
}

func getProfile(service user.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("x-user-id")
		if userID == "" {
			http.Error(w, "user ID required", http.StatusBadRequest)
			return
		}

		profile, err := service.GetProfile(userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(profile)
	}
}

func updateProfile(service user.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("x-user-id")
		if userID == "" {
			http.Error(w, "user ID required", http.StatusBadRequest)
			return
		}

		var updates map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		profile, err := service.UpdateProfile(userID, updates)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(profile)
	}
}

func getStreaks(service user.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("x-user-id")
		if userID == "" {
			http.Error(w, "user ID required", http.StatusBadRequest)
			return
		}

		streaks, err := service.GetStreaks(userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(streaks)
	}
}
