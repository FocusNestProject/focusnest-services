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
	Port         string `validate:"required"`
	GCPProjectID string `validate:"required"`
	BucketName   string `validate:"required"`
}

func loadConfig() (config, error) {
	cfg := config{
		Port:         envconfig.Get("PORT", "8080"),
		GCPProjectID: envconfig.Get("GCP_PROJECT_ID", "focusnest-dev"),
		BucketName:   envconfig.Get("BUCKET_NAME", "focusnest-media"),
	}
	return cfg, envconfig.Validate(cfg)
}

func main() {
	ctx := context.Background()
	cfg, err := loadConfig()
	if err != nil {
		panic(fmt.Errorf("config error: %w", err))
	}

	logger := logging.NewLogger("media-service")

	router := sharedserver.NewRouter("media-service", func(r chi.Router) {
		// TODO: implement media presign endpoints.
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
