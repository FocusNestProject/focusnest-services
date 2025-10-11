package config

import (
	"github.com/focusnest/shared-libs/envconfig"
)

type Config struct {
	Port         string `validate:"required"`
	GCPProjectID string `validate:"required"`
	DataStore    string `validate:"required"`
	Auth         AuthConfig
	Firestore    FirestoreConfig
}

type AuthConfig struct {
	Mode     string `validate:"required"`
	JWKSURL  string
	Audience string
	Issuer   string
}

type FirestoreConfig struct {
	EmulatorHost string
}

func Load() (Config, error) {
	cfg := Config{
		Port:         envconfig.Get("PORT", "8080"),
		GCPProjectID: envconfig.Get("GCP_PROJECT_ID", "focusnest-dev"),
		DataStore:    envconfig.Get("DATASTORE", "firestore"),
		Auth: AuthConfig{
			Mode:     envconfig.Get("AUTH_MODE", "clerk"),
			JWKSURL:  envconfig.Get("CLERK_JWKS_URL", ""),
			Audience: envconfig.Get("CLERK_AUDIENCE", ""),
			Issuer:   envconfig.Get("CLERK_ISSUER", ""),
		},
		Firestore: FirestoreConfig{
			EmulatorHost: envconfig.Get("FIRESTORE_EMULATOR_HOST", ""),
		},
	}
	return cfg, envconfig.Validate(cfg)
}
