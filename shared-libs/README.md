# FocusNest Shared Libraries

Common utilities, DTOs, and typed event definitions shared across FocusNest microservices.

## Packages

- `dto`: Type-safe payloads for REST APIs.
- `events`: Pub/Sub contracts for inter-service messaging.
- `errors`: Canonical error envelopes and helpers.
- `envconfig`: Tiny helpers for reading environment variables with validation.
- `logging`: slog helpers tuned for Cloud Logging.
- `pubsub`: Shared topic constants.
- `server`: HTTP router scaffolding with consistent middleware and `/healthz` handling.

## Development

```sh
make lint
make test
```

Generated code from OpenAPI specs will live under each service, while canonical specs remain in `openapi/`.
