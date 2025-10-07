package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	sharedauth "github.com/focusnest/shared-libs/auth"

	"github.com/focusnest/activity-service/internal/user"
)

// RegisterUserRoutes wires user profile routes onto the provided router
func RegisterUserRoutes(r chi.Router, svc *user.Service) {
	h := &userHandler{service: svc}

	r.Route("/v1/users", func(r chi.Router) {
		r.Get("/profile", h.getProfile)
		r.Post("/profile", h.createProfile)
		r.Put("/profile", h.updateProfile)
		r.Delete("/profile", h.deleteProfile)
	})
}

type userHandler struct {
	service *user.Service
}

type profileResponse struct {
	ID              string  `json:"id"`
	UserID          string  `json:"userId"`
	Bio             string  `json:"bio"`
	Birthdate       *string `json:"birthdate,omitempty"`
	BackgroundImage string  `json:"backgroundImage"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

type updateProfileRequest struct {
	Bio             string  `json:"bio"`
	Birthdate       *string `json:"birthdate,omitempty"`
	BackgroundImage string  `json:"backgroundImage"`
}

func (h *userHandler) getProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := sharedauth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	profile, err := h.service.Get(r.Context(), user.UserID)
	if err != nil {
		respondUserServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, mapProfileResponse(profile))
}

func (h *userHandler) createProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := sharedauth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	profile, err := h.service.Create(r.Context(), user.UserID)
	if err != nil {
		respondUserServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, mapProfileResponse(profile))
}

func (h *userHandler) updateProfile(w http.ResponseWriter, r *http.Request) {
	authUser, ok := sharedauth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	input := user.UpdateInput{
		UserID:          authUser.UserID,
		Bio:             body.Bio,
		BackgroundImage: body.BackgroundImage,
	}

	// Parse birthdate if provided
	if body.Birthdate != nil && *body.Birthdate != "" {
		if parsed, err := time.Parse("2006-01-02", *body.Birthdate); err == nil {
			input.Birthdate = &parsed
		}
	}

	profile, err := h.service.Update(r.Context(), input)
	if err != nil {
		respondUserServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, mapProfileResponse(profile))
}

func (h *userHandler) deleteProfile(w http.ResponseWriter, r *http.Request) {
	user, ok := sharedauth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	if err := h.service.Delete(r.Context(), user.UserID); err != nil {
		respondUserServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func respondUserServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, user.ErrNotFound):
		writeError(w, http.StatusNotFound, "user profile not found")
	case errors.Is(err, user.ErrConflict):
		writeError(w, http.StatusConflict, "user profile already exists")
	case errors.Is(err, user.ErrInvalidInput):
		message := err.Error()
		if idx := strings.Index(message, ":"); idx >= 0 {
			message = strings.TrimSpace(message[idx+1:])
		}
		writeError(w, http.StatusBadRequest, message)
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func mapProfileResponse(profile user.Profile) profileResponse {
	var birthdate *string
	if profile.Birthdate != nil {
		formatted := profile.Birthdate.Format("2006-01-02")
		birthdate = &formatted
	}

	return profileResponse{
		ID:              profile.ID,
		UserID:          profile.UserID,
		Bio:             profile.Bio,
		Birthdate:       birthdate,
		BackgroundImage: profile.BackgroundImage,
		CreatedAt:       profile.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       profile.UpdatedAt.Format(time.RFC3339),
	}
}
