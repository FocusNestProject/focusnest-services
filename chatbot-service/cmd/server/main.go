package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/go-chi/chi/v5"

	sharedauth "github.com/focusnest/shared-libs/auth"
	"github.com/focusnest/shared-libs/logging"
	sharedserver "github.com/focusnest/shared-libs/server"

	"github.com/focusnest/chatbot-service/internal/chatbot"
	"github.com/focusnest/chatbot-service/internal/config"
	"github.com/focusnest/chatbot-service/internal/httpapi"
)

func main() {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		panic(fmt.Errorf("config error: %w", err))
	}

	logger := logging.NewLogger("chatbot-service")

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

	chatbotRepo := chatbot.NewFirestoreRepository(client)
	assistant, err := chatbot.NewGeminiAssistant(ctx, chatbot.AssistantConfig{
		APIKey:          cfg.LLM.APIKey,
		Model:           cfg.LLM.Model,
		MaxOutputTokens: cfg.LLM.MaxOutputTokens,
		UseVertex:       cfg.LLM.UseVertex,
		Project:         cfg.GCPProjectID,
		Location:        cfg.LLM.Location,
	})
	if err != nil {
		logger.Warn("falling back to template assistant", slog.String("reason", err.Error()))
		assistant = chatbot.NewTemplateAssistant()
	} else {
		defer assistant.Close()
	}

	chatbotService, err := chatbot.NewService(chatbotRepo, assistant, cfg.LLM.ContextMessages)
	if err != nil {
		panic(fmt.Errorf("chatbot service init error: %w", err))
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

	router := sharedserver.NewRouter("chatbot-service", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(sharedauth.Middleware(verifier))

			// Register chatbot routes
			httpapi.RegisterRoutes(r, chatbotService, logger)
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
