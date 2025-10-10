package auth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

// Mode represents the authentication strategy to apply for incoming requests.
type Mode string

const (
	// ModeClerk enables Clerk JWT verification using a JWKS endpoint.
	ModeClerk Mode = "clerk"
	// ModeNoop disables signature verification and treats the bearer token as the user ID (useful for local development and tests).
	ModeNoop Mode = "noop"
)

// Config captures the inputs required to initialize an authenticator.
type Config struct {
	Mode     Mode
	JWKSURL  string
	Audience string
	Issuer   string
}

// AuthenticatedUser represents the currently authenticated subject extracted from the bearer token.
type AuthenticatedUser struct {
	UserID    string
	SessionID string
	ExpiresAt int64
	Token     string
}

// Verifier verifies a bearer token and returns the associated user context.
type Verifier interface {
	Verify(ctx context.Context, token string) (AuthenticatedUser, error)
}

var (
	errMissingAuthHeader = errors.New("authorization header missing")
	errInvalidAuthHeader = errors.New("authorization header is malformed")
)

type ctxKey string

const userCtxKey ctxKey = "focusnest:user"

// Middleware enforces authentication for the wrapped handler using the provided verifier.
func Middleware(verifier Verifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if verifier == nil {
				next.ServeHTTP(w, r)
				return
			}

			token, err := tokenFromRequest(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			claims, err := verifier.Verify(r.Context(), token)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), userCtxKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func tokenFromRequest(r *http.Request) (string, error) {
	// Check for X-User-ID header first (for internal service-to-service calls)
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		return userID, nil
	}

	// Fall back to Authorization Bearer token
	header := r.Header.Get("Authorization")
	if header == "" {
		return "", errMissingAuthHeader
	}

	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", errInvalidAuthHeader
	}

	token := strings.TrimSpace(parts[1])
	if token == "" {
		return "", errInvalidAuthHeader
	}

	return token, nil
}

// UserFromContext extracts the authenticated user from the request context.
func UserFromContext(ctx context.Context) (AuthenticatedUser, bool) {
	value, ok := ctx.Value(userCtxKey).(AuthenticatedUser)
	return value, ok
}

// NewVerifier constructs a Verifier matching the supplied configuration.
func NewVerifier(cfg Config) (Verifier, error) {
	switch cfg.Mode {
	case ModeClerk:
		return newClerkVerifier(cfg)
	case ModeNoop:
		return newNoopVerifier(cfg), nil
	default:
		return nil, fmt.Errorf("unsupported auth mode: %s", cfg.Mode)
	}
}
