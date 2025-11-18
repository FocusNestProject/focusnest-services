package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/focusnest/focus-service/internal/productivity"
	"github.com/focusnest/focus-service/internal/storage"
)

const (
	defaultPageSize       = 20
	maxPageSize           = 100
	serviceTimeout        = 10 * time.Second
	maxCreatePayloadBytes = 1 << 20 // 1MB
	imageSignedURLTTL     = 24 * time.Hour
)

var (
	validCategories        = productivity.ValidCategories
	validTimeModes         = productivity.ValidTimeModes
	validMoods             = productivity.ValidMoods
	allowedImageExtensions = map[string]struct{}{
		".jpg":  {},
		".jpeg": {},
		".png":  {},
		".webp": {},
		".heic": {},
		".heif": {},
	}
	allowedImageMIMEs = map[string]struct{}{
		"image/jpeg": {},
		"image/png":  {},
		"image/webp": {},
		"image/heic": {},
		"image/heif": {},
	}
)

type handler struct {
	service *productivity.Service
	storage *storage.Service
}

type createProductivityRequest struct {
	ActivityName string     `json:"activity_name"`
	TimeElapsed  int        `json:"time_elapsed"`
	NumCycle     int        `json:"num_cycle"`
	TimeMode     string     `json:"time_mode"`
	Category     string     `json:"category"`
	Description  string     `json:"description"`
	Mood         string     `json:"mood"`
	Image        string     `json:"image"`
	StartTime    *time.Time `json:"start_time"`
	EndTime      *time.Time `json:"end_time"`
}

type updateProductivityRequest struct {
	ActivityName *string    `json:"activity_name"`
	TimeElapsed  *int       `json:"time_elapsed"`
	NumCycle     *int       `json:"num_cycle"`
	TimeMode     *string    `json:"time_mode"`
	Category     *string    `json:"category"`
	Description  *string    `json:"description"`
	Mood         *string    `json:"mood"`
	Image        *string    `json:"image"`
	StartTime    *time.Time `json:"start_time"`
	EndTime      *time.Time `json:"end_time"`
}

func RegisterRoutes(r chi.Router, svc *productivity.Service, storageSvc *storage.Service) {
	h := &handler{service: svc, storage: storageSvc}
	r.Route("/v1/productivities", func(r chi.Router) {
		r.Get("/", h.listProductivities)
		r.Post("/", h.createProductivity)
		r.Patch("/{id}", h.updateProductivity)
		r.Get("/{id}", h.getProductivity)
		r.Delete("/{id}", h.deleteProductivity)
	})
}

func (h *handler) listProductivities(w http.ResponseWriter, r *http.Request) {
	userID := headerUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing user ID")
		return
	}

	pageSize := clampInt(parsePositiveInt(queryFirst(r, "page_size", "pageSize"), defaultPageSize), 1, maxPageSize)
	pageToken := queryFirst(r, "page_token", "pageToken")

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
	for i := range resp.Items {
		resp.Items[i].Image = h.resolveImageURL(ctx, resp.Items[i].Image)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":           resp.Items,
		"next_page_token": resp.PageInfo.NextToken,
		"total_items":     resp.PageInfo.TotalItems,
	})
}

func (h *handler) createProductivity(w http.ResponseWriter, r *http.Request) {
	userID := headerUserID(r)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "missing user ID")
		return
	}

	req, imageFile, imageHeader, err := h.decodeCreateRequest(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if imageFile != nil {
		defer imageFile.Close()
	}
	activityName := strings.TrimSpace(req.ActivityName)
	category := strings.TrimSpace(req.Category)
	timeMode := strings.TrimSpace(req.TimeMode)
	mood := strings.TrimSpace(req.Mood)
	image := strings.TrimSpace(req.Image)

	if activityName == "" || req.TimeElapsed <= 0 || req.NumCycle <= 0 {
		writeError(w, http.StatusBadRequest, "activity_name, time_elapsed, and num_cycle are required")
		return
	}
	if req.StartTime == nil || req.EndTime == nil {
		writeError(w, http.StatusBadRequest, "start_time and end_time are required")
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
	if len(req.Description) > 2000 {
		writeError(w, http.StatusBadRequest, "description must be ≤ 2000 characters")
		return
	}
	if mood != "" && !contains(validMoods, mood) {
		writeError(w, http.StatusBadRequest, "invalid mood; allowed: "+strings.Join(validMoods, ", "))
		return
	}
	if req.EndTime.Before(*req.StartTime) {
		writeError(w, http.StatusBadRequest, "end_time must be ≥ start_time")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), serviceTimeout)
	defer cancel()

	var storedImagePath string
	var responseImage string
	if imageFile != nil {
		if h.storage == nil {
			writeError(w, http.StatusInternalServerError, "image uploads are not configured")
			return
		}
		if err := validateImageFile(imageHeader); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		uploadResult, uploadErr := h.storage.UploadImage(ctx, userID, imageFile, imageHeader.Filename)
		if uploadErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to upload image")
			return
		}
		storedImagePath = uploadResult.OriginalPath
		responseImage = uploadResult.OriginalURL
	} else {
		storedImagePath = image
	}

	input := productivity.CreateInput{
		UserID:       userID,
		ActivityName: activityName,
		TimeElapsed:  req.TimeElapsed,
		NumCycle:     req.NumCycle,
		TimeMode:     timeMode,
		Category:     category,
		Description:  req.Description,
		Mood:         mood,
		Image:        storedImagePath,
		StartTime:    req.StartTime.UTC(),
		EndTime:      req.EndTime.UTC(),
	}
	if err := input.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	entry, svcErr := h.service.Create(ctx, input)
	if svcErr != nil {
		respondProductivityServiceError(w, svcErr)
		return
	}
	if responseImage != "" {
		entry.Image = responseImage
	} else {
		entry.Image = h.resolveImageURL(ctx, entry.Image)
	}
	writeJSON(w, http.StatusCreated, entry)
}

func (h *handler) updateProductivity(w http.ResponseWriter, r *http.Request) {
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

	req, imageFile, imageHeader, err := h.decodeUpdateRequest(w, r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if imageFile != nil {
		defer imageFile.Close()
	}
	if imageFile == nil && isEmptyPatch(req) {
		writeError(w, http.StatusBadRequest, "at least one field must be provided")
		return
	}
	if req.TimeMode != nil && !contains(validTimeModes, strings.TrimSpace(*req.TimeMode)) {
		writeError(w, http.StatusBadRequest, "invalid time_mode")
		return
	}
	if req.Category != nil && !contains(validCategories, strings.TrimSpace(*req.Category)) {
		writeError(w, http.StatusBadRequest, "invalid category")
		return
	}
	if req.Mood != nil && !contains(validMoods, strings.TrimSpace(*req.Mood)) {
		writeError(w, http.StatusBadRequest, "invalid mood")
		return
	}
	if req.StartTime != nil && req.EndTime != nil && req.EndTime.Before(*req.StartTime) {
		writeError(w, http.StatusBadRequest, "end_time must be ≥ start_time")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), serviceTimeout)
	defer cancel()

	var storedImagePath string
	var updatedImagePtr *string
	var responseImage string
	if imageFile != nil {
		if h.storage == nil {
			writeError(w, http.StatusInternalServerError, "image uploads are not configured")
			return
		}
		if req.Image != nil {
			writeError(w, http.StatusBadRequest, "provide either image file or image_url, not both")
			return
		}
		if err := validateImageFile(imageHeader); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		uploadResult, uploadErr := h.storage.UploadImage(ctx, userID, imageFile, imageHeader.Filename)
		if uploadErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to upload image")
			return
		}
		storedImagePath = uploadResult.OriginalPath
		updatedImagePtr = &storedImagePath
		responseImage = uploadResult.OriginalURL
	}

	patch := productivity.PatchInput{
		ActivityName: req.ActivityName,
		TimeElapsed:  req.TimeElapsed,
		NumCycle:     req.NumCycle,
		TimeMode:     req.TimeMode,
		Category:     req.Category,
		Description:  req.Description,
		Mood:         req.Mood,
		Image:        req.Image,
		StartTime:    req.StartTime,
		EndTime:      req.EndTime,
	}
	if updatedImagePtr != nil {
		patch.Image = updatedImagePtr
	}

	entry, updateErr := h.service.Update(ctx, userID, id, patch)
	if updateErr != nil {
		respondProductivityServiceError(w, updateErr)
		return
	}
	if responseImage != "" {
		entry.Image = responseImage
	} else {
		entry.Image = h.resolveImageURL(ctx, entry.Image)
	}
	writeJSON(w, http.StatusOK, entry)
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
	entry.Image = h.resolveImageURL(ctx, entry.Image)
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
	if v := r.Header.Get("X-User-ID"); v != "" {
		return v
	}
	return r.Header.Get("x-user-id")
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

func queryFirst(r *http.Request, keys ...string) string {
	q := r.URL.Query()
	for _, key := range keys {
		if v := q.Get(key); v != "" {
			return v
		}
	}
	return ""
}

func isEmptyPatch(req updateProductivityRequest) bool {
	return req.ActivityName == nil &&
		req.TimeElapsed == nil &&
		req.NumCycle == nil &&
		req.TimeMode == nil &&
		req.Category == nil &&
		req.Description == nil &&
		req.Mood == nil &&
		req.Image == nil &&
		req.StartTime == nil &&
		req.EndTime == nil
}

func (h *handler) decodeCreateRequest(w http.ResponseWriter, r *http.Request) (createProductivityRequest, multipart.File, *multipart.FileHeader, error) {
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(ct, "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, maxCreatePayloadBytes)
		if err := r.ParseMultipartForm(maxCreatePayloadBytes); err != nil {
			return createProductivityRequest{}, nil, nil, fmt.Errorf("invalid multipart payload")
		}
		req := createProductivityRequest{
			ActivityName: r.FormValue("activity_name"),
			TimeMode:     r.FormValue("time_mode"),
			Category:     r.FormValue("category"),
			Description:  r.FormValue("description"),
			Mood:         r.FormValue("mood"),
			Image:        r.FormValue("image_url"),
		}
		if v := strings.TrimSpace(r.FormValue("time_elapsed")); v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return req, nil, nil, fmt.Errorf("time_elapsed must be an integer")
			}
			req.TimeElapsed = val
		}
		if v := strings.TrimSpace(r.FormValue("num_cycle")); v != "" {
			val, err := strconv.Atoi(v)
			if err != nil {
				return req, nil, nil, fmt.Errorf("num_cycle must be an integer")
			}
			req.NumCycle = val
		}
		if start, err := parseRFC3339Pointer(r.FormValue("start_time"), "start_time"); err != nil {
			return req, nil, nil, err
		} else {
			req.StartTime = start
		}
		if end, err := parseRFC3339Pointer(r.FormValue("end_time"), "end_time"); err != nil {
			return req, nil, nil, err
		} else {
			req.EndTime = end
		}
		file, header, err := r.FormFile("image")
		if err == http.ErrMissingFile {
			return req, nil, nil, nil
		}
		if err != nil {
			return req, nil, nil, fmt.Errorf("invalid image upload: %w", err)
		}
		return req, file, header, nil
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCreatePayloadBytes))
	decoder.DisallowUnknownFields()
	var req createProductivityRequest
	if err := decoder.Decode(&req); err != nil {
		return createProductivityRequest{}, nil, nil, fmt.Errorf("invalid JSON payload")
	}
	return req, nil, nil, nil
}

func (h *handler) decodeUpdateRequest(w http.ResponseWriter, r *http.Request) (updateProductivityRequest, multipart.File, *multipart.FileHeader, error) {
	ct := strings.ToLower(r.Header.Get("Content-Type"))
	if strings.HasPrefix(ct, "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, maxCreatePayloadBytes)
		if err := r.ParseMultipartForm(maxCreatePayloadBytes); err != nil {
			return updateProductivityRequest{}, nil, nil, fmt.Errorf("invalid multipart payload")
		}
		values := map[string][]string{}
		if r.MultipartForm != nil {
			values = r.MultipartForm.Value
		}
		req := updateProductivityRequest{}
		if v := stringPtrFromForm(values, "activity_name"); v != nil {
			req.ActivityName = v
		}
		if v, err := intPtrFromForm(values, "time_elapsed"); err != nil {
			return updateProductivityRequest{}, nil, nil, err
		} else if v != nil {
			req.TimeElapsed = v
		}
		if v, err := intPtrFromForm(values, "num_cycle"); err != nil {
			return updateProductivityRequest{}, nil, nil, err
		} else if v != nil {
			req.NumCycle = v
		}
		if v := stringPtrFromForm(values, "time_mode"); v != nil {
			req.TimeMode = v
		}
		if v := stringPtrFromForm(values, "category"); v != nil {
			req.Category = v
		}
		if v := stringPtrFromForm(values, "description"); v != nil {
			req.Description = v
		}
		if v := stringPtrFromForm(values, "mood"); v != nil {
			req.Mood = v
		}
		if t, err := timePtrFromForm(values, "start_time"); err != nil {
			return updateProductivityRequest{}, nil, nil, err
		} else if t != nil {
			req.StartTime = t
		}
		if t, err := timePtrFromForm(values, "end_time"); err != nil {
			return updateProductivityRequest{}, nil, nil, err
		} else if t != nil {
			req.EndTime = t
		}
		if v := stringPtrFromForm(values, "image_url"); v != nil {
			req.Image = v
		}
		file, header, err := r.FormFile("image")
		if err == http.ErrMissingFile {
			return req, nil, nil, nil
		}
		if err != nil {
			return updateProductivityRequest{}, nil, nil, fmt.Errorf("invalid image upload: %w", err)
		}
		return req, file, header, nil
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxCreatePayloadBytes))
	decoder.DisallowUnknownFields()
	var req updateProductivityRequest
	if err := decoder.Decode(&req); err != nil {
		return updateProductivityRequest{}, nil, nil, fmt.Errorf("invalid JSON payload")
	}
	return req, nil, nil, nil
}

func stringPtrFromForm(values map[string][]string, key string) *string {
	if v, ok := formValue(values, key); ok {
		s := v
		return &s
	}
	return nil
}

func intPtrFromForm(values map[string][]string, key string) (*int, error) {
	v, ok := formValue(values, key)
	if !ok {
		return nil, nil
	}
	trimmed := strings.TrimSpace(v)
	if trimmed == "" {
		return nil, fmt.Errorf("%s must be an integer", key)
	}
	val, err := strconv.Atoi(trimmed)
	if err != nil {
		return nil, fmt.Errorf("%s must be an integer", key)
	}
	return &val, nil
}

func timePtrFromForm(values map[string][]string, key string) (*time.Time, error) {
	v, ok := formValue(values, key)
	if !ok {
		return nil, nil
	}
	if strings.TrimSpace(v) == "" {
		return nil, fmt.Errorf("%s must be RFC3339", key)
	}
	return parseRFC3339Pointer(v, key)
}

func formValue(values map[string][]string, key string) (string, bool) {
	if values == nil {
		return "", false
	}
	if vs, ok := values[key]; ok && len(vs) > 0 {
		return vs[0], true
	}
	return "", false
}

func validateImageFile(header *multipart.FileHeader) error {
	if header == nil {
		return fmt.Errorf("invalid image upload")
	}
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if _, ok := allowedImageExtensions[ext]; !ok {
		return fmt.Errorf("unsupported image type; allowed formats: jpg, jpeg, png, webp, heic, heif")
	}
	if ct := strings.ToLower(header.Header.Get("Content-Type")); ct != "" {
		if _, ok := allowedImageMIMEs[ct]; !ok {
			return fmt.Errorf("unsupported image content type; allowed: image/jpeg, image/png, image/webp, image/heic, image/heif")
		}
	}
	return nil
}

func (h *handler) resolveImageURL(ctx context.Context, raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	if h.storage == nil {
		return ""
	}
	url, err := h.storage.GenerateSignedURL(ctx, trimmed, imageSignedURLTTL)
	if err != nil {
		return ""
	}
	return url
}

func parseRFC3339Pointer(value, field string) (*time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, nil
	}
	t, err := time.Parse(time.RFC3339, trimmed)
	if err != nil {
		return nil, fmt.Errorf("%s must be RFC3339", field)
	}
	utc := t.UTC()
	return &utc, nil
}
