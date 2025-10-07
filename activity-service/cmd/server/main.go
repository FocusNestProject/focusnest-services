package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/go-chi/chi/v5"

	sharedauth "github.com/focusnest/shared-libs/auth"
	"github.com/focusnest/shared-libs/logging"
	sharedserver "github.com/focusnest/shared-libs/server"

	"github.com/focusnest/activity-service/internal/analytics"
	"github.com/focusnest/activity-service/internal/chatbot"
	"github.com/focusnest/activity-service/internal/config"
	"github.com/focusnest/activity-service/internal/httpapi"
	"github.com/focusnest/activity-service/internal/productivity"
	"github.com/focusnest/activity-service/internal/user"
)

func main() {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Errorf("config error: %w", err))
	}

	logger := logging.NewLogger("activity-service")

	repo, cleanup, err := newRepository(ctx, cfg)
	if err != nil {
		panic(fmt.Errorf("repository init error: %w", err))
	}
	defer cleanup()

	clock := productivity.NewSystemClock()
	ids := productivity.NewUUIDGenerator()

	// Initialize productivity service
	productivityService, err := productivity.NewService(repo, clock, ids)
	if err != nil {
		panic(fmt.Errorf("productivity service init error: %w", err))
	}

	// Initialize chatbot service (using memory repository for now)
	chatbotRepo := chatbot.NewMemoryRepository()
	chatbotService, err := chatbot.NewService(chatbotRepo, clock, ids)
	if err != nil {
		panic(fmt.Errorf("chatbot service init error: %w", err))
	}

	// Initialize analytics service (using memory repository for now)
	analyticsRepo := analytics.NewMemoryRepository()
	analyticsService := analytics.NewService(analyticsRepo)

	// Initialize user service (using memory repository for now)
	userRepo := user.NewMemoryRepository()
	userService, err := user.NewService(userRepo, clock, ids)
	if err != nil {
		panic(fmt.Errorf("user service init error: %w", err))
	}

	verifier, err := sharedauth.NewVerifier(sharedauth.Config{
		Mode:     cfg.Auth.Mode,
		JWKSURL:  cfg.Auth.JWKSURL,
		Audience: cfg.Auth.Audience,
		Issuer:   cfg.Auth.Issuer,
	})
	if err != nil {
		panic(fmt.Errorf("auth verifier error: %w", err))
	}

	router := sharedserver.NewRouter("activity-service", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(sharedauth.Middleware(verifier))

			// Register all service routes
			httpapi.RegisterRoutes(r, productivityService)
			httpapi.RegisterChatbotRoutes(r, chatbotService)
			httpapi.RegisterAnalyticsRoutes(r, analyticsService)
			httpapi.RegisterUserRoutes(r, userService)
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

func newRepository(ctx context.Context, cfg config.Config) (productivity.Repository, func(), error) {
	switch cfg.DataStore {
	case config.DataStoreFirestore:
		if cfg.Firestore.EmulatorHost != "" {
			if err := os.Setenv("FIRESTORE_EMULATOR_HOST", cfg.Firestore.EmulatorHost); err != nil {
				return nil, nil, fmt.Errorf("set FIRESTORE_EMULATOR_HOST: %w", err)
			}
		}

		client, err := firestore.NewClient(ctx, cfg.GCPProjectID)
		if err != nil {
			return nil, nil, fmt.Errorf("firestore client: %w", err)
		}

		repo := productivity.NewFirestoreRepository(client)
		cleanup := func() {
			_ = client.Close()
		}
		return repo, cleanup, nil
	default:
		repo := productivity.NewMemoryRepository()
		return repo, func() {}, nil
	}
}
