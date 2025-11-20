package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/focusnest/user-service/internal/user"
)

const (
	serviceTimeout    = 8 * time.Second
	dateLayout        = "2006-01-02"
	maxPatchBodyBytes = 64 * 1024 // 64KB of JSON is more than enough for profile updates
)

// RegisterRoutes registers all user routes
func RegisterRoutes(r chi.Router, service user.Service, logger *slog.Logger) {
	r.Route("/v1/users", func(r chi.Router) {
		r.Use(middleware.Recoverer)

		r.Get("/me", getProfile(service, logger))
		r.Patch("/me", updateProfile(service, logger))
	})
}

func getProfile(service user.Service, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := headerUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "missing user ID")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), serviceTimeout)
		defer cancel()

		profile, err := service.GetProfile(ctx, userID)
		if err != nil {
			logRequestError(r.Context(), logger, "failed to load profile", err, userID)
			writeError(w, http.StatusInternalServerError, "failed to load profile")
			return
		}

		writeJSON(w, http.StatusOK, profile)
	}
}

func updateProfile(service user.Service, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := headerUserID(r)
		if userID == "" {
			writeError(w, http.StatusUnauthorized, "missing user ID")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxPatchBodyBytes)
		defer r.Body.Close()

		payload, err := decodePatchPayload(r)
		if err != nil {
			var maxErr *http.MaxBytesError
			switch {
			case errors.Is(err, errInvalidPayload):
				writeError(w, http.StatusBadRequest, errInvalidPayload.Error())
			case errors.As(err, &maxErr):
				writeError(w, http.StatusRequestEntityTooLarge, "payload too large")
			default:
				writeError(w, http.StatusInternalServerError, "failed to decode profile update")
			}
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), serviceTimeout)
		defer cancel()

		profile, err := service.UpdateProfile(ctx, userID, payload)
		if err != nil {
			logRequestError(r.Context(), logger, "failed to update profile", err, userID)
			writeError(w, http.StatusInternalServerError, "failed to update profile")
			return
		}

		writeJSON(w, http.StatusOK, profile)
	}
}

var errInvalidPayload = errors.New("invalid request body")

func decodePatchPayload(r *http.Request) (user.ProfileUpdateInput, error) {
	var (
		input user.ProfileUpdateInput
		body  struct {
			Bio       *string          `json:"bio"`
			Birthdate *json.RawMessage `json:"birthdate"`
		}
	)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return input, err
		}
		return input, errInvalidPayload
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return input, errInvalidPayload
	}
	if body.Bio == nil && body.Birthdate == nil {
		return input, errInvalidPayload
	}

	input.Bio = body.Bio

	if body.Birthdate != nil {
		patch := &user.BirthdatePatch{IsSet: true}
		if string(*body.Birthdate) != "null" {
			var raw string
			if err := json.Unmarshal(*body.Birthdate, &raw); err != nil {
				return input, errInvalidPayload
			}
			t, err := time.Parse(dateLayout, raw)
			if err != nil {
				return input, errInvalidPayload
			}
			patch.Value = ptrTime(t)
		}
		input.Birthdate = patch
	}

	return input, nil
}

func ptrTime(t time.Time) *time.Time {
	tCopy := t
	return &tCopy
}

func headerUserID(r *http.Request) string {
	if v := r.Header.Get("X-User-ID"); v != "" {
		return v
	}
	return r.Header.Get("x-user-id")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func logRequestError(ctx context.Context, logger *slog.Logger, message string, err error, userID string) {
	if logger == nil || err == nil {
		return
	}
	attrs := []any{
		slog.String("userId", userID),
		slog.Any("error", err),
	}
	if reqID := middleware.GetReqID(ctx); reqID != "" {
		attrs = append(attrs, slog.String("requestId", reqID))
	}
	logger.Error(message, attrs...)
}
