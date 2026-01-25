package main

import (
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/focusnest/gateway-api/internal/config"
	"github.com/focusnest/gateway-api/internal/httpapi"
	"github.com/focusnest/shared-libs/auth"
	"github.com/focusnest/shared-libs/envconfig"
	"github.com/focusnest/shared-libs/logging"
)

func main() {
	cfg := mustLoad()
	logger := logging.NewLogger("gateway-api")

	verifier, err := auth.NewVerifier(auth.Config{
		Mode:     auth.Mode(strings.ToLower(strings.TrimSpace(cfg.AuthMode))),
		JWKSURL:  cfg.ClerkJWKSURL,
		Audience: cfg.ClerkAudience,
		Issuer:   cfg.ClerkIssuer,
	})
	if err != nil {
		logger.Error("failed to init auth verifier", slog.Any("error", err))
		os.Exit(1)
	}

	handler := httpapi.Router(verifier, httpapi.Targets{
		Activity:  cfg.ActivityURL,
		User:      cfg.UserURL,
		Analytics: cfg.AnalyticsURL,
		Chatbot:   cfg.ChatbotURL,
	}, logger)

	addr := ":" + cfg.Port
	logger.Info("gateway starting", slog.String("addr", addr))
	if err := http.ListenAndServe(addr, handler); err != nil {
		logger.Error("gateway crashed", slog.Any("error", err))
		os.Exit(1)
	}
}

func mustLoad() config.Config {
	port := envconfig.Get("PORT", "8080")
	authMode := envconfig.Get("AUTH_MODE", "clerk")

	activityRaw := envconfig.Get("ACTIVITY_URL", envconfig.Get("FOCUS_URL", ""))
	userRaw := envconfig.Get("USER_URL", "")
	analyticsRaw := envconfig.Get("ANALYTICS_URL", envconfig.Get("PROGRESS_URL", ""))
	chatbotRaw := envconfig.Get("CHATBOT_URL", "")

	activityURL, err := config.ParseURLCompat(activityRaw)
	if err != nil {
		panic("ACTIVITY_URL invalid: " + err.Error())
	}
	userURL, err := config.ParseURLCompat(userRaw)
	if err != nil {
		panic("USER_URL invalid: " + err.Error())
	}
	analyticsURL, err := config.ParseURLCompat(analyticsRaw)
	if err != nil {
		panic("ANALYTICS_URL invalid: " + err.Error())
	}
	chatbotURL, err := config.ParseURLCompat(chatbotRaw)
	if err != nil {
		panic("CHATBOT_URL invalid: " + err.Error())
	}

	return config.Config{
		Port:          port,
		AuthMode:      authMode,
		ClerkJWKSURL:  envconfig.Get("CLERK_JWKS_URL", ""),
		ClerkAudience: envconfig.Get("CLERK_AUDIENCE", ""),
		ClerkIssuer:   envconfig.Get("CLERK_ISSUER", ""),
		ActivityURL:   activityURL,
		UserURL:       userURL,
		AnalyticsURL:  analyticsURL,
		ChatbotURL:    chatbotURL,
	}
}
