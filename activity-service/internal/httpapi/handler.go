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

	"github.com/focusnest/activity-service/internal/productivity"
)

// RegisterRoutes wires productivity routes onto the provided router.
func RegisterRoutes(r chi.Router, svc *productivity.Service) {
	h := &handler{service: svc}

	r.Route("/v1/productivities", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.get)
			r.Delete("/", h.delete)
		})
	})
}

type handler struct {
	service *productivity.Service
}

type productivityResponse struct {
	ID                  string    `json:"id"`
	UserID              string    `json:"userId"`
	Category            string    `json:"category"`
	TimeConsumedMinutes int       `json:"timeConsumedMinutes"`
	CycleMode           string    `json:"cycleMode,omitempty"`
	CycleCount          int       `json:"cycleCount,omitempty"`
	Description         string    `json:"description,omitempty"`
	Mood                string    `json:"mood,omitempty"`
	ImageURL            string    `json:"imageUrl,omitempty"`
	StartedAt           time.Time `json:"startedAt"`
	EndedAt             time.Time `json:"endedAt"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
}

type listResponse struct {
	Month      string                 `json:"month"`
	Range      rangeResponse          `json:"range"`
	Data       []productivityResponse `json:"data"`
	Pagination productivity.PageInfo  `json:"pagination"`
}

type rangeResponse struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type createRequest struct {
	Category            string  `json:"category"`
	TimeConsumedMinutes int     `json:"timeConsumedMinutes"`
	CycleMode           string  `json:"cycleMode"`
	CycleCount          int     `json:"cycleCount"`
	Description         string  `json:"description"`
	Mood                string  `json:"mood"`
	ImageURL            string  `json:"imageUrl"`
	StartedAt           *string `json:"startedAt"`
	EndedAt             *string `json:"endedAt"`
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	user, ok := sharedauth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	query := r.URL.Query()
	monthParam := query.Get("month")
	anchor := time.Now().UTC()
	var monthLabel string
	if monthParam != "" {
		parsed, err := time.Parse("2006-01", monthParam)
		if err != nil {
			writeError(w, http.StatusBadRequest, "month must be in YYYY-MM format")
			return
		}
		anchor = parsed
		monthLabel = monthParam
	} else {
		monthLabel = anchor.Format("2006-01")
	}

	page := parsePositiveInt(query.Get("page"), 1)
	pageSize := parsePositiveInt(query.Get("pageSize"), 20)
	if pageSize > 100 {
		pageSize = 100
	}

	entries, pageInfo, err := h.service.ListMonth(r.Context(), user.UserID, anchor, productivity.Pagination{Page: page, PageSize: pageSize})
	if err != nil {
		respondProductivityServiceError(w, err)
		return
	}

	monthStart := time.Date(anchor.Year(), anchor.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)

	payload := listResponse{
		Month:      monthLabel,
		Range:      rangeResponse{Start: monthStart, End: monthEnd},
		Data:       make([]productivityResponse, len(entries)),
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
		UserID:              user.UserID,
		Category:            body.Category,
		TimeConsumedMinutes: body.TimeConsumedMinutes,
		CycleMode:           body.CycleMode,
		CycleCount:          body.CycleCount,
		Description:         body.Description,
		Mood:                body.Mood,
		ImageURL:            body.ImageURL,
	}

	if body.StartedAt != nil && *body.StartedAt != "" {
		parsed, err := time.Parse(time.RFC3339, *body.StartedAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "startedAt must be RFC3339 timestamp")
			return
		}
		input.StartedAt = &parsed
	}

	if body.EndedAt != nil && *body.EndedAt != "" {
		parsed, err := time.Parse(time.RFC3339, *body.EndedAt)
		if err != nil {
			writeError(w, http.StatusBadRequest, "endedAt must be RFC3339 timestamp")
			return
		}
		input.EndedAt = &parsed
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

	id := chi.URLParam(r, "id")
	entry, err := h.service.Get(r.Context(), user.UserID, id)
	if err != nil {
		respondProductivityServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, mapEntry(entry))
}

func (h *handler) delete(w http.ResponseWriter, r *http.Request) {
	user, ok := sharedauth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.service.Delete(r.Context(), user.UserID, id); err != nil {
		respondProductivityServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func respondProductivityServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, productivity.ErrNotFound):
		writeError(w, http.StatusNotFound, "productivity entry not found")
	case errors.Is(err, productivity.ErrConflict):
		writeError(w, http.StatusConflict, "productivity entry already exists")
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

func mapEntry(entry productivity.Entry) productivityResponse {
	return productivityResponse{
		ID:                  entry.ID,
		UserID:              entry.UserID,
		Category:            entry.Category,
		TimeConsumedMinutes: entry.TimeConsumedMinutes,
		CycleMode:           entry.CycleMode,
		CycleCount:          entry.CycleCount,
		Description:         entry.Description,
		Mood:                entry.Mood,
		ImageURL:            entry.ImageURL,
		StartedAt:           entry.StartedAt,
		EndedAt:             entry.EndedAt,
		CreatedAt:           entry.CreatedAt,
		UpdatedAt:           entry.UpdatedAt,
	}
}
