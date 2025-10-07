package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	sharedauth "github.com/focusnest/shared-libs/auth"

	"github.com/focusnest/activity-service/internal/analytics"
)

// RegisterAnalyticsRoutes wires analytics routes onto the provided router
func RegisterAnalyticsRoutes(r chi.Router, svc *analytics.Service) {
	h := &analyticsHandler{service: svc}

	r.Route("/v1/analytics", func(r chi.Router) {
		r.Get("/progress", h.getProgress)
		r.Get("/streak", h.getStreak)
		r.Get("/categories", h.getCategories)
	})
}

type analyticsHandler struct {
	service *analytics.Service
}

type progressResponse struct {
	Period      string                `json:"period"`
	Range       timeRangeResponse     `json:"range"`
	Stats       progressStatsResponse `json:"stats"`
	GeneratedAt string                `json:"generatedAt"`
}

type timeRangeResponse struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type progressStatsResponse struct {
	TimeConsumedMinutes int                            `json:"timeConsumedMinutes"`
	TotalSessions       int                            `json:"totalSessions"`
	TotalHours          float64                        `json:"totalHours"`
	MostProductiveHours []int                          `json:"mostProductiveHours"`
	Streak              streakInfoResponse             `json:"streak"`
	ByCategory          map[string]int                 `json:"byCategory"`
	ByPeriod            map[string]periodStatsResponse `json:"byPeriod"`
}

type streakInfoResponse struct {
	Current    int    `json:"current"`
	Longest    int    `json:"longest"`
	LastActive string `json:"lastActive"`
}

type periodStatsResponse struct {
	TimeConsumedMinutes int            `json:"timeConsumedMinutes"`
	TotalSessions       int            `json:"totalSessions"`
	TotalHours          float64        `json:"totalHours"`
	ByCategory          map[string]int `json:"byCategory"`
}

func (h *analyticsHandler) getProgress(w http.ResponseWriter, r *http.Request) {
	user, ok := sharedauth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	query := r.URL.Query()
	period := query.Get("period")
	category := query.Get("category")
	startDate := query.Get("startDate")
	endDate := query.Get("endDate")

	req := analytics.AnalyticsRequest{
		UserID:   user.UserID,
		Period:   analytics.PeriodType(period),
		Category: category,
	}

	// Parse optional date range
	if startDate != "" {
		if parsed, err := time.Parse("2006-01-02", startDate); err == nil {
			req.StartDate = &parsed
		}
	}
	if endDate != "" {
		if parsed, err := time.Parse("2006-01-02", endDate); err == nil {
			req.EndDate = &parsed
		}
	}

	response, err := h.service.GetProgress(r.Context(), req)
	if err != nil {
		respondServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, mapProgressResponse(response))
}

func (h *analyticsHandler) getStreak(w http.ResponseWriter, r *http.Request) {
	_, ok := sharedauth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Placeholder implementation - return empty streak
	streak := analytics.StreakInfo{
		Current:    0,
		Longest:    0,
		LastActive: time.Time{},
	}

	writeJSON(w, http.StatusOK, mapStreakResponse(streak))
}

func (h *analyticsHandler) getCategories(w http.ResponseWriter, r *http.Request) {
	_, ok := sharedauth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	query := r.URL.Query()
	startDate := query.Get("startDate")
	endDate := query.Get("endDate")

	var start, end time.Time
	if startDate != "" {
		if parsed, err := time.Parse("2006-01-02", startDate); err == nil {
			start = parsed
		}
	} else {
		start = time.Now().AddDate(0, 0, -30) // Default to last 30 days
	}

	if endDate != "" {
		if parsed, err := time.Parse("2006-01-02", endDate); err == nil {
			end = parsed
		}
	} else {
		end = time.Now()
	}

	// Placeholder implementation - return empty categories
	categories := make(map[string]int)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"categories": categories,
		"range": timeRangeResponse{
			Start: start.Format(time.RFC3339),
			End:   end.Format(time.RFC3339),
		},
	})
}

func mapProgressResponse(response analytics.AnalyticsResponse) progressResponse {
	return progressResponse{
		Period: response.Period,
		Range: timeRangeResponse{
			Start: response.Range.Start.Format(time.RFC3339),
			End:   response.Range.End.Format(time.RFC3339),
		},
		Stats:       mapProgressStats(response.Stats),
		GeneratedAt: response.GeneratedAt.Format(time.RFC3339),
	}
}

func mapProgressStats(stats analytics.ProgressStats) progressStatsResponse {
	byPeriod := make(map[string]periodStatsResponse)
	for period, periodStats := range stats.ByPeriod {
		byPeriod[period] = periodStatsResponse{
			TimeConsumedMinutes: periodStats.TimeConsumedMinutes,
			TotalSessions:       periodStats.TotalSessions,
			TotalHours:          periodStats.TotalHours,
			ByCategory:          periodStats.ByCategory,
		}
	}

	return progressStatsResponse{
		TimeConsumedMinutes: stats.TimeConsumedMinutes,
		TotalSessions:       stats.TotalSessions,
		TotalHours:          stats.TotalHours,
		MostProductiveHours: stats.MostProductiveHours,
		Streak:              mapStreakInfo(stats.Streak),
		ByCategory:          stats.ByCategory,
		ByPeriod:            byPeriod,
	}
}

func mapStreakInfo(streak analytics.StreakInfo) streakInfoResponse {
	return streakInfoResponse{
		Current:    streak.Current,
		Longest:    streak.Longest,
		LastActive: streak.LastActive.Format(time.RFC3339),
	}
}

func mapStreakResponse(streak analytics.StreakInfo) streakInfoResponse {
	return mapStreakInfo(streak)
}
