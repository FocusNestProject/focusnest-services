package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/focusnest/gateway-api/internal/config"
	"github.com/focusnest/gateway-api/internal/httpapi"
	"github.com/focusnest/shared-libs/auth"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := loadConfig()
	if err != nil {
		logger.Error("failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("starting gateway-api",
		slog.String("port", cfg.Port),
		slog.String("auth_mode", cfg.AuthMode),
	)

	// Initialize auth verifier
	verifier, err := auth.NewVerifier(auth.Config{
		Mode:     auth.Mode(cfg.AuthMode),
		JWKSURL:  cfg.ClerkJWKSURL,
		Audience: cfg.ClerkAudience,
		Issuer:   cfg.ClerkIssuer,
	})
	if err != nil {
		logger.Error("failed to initialize auth verifier", slog.Any("error", err))
		os.Exit(1)
	}

	logger.Info("auth verifier initialized",
		slog.String("mode", cfg.AuthMode),
		slog.String("jwks_url", cfg.ClerkJWKSURL),
		slog.String("issuer", cfg.ClerkIssuer),
	)

	// Setup proxy targets
	targets := httpapi.Targets{
		Activity:  cfg.ActivityURL,
		User:      cfg.UserURL,
		Analytics: cfg.AnalyticsURL,
		Chatbot:   cfg.ChatbotURL,
	}

	router := httpapi.Router(verifier, targets, logger)

	addr := ":" + cfg.Port
	logger.Info("listening", slog.String("addr", addr))

	if err := http.ListenAndServe(addr, router); err != nil {
		logger.Error("server error", slog.Any("error", err))
		os.Exit(1)
	}
}

func loadConfig() (*config.Config, error) {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	authMode := os.Getenv("AUTH_MODE")
	if authMode == "" {
		authMode = "clerk" // default to clerk in production
	}

	cfg := &config.Config{
		Port:          port,
		AuthMode:      authMode,
		ClerkJWKSURL:  os.Getenv("CLERK_JWKS_URL"),
		ClerkAudience: os.Getenv("CLERK_AUDIENCE"),
		ClerkIssuer:   os.Getenv("CLERK_ISSUER"),
	}

	// Parse upstream service URLs
	if activityURL := os.Getenv("ACTIVITY_URL"); activityURL != "" {
		u, err := config.ParseURLCompat(activityURL)
		if err != nil {
			return nil, err
		}
		cfg.ActivityURL = u
	}

	if userURL := os.Getenv("USER_URL"); userURL != "" {
		u, err := config.ParseURLCompat(userURL)
		if err != nil {
			return nil, err
		}
		cfg.UserURL = u
	}

	if analyticsURL := os.Getenv("ANALYTICS_URL"); analyticsURL != "" {
		u, err := config.ParseURLCompat(analyticsURL)
		if err != nil {
			return nil, err
		}
		cfg.AnalyticsURL = u
	}

	if chatbotURL := os.Getenv("CHATBOT_URL"); chatbotURL != "" {
		u, err := config.ParseURLCompat(chatbotURL)
		if err != nil {
			return nil, err
		}
		cfg.ChatbotURL = u
	}

	return cfg, nil
}
