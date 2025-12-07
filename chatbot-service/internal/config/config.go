package config

import (
	"fmt"
	"strconv"
	"strings"

	sharedauth "github.com/focusnest/shared-libs/auth"
	"github.com/focusnest/shared-libs/envconfig"
)

// Config encapsulates the runtime configuration for the chatbot service.
type Config struct {
	Port         string
	GCPProjectID string
	Auth         AuthConfig
	Firestore    FirestoreConfig
	LLM          LLMConfig
}

// AuthConfig stores authentication middleware setup.
type AuthConfig struct {
	Mode     sharedauth.Mode
	JWKSURL  string
	Audience string
	Issuer   string
}

// FirestoreConfig tailors Firestore client behavior.
type FirestoreConfig struct {
	EmulatorHost string
}

// LLMConfig defines how the chatbot talks to Gemini.
type LLMConfig struct {
	APIKey          string
	Model           string
	ContextMessages int
	MaxOutputTokens int
	UseVertex       bool
	Location        string
}

// Load reads environment variables into Config with validation.
func Load() (Config, error) {
	cfg := Config{
		Port:         envconfig.Get("PORT", "8080"),
		GCPProjectID: envconfig.Get("GCP_PROJECT_ID", ""),
		Auth: AuthConfig{
			Mode:    sharedauth.Mode(strings.ToLower(envconfig.Get("AUTH_MODE", string(sharedauth.ModeNoop)))),
			JWKSURL: envconfig.Get("CLERK_JWKS_URL", ""),
			Issuer:  envconfig.Get("CLERK_ISSUER", ""),
		},
		Firestore: FirestoreConfig{
			EmulatorHost: envconfig.Get("FIRESTORE_EMULATOR_HOST", ""),
		},
		LLM: LLMConfig{
			APIKey:          resolveAPIKey(),
			Model:           envconfig.Get("GEMINI_MODEL", "gemini-2.0-flash-exp"),
			ContextMessages: parseIntFallback(envconfig.Get("CHATBOT_CONTEXT_MESSAGES", "32"), 32),
			MaxOutputTokens: parseIntFallback(envconfig.Get("CHATBOT_MAX_OUTPUT_TOKENS", "1024"), 1024),
			UseVertex:       parseBool(envconfig.Get("GOOGLE_GENAI_USE_VERTEXAI", "false")),
			Location:        envconfig.Get("GOOGLE_CLOUD_LOCATION", ""),
		},
	}

	if err := validate(cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func validate(cfg Config) error {
	if strings.TrimSpace(cfg.Port) == "" {
		return fmt.Errorf("port must be specified")
	}

	if cfg.GCPProjectID == "" {
		return fmt.Errorf("gcp project id required")
	}

	switch cfg.Auth.Mode {
	case sharedauth.ModeClerk:
		if cfg.Auth.JWKSURL == "" {
			return fmt.Errorf("CLERK_JWKS_URL is required when AUTH_MODE=clerk")
		}
	case sharedauth.ModeNoop:
		// no-op
	default:
		return fmt.Errorf("unsupported auth mode: %s", cfg.Auth.Mode)
	}

	if cfg.LLM.ContextMessages <= 0 {
		return fmt.Errorf("CHATBOT_CONTEXT_MESSAGES must be > 0")
	}
	if cfg.LLM.MaxOutputTokens <= 0 {
		return fmt.Errorf("CHATBOT_MAX_OUTPUT_TOKENS must be > 0")
	}
	if cfg.LLM.UseVertex {
		if strings.TrimSpace(cfg.LLM.Location) == "" {
			return fmt.Errorf("GOOGLE_CLOUD_LOCATION is required when GOOGLE_GENAI_USE_VERTEXAI=true")
		}
	} else if strings.TrimSpace(cfg.LLM.APIKey) == "" {
		return fmt.Errorf("GEMINI_API_KEY or GOOGLE_API_KEY is required when GOOGLE_GENAI_USE_VERTEXAI is false")
	}

	return nil
}

func resolveAPIKey() string {
	if apiKey := envconfig.Get("GEMINI_API_KEY", ""); strings.TrimSpace(apiKey) != "" {
		return apiKey
	}
	return envconfig.Get("GOOGLE_API_KEY", "")
}

func parseIntFallback(raw string, fallback int) int {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	val, err := strconv.Atoi(raw)
	if err != nil || val <= 0 {
		return fallback
	}
	return val
}

func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
