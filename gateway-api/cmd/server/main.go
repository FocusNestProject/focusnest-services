package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

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
		// Health check endpoint
		r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"ok","service":"gateway-api","version":"1.0.0"}`)
		})

		// Public routes (no authentication required)
		r.Route("/public", func(r chi.Router) {
			// Add any public endpoints here if needed
		})

		// Protected routes with authentication middleware
		r.Group(func(r chi.Router) {
			r.Use(sharedauth.Middleware(verifier))
			r.Use(proxyMiddleware(cfg, logger))

			// Activity Service Routes
			r.Route("/v1/productivities", func(r chi.Router) {
				r.Get("/*", proxyHandler(cfg.ActivityURL, "/v1/productivities"))
				r.Post("/*", proxyHandler(cfg.ActivityURL, "/v1/productivities"))
				r.Delete("/*", proxyHandler(cfg.ActivityURL, "/v1/productivities"))
			})

			// Chatbot Routes
			r.Route("/v1/chatbot", func(r chi.Router) {
				r.Get("/*", proxyHandler(cfg.ActivityURL, "/v1/chatbot"))
				r.Post("/*", proxyHandler(cfg.ActivityURL, "/v1/chatbot"))
			})

			// Analytics Routes
			r.Route("/v1/analytics", func(r chi.Router) {
				r.Get("/*", proxyHandler(cfg.ActivityURL, "/v1/analytics"))
			})

			// User Profile Routes
			r.Route("/v1/users", func(r chi.Router) {
				r.Get("/*", proxyHandler(cfg.ActivityURL, "/v1/users"))
				r.Post("/*", proxyHandler(cfg.ActivityURL, "/v1/users"))
				r.Put("/*", proxyHandler(cfg.ActivityURL, "/v1/users"))
				r.Delete("/*", proxyHandler(cfg.ActivityURL, "/v1/users"))
			})

			// Session Service Routes
			r.Route("/v1/sessions", func(r chi.Router) {
				r.Get("/*", proxyHandler(cfg.SessionURL, "/v1/sessions"))
				r.Post("/*", proxyHandler(cfg.SessionURL, "/v1/sessions"))
				r.Put("/*", proxyHandler(cfg.SessionURL, "/v1/sessions"))
				r.Delete("/*", proxyHandler(cfg.SessionURL, "/v1/sessions"))
			})

			// Media Service Routes
			r.Route("/v1/media", func(r chi.Router) {
				r.Get("/*", proxyHandler(cfg.MediaURL, "/v1/media"))
				r.Post("/*", proxyHandler(cfg.MediaURL, "/v1/media"))
				r.Delete("/*", proxyHandler(cfg.MediaURL, "/v1/media"))
			})

			// Webhook Service Routes
			r.Route("/v1/webhooks", func(r chi.Router) {
				r.Post("/*", proxyHandler(cfg.WebhookURL, "/v1/webhooks"))
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
func proxyHandler(targetURL, pathPrefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Create a new request to the target service
		target := targetURL + r.URL.Path

		// Create HTTP client with timeout
		client := &http.Client{
			Timeout: 30 * time.Second,
		}

		// Create new request
		req, err := http.NewRequest(r.Method, target, r.Body)
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

		// Make request to target service
		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()

		// Copy response headers
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}

		// Set status code and copy body
		w.WriteHeader(resp.StatusCode)

		// Copy response body
		buf := make([]byte, 32*1024)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
			}
			if err != nil {
				break
			}
		}
	}
}
