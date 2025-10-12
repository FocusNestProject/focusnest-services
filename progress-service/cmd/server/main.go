package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/go-chi/chi/v5"

	sharedauth "github.com/focusnest/shared-libs/auth"
	"github.com/focusnest/shared-libs/logging"
	sharedserver "github.com/focusnest/shared-libs/server"

	"github.com/focusnest/progress-service/internal/config"
	"github.com/focusnest/progress-service/internal/httpapi"
	"github.com/focusnest/progress-service/internal/progress"
)

func main() {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Errorf("config error: %w", err))
	}

	logger := logging.NewLogger("progress-service")

	// Initialize Firestore client with focusnest-prod database
	client, err := firestore.NewClientWithDatabase(ctx, cfg.GCPProjectID, "focusnest-prod")
	if err != nil {
		panic(fmt.Errorf("firestore client: %w", err))
	}
	defer client.Close()

	// Initialize progress service
	progressRepo := progress.NewFirestoreRepository(client)
	progressService := progress.NewService(progressRepo)

	verifier, err := sharedauth.NewVerifier(sharedauth.Config{
		Mode:     cfg.Auth.Mode,
		JWKSURL:  cfg.Auth.JWKSURL,
		Audience: cfg.Auth.Audience,
		Issuer:   cfg.Auth.Issuer,
	})
	if err != nil {
		panic(fmt.Errorf("auth verifier error: %w", err))
	}

	router := sharedserver.NewRouter("progress-service", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(sharedauth.Middleware(verifier))

			// Register progress routes
			httpapi.RegisterRoutes(r, progressService)
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
