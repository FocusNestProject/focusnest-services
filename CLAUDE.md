# Focuzen Services - CLAUDE.md

Go microservices backend. See `../CLAUDE.md` for full project context.

## Quick Reference

- **Default branch**: `main`
- **Go version**: 1.24
- **Workspace**: `go.work` (all services linked)
- **Deploy target**: Google Cloud Run via Cloud Build

## Validation Commands (Run Before Commit)

```bash
go build ./...                       # Must compile
go vet ./...                         # Static analysis
go test ./...                        # Run all tests

# Services with Makefile (gateway-api, user-service):
make lint                            # golangci-lint
make test                            # go test
make tidy                            # go mod tidy
```

Always run `go build ./...` and `go vet ./...` at minimum before committing.

## Service Overview

| Service | Purpose | Has Makefile |
|---------|---------|-------------|
| `gateway-api/` | API gateway, routing, RevenueCat webhooks | Yes |
| `chatbot-service/` | AI chatbot (Gemini) | No |
| `focus-service/` | Focus session CRUD | No |
| `progress-service/` | Progress tracking & analytics | No |
| `user-service/` | User management | Yes |
| `shared-libs/` | Shared Go packages | N/A |

## Where Things Go

| What | Where |
|------|-------|
| New service entrypoint | `<service>/cmd/server/` |
| Business logic | `<service>/internal/` |
| Shared utilities | `shared-libs/` |
| API spec / codegen config | `<service>/api/` |
| Docker config | `<service>/Dockerfile` |
| Cloud Build config | `<service>/cloudbuild.yaml` |

## Gotchas

- Services without Makefile: use `go build`, `go test`, `go vet` directly
- `shared-libs/` is referenced via Go workspace — changes here affect all services
- Each service has its own `go.mod` — run `go mod tidy` in the specific service after adding dependencies
