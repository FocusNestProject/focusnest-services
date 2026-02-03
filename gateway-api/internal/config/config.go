package config

import (
	"fmt"
	"net/url"
	"strings"
)

type Config struct {
	Port string

	AuthMode     string
	ClerkJWKSURL string
	ClerkAudience string
	ClerkIssuer  string

	ActivityURL  *url.URL
	UserURL      *url.URL
	AnalyticsURL *url.URL
	ChatbotURL   *url.URL

	// RevenueCat: optional; used to set X-Premium for POST /v1/progress/streak/recover
	RevenueCatSecretKey    string
	RevenueCatEntitlementID string
}

// ParseURLCompat parses a required absolute URL from env.
// It is intentionally strict (requires scheme + host).
func ParseURLCompat(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("missing url")
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid url: %s", raw)
	}
	return u, nil
}

