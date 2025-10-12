package productivity

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// ===== Clock =====

type systemClock struct{}

// NewSystemClock returns a Clock implementation backed by time.Now.
func NewSystemClock() Clock {
	return systemClock{}
}

func (systemClock) Now() time.Time {
	return time.Now()
}

// ===== ID Generator =====

type uuidGenerator struct{}

// NewUUIDGenerator returns an IDGenerator that produces v7 UUIDs where available, falling back to v4.
func NewUUIDGenerator() IDGenerator {
	return uuidGenerator{}
}

func (uuidGenerator) NewID() string {
	if id, err := uuid.NewV7(); err == nil {
		return id.String()
	}
	return uuid.NewString()
}

// ===== Cursor Token Helpers =====
//
// We encode page cursors as URL-safe base64 of:
//   v1|<RFC3339Nano timestamp>|<docID>
//
// - Use RFC3339Nano to avoid 32/64-bit time representation issues.
// - URL-safe base64 without padding so tokens are pretty in URLs.
//

const tokenVersion = "v1"

func encodePageToken(anchor time.Time, docID string) string {
	raw := strings.Join([]string{
		tokenVersion,
		anchor.UTC().Format(time.RFC3339Nano),
		docID,
	}, "|")
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// decodePageToken parses a previously produced page token.
// Returns (anchor, docID, ok, err).
func decodePageToken(token string) (time.Time, string, bool, error) {
	if token == "" {
		return time.Time{}, "", false, nil
	}
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return time.Time{}, "", false, fmt.Errorf("invalid pageToken encoding: %w", err)
	}
	parts := strings.Split(string(b), "|")
	if len(parts) != 3 {
		return time.Time{}, "", false, errors.New("invalid pageToken format")
	}
	if parts[0] != tokenVersion {
		return time.Time{}, "", false, fmt.Errorf("unsupported pageToken version: %s", parts[0])
	}
	t, err := time.Parse(time.RFC3339Nano, parts[1])
	if err != nil {
		return time.Time{}, "", false, fmt.Errorf("invalid pageToken timestamp: %w", err)
	}
	docID := parts[2]
	if strings.TrimSpace(docID) == "" {
		return time.Time{}, "", false, errors.New("invalid pageToken docID")
	}
	return t, docID, true, nil
}
