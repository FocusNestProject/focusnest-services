package productivity

import (
	"time"

	"github.com/google/uuid"
)

type systemClock struct{}

// NewSystemClock returns a Clock implementation backed by time.Now.
func NewSystemClock() Clock {
	return systemClock{}
}

func (systemClock) Now() time.Time {
	return time.Now()
}

type uuidGenerator struct{}

// NewUUIDGenerator returns an IDGenerator that produces v7 UUIDs where available, falling back to v4.
func NewUUIDGenerator() IDGenerator {
	return uuidGenerator{}
}

func (uuidGenerator) NewID() string {
	if id, err := uuid.NewV7(); err == nil {
		return id.String()
	}
	return uuid.NewString()
}
