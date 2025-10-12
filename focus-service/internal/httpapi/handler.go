package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/focusnest/focus-service/internal/productivity"
	"github.com/focusnest/focus-service/internal/storage"
)

// RegisterRoutes wires productivity routes onto the provided router.
func RegisterRoutes(r chi.Router, svc *productivity.Service, storageSvc *storage.Service) {
	h := &handler{service: svc, storage: storageSvc}

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
	storage *storage.Service
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

// contains checks if a string slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// validateAndUploadImage validates and uploads an image to Cloud Storage
func (h *handler) validateAndUploadImage(ctx context.Context, file multipart.File, header *multipart.FileHeader, userID string) (*productivity.ImageInfo, error) {
	// Check content type - accept all image types
	contentType := header.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return nil, fmt.Errorf("file must be an image")
	}

	// Check file extension - accept all common image formats
	filename := strings.ToLower(header.Filename)
	validExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".tiff", ".svg"}
	hasValidExtension := false
	for _, ext := range validExtensions {
		if strings.HasSuffix(filename, ext) {
			hasValidExtension = true
			break
		}
	}
	if !hasValidExtension {
		return nil, fmt.Errorf("image must have a valid extension (.jpg, .jpeg, .png, .gif, .webp, .bmp, .tiff, .svg)")
	}

	// Check file size (max 10MB, min 1KB)
	if header.Size > 10*1024*1024 {
		return nil, fmt.Errorf("image too large (max 10MB)")
	}
	if header.Size < 1024 {
		return nil, fmt.Errorf("image too small (min 1KB)")
	}

	// Read first few bytes to check magic bytes for common formats
	headerBytes := make([]byte, 4)
	if _, err := file.Read(headerBytes); err != nil {
		return nil, fmt.Errorf("failed to read image header")
	}

	// Check magic bytes for common image formats
	isValidImage := false

	// JPEG: FF D8 FF
	if headerBytes[0] == 0xFF && headerBytes[1] == 0xD8 && headerBytes[2] == 0xFF {
		isValidImage = true
	}
	// PNG: 89 50 4E 47
	if headerBytes[0] == 0x89 && headerBytes[1] == 0x50 && headerBytes[2] == 0x4E && headerBytes[3] == 0x47 {
		isValidImage = true
	}
	// GIF: 47 49 46 38
	if headerBytes[0] == 0x47 && headerBytes[1] == 0x49 && headerBytes[2] == 0x46 && headerBytes[3] == 0x38 {
		isValidImage = true
	}
	// WebP: 52 49 46 46 (RIFF)
	if headerBytes[0] == 0x52 && headerBytes[1] == 0x49 && headerBytes[2] == 0x46 && headerBytes[3] == 0x46 {
		isValidImage = true
	}
	// BMP: 42 4D
	if headerBytes[0] == 0x42 && headerBytes[1] == 0x4D {
		isValidImage = true
	}
	// TIFF: 49 49 2A 00 or 4D 4D 00 2A
	if (headerBytes[0] == 0x49 && headerBytes[1] == 0x49 && headerBytes[2] == 0x2A && headerBytes[3] == 0x00) ||
		(headerBytes[0] == 0x4D && headerBytes[1] == 0x4D && headerBytes[2] == 0x00 && headerBytes[3] == 0x2A) {
		isValidImage = true
	}

	if !isValidImage {
		return nil, fmt.Errorf("invalid image file format")
	}

	// Reset file pointer
	if _, err := file.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("failed to reset file pointer")
	}

	// Upload to Cloud Storage with signed URLs
	uploadResult, err := h.storage.UploadImage(ctx, userID, file, header.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to upload image: %w", err)
	}

	return &productivity.ImageInfo{
		OriginalURL: uploadResult.OriginalURL,
		OverviewURL: uploadResult.OverviewURL,
	}, nil
}

// getFileExtension extracts the file extension from filename
func getFileExtension(filename string) string {
	lastDot := strings.LastIndex(filename, ".")
	if lastDot == -1 {
		return ".jpg" // default fallback
	}
	return filename[lastDot:]
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
		writeError(w, http.StatusUnauthorized, "missing user ID")
		return
	}

	// Parse multipart form with 16MB limit
	err := r.ParseMultipartForm(16 << 20)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	// Extract and validate required fields
	category := r.FormValue("category")
	timeMode := r.FormValue("time_mode")
	elapsedMsStr := r.FormValue("elapsed_ms")

	if category == "" {
		writeError(w, http.StatusBadRequest, "category is required")
		return
	}
	if timeMode == "" {
		writeError(w, http.StatusBadRequest, "time_mode is required")
		return
	}
	if elapsedMsStr == "" {
		writeError(w, http.StatusBadRequest, "elapsed_ms is required")
		return
	}

	// Validate enums
	validCategories := []string{"Work", "Study", "Reading", "Journaling", "Cooking", "Workout", "Other"}
	validTimeModes := []string{"Pomodoro", "QuickFocus", "FreeTimer", "CustomTimer"}
	validMoods := []string{"excited", "focused", "calm", "energetic", "tired", "motivated", "stressed", "relaxed"}

	if !contains(validCategories, category) {
		writeError(w, http.StatusBadRequest, "invalid category")
		return
	}
	if !contains(validTimeModes, timeMode) {
		writeError(w, http.StatusBadRequest, "invalid time_mode")
		return
	}

	// Parse elapsed_ms
	elapsedMs, err := strconv.Atoi(elapsedMsStr)
	if err != nil || elapsedMs <= 0 {
		writeError(w, http.StatusBadRequest, "elapsed_ms must be a positive integer")
		return
	}

	// Extract optional fields
	description := r.FormValue("description")
	mood := r.FormValue("mood")
	cyclesStr := r.FormValue("cycles")
	startAtStr := r.FormValue("start_at")
	endAtStr := r.FormValue("end_at")

	// Validate description length
	if len(description) > 2000 {
		writeError(w, http.StatusBadRequest, "description must be ≤ 2000 characters")
		return
	}

	// Validate mood if provided
	if mood != "" && !contains(validMoods, mood) {
		writeError(w, http.StatusBadRequest, "invalid mood")
		return
	}

	// Parse cycles (optional, default 1)
	cycles := 1
	if cyclesStr != "" {
		cycles, err = strconv.Atoi(cyclesStr)
		if err != nil || cycles < 0 {
			writeError(w, http.StatusBadRequest, "cycles must be a non-negative integer")
			return
		}
	}

	// Parse and validate timestamps
	var startAt, endAt *time.Time
	if startAtStr != "" {
		if t, err := time.Parse(time.RFC3339, startAtStr); err == nil {
			startAt = &t
		} else {
			writeError(w, http.StatusBadRequest, "start_at must be RFC3339 timestamp")
			return
		}
	}
	if endAtStr != "" {
		if t, err := time.Parse(time.RFC3339, endAtStr); err == nil {
			endAt = &t
		} else {
			writeError(w, http.StatusBadRequest, "end_at must be RFC3339 timestamp")
			return
		}
	}

	// Validate timestamp relationship
	if startAt != nil && endAt != nil && endAt.Before(*startAt) {
		writeError(w, http.StatusBadRequest, "end_at must be ≥ start_at")
		return
	}

	// Handle image upload with validation (REQUIRED)
	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "image is required")
		return
	}
	defer file.Close()

	// Validate image
	image, err := h.validateAndUploadImage(r.Context(), file, header, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Create the productivity entry
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

	// Validate input
	if err := input.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	entry, err := h.service.Create(r.Context(), input)
	if err != nil {
		respondProductivityServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, entry)
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
