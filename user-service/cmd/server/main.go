package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/focusnest/shared-libs/envconfig"
	"github.com/focusnest/shared-libs/logging"
	sharedserver "github.com/focusnest/shared-libs/server"
)

type config struct {
	Port           string `validate:"required"`
	GCPProjectID   string `validate:"required"`
	FirestoreEmulator string `validate:"omitempty"`
}

func loadConfig() (config, error) {
	cfg := config{
		Port:              envconfig.Get("PORT", "8080"),
		GCPProjectID:      envconfig.Get("GCP_PROJECT_ID", "focusnest-dev"),
		FirestoreEmulator: envconfig.Get("FIRESTORE_EMULATOR_HOST", ""),
	}
	return cfg, envconfig.Validate(cfg)
}

func main() {
	ctx := context.Background()
	cfg, err := loadConfig()
	if err != nil {
		panic(fmt.Errorf("config error: %w", err))
	}

	logger := logging.NewLogger("user-service")

	router := sharedserver.NewRouter("user-service", func(r chi.Router) {
		// TODO: register user routes.
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:          router,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	if err := sharedserver.Run(ctx, srv, logger); err != nil && !errors.Is(err, http.ErrServerClosed) {
		panic(err)
	}
}
