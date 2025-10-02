package envconfig

import (
	"fmt"
	"os"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// Get returns the value of the requested environment variable or the supplied fallback when empty.
func Get(name string, fallback string) string {
	if value, ok := os.LookupEnv(name); ok && value != "" {
		return value
	}
	return fallback
}

// MustGet returns the value of the requested environment variable or panics if it's empty.
func MustGet(name string) string {
	value := os.Getenv(name)
	if value == "" {
		panic(fmt.Sprintf("expected env %s to be set", name))
	}
	return value
}

// Validate validates a struct using validator tags.
func Validate(v any) error {
	return validate.Struct(v)
}
