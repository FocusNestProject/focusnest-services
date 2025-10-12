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


	"github.com/focusnest/focus-service/internal/productivity"
)

// RegisterRoutes wires productivity routes onto the provided router.
func RegisterRoutes(r chi.Router, svc *productivity.Service) {
	h := &handler{service: svc}

	// New API routes according to spec
	r.Route("/v1/productivities", func(r chi.Router) {
		r.Get("/", h.listProductivities)
		r.Post("/", h.createProductivity)
		r.Get("/{id}", h.getProductivity)
		r.Delete("/{id}", h.deleteProductivity)
		r.Post("/{id}/image", h.uploadProductivityImage)
		r.Post("/{id}/image:retry", h.retryProductivityImageOverview)
	})
}

type handler struct {
	service *productivity.Service
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


// New API handlers according to the specification

func (h *handler) listProductivities(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("x-user-id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user ID required")
		return
	}

	// Parse query parameters
	pageSize := parsePositiveInt(r.URL.Query().Get("pageSize"), 20)
	if pageSize > 100 {
		pageSize = 100
	}
	pageToken := r.URL.Query().Get("pageToken")
	
	var month, year *int
	if monthStr := r.URL.Query().Get("month"); monthStr != "" {
		if m, err := strconv.Atoi(monthStr); err == nil && m >= 1 && m <= 12 {
			month = &m
		}
	}
	if yearStr := r.URL.Query().Get("year"); yearStr != "" {
		if y, err := strconv.Atoi(yearStr); err == nil {
			year = &y
		}
	}

	input := productivity.ListInput{
		UserID:    userID,
		PageSize:  pageSize,
		PageToken: pageToken,
		Month:     month,
		Year:      year,
	}

	response, err := h.service.List(r.Context(), input)
	if err != nil {
		respondProductivityServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *handler) createProductivity(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("x-user-id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user ID required")
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		writeError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	// Extract form fields
	category := r.FormValue("category")
	timeMode := r.FormValue("time_mode")
	description := r.FormValue("description")
	mood := r.FormValue("mood")
	
	var cycles int
	if cyclesStr := r.FormValue("cycles"); cyclesStr != "" {
		if c, err := strconv.Atoi(cyclesStr); err == nil {
			cycles = c
		}
	}
	
	elapsedMsStr := r.FormValue("elapsed_ms")
	if elapsedMsStr == "" {
		writeError(w, http.StatusBadRequest, "elapsed_ms is required")
		return
	}
	elapsedMs, err := strconv.Atoi(elapsedMsStr)
	if err != nil || elapsedMs <= 0 {
		writeError(w, http.StatusBadRequest, "elapsed_ms must be a positive integer")
		return
	}

	// Parse timestamps
	var startAt, endAt *time.Time
	if startAtStr := r.FormValue("start_at"); startAtStr != "" {
		if t, err := time.Parse(time.RFC3339, startAtStr); err == nil {
			startAt = &t
		}
	}
	if endAtStr := r.FormValue("end_at"); endAtStr != "" {
		if t, err := time.Parse(time.RFC3339, endAtStr); err == nil {
			endAt = &t
		}
	}

	// Handle image upload
	var image *productivity.ImageInfo
	if file, _, err := r.FormFile("image"); err == nil {
		defer file.Close()
		// TODO: Upload to Cloud Storage and get URLs
		// For now, create placeholder image info
		image = &productivity.ImageInfo{
			OriginalPath: fmt.Sprintf("users/%s/activities/%s/original.jpg", userID, "temp-id"),
			OverviewPath: fmt.Sprintf("users/%s/activities/%s/overview.png", userID, "temp-id"),
			OriginalURL:  "https://storage.googleapis.com/focusnest-media/users/temp/original.jpg",
			OverviewURL:  "https://storage.googleapis.com/focusnest-media/users/temp/overview.png",
		}
	}

	input := productivity.CreateInput{
		UserID:      userID,
		Category:    category,
		TimeMode:    timeMode,
		Description: description,
		Mood:        mood,
		Cycles:      cycles,
		ElapsedMs:   elapsedMs,
		StartAt:     startAt,
		EndAt:       endAt,
		Image:       image,
	}

	entry, err := h.service.Create(r.Context(), input)
	if err != nil {
		respondProductivityServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(entry)
}

func (h *handler) getProductivity(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("x-user-id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user ID required")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "productivity ID required")
		return
	}

	entry, err := h.service.Get(r.Context(), userID, id)
	if err != nil {
		respondProductivityServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(entry)
}

func (h *handler) deleteProductivity(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("x-user-id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user ID required")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "productivity ID required")
		return
	}

	err := h.service.Delete(r.Context(), userID, id)
	if err != nil {
		respondProductivityServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) uploadProductivityImage(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("x-user-id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user ID required")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "productivity ID required")
		return
	}

	// Parse multipart form
	if err := r.ParseMultipartForm(32 << 20); err != nil { // 32MB max
		writeError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "image file required")
		return
	}
	defer file.Close()

	// Read file data
	imageData := make([]byte, header.Size)
	if _, err := file.Read(imageData); err != nil {
		writeError(w, http.StatusBadRequest, "failed to read image data")
		return
	}

	response, err := h.service.UploadImage(r.Context(), userID, id, imageData, header.Filename)
	if err != nil {
		respondProductivityServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func (h *handler) retryProductivityImageOverview(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("x-user-id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "user ID required")
		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "productivity ID required")
		return
	}

	response, err := h.service.RetryImageOverview(r.Context(), userID, id)
	if err != nil {
		respondProductivityServiceError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
