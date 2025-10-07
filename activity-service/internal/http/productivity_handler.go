package http

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/focusnest/activity-service/internal/auth"
	"github.com/focusnest/activity-service/internal/productivity"
	sharederrors "github.com/focusnest/shared-libs/errors"
)

type ProductivityHandler struct {
	service *productivity.Service
}

func NewProductivityHandler(service *productivity.Service) *ProductivityHandler {
	return &ProductivityHandler{service: service}
}

func (h *ProductivityHandler) RegisterRoutes(r chi.Router, verifier auth.Verifier) {
	r.Route("/v1/productivities", func(r chi.Router) {
		r.Use(auth.Middleware(verifier))

		r.Get("/", h.listProductivities)
		r.Post("/", h.createProductivity)
		r.Get("/{id}", h.getProductivity)
		r.Delete("/{id}", h.deleteProductivity)
	})
}

func (h *ProductivityHandler) listProductivities(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpError(w, sharederrors.ErrorResponse{Code: "unauthorized", Message: "missing authentication"}, http.StatusUnauthorized)
		return
	}

	monthParam := strings.TrimSpace(r.URL.Query().Get("month"))
	anchor := time.Now().UTC()
	if monthParam != "" {
		parsed, err := time.Parse("2006-01", monthParam)
		if err != nil {
			httpError(w, sharederrors.ErrorResponse{Code: "bad_request", Message: "invalid month format, expected YYYY-MM", RequestID: requestID(r)}, http.StatusBadRequest)
			return
		}
		anchor = parsed
	}

	page := parseIntDefault(r.URL.Query().Get("page"), 1)
	pageSize := parseIntDefault(r.URL.Query().Get("pageSize"), 20)
	if pageSize > 100 {
		pageSize = 100
	}

	entries, pageInfo, err := h.service.ListMonth(r.Context(), user.UserID, anchor, productivity.Pagination{Page: page, PageSize: pageSize})
	if err != nil {
		httpError(w, sharederrors.ErrorResponse{Code: "internal", Message: err.Error(), RequestID: requestID(r)}, http.StatusInternalServerError)
		return
	}

	monthStart := time.Date(anchor.Year(), anchor.Month(), 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, 0)

	respondJSON(w, http.StatusOK, listResponse{
		Month: monthStart.Format("2006-01"),
		Range: rangeEnvelope{Start: monthStart, End: monthEnd},
		Data:  entriesToDTO(entries),
		Pagination: paginationEnvelope{
			Page:       pageInfo.Page,
			PageSize:   pageInfo.PageSize,
			TotalPages: pageInfo.TotalPages,
			TotalItems: pageInfo.TotalItems,
			HasNext:    pageInfo.HasNext,
		},
	})
}

func (h *ProductivityHandler) createProductivity(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpError(w, sharederrors.ErrorResponse{Code: "unauthorized", Message: "missing authentication"}, http.StatusUnauthorized)
		return
	}

	var payload createRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httpError(w, sharederrors.ErrorResponse{Code: "bad_request", Message: "invalid JSON payload", RequestID: requestID(r)}, http.StatusBadRequest)
		return
	}

	input, err := payload.toInput(user.UserID)
	if err != nil {
		httpError(w, sharederrors.ErrorResponse{Code: "bad_request", Message: err.Error(), RequestID: requestID(r)}, http.StatusBadRequest)
		return
	}

	entry, err := h.service.Create(r.Context(), input)
	if err != nil {
		httpError(w, sharederrors.ErrorResponse{Code: "bad_request", Message: err.Error(), RequestID: requestID(r)}, http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusCreated, entryToDTO(entry))
}

func (h *ProductivityHandler) getProductivity(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpError(w, sharederrors.ErrorResponse{Code: "unauthorized", Message: "missing authentication"}, http.StatusUnauthorized)
		return
	}

	entryID := chi.URLParam(r, "id")
	entry, err := h.service.Get(r.Context(), user.UserID, entryID)
	if err != nil {
		status := http.StatusInternalServerError
		code := "internal"
		if err == productivity.ErrNotFound {
			status = http.StatusNotFound
			code = "not_found"
		}
		httpError(w, sharederrors.ErrorResponse{Code: code, Message: err.Error(), RequestID: requestID(r)}, status)
		return
	}

	respondJSON(w, http.StatusOK, entryToDTO(entry))
}

func (h *ProductivityHandler) deleteProductivity(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok {
		httpError(w, sharederrors.ErrorResponse{Code: "unauthorized", Message: "missing authentication"}, http.StatusUnauthorized)
		return
	}

	entryID := chi.URLParam(r, "id")
	if err := h.service.Delete(r.Context(), user.UserID, entryID); err != nil {
		status := http.StatusInternalServerError
		code := "internal"
		if err == productivity.ErrNotFound {
			status = http.StatusNotFound
			code = "not_found"
		}
		httpError(w, sharederrors.ErrorResponse{Code: code, Message: err.Error(), RequestID: requestID(r)}, status)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func parseIntDefault(raw string, fallback int) int {
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return value
}

func entriesToDTO(entries []productivity.Entry) []entryDTO {
	result := make([]entryDTO, len(entries))
	for i, entry := range entries {
		result[i] = entryToDTO(entry)
	}
	return result
}

func entryToDTO(entry productivity.Entry) entryDTO {
	return entryDTO{
		ID:                  entry.ID,
		UserID:              entry.UserID,
		Category:            entry.Category,
		TimeConsumedMinutes: entry.TimeConsumedMinutes,
		CycleMode:           entry.CycleMode,
		Description:         entry.Description,
		Mood:                entry.Mood,
		ImageURL:            entry.ImageURL,
		StartedAt:           entry.StartedAt,
		EndedAt:             entry.EndedAt,
		CreatedAt:           entry.CreatedAt,
		UpdatedAt:           entry.UpdatedAt,
	}
}

func requestID(r *http.Request) string {
	return middleware.GetReqID(r.Context())
}

func respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func httpError(w http.ResponseWriter, payload sharederrors.ErrorResponse, status int) {
	respondJSON(w, status, payload)
}

type rangeEnvelope struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

type paginationEnvelope struct {
	Page       int  `json:"page"`
	PageSize   int  `json:"page_size"`
	TotalPages int  `json:"total_pages"`
	TotalItems int  `json:"total_items"`
	HasNext    bool `json:"has_next"`
}

type listResponse struct {
	Month      string             `json:"month"`
	Range      rangeEnvelope      `json:"range"`
	Data       []entryDTO         `json:"data"`
	Pagination paginationEnvelope `json:"pagination"`
}

type entryDTO struct {
	ID                  string    `json:"id"`
	UserID              string    `json:"user_id"`
	Category            string    `json:"category"`
	TimeConsumedMinutes int       `json:"time_consumed_minutes"`
	CycleMode           string    `json:"cycle_mode,omitempty"`
	Description         string    `json:"description,omitempty"`
	Mood                string    `json:"mood,omitempty"`
	ImageURL            string    `json:"image_url,omitempty"`
	StartedAt           time.Time `json:"started_at"`
	EndedAt             time.Time `json:"ended_at"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type createRequest struct {
	Category            string     `json:"category"`
	TimeConsumedMinutes int        `json:"time_consumed_minutes"`
	CycleMode           string     `json:"cycle_mode"`
	Description         string     `json:"description"`
	Mood                string     `json:"mood"`
	ImageURL            string     `json:"image_url"`
	StartedAt           *time.Time `json:"started_at"`
	EndedAt             *time.Time `json:"ended_at"`
}

func (c createRequest) toInput(userID string) (productivity.CreateInput, error) {
	return productivity.CreateInput{
		UserID:              userID,
		Category:            strings.TrimSpace(c.Category),
		TimeConsumedMinutes: c.TimeConsumedMinutes,
		CycleMode:           strings.TrimSpace(c.CycleMode),
		Description:         strings.TrimSpace(c.Description),
		Mood:                strings.TrimSpace(c.Mood),
		ImageURL:            strings.TrimSpace(c.ImageURL),
		StartedAt:           c.StartedAt,
		EndedAt:             c.EndedAt,
	}, nil
}
