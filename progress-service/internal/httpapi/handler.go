package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/focusnest/progress-service/internal/progress"
)

// Streak response types
type monthlyStreakResponse struct {
	Month         int         `json:"month"`
	Year          int         `json:"year"`
	TotalStreak   int         `json:"total_streak"`
	CurrentStreak int         `json:"current_streak"`
	Days          []dayStatus `json:"days"`
}

type weeklyStreakResponse struct {
	Week          string      `json:"week"` // YYYY-WW format
	TotalStreak   int         `json:"total_streak"`
	CurrentStreak int         `json:"current_streak"`
	Days          []dayStatus `json:"days"`
}

type currentStreakResponse struct {
	TotalStreak   int `json:"total_streak"`
	CurrentStreak int `json:"current_streak"`
}

type dayStatus struct {
	Date   string `json:"date"`   // YYYY-MM-DD format
	Day    string `json:"day"`    // Monday, Tuesday, etc.
	Status string `json:"status"` // active, skipped, upcoming
}

// RegisterRoutes registers all progress routes
func RegisterRoutes(r chi.Router, service progress.Service) {
	r.Route("/v1/progress", func(r chi.Router) {
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)

		r.Get("/", getProgress(service))
	})

	// Streak endpoints
	r.Route("/v1/streaks", func(r chi.Router) {
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)

		r.Get("/month", getMonthlyStreak(service))
		r.Get("/week", getWeeklyStreak(service))
		r.Get("/current", getCurrentStreak(service))
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
			"total_time":     stats.TotalTime,
			"total_sessions": stats.TotalSessions,
			"categories":     stats.Categories,
			"periods":        stats.Periods,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}
}

// getMonthlyStreak returns monthly streak data
func getMonthlyStreak(service progress.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("x-user-id")
		if userID == "" {
			http.Error(w, "user ID required", http.StatusBadRequest)
			return
		}

		// Parse query parameters
		query := r.URL.Query()
		monthStr := query.Get("month")
		yearStr := query.Get("year")

		// Default to current month/year if not provided
		now := time.Now().UTC()
		month := int(now.Month())
		year := now.Year()

		if monthStr != "" {
			if m, err := strconv.Atoi(monthStr); err == nil && m >= 1 && m <= 12 {
				month = m
			}
		}
		if yearStr != "" {
			if y, err := strconv.Atoi(yearStr); err == nil && y >= 2020 && y <= 2030 {
				year = y
			}
		}

		// Get monthly streak data from service
		streakData, err := service.GetMonthlyStreak(r.Context(), userID, month, year)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(streakData)
	}
}

// getWeeklyStreak returns weekly streak data
func getWeeklyStreak(service progress.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("x-user-id")
		if userID == "" {
			http.Error(w, "user ID required", http.StatusBadRequest)
			return
		}

		// Parse query parameters
		query := r.URL.Query()
		dateStr := query.Get("date") // Optional date to get specific week

		var targetDate time.Time
		if dateStr != "" {
			if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
				targetDate = parsed
			} else {
				http.Error(w, "invalid date format, use YYYY-MM-DD", http.StatusBadRequest)
				return
			}
		} else {
			targetDate = time.Now().UTC()
		}

		// Get weekly streak data from service
		streakData, err := service.GetWeeklyStreak(r.Context(), userID, targetDate)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(streakData)
	}
}

// getCurrentStreak returns current running streak
func getCurrentStreak(service progress.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("x-user-id")
		if userID == "" {
			http.Error(w, "user ID required", http.StatusBadRequest)
			return
		}

		// Get current streak data from service
		streakData, err := service.GetCurrentStreak(r.Context(), userID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(streakData)
	}
}
