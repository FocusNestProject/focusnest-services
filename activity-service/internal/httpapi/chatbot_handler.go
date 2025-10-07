package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	sharedauth "github.com/focusnest/shared-libs/auth"

	"github.com/focusnest/activity-service/internal/chatbot"
)

// RegisterChatbotRoutes wires chatbot routes onto the provided router
func RegisterChatbotRoutes(r chi.Router, svc *chatbot.Service) {
	h := &chatbotHandler{service: svc}

	r.Route("/v1/chatbot", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.create)
		r.Post("/ask", h.ask)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.get)
			r.Delete("/", h.delete)
		})
	})
}

type chatbotHandler struct {
	service *chatbot.Service
}

type chatResponse struct {
	ID        string            `json:"id"`
	UserID    string            `json:"userId"`
	Title     string            `json:"title"`
	Messages  []messageResponse `json:"messages"`
	CreatedAt string            `json:"createdAt"`
	UpdatedAt string            `json:"updatedAt"`
}

type messageResponse struct {
	ID        string `json:"id"`
	Role      string `json:"role"`
	Content   string `json:"content"`
	Timestamp string `json:"timestamp"`
}

type listChatResponse struct {
	Data       []chatResponse   `json:"data"`
	Pagination chatbot.PageInfo `json:"pagination"`
}

type createChatRequest struct {
	Title    string           `json:"title"`
	Messages []messageRequest `json:"messages"`
}

type messageRequest struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type askRequest struct {
	Message string `json:"message"`
}

type askResponse struct {
	Message   string `json:"message"`
	SessionID string `json:"sessionId"`
}

func (h *chatbotHandler) list(w http.ResponseWriter, r *http.Request) {
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

	entries, pageInfo, err := h.service.List(r.Context(), user.UserID, chatbot.Pagination{Page: page, PageSize: pageSize})
	if err != nil {
		respondChatbotServiceError(w, err)
		return
	}

	payload := listChatResponse{
		Data:       make([]chatResponse, len(entries)),
		Pagination: pageInfo,
	}

	for i, entry := range entries {
		payload.Data[i] = mapChatEntry(entry)
	}

	writeJSON(w, http.StatusOK, payload)
}

func (h *chatbotHandler) create(w http.ResponseWriter, r *http.Request) {
	user, ok := sharedauth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body createChatRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	// Convert request messages to domain messages
	messages := make([]chatbot.Message, len(body.Messages))
	for i, msg := range body.Messages {
		messages[i] = chatbot.Message{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	input := chatbot.CreateInput{
		UserID:   user.UserID,
		Title:    body.Title,
		Messages: messages,
	}

	entry, err := h.service.Create(r.Context(), input)
	if err != nil {
		respondChatbotServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, mapChatEntry(entry))
}

func (h *chatbotHandler) ask(w http.ResponseWriter, r *http.Request) {
	user, ok := sharedauth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var body askRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON payload")
		return
	}

	input := chatbot.AskInput{
		UserID:  user.UserID,
		Message: body.Message,
	}

	response, err := h.service.Ask(r.Context(), input)
	if err != nil {
		respondChatbotServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, askResponse{
		Message:   response.Message,
		SessionID: response.SessionID,
	})
}

func (h *chatbotHandler) get(w http.ResponseWriter, r *http.Request) {
	user, ok := sharedauth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	entry, err := h.service.Get(r.Context(), user.UserID, id)
	if err != nil {
		respondChatbotServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, mapChatEntry(entry))
}

func (h *chatbotHandler) delete(w http.ResponseWriter, r *http.Request) {
	user, ok := sharedauth.UserFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	id := chi.URLParam(r, "id")
	if err := h.service.Delete(r.Context(), user.UserID, id); err != nil {
		respondChatbotServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func respondChatbotServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, chatbot.ErrNotFound):
		writeError(w, http.StatusNotFound, "chatbot entry not found")
	case errors.Is(err, chatbot.ErrConflict):
		writeError(w, http.StatusConflict, "chatbot entry already exists")
	case errors.Is(err, chatbot.ErrInvalidInput):
		message := err.Error()
		if idx := strings.Index(message, ":"); idx >= 0 {
			message = strings.TrimSpace(message[idx+1:])
		}
		writeError(w, http.StatusBadRequest, message)
	default:
		writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

func mapChatEntry(entry chatbot.ChatEntry) chatResponse {
	messages := make([]messageResponse, len(entry.Messages))
	for i, msg := range entry.Messages {
		messages[i] = messageResponse{
			ID:        msg.ID,
			Role:      msg.Role,
			Content:   msg.Content,
			Timestamp: msg.Timestamp.Format(time.RFC3339),
		}
	}

	return chatResponse{
		ID:        entry.ID,
		UserID:    entry.UserID,
		Title:     entry.Title,
		Messages:  messages,
		CreatedAt: entry.CreatedAt.Format(time.RFC3339),
		UpdatedAt: entry.UpdatedAt.Format(time.RFC3339),
	}
}
