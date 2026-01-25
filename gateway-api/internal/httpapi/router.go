package httpapi

import (
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/focusnest/shared-libs/auth"
)

type Targets struct {
	Activity  *url.URL
	User      *url.URL
	Analytics *url.URL
	Chatbot   *url.URL
}

func Router(verifier auth.Verifier, targets Targets, logger *slog.Logger) http.Handler {
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
		r.Mount("/v1/productivities", proxyHandler(targets.Activity, logger))
		r.Mount("/v1/progress", proxyHandler(targets.Analytics, logger))
		r.Mount("/v1/chatbot", proxyHandler(targets.Chatbot, logger))
		r.Mount("/v1/users", proxyHandler(targets.User, logger))
		r.Mount("/v1/challenges", proxyHandler(targets.User, logger))
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

func proxyHandler(target *url.URL, logger *slog.Logger) http.Handler {
	if target == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "upstream not configured", http.StatusBadGateway)
		})
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origDirector(req)
		// Ensure the upstream sees the right Host and preserve original path.
		req.Host = target.Host
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		if logger != nil && err != nil {
			logger.Error("proxy error", slog.Any("error", err), slog.String("path", r.URL.Path))
		}
		http.Error(w, "bad gateway", http.StatusBadGateway)
	}

	return proxy
}

