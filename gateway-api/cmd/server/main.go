package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
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
	Audience     string
	Issuer       string
	ActivityURL  string
	UserURL      string
	SessionURL   string
	MediaURL     string
	AnalyticsURL string
	WebhookURL   string
}

func loadConfig() (config, error) {
	cfg := config{
		Port:         envconfig.Get("PORT", "8080"),
		JWKSURL:      envconfig.Get("CLERK_JWKS_URL", ""),
		Audience:     envconfig.Get("CLERK_AUDIENCE", ""),
		Issuer:       envconfig.Get("CLERK_ISSUER", ""),
		ActivityURL:  envconfig.Get("ACTIVITY_URL", "http://activity-service:8080"),
		UserURL:      envconfig.Get("USER_URL", "http://user-service:8080"),
		SessionURL:   envconfig.Get("SESSION_URL", "http://session-service:8080"),
		MediaURL:     envconfig.Get("MEDIA_URL", "http://media-service:8080"),
		AnalyticsURL: envconfig.Get("ANALYTICS_URL", "http://analytics-service:8080"),
		WebhookURL:   envconfig.Get("WEBHOOK_URL", "http://webhook-service:8080"),
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

	// Initialize JWT verifier for authentication
	verifier, err := sharedauth.NewVerifier(sharedauth.Config{
		Mode:     sharedauth.ModeClerk,
		JWKSURL:  cfg.JWKSURL,
		Audience: cfg.Audience,
		Issuer:   cfg.Issuer,
	})
	if err != nil {
		panic(fmt.Errorf("auth verifier error: %w", err))
	}

	router := sharedserver.NewRouter("gateway-api", func(r chi.Router) {
		// Public routes (no authentication required)
		r.Route("/public", func(r chi.Router) {
			// Add any public endpoints here if needed
		})

		// Protected routes with authentication middleware
		r.Group(func(r chi.Router) {
			r.Use(sharedauth.Middleware(verifier))
			r.Use(proxyMiddleware(cfg, logger))

			r.Route("/v1", func(r chi.Router) {
				// Activity Service Routes
				r.Route("/productivities", func(r chi.Router) {
					h := proxyHandler(cfg.ActivityURL, "/v1/productivities", logger)
					r.Get("/", h)
					r.Post("/", h)
					r.Get("/*", h)
					r.Delete("/*", h)
				})

				// Chatbot Routes
				r.Route("/chatbot", func(r chi.Router) {
					h := proxyHandler(cfg.ActivityURL, "/v1/chatbot", logger)
					r.Get("/", h)
					r.Post("/", h)
					r.Get("/*", h)
					r.Post("/*", h)
				})

				// Analytics Routes
				r.Route("/analytics", func(r chi.Router) {
					h := proxyHandler(cfg.ActivityURL, "/v1/analytics", logger)
					r.Get("/", h)
					r.Get("/*", h)
				})

				// User Profile Routes
				r.Route("/users", func(r chi.Router) {
					h := proxyHandler(cfg.UserURL, "/v1/users", logger)
					r.Get("/", h)
					r.Post("/", h)
					r.Put("/", h)
					r.Delete("/", h)
					r.Get("/*", h)
					r.Post("/*", h)
					r.Put("/*", h)
					r.Delete("/*", h)
				})

				// Session Service Routes
				r.Route("/sessions", func(r chi.Router) {
					h := proxyHandler(cfg.SessionURL, "/v1/sessions", logger)
					r.Get("/", h)
					r.Post("/", h)
					r.Put("/", h)
					r.Delete("/", h)
					r.Get("/*", h)
					r.Post("/*", h)
					r.Put("/*", h)
					r.Delete("/*", h)
				})

				// Media Service Routes
				r.Route("/media", func(r chi.Router) {
					h := proxyHandler(cfg.MediaURL, "/v1/media", logger)
					r.Get("/", h)
					r.Post("/", h)
					r.Delete("/", h)
					r.Get("/*", h)
					r.Post("/*", h)
					r.Delete("/*", h)
				})

				// Webhook Service Routes
				r.Route("/webhooks", func(r chi.Router) {
					h := proxyHandler(cfg.WebhookURL, "/v1/webhooks", logger)
					r.Post("/", h)
					r.Post("/*", h)
				})
			})
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

// proxyMiddleware adds user context to headers for downstream services
func proxyMiddleware(cfg config, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add user context to headers for downstream services
			if user, ok := sharedauth.UserFromContext(r.Context()); ok {
				r.Header.Set("X-User-ID", user.UserID)
				r.Header.Set("X-User-Session-ID", user.SessionID)
				logger.Info("proxying request", "user_id", user.UserID, "path", r.URL.Path)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// proxyHandler creates a reverse proxy handler for the given target URL
func proxyHandler(targetURL, pathPrefix string, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Create a new request to the target service
		// Chi strips the matched route prefix, so we need to reconstruct the full path
		wildcardPath := chi.URLParam(r, "*")
		fullPath := pathPrefix
		if wildcardPath != "" {
			fullPath = pathPrefix + "/" + wildcardPath
		}
		target := targetURL + fullPath

		// Add query parameters if present
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}

		logger.Info("proxy target", "url", target, "method", r.Method, "original_path", r.URL.Path, "wildcard", wildcardPath)

		// Create authenticated HTTP client for Cloud Run service-to-service calls
		ctx := r.Context()
		// The audience for the token MUST be the full URL of the request.
		client, err := idtoken.NewClient(ctx, target)
		if err != nil {
			http.Error(w, "Internal Server Error creating auth client", http.StatusInternalServerError)
			return
		}

		// Create new request
		req, err := http.NewRequestWithContext(ctx, r.Method, target, r.Body)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Copy headers from original request
		for key, values := range r.Header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}

		// Log the complete outgoing request details for debugging
		var headersBuilder strings.Builder
		for key, values := range req.Header {
			headersBuilder.WriteString(fmt.Sprintf("%s: %s\n", key, strings.Join(values, ", ")))
		}
		logger.Info("sending request to downstream service",
			"target", target,
			"method", req.Method,
			"headers", headersBuilder.String(),
		)

		// Make request to target service
		resp, err := client.Do(req)
		if err != nil {
			logger.Error("failed to make request to downstream service", "error", err, "target", target)
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()

		// Read the body of the response
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Error("failed to read downstream response body", "error", err, "target", target)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Log the downstream response details
		logger.Info("received response from downstream", "target", target, "status_code", resp.StatusCode, "body", string(body))

		// Copy response headers
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		// Set status code and write the body we already read
		w.WriteHeader(resp.StatusCode)
		w.Write(body)
	}
}
