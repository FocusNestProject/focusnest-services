package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	sharedauth "github.com/focusnest/shared-libs/auth"

	"github.com/focusnest/focus-service/internal/productivity"
)

// RegisterRoutes wires productivity routes onto the provided router.
func RegisterRoutes(r chi.Router, svc *productivity.Service) {
	h := &handler{service: svc}

	r.Route("/v1/focus", func(r chi.Router) {
		r.Get("/history-month", h.historyMonth)
		r.Route("/productivity", func(r chi.Router) {
			r.Get("/", h.list)
			r.Post("/", h.create)
			r.Get("/{activity_id}", h.get)
		})
		r.Post("/image-overview:retry", h.retryImageOverview)
	})
}

type handler struct {
	service *productivity.Service
}

type activityResponse struct {
	ID          string                  `json:"id"`
	UserID      string                  `json:"user_id"`
	Category    string                  `json:"category"`
	TimeMode    string                  `json:"time_mode"`
	Description string                  `json:"description,omitempty"`
	Mood        string                  `json:"mood,omitempty"`
	Cycles      int                     `json:"cycles"`
	ElapsedMs   int                     `json:"elapsed_ms"`
	StartAt     time.Time               `json:"start_at"`
	EndAt       time.Time               `json:"end_at"`
	Image       *productivity.ImageInfo `json:"image,omitempty"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
}

type monthHistoryResponse struct {
	Month int         `json:"month"`
	Year  int         `json:"year"`
	Days  []dayStatus `json:"days"`
}

type dayStatus struct {
	Date           string `json:"date"`
	Status         string `json:"status"`
	TotalElapsedMs int    `json:"total_elapsed_ms"`
	Sessions       int    `json:"sessions"`
}

type listResponse struct {
	Data       []activityResponse    `json:"data"`
	Pagination productivity.PageInfo `json:"pagination"`
}

type createRequest struct {
	Category    string                  `json:"category"`
	TimeMode    string                  `json:"time_mode"`
	Description string                  `json:"description"`
	Mood        string                  `json:"mood"`
	Cycles      int                     `json:"cycles"`
	ElapsedMs   int                     `json:"elapsed_ms"`
	StartAt     *string                 `json:"start_at"`
	EndAt       *string                 `json:"end_at"`
	Image       *productivity.ImageInfo `json:"image,omitempty"`
}

type retryImageOverviewRequest struct {
	ActivityID string `json:"activity_id"`
}

type retryImageOverviewResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func (h *handler) historyMonth(w http.ResponseWriter, r *http.Request) {
	user, ok := sharedauth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	query := r.URL.Query()
	month := parsePositiveInt(query.Get("month"), int(time.Now().Month()))
	year := parsePositiveInt(query.Get("year"), time.Now().Year())

	// Create anchor time for the requested month
	anchor := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)

	// Get all activities for the month
	monthStart := time.Date(anchor.Year(), anchor.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)

	entries, _, err := h.service.ListMonth(r.Context(), user.UserID, anchor, productivity.Pagination{Page: 1, PageSize: 1000})
	if err != nil {
		respondProductivityServiceError(w, err)
		return
	}

	// Group activities by day
	dayMap := make(map[string][]productivity.Entry)
	for _, entry := range entries {
		day := entry.StartAt.Format("2006-01-02")
		dayMap[day] = append(dayMap[day], entry)
	}

	// Generate day status for each day in the month
	days := make([]dayStatus, 0)
	current := monthStart
	for current.Before(monthEnd) {
		day := current.Format("2006-01-02")
		dayEntries := dayMap[day]

		status := "inactive"
		totalElapsedMs := 0
		sessions := len(dayEntries)

		if sessions > 0 {
			status = "active"
			for _, entry := range dayEntries {
				totalElapsedMs += entry.ElapsedMs
			}
		}

		days = append(days, dayStatus{
			Date:           day,
			Status:         status,
			TotalElapsedMs: totalElapsedMs,
			Sessions:       sessions,
		})

		current = current.AddDate(0, 0, 1)
	}

	payload := monthHistoryResponse{
		Month: month,
		Year:  year,
		Days:  days,
	}

	writeJSON(w, http.StatusOK, payload)
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	user, ok := sharedauth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	query := r.URL.Query()
	page := parsePositiveInt(query.Get("page"), 1)
	pageSize := parsePositiveInt(query.Get("pageSize"), 20)
	if pageSize > 100 {
		pageSize = 100
	}

	// Get current month as default
	anchor := time.Now().UTC()
	entries, pageInfo, err := h.service.ListMonth(r.Context(), user.UserID, anchor, productivity.Pagination{Page: page, PageSize: pageSize})
	if err != nil {
		respondProductivityServiceError(w, err)
		return
	}

	payload := listResponse{
		Data:       make([]activityResponse, len(entries)),
		Pagination: pageInfo,
	}

	for i, entry := range entries {
		payload.Data[i] = mapEntry(entry)
	}

	writeJSON(w, http.StatusOK, payload)
}

func (h *handler) create(w http.ResponseWriter, r *http.Request) {
	user, ok := sharedauth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body createRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	input := productivity.CreateInput{
		UserID:      user.UserID,
		Category:    body.Category,
		TimeMode:    body.TimeMode,
		Description: body.Description,
		Mood:        body.Mood,
		Cycles:      body.Cycles,
		ElapsedMs:   body.ElapsedMs,
		Image:       body.Image,
	}

	if body.StartAt != nil && *body.StartAt != "" {
		parsed, err := time.Parse(time.RFC3339, *body.StartAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "start_at must be RFC3339 timestamp")
			return
		}
		input.StartAt = &parsed
	}

	if body.EndAt != nil && *body.EndAt != "" {
		parsed, err := time.Parse(time.RFC3339, *body.EndAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "end_at must be RFC3339 timestamp")
			return
		}
		input.EndAt = &parsed
	}

	entry, err := h.service.Create(r.Context(), input)
	if err != nil {
		respondProductivityServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, mapEntry(entry))
}

func (h *handler) get(w http.ResponseWriter, r *http.Request) {
	user, ok := sharedauth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "activity_id")
	entry, err := h.service.Get(r.Context(), user.UserID, id)
	if err != nil {
		respondProductivityServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, mapEntry(entry))
}

func (h *handler) retryImageOverview(w http.ResponseWriter, r *http.Request) {
	user, ok := sharedauth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body retryImageOverviewRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	// Verify the activity exists and belongs to the user
	_, err := h.service.Get(r.Context(), user.UserID, body.ActivityID)
	if err != nil {
		respondProductivityServiceError(w, err)
		return
	}

	// TODO: Implement image overview generation
	// For now, just return success
	payload := retryImageOverviewResponse{
		Success: true,
		Message: "Image overview generation triggered",
	}

	writeJSON(w, http.StatusOK, payload)
}

func respondProductivityServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, productivity.ErrNotFound):
		writeError(w, http.StatusNotFound, "activity not found")
	case errors.Is(err, productivity.ErrConflict):
		writeError(w, http.StatusConflict, "activity already exists")
	case errors.Is(err, productivity.ErrInvalidInput):
		message := err.Error()
		if idx := strings.Index(message, ":"); idx >= 0 {
			message = strings.TrimSpace(message[idx+1:])
		}
		writeError(w, http.StatusBadRequest, message)
	default:
		// Log the actual error for debugging
		fmt.Printf("ERROR: Unhandled productivity service error: %v\n", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func parsePositiveInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func mapEntry(entry productivity.Entry) activityResponse {
	return activityResponse{
		ID:          entry.ID,
		UserID:      entry.UserID,
		Category:    entry.Category,
		TimeMode:    entry.TimeMode,
		Description: entry.Description,
		Mood:        entry.Mood,
		Cycles:      entry.Cycles,
		ElapsedMs:   entry.ElapsedMs,
		StartAt:     entry.StartAt,
		EndAt:       entry.EndAt,
		Image:       entry.Image,
		CreatedAt:   entry.CreatedAt,
		UpdatedAt:   entry.UpdatedAt,
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
