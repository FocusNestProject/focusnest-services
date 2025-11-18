package config

import (
	"fmt"
	"strings"

	sharedauth "github.com/focusnest/shared-libs/auth"
	"github.com/focusnest/shared-libs/envconfig"
)

// Config encapsulates the runtime configuration for the focus service.
type Config struct {
	Port         string
	GCPProjectID string
	DataStore    DataStore
	Auth         AuthConfig
	Firestore    FirestoreConfig
	Storage      StorageConfig
}

// DataStore enumerates supported persistence backends.
type DataStore string

const (
	// DataStoreMemory stores productivity entries in-memory (useful for local development/testing).
	DataStoreMemory DataStore = "memory"
	// DataStoreFirestore stores entries in Google Cloud Firestore.
	DataStoreFirestore DataStore = "firestore"
)

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

// StorageConfig contains Cloud Storage settings.
type StorageConfig struct {
	Bucket string
}

// Load reads environment variables into Config with validation.
func Load() (Config, error) {
	cfg := Config{
		Port:         envconfig.Get("PORT", "8080"),
		GCPProjectID: envconfig.Get("GCP_PROJECT_ID", ""),
		DataStore:    DataStore(strings.ToLower(envconfig.Get("DATASTORE", string(DataStoreMemory)))),
		Auth: AuthConfig{
			Mode:    sharedauth.Mode(strings.ToLower(envconfig.Get("AUTH_MODE", string(sharedauth.ModeNoop)))),
			JWKSURL: envconfig.Get("CLERK_JWKS_URL", ""),
			Issuer:  envconfig.Get("CLERK_ISSUER", ""),
		},
		Firestore: FirestoreConfig{
			EmulatorHost: envconfig.Get("FIRESTORE_EMULATOR_HOST", ""),
		},
		Storage: StorageConfig{
			Bucket: envconfig.Get("FOCUS_STORAGE_BUCKET", ""),
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

	switch cfg.DataStore {
	case DataStoreMemory:
		// no-op
	case DataStoreFirestore:
		if cfg.GCPProjectID == "" {
			return fmt.Errorf("gcp project id required when datastore=firestore")
		}
	default:
		return fmt.Errorf("unsupported datastore: %s", cfg.DataStore)
	}

	if strings.TrimSpace(cfg.Storage.Bucket) == "" {
		return fmt.Errorf("FOCUS_STORAGE_BUCKET is required")
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

	return nil
}
