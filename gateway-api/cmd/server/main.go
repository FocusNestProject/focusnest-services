package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/go-chi/chi/v5"
	"google.golang.org/api/idtoken"

	sharedauth "github.com/focusnest/shared-libs/auth"
	"github.com/focusnest/shared-libs/envconfig"
	"github.com/focusnest/shared-libs/logging"
	sharedserver "github.com/focusnest/shared-libs/server"
)

type config struct {
	Port         string `validate:"required"`
	JWKSURL      string
	Issuer       string
	ActivityURL  string // productivities
	UserURL      string // users
	AnalyticsURL string // progress
	ChatbotURL   string // chatbot
}

func loadConfig() (config, error) {
	cfg := config{
		Port:         envconfig.Get("PORT", "8080"),
		JWKSURL:      envconfig.Get("CLERK_JWKS_URL", ""),
		Issuer:       envconfig.Get("CLERK_ISSUER", ""),
		ActivityURL:  envconfig.Get("ACTIVITY_URL", "http://focus-service:8080"),
		UserURL:      envconfig.Get("USER_URL", "http://user-service:8080"),
		AnalyticsURL: envconfig.Get("ANALYTICS_URL", "http://progress-service:8080"),
		ChatbotURL:   envconfig.Get("CHATBOT_URL", "http://chatbot-service:8080"),
	}
	return cfg, envconfig.Validate(cfg)
}

func main() {
	ctx := context.Background()
	cfg, err := loadConfig()
	if err != nil {
		panic(fmt.Errorf("config error: %w", err))
	}

	logger := logging.NewLogger("gateway-api")

	verifier, err := sharedauth.NewVerifier(sharedauth.Config{
		Mode:    sharedauth.ModeClerk,
		JWKSURL: cfg.JWKSURL,
		Issuer:  cfg.Issuer,
	})
	if err != nil {
		panic(fmt.Errorf("auth verifier error: %w", err))
	}

	router := sharedserver.NewRouter("gateway-api", func(r chi.Router) {
		// Public: add if needed under /public
		r.Route("/public", func(r chi.Router) {})

		// Protected
		r.Group(func(r chi.Router) {
			r.Use(sharedauth.Middleware(verifier))
			r.Use(userHeadersMiddleware(logger))

			// Map each subtree to its upstream. We forward the full original path/query.
			r.Mount("/v1/productivities", proxyTo(cfg.ActivityURL, logger))
			r.Mount("/v1/progress", proxyTo(cfg.AnalyticsURL, logger))
			r.Mount("/v1/chatbot", proxyTo(cfg.ChatbotURL, logger))
			r.Mount("/v1/users", proxyTo(cfg.UserURL, logger))
		})
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	if err := sharedserver.Run(ctx, srv, logger); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}

// Adds user context headers for downstream services.
func userHeadersMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if user, ok := sharedauth.UserFromContext(r.Context()); ok {
				r.Header.Set("X-User-ID", user.UserID)
				r.Header.Set("X-User-Session-ID", user.SessionID)
				logger.Info("proxying request", "user_id", user.UserID, "path", r.URL.Path)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// proxyTo forwards requests to the given origin, preserving the original path and query.
// It mints a Google ID token for the origin's audience (Cloud Run service-to-service).
func proxyTo(origin string, logger *slog.Logger) http.Handler {
	parsedOrigin, err := url.Parse(origin)
	if err != nil {
		panic(fmt.Errorf("invalid upstream origin %q: %w", origin, err))
	}
	audience := parsedOrigin.Scheme + "://" + parsedOrigin.Host

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create an ID-token-authenticated client for this audience.
		client, err := idtoken.NewClient(r.Context(), audience)
		if err != nil {
			logger.Error("idtoken client error", "audience", audience, "err", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Build target URL: origin + original path + query.
		targetURL := &url.URL{
			Scheme:   parsedOrigin.Scheme,
			Host:     parsedOrigin.Host,
			Path:     r.URL.Path,     // preserve full path (already includes /v1/...)
			RawQuery: r.URL.RawQuery, // preserve query
		}

		// Prepare outgoing request with same method/body and copied headers (except hop-by-hop).
		req, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL.String(), r.Body)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		copyHeaders(req.Header, r.Header)

		// Do the request
		resp, err := client.Do(req)
		if err != nil {
			logger.Error("downstream request failed", "target", targetURL.String(), "err", err)
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Error("read downstream body failed", "target", targetURL.String(), "err", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Mirror response
		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(body)
	})
}

func copyHeaders(dst, src http.Header) {
	// Hop-by-hop headers to skip
	skip := map[string]struct{}{
		"Host":                {},
		"Authorization":       {}, // idtoken client sets its own auth
		"Connection":          {},
		"Keep-Alive":          {},
		"Proxy-Authenticate":  {},
		"Proxy-Authorization": {},
		"Te":                  {},
		"Trailer":             {},
		"Transfer-Encoding":   {},
		"Upgrade":             {},
	}
	for k, vv := range src {
		if _, found := skip[k]; found {
			continue
		}
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
