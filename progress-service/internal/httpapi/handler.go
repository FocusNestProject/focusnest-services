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

// Response shapes (kept for docs; actual data for streaks comes from service)
type monthlyStreakResponse struct {
	Month         int         `json:"month"`
	Year          int         `json:"year"`
	TotalStreak   int         `json:"total_streak"`
	CurrentStreak int         `json:"current_streak"`
	Days          []dayStatus `json:"days"`
}

type weeklyStreakResponse struct {
	Week          string      `json:"week"` // YYYY-WW
	TotalStreak   int         `json:"total_streak"`
	CurrentStreak int         `json:"current_streak"`
	Days          []dayStatus `json:"days"`
}

type dayStatus struct {
	Date   string `json:"date"`   // YYYY-MM-DD
	Day    string `json:"day"`    // Monday, Tuesday, etc.
	Status string `json:"status"` // active, skipped, upcoming
}

// RegisterRoutes registers all progress routes under /v1/progress
// Streak endpoints live under /v1/progress/streaks/*
func RegisterRoutes(r chi.Router, service progress.Service) {
	r.Route("/v1/progress", func(r chi.Router) {
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)

		// Summary/progress stats (last 30 days by default)
		r.Get("/", getProgress(service))

		// Streaks (no /current)
		r.Route("/streaks", func(r chi.Router) {
			r.Get("/month", getMonthlyStreak(service))
			r.Get("/week", getWeeklyStreak(service))
		})
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

		resp := map[string]interface{}{
			"total_time":     stats.TotalTime,
			"total_sessions": stats.TotalSessions,
			"categories":     stats.Categories,
			"periods":        stats.Periods,
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}

// GET /v1/progress/streaks/month?month=1-12&year=YYYY
func getMonthlyStreak(service progress.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("x-user-id")
		if userID == "" {
			http.Error(w, "user ID required", http.StatusBadRequest)
			return
		}

		q := r.URL.Query()
		monthStr := q.Get("month")
		yearStr := q.Get("year")

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

		data, err := service.GetMonthlyStreak(r.Context(), userID, month, year)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(data)
	}
}

// GET /v1/progress/streaks/week?date=YYYY-MM-DD (optional; defaults to current week)
func getWeeklyStreak(service progress.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("x-user-id")
		if userID == "" {
			http.Error(w, "user ID required", http.StatusBadRequest)
			return
		}

		q := r.URL.Query()
		dateStr := q.Get("date") // optional

		var target time.Time
		if dateStr != "" {
			parsed, err := time.Parse("2006-01-02", dateStr)
			if err != nil {
				http.Error(w, "invalid date format, use YYYY-MM-DD", http.StatusBadRequest)
				return
			}
			target = parsed
		} else {
			target = time.Now().UTC()
		}

		data, err := service.GetWeeklyStreak(r.Context(), userID, target)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(data)
	}
}
