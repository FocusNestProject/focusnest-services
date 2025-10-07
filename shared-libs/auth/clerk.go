package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var errMissingSubject = errors.New("token missing subject claim")
var errMissingKeyID = errors.New("token missing kid header")

// clerkVerifier validates Clerk-issued JWTs using JWKS.
type clerkVerifier struct {
	jwksURL       string
	audience      string
	issuer        string
	client        *http.Client
	cacheDuration time.Duration

	mu         sync.RWMutex
	keys       map[string]*rsa.PublicKey
	lastLoaded time.Time
}

func newClerkVerifier(cfg Config) (Verifier, error) {

	if cfg.JWKSURL == "" {
		return nil, fmt.Errorf("clerk JWKS URL is required")
	}

	return &clerkVerifier{
		jwksURL:       cfg.JWKSURL,
		audience:      cfg.Audience,
		issuer:        cfg.Issuer,
		client:        &http.Client{Timeout: 5 * time.Second},
		cacheDuration: 10 * time.Minute,
		keys:          make(map[string]*rsa.PublicKey),
	}, nil
}

func (v *clerkVerifier) Verify(ctx context.Context, token string) (AuthenticatedUser, error) {
	options := []jwt.ParserOption{jwt.WithLeeway(5 * time.Second)}
	if v.audience != "" {
		options = append(options, jwt.WithAudience(v.audience))
	}
	if v.issuer != "" {
		options = append(options, jwt.WithIssuer(v.issuer))
	}

	t, err := jwt.Parse(token, v.keyFunc(ctx), options...)
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

func (v *clerkVerifier) keyFunc(ctx context.Context) jwt.Keyfunc {
	return func(t *jwt.Token) (any, error) {
		kid, _ := t.Header["kid"].(string)
		if kid == "" {
			return nil, errMissingKeyID
		}

		if key, ok := v.lookupKey(kid); ok {
			return key, nil
		}

		if err := v.refreshKeys(ctx); err != nil {
			return nil, err
		}

		if key, ok := v.lookupKey(kid); ok {
			return key, nil
		}

		return nil, fmt.Errorf("jwks key %s not found", kid)
	}
}

func (v *clerkVerifier) lookupKey(kid string) (*rsa.PublicKey, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	key, ok := v.keys[kid]
	return key, ok
}

func (v *clerkVerifier) refreshKeys(ctx context.Context) error {
	v.mu.RLock()
	if time.Since(v.lastLoaded) < v.cacheDuration && len(v.keys) > 0 {
		v.mu.RUnlock()
		return nil
	}
	v.mu.RUnlock()

	v.mu.Lock()
	defer v.mu.Unlock()

	if time.Since(v.lastLoaded) < v.cacheDuration && len(v.keys) > 0 {
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("create jwks request: %w", err)
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("fetch jwks: unexpected status %d", resp.StatusCode)
	}

	var document jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&document); err != nil {
		return fmt.Errorf("decode jwks: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(document.Keys))
	for _, key := range document.Keys {
		if key.Kty != "RSA" {
			continue
		}

		pubKey, err := key.rsaPublicKey()
		if err != nil {
			return fmt.Errorf("parse jwks key %s: %w", key.Kid, err)
		}
		keys[key.Kid] = pubKey
	}

	if len(keys) == 0 {
		return errors.New("jwks contained no supported keys")
	}

	v.keys = keys
	v.lastLoaded = time.Now()
	return nil
}

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func (j jwk) rsaPublicKey() (*rsa.PublicKey, error) {
	if j.N == "" || j.E == "" {
		return nil, errors.New("missing modulus or exponent")
	}

	nBytes, err := base64.RawURLEncoding.DecodeString(j.N)
	if err != nil {
		return nil, fmt.Errorf("invalid modulus: %w", err)
	}

	eBytes, err := base64.RawURLEncoding.DecodeString(j.E)
	if err != nil {
		return nil, fmt.Errorf("invalid exponent: %w", err)
	}

	var eInt int
	for _, b := range eBytes {
		eInt = eInt<<8 + int(b)
	}
	if eInt == 0 {
		return nil, errors.New("invalid exponent value")
	}

	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: eInt,
	}, nil
}
