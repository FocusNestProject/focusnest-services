package logging

import (
	"context"
	"log/slog"
	"os"
)

// NewLogger returns a slog logger configured for Cloud Logging compatibility.
func NewLogger(service string) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: true})
	return slog.New(handler).With(slog.String("service", service))
}

// WithRequestID attaches a request identifier to the logger context.
func WithRequestID(ctx context.Context, logger *slog.Logger, requestID string) *slog.Logger {
	return logger.With(slog.String("requestId", requestID))
}
