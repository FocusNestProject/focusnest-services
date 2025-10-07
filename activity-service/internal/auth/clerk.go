package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/golang-jwt/jwt/v5"
)

var errMissingSubject = errors.New("token missing subject claim")

// clerkVerifier validates Clerk-issued JWTs using JWKS.
type clerkVerifier struct {
	jwks     *keyfunc.JWKS
	audience string
	issuer   string
}

func newClerkVerifier(cfg Config) (Verifier, error) {
	options := keyfunc.Options{RefreshErrorHandler: func(err error) {
		// Intentionally swallow refresh errors; the handler will log downstream if required.
	}}

	jwks, err := keyfunc.Get(cfg.JWKSURL, options)
	if err != nil {
		return nil, fmt.Errorf("failed to load JWKS: %w", err)
	}

	return &clerkVerifier{jwks: jwks, audience: cfg.Audience, issuer: cfg.Issuer}, nil
}

func (v *clerkVerifier) Verify(ctx context.Context, token string) (AuthenticatedUser, error) {
	// Build parse options including issuer/audience validation when configured
	options := []jwt.ParserOption{jwt.WithLeeway(5 * time.Second)}
	if v.audience != "" {
		options = append(options, jwt.WithAudience(v.audience))
	}
	if v.issuer != "" {
		options = append(options, jwt.WithIssuer(v.issuer))
	}

	t, err := jwt.Parse(token, v.jwks.Keyfunc, options...)
	if err != nil {
		return AuthenticatedUser{}, fmt.Errorf("token verification failed: %w", err)
	}

	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return AuthenticatedUser{}, errors.New("unexpected claims type")
	}

	subjectRaw, ok := claims["sub"].(string)
	if !ok || subjectRaw == "" {
		return AuthenticatedUser{}, errMissingSubject
	}

	sessionID, _ := claims["sid"].(string)

	expiresAt := int64(0)
	if expRaw, ok := claims["exp"].(float64); ok {
		expiresAt = int64(expRaw)
	}

	return AuthenticatedUser{
		UserID:    subjectRaw,
		SessionID: sessionID,
		ExpiresAt: expiresAt,
		Token:     token,
	}, nil
}
