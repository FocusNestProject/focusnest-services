package revenuecat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const baseURL = "https://api.revenuecat.com/v1"

// Client checks RevenueCat subscriber entitlements (server-side).
type Client struct {
	httpClient   *http.Client
	secretKey    string
	entitlementID string
}

// NewClient creates a RevenueCat API client. secretKey and entitlementID must be non-empty.
func NewClient(secretKey, entitlementID string) *Client {
	return &Client{
		httpClient:    &http.Client{Timeout: 10 * time.Second},
		secretKey:     strings.TrimSpace(secretKey),
		entitlementID: strings.TrimSpace(entitlementID),
	}
}

// subscriberResponse matches RevenueCat API v1 GET /subscribers/{app_user_id} response.
type subscriberResponse struct {
	Subscriber struct {
		Entitlements map[string]struct {
			ExpiresDate string `json:"expires_date"`
		} `json:"entitlements"`
	} `json:"subscriber"`
}

// HasEntitlement returns true if the given app user ID has an active entitlement (expires_date null or in future).
// Returns false on API error or missing/inactive entitlement.
func (c *Client) HasEntitlement(ctx context.Context, appUserID string) (bool, error) {
	if c.secretKey == "" || c.entitlementID == "" || strings.TrimSpace(appUserID) == "" {
		return false, nil
	}

	url := baseURL + "/subscribers/" + appUserID
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Authorization", "Bearer "+c.secretKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("revenuecat api status %d", resp.StatusCode)
	}

	var body subscriberResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return false, err
	}

	ent, ok := body.Subscriber.Entitlements[c.entitlementID]
	if !ok {
		return false, nil
	}
	// null or empty expires_date = lifetime
	if ent.ExpiresDate == "" {
		return true, nil
	}
	t, err := time.Parse(time.RFC3339, ent.ExpiresDate)
	if err != nil {
		return false, nil
	}
	return time.Now().Before(t), nil
}
