package feedback

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ResendClient is a lightweight HTTP client for the Resend email API.
// No SDK needed — just a simple POST to https://api.resend.com/emails.
type ResendClient struct {
	apiKey     string
	httpClient *http.Client
}

// ResendEmail represents the payload sent to the Resend API.
type ResendEmail struct {
	From        string             `json:"from"`
	To          []string           `json:"to"`
	Subject     string             `json:"subject"`
	HTML        string             `json:"html"`
	Attachments []ResendAttachment `json:"attachments,omitempty"`
}

// ResendAttachment represents a file attachment (base64 encoded).
type ResendAttachment struct {
	Filename string `json:"filename"`
	Content  string `json:"content"` // base64-encoded content
}

// NewResendClient creates a new Resend API client.
func NewResendClient(apiKey string) *ResendClient {
	return &ResendClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Send sends an email via the Resend API.
func (c *ResendClient) Send(email ResendEmail) error {
	body, err := json.Marshal(email)
	if err != nil {
		return fmt.Errorf("marshal email: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend API error (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}
