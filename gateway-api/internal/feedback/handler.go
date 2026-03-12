package feedback

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
)

// FeedbackRequest is the JSON body from the mobile app.
type FeedbackRequest struct {
	Type    string           `json:"type"`    // "bug" | "feature" | "help"
	Message string           `json:"message"`
	Photo   *PhotoAttachment `json:"photo,omitempty"`
}

// PhotoAttachment carries a base64-encoded image.
type PhotoAttachment struct {
	Filename string `json:"filename"`
	Content  string `json:"content"` // base64
}

// Handler serves POST /v1/feedback.
type Handler struct {
	resend    *ResendClient
	firestore *firestore.Client // nil = skip Firestore
	recipient string
	sender    string
	logger    *slog.Logger
}

// NewHandler wires up the feedback handler.
// Pass fs=nil to disable Firestore persistence.
func NewHandler(resend *ResendClient, fs *firestore.Client, recipient, sender string, logger *slog.Logger) *Handler {
	return &Handler{
		resend:    resend,
		firestore: fs,
		recipient: recipient,
		sender:    sender,
		logger:    logger,
	}
}

// ServeHTTP handles the incoming feedback request.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	// Limit body to 10 MB to prevent abuse (base64 photo can be large).
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)

	var req FeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Validate fields.
	req.Type = strings.TrimSpace(strings.ToLower(req.Type))
	req.Message = strings.TrimSpace(req.Message)

	if req.Type == "" || req.Message == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "type and message are required"})
		return
	}

	validTypes := map[string]bool{"bug": true, "feature": true, "help": true}
	if !validTypes[req.Type] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "type must be one of: bug, feature, help"})
		return
	}

	userID := r.Header.Get("X-User-ID") // injected by auth middleware

	// ── 1) Send email via Resend ──────────────────────────────────────
	email := h.buildEmail(req, userID)
	if err := h.resend.Send(email); err != nil {
		h.logger.Error("failed to send feedback email",
			slog.String("type", req.Type),
			slog.String("user_id", userID),
			slog.Any("error", err),
		)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to send feedback"})
		return
	}

	h.logger.Info("feedback email sent",
		slog.String("type", req.Type),
		slog.String("user_id", userID),
	)

	// ── 2) Persist to Firestore (best-effort) ────────────────────────
	if h.firestore != nil {
		go h.saveFeedback(context.Background(), req, userID)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}

// buildEmail constructs the Resend email payload.
func (h *Handler) buildEmail(req FeedbackRequest, userID string) ResendEmail {
	subject := h.subject(req.Type)
	html := h.html(req, userID)

	email := ResendEmail{
		From:    h.sender,
		To:      []string{h.recipient},
		Subject: subject,
		HTML:    html,
	}

	if req.Photo != nil && req.Photo.Content != "" {
		filename := req.Photo.Filename
		if filename == "" {
			filename = "screenshot.jpg"
		}
		email.Attachments = append(email.Attachments, ResendAttachment{
			Filename: filename,
			Content:  req.Photo.Content,
		})
	}

	return email
}

func (h *Handler) subject(feedbackType string) string {
	switch feedbackType {
	case "bug":
		return "[Bug Report] Focuzen App"
	case "feature":
		return "[Feature Request] Focuzen App"
	case "help":
		return "[Help Request] Focuzen App"
	default:
		return "[Feedback] Focuzen App"
	}
}

func (h *Handler) html(req FeedbackRequest, userID string) string {
	typeLabel := strings.ToUpper(req.Type[:1]) + req.Type[1:]
	hasPhoto := "No"
	if req.Photo != nil && req.Photo.Content != "" {
		hasPhoto = fmt.Sprintf("Yes (%s)", req.Photo.Filename)
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head><meta charset="UTF-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; padding: 24px; color: #1a1a1a; background: #f9fafb;">
  <div style="max-width: 600px; margin: 0 auto; background: #ffffff; border-radius: 12px; padding: 32px; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
    <h2 style="margin: 0 0 8px 0; color: #111827;">%s</h2>
    <p style="margin: 0 0 24px 0; color: #6b7280; font-size: 14px;">From Focuzen App</p>

    <table style="width: 100%%; border-collapse: collapse; margin-bottom: 24px;">
      <tr>
        <td style="padding: 8px 12px; font-weight: 600; color: #374151; width: 120px; vertical-align: top;">Type</td>
        <td style="padding: 8px 12px; color: #111827;">%s</td>
      </tr>
      <tr style="background: #f9fafb;">
        <td style="padding: 8px 12px; font-weight: 600; color: #374151; vertical-align: top;">User ID</td>
        <td style="padding: 8px 12px; color: #111827; font-family: monospace; font-size: 13px;">%s</td>
      </tr>
      <tr>
        <td style="padding: 8px 12px; font-weight: 600; color: #374151; vertical-align: top;">Attachment</td>
        <td style="padding: 8px 12px; color: #111827;">%s</td>
      </tr>
    </table>

    <div style="background: #f3f4f6; border-radius: 8px; padding: 16px; white-space: pre-wrap; font-size: 15px; line-height: 1.6; color: #1f2937;">%s</div>

    <p style="margin: 24px 0 0 0; color: #9ca3af; font-size: 12px;">Sent via Focuzen App feedback form</p>
  </div>
</body>
</html>`, typeLabel+" Feedback", typeLabel, userID, hasPhoto, req.Message)
}

// saveFeedback stores feedback in Firestore (fire-and-forget).
func (h *Handler) saveFeedback(ctx context.Context, req FeedbackRequest, userID string) {
	doc := map[string]interface{}{
		"type":       req.Type,
		"message":    req.Message,
		"user_id":    userID,
		"has_photo":  req.Photo != nil && req.Photo.Content != "",
		"status":     "new",
		"created_at": time.Now().UTC(),
	}

	if req.Photo != nil {
		doc["photo_filename"] = req.Photo.Filename
	}

	_, _, err := h.firestore.Collection("feedbacks").Add(ctx, doc)
	if err != nil {
		h.logger.Error("failed to save feedback to firestore",
			slog.String("type", req.Type),
			slog.String("user_id", userID),
			slog.Any("error", err),
		)
	} else {
		h.logger.Info("feedback saved to firestore",
			slog.String("type", req.Type),
			slog.String("user_id", userID),
		)
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
