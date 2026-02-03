package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/focusnest/progress-service/internal/progress"
)

const (
	serviceTimeout = 8 * time.Second
	minYear        = 1970
	maxYear        = 2100
	dateLayout     = "2006-01-02"
)

// RegisterRoutes defines all /v1/progress routes
func RegisterRoutes(r chi.Router, service progress.Service) {
	r.Route("/v1/progress", func(r chi.Router) {
		r.Use(middleware.Recoverer)

		r.Get("/summary", getSummary(service))

		r.Route("/streak", func(r chi.Router) {
			r.Get("/monthly", getMonthlyStreak(service))
			r.Get("/weekly", getWeeklyStreak(service))
			r.Get("/current", getCurrentStreak(service))
			r.Post("/recover", recoverStreak(service))
		})
	})
}

// GET /v1/progress/summary
func getSummary(service progress.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := headerUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "missing user ID")
			return
		}

		rangeParam := r.URL.Query().Get("range")
		if rangeParam == "" {
			rangeParam = string(progress.SummaryRangeWeek)
		}
		category := r.URL.Query().Get("category")
		var reference time.Time
		if raw := r.URL.Query().Get("reference_date"); raw != "" {
			t, err := time.Parse(dateLayout, raw)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid reference_date, use YYYY-MM-DD")
				return
			}
			reference = t
		}

		ctx, cancel := context.WithTimeout(r.Context(), serviceTimeout)
		defer cancel()

		resp, err := service.GetSummary(ctx, userID, progress.SummaryInput{
			Range:         progress.SummaryRange(rangeParam),
			Category:      category,
			ReferenceDate: reference,
		})
		if err != nil {
			status := http.StatusInternalServerError
			message := "internal server error"
			if errors.Is(err, progress.ErrInvalidSummaryRange) || errors.Is(err, progress.ErrMissingUserID) {
				status = http.StatusBadRequest
				message = err.Error()
			}
			writeError(w, status, message)
			return
		}

		writeJSON(w, http.StatusOK, resp)
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

// GET /v1/progress/streak/current
// Returns the current (last 30 days) streak data with status (active/grace/expired) and recovery quota.
// Optional header: X-Timezone (IANA, e.g. Asia/Jakarta); default Asia/Jakarta.
func getCurrentStreak(service progress.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := headerUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "missing user ID")
			return
		}
		timezone := r.Header.Get("X-Timezone")
		if timezone == "" {
			timezone = r.Header.Get("x-timezone")
		}

		ctx, cancel := context.WithTimeout(r.Context(), serviceTimeout)
		defer cancel()

		data, err := service.GetCurrentStreak(ctx, userID, timezone)
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

// POST /v1/progress/streak/recover
// Restores streak after expiry (premium only; quota 5/month). Requires X-Premium: true (gateway sets after RevenueCat check).
// Optional header: X-Timezone (IANA); default Asia/Jakarta.
func recoverStreak(service progress.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := headerUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "missing user ID")
			return
		}
		isPremium := r.Header.Get("X-Premium") == "true" || r.Header.Get("x-premium") == "true"
		timezone := r.Header.Get("X-Timezone")
		if timezone == "" {
			timezone = r.Header.Get("x-timezone")
		}

		ctx, cancel := context.WithTimeout(r.Context(), serviceTimeout)
		defer cancel()

		data, err := service.RecoverStreak(ctx, userID, isPremium, timezone)
		if err != nil {
			status := http.StatusInternalServerError
			message := "internal server error"
			switch {
			case errors.Is(err, progress.ErrNotPremium):
				status = http.StatusForbidden
				message = err.Error()
			case errors.Is(err, progress.ErrStreakNotRecoverable):
				status = http.StatusBadRequest
				message = err.Error()
			case errors.Is(err, progress.ErrRecoveryQuotaExceeded):
				status = http.StatusBadRequest
				message = err.Error()
			}
			writeError(w, status, message)
			return
		}

		writeJSON(w, http.StatusOK, data)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
