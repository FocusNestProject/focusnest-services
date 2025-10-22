package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/focusnest/progress-service/internal/progress"
)

const (
	serviceTimeout  = 8 * time.Second
	defaultDaysBack = 30
	minYear         = 1970
	maxYear         = 2100
	dateLayout      = "2006-01-02"
)

// RegisterRoutes defines all /v1/progress routes
func RegisterRoutes(r chi.Router, service progress.Service) {
	r.Route("/v1/progress", func(r chi.Router) {
		r.Use(middleware.Recoverer)

		// Summary: last 30 days
		r.Get("/", getProgress(service))

		// Streaks
		r.Route("/streaks", func(r chi.Router) {
			r.Get("/month", getMonthlyStreak(service))
			r.Get("/week", getWeeklyStreak(service))
			r.Get("/current", getCurrentStreak(service))
		})
	})
}

// GET /v1/progress
// Returns summary stats for the last 30 days
func getProgress(service progress.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := headerUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "missing user ID")
			return
		}

		end := time.Now().UTC()
		start := end.AddDate(0, 0, -defaultDaysBack)

		ctx, cancel := context.WithTimeout(r.Context(), serviceTimeout)
		defer cancel()

		stats, err := service.GetProgress(ctx, userID, start, end)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"total_time":     stats.TotalTime,
			"total_sessions": stats.TotalSessions,
			"categories":     stats.Categories,
			"periods":        stats.Periods,
		})
	}
}

// GET /v1/progress/streaks/month?date=YYYY-MM-DD
func getMonthlyStreak(service progress.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := headerUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "missing user ID")
			return
		}

		target, ok := optionalDate(r.URL.Query().Get("date"))
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid date format, use YYYY-MM-DD")
			return
		}
		target = target.UTC()

		year, month := target.Year(), target.Month()
		if year < minYear || year > maxYear {
			writeError(w, http.StatusBadRequest, "invalid year (1970–2100)")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), serviceTimeout)
		defer cancel()

		data, err := service.GetMonthlyStreak(ctx, userID, int(month), year)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		writeJSON(w, http.StatusOK, data)
	}
}

// GET /v1/progress/streaks/week?date=YYYY-MM-DD
func getWeeklyStreak(service progress.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := headerUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "missing user ID")
			return
		}

		target, ok := optionalDate(r.URL.Query().Get("date"))
		if !ok {
			writeError(w, http.StatusBadRequest, "invalid date format, use YYYY-MM-DD")
			return
		}
		target = startOfDayUTC(target.UTC())

		ctx, cancel := context.WithTimeout(r.Context(), serviceTimeout)
		defer cancel()

		data, err := service.GetWeeklyStreak(ctx, userID, target)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		writeJSON(w, http.StatusOK, data)
	}
}

// GET /v1/progress/streaks/current
// Returns the current (last 30 days) streak data
func getCurrentStreak(service progress.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := headerUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "missing user ID")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), serviceTimeout)
		defer cancel()

		data, err := service.GetCurrentStreak(ctx, userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		writeJSON(w, http.StatusOK, data)
	}
}

// Helpers

// headerUserID gets the user ID from headers, case-insensitive.
func headerUserID(r *http.Request) string {
	if v := r.Header.Get("X-User-ID"); v != "" {
		return v
	}
	return r.Header.Get("x-user-id")
}

// optionalDate parses YYYY-MM-DD; if empty, returns today UTC.
func optionalDate(s string) (time.Time, bool) {
	if s == "" {
		return time.Now().UTC(), true
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func startOfDayUTC(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
