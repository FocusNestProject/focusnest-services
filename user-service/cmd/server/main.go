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

	"github.com/focusnest/user-service/internal/config"
	"github.com/focusnest/user-service/internal/httpapi"
	"github.com/focusnest/user-service/internal/user"
)

func main() {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Errorf("config error: %w", err))
	}

	logger := logging.NewLogger("user-service")

	// Initialize Firestore client
	databaseID := "focusnest-prod"
	if cfg.Firestore.EmulatorHost != "" {
		if err := os.Setenv("FIRESTORE_EMULATOR_HOST", cfg.Firestore.EmulatorHost); err != nil {
			panic(fmt.Errorf("set FIRESTORE_EMULATOR_HOST: %w", err))
		}
		databaseID = "(default)"
	}
	client, err := firestore.NewClientWithDatabase(ctx, cfg.GCPProjectID, databaseID)
	if err != nil {
		panic(fmt.Errorf("firestore client: %w", err))
	}
	defer client.Close()

	// Initialize user service
	userRepo := user.NewFirestoreRepository(client)
	userService := user.NewService(userRepo)

	verifier, err := sharedauth.NewVerifier(sharedauth.Config{
		Mode:     sharedauth.Mode(cfg.Auth.Mode),
		JWKSURL:  cfg.Auth.JWKSURL,
		Audience: cfg.Auth.Audience,
		Issuer:   cfg.Auth.Issuer,
	})
	if err != nil {
		panic(fmt.Errorf("auth verifier error: %w", err))
	}

	router := sharedserver.NewRouter("user-service", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(sharedauth.Middleware(verifier))

			// Register user routes
			httpapi.RegisterRoutes(r, userService, logger)
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
