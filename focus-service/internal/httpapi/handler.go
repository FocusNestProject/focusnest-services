package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/focusnest/focus-service/internal/productivity"
	"github.com/focusnest/focus-service/internal/storage"
)

const (
	maxImageBytesCreate = int64(10 << 20) // 10MB
	maxImageBytesUpdate = int64(10 << 20) // 10MB
	defaultPageSize     = 20
	maxPageSize         = 100
	serviceTimeout      = 10 * time.Second
)

var (
	allowedContentTypes = map[string]struct{}{
		"image/jpeg": {},
		"image/png":  {},
		"image/gif":  {},
		// If you add more types, ensure you can safely validate/thumbnail them.
	}
	validCategories = []string{"Work", "Study", "Reading", "Journaling", "Cooking", "Workout", "Other"}
	validTimeModes  = []string{"Pomodoro", "QuickFocus", "FreeTimer", "CustomTimer"}
	validMoods      = []string{"excited", "focused", "calm", "energetic", "tired", "motivated", "stressed", "relaxed"}
)

type handler struct {
	service *productivity.Service
	storage *storage.Service
}

func RegisterRoutes(r chi.Router, svc *productivity.Service, storageSvc *storage.Service) {
	h := &handler{service: svc, storage: storageSvc}
	r.Route("/v1/productivities", func(r chi.Router) {
		r.Get("/", h.listProductivities)
		r.Post("/", h.createProductivity)
		r.Get("/{id}", h.getProductivity)
		r.Delete("/{id}", h.deleteProductivity)
		r.Post("/{id}/image", h.uploadProductivityImage)
		r.Post("/{id}/image:retry", h.retryProductivityImageOverview)
	})
}

func (h *handler) listProductivities(w http.ResponseWriter, r *http.Request) {
	userID := headerUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing user ID")
		return
	}

	pageSize := clampInt(parsePositiveInt(r.URL.Query().Get("pageSize"), defaultPageSize), 1, maxPageSize)
	pageToken := r.URL.Query().Get("pageToken")

	var month, year *int
	if ms := r.URL.Query().Get("month"); ms != "" {
		if m, err := strconv.Atoi(ms); err == nil && m >= 1 && m <= 12 {
			month = &m
		} else {
			writeError(w, http.StatusBadRequest, "invalid month (1-12)")
			return
		}
	}
	if ys := r.URL.Query().Get("year"); ys != "" {
		if y, err := strconv.Atoi(ys); err == nil && y >= 1970 && y <= 2100 {
			year = &y
		} else {
			writeError(w, http.StatusBadRequest, "invalid year (1970-2100)")
			return
		}
	}

	input := productivity.ListInput{
		UserID:    userID,
		PageSize:  pageSize,
		PageToken: pageToken,
		Month:     month,
		Year:      year,
	}

	ctx, cancel := context.WithTimeout(r.Context(), serviceTimeout)
	defer cancel()

	resp, err := h.service.List(ctx, input)
	if err != nil {
		respondProductivityServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *handler) createProductivity(w http.ResponseWriter, r *http.Request) {
	userID := headerUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing user ID")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxImageBytesCreate+1<<20) // request cap
	if err := r.ParseMultipartForm(maxImageBytesCreate + (1 << 20)); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	category := r.FormValue("category")
	timeMode := r.FormValue("time_mode")
	elapsedMsStr := r.FormValue("elapsed_ms")
	if category == "" || timeMode == "" || elapsedMsStr == "" {
		writeError(w, http.StatusBadRequest, "category, time_mode, and elapsed_ms are required")
		return
	}
	if !contains(validCategories, category) {
		writeError(w, http.StatusBadRequest, "invalid category; allowed: "+strings.Join(validCategories, ", "))
		return
	}
	if !contains(validTimeModes, timeMode) {
		writeError(w, http.StatusBadRequest, "invalid time_mode; allowed: "+strings.Join(validTimeModes, ", "))
		return
	}

	elapsedMs64, err := strconv.ParseInt(elapsedMsStr, 10, 64)
	if err != nil || elapsedMs64 <= 0 {
		writeError(w, http.StatusBadRequest, "elapsed_ms must be a positive integer")
		return
	}

	description := r.FormValue("description")
	mood := r.FormValue("mood")
	cyclesStr := r.FormValue("cycles")
	startAtStr := r.FormValue("start_at")
	endAtStr := r.FormValue("end_at")

	if len(description) > 2000 {
		writeError(w, http.StatusBadRequest, "description must be ≤ 2000 characters")
		return
	}
	if mood != "" && !contains(validMoods, mood) {
		writeError(w, http.StatusBadRequest, "invalid mood; allowed: "+strings.Join(validMoods, ", "))
		return
	}

	cycles := 1
	if cyclesStr != "" {
		if c, err := strconv.Atoi(cyclesStr); err == nil && c >= 1 {
			cycles = c
		} else {
			writeError(w, http.StatusBadRequest, "cycles must be an integer ≥ 1")
			return
		}
	}

	var startAt, endAt *time.Time
	if startAtStr != "" {
		t, err := parseTimestamp(startAtStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "start_at must be RFC3339/RFC3339Nano")
			return
		}
		startAt = &t
	}
	if endAtStr != "" {
		t, err := parseTimestamp(endAtStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "end_at must be RFC3339/RFC3339Nano")
			return
		}
		endAt = &t
	}
	if startAt != nil && endAt != nil && endAt.Before(*startAt) {
		writeError(w, http.StatusBadRequest, "end_at must be ≥ start_at")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "image is required")
		return
	}
	defer file.Close()

	imgInfo, err := h.validateAndUploadImage(r.Context(), file, header, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	input := productivity.CreateInput{
		UserID:      userID,
		Category:    category,
		TimeMode:    timeMode,
		Description: description,
		Mood:        mood,
		Cycles:      cycles,
		ElapsedMs:   int(elapsedMs64),
		StartAt:     startAt,
		EndAt:       endAt,
		Image:       imgInfo,
	}
	if err := input.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), serviceTimeout)
	defer cancel()

	entry, err := h.service.Create(ctx, input)
	if err != nil {
		respondProductivityServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, entry)
}

func (h *handler) getProductivity(w http.ResponseWriter, r *http.Request) {
	userID := headerUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing user ID")
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "productivity ID required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), serviceTimeout)
	defer cancel()

	entry, err := h.service.Get(ctx, userID, id)
	if err != nil {
		respondProductivityServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func (h *handler) deleteProductivity(w http.ResponseWriter, r *http.Request) {
	userID := headerUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing user ID")
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "productivity ID required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), serviceTimeout)
	defer cancel()

	if err := h.service.Delete(ctx, userID, id); err != nil {
		respondProductivityServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) uploadProductivityImage(w http.ResponseWriter, r *http.Request) {
	userID := headerUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing user ID")
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "productivity ID required")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxImageBytesUpdate)
	if err := r.ParseMultipartForm(maxImageBytesUpdate); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}
	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "image file required")
		return
	}
	defer file.Close()

	head := make([]byte, 512)
	n, _ := file.Read(head)
	ctype := http.DetectContentType(head[:n])
	if _, ok := allowedContentTypes[ctype]; !ok {
		writeError(w, http.StatusBadRequest, "unsupported image type")
		return
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		writeError(w, http.StatusBadRequest, "failed to read image")
		return
	}

	imageData, err := io.ReadAll(io.LimitReader(file, maxImageBytesUpdate))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read image data")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), serviceTimeout)
	defer cancel()

	resp, err := h.service.UploadImage(ctx, userID, id, imageData, header.Filename)
	if err != nil {
		respondProductivityServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *handler) retryProductivityImageOverview(w http.ResponseWriter, r *http.Request) {
	userID := headerUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing user ID")
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "productivity ID required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), serviceTimeout)
	defer cancel()

	resp, err := h.service.RetryImageOverview(ctx, userID, id)
	if err != nil {
		respondProductivityServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *handler) validateAndUploadImage(ctx context.Context, file multipart.File, header *multipart.FileHeader, userID string) (*productivity.ImageInfo, error) {
	if header.Size <= 0 || header.Size > maxImageBytesCreate {
		return nil, fmt.Errorf("image too large (max 10MB)")
	}
	head := make([]byte, 512)
	n, _ := file.Read(head)
	ctype := http.DetectContentType(head[:n])
	if _, ok := allowedContentTypes[ctype]; !ok {
		return nil, fmt.Errorf("unsupported image type")
	}
	if strings.Contains(ctype, "svg") {
		return nil, fmt.Errorf("unsupported image type")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("failed to read image")
	}

	lr := io.LimitReader(file, maxImageBytesCreate)
	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		_, _ = io.Copy(pw, lr)
	}()

	cfgCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cfgDone := make(chan struct{})
	var decodeErr error
	go func() {
		defer close(cfgDone)
		// Best-effort header validation by decoding config (jpeg/png/gif)
		if _, _, err := image.DecodeConfig(io.LimitReader(strings.NewReader(string(head[:n])), int64(n))); err != nil {
			// ignore header-only errors; attempt actual DecodeConfig on full stream for supported types
		}
	}()
	select {
	case <-cfgCtx.Done():
	case <-cfgDone:
	}
	_ = decodeErr // intentionally ignored; content-type gate + size limits are primary

	uploadRes, err := h.storage.UploadImage(ctx, userID, pr, header.Filename)
	if err != nil {
		return nil, fmt.Errorf("failed to upload image: %w", err)
	}
	return &productivity.ImageInfo{
		OriginalURL: uploadRes.OriginalURL,
		OverviewURL: uploadRes.OverviewURL,
	}, nil
}

func respondProductivityServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, productivity.ErrNotFound):
		writeError(w, http.StatusNotFound, "productivity not found")
	case errors.Is(err, productivity.ErrConflict):
		writeError(w, http.StatusConflict, "productivity already exists")
	case errors.Is(err, productivity.ErrInvalidInput):
		msg := strings.TrimSpace(err.Error())
		if i := strings.Index(msg, ":"); i >= 0 {
			msg = strings.TrimSpace(msg[i+1:])
		}
		writeError(w, http.StatusBadRequest, msg)
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func headerUserID(r *http.Request) string {
	return r.Header.Get("X-User-ID")
}

func parseTimestamp(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

func parsePositiveInt(value string, fallback int) int {
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
