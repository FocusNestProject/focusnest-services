package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"google.golang.org/api/idtoken"

	"github.com/focusnest/gateway-api/internal/revenuecat"
	"github.com/focusnest/shared-libs/auth"
)

type Targets struct {
	Activity  *url.URL
	User      *url.URL
	Analytics *url.URL
	Chatbot   *url.URL
}

func Router(verifier auth.Verifier, targets Targets, premiumChecker *revenuecat.Client, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))

	// Unauthenticated health.
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	// Everything else is authenticated.
	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(verifier))
		r.Use(injectUserIDHeader())

		// Routing table (matches the root handbook).
		r.Mount("/v1/productivities", proxyHandler(targets.Activity, nil, logger))
		r.Mount("/v1/progress", proxyHandler(targets.Analytics, premiumChecker, logger))
		r.Mount("/v1/chatbot", proxyHandler(targets.Chatbot, nil, logger))
		r.Mount("/v1/users", proxyHandler(targets.User, nil, logger))
		r.Mount("/v1/challenges", proxyHandler(targets.User, nil, logger))
		r.Mount("/v1/shares", proxyHandler(targets.User, nil, logger))
		r.Mount("/v1/mindfulness", proxyHandler(targets.User, nil, logger))
	})

	return r
}

func injectUserIDHeader() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if u, ok := auth.UserFromContext(r.Context()); ok && strings.TrimSpace(u.UserID) != "" {
				r.Header.Set("X-User-ID", u.UserID)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func proxyHandler(target *url.URL, premiumChecker *revenuecat.Client, logger *slog.Logger) http.Handler {
	if target == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "upstream not configured", http.StatusBadGateway)
		})
	}

	// Create ID token source for service-to-service authentication
	audience := target.Scheme + "://" + target.Host
	tokenSource, err := idtoken.NewTokenSource(context.Background(), audience)
	if err != nil {
		if logger != nil {
			logger.Error("failed to create token source", slog.Any("error", err), slog.String("audience", audience))
		}
		// Continue without token source - will fail if backend requires auth
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origDirector(req)
		// Ensure the upstream sees the right Host and preserve original path.
		req.Host = target.Host

		// For POST /v1/progress/streak/recover: verify premium with RevenueCat and set X-Premium
		if premiumChecker != nil && req.Method == http.MethodPost && strings.Contains(req.URL.Path, "/streak/recover") {
			userID := req.Header.Get("X-User-ID")
			if userID != "" {
				ok, err := premiumChecker.HasEntitlement(req.Context(), userID)
				if err != nil && logger != nil {
					logger.Warn("revenuecat entitlement check failed", slog.String("user_id", userID), slog.Any("error", err))
				}
				if ok {
					req.Header.Set("X-Premium", "true")
				}
			}
		}

		// Add ID token for service-to-service authentication
		if tokenSource != nil {
			token, err := tokenSource.Token()
			if err != nil {
				if logger != nil {
					logger.Error("failed to get ID token", slog.Any("error", err))
				}
			} else {
				req.Header.Set("Authorization", "Bearer "+token.AccessToken)
			}
		}
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if logger != nil && err != nil {
			logger.Error("proxy error", slog.Any("error", err), slog.String("path", r.URL.Path))
		}
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}

	return proxy
}

