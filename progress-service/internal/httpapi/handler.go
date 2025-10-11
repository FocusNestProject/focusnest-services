package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/focusnest/progress-service/internal/progress"
)

// RegisterRoutes registers all progress routes
func RegisterRoutes(r chi.Router, service progress.Service) {
	r.Route("/v1/progress", func(r chi.Router) {
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)

		r.Get("/", getProgress(service))
	})
}

func getProgress(service progress.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("x-user-id")
		if userID == "" {
			http.Error(w, "user ID required", http.StatusBadRequest)
			return
		}

		// Default to last 30 days
		endDate := time.Now()
		startDate := endDate.AddDate(0, 0, -30)

		stats, err := service.GetProgress(userID, startDate, endDate)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response := map[string]interface{}{
			"total_time":      stats.TotalTime,
			"total_sessions":  stats.TotalSessions,
			"categories":      stats.Categories,
			"periods":         stats.Periods,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}
