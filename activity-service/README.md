# Activity Service

Productivity tracking microservice for FocusNest.

## Features

- **Create productivity entries** with category, time consumed, cycle mode, description, mood, and optional images
- **List productivity entries** by month with pagination
- **Retrieve single productivity entry** by ID
- **Delete productivity entries**
- **Clerk JWT authentication** via bearer tokens
- **Firestore persistence** for production with in-memory fallback for local development
- **Cloud Storage integration** for image uploads

## Architecture

This service follows clean architecture principles:
- `internal/productivity`: Domain models and business logic
- `internal/httpapi`: HTTP handlers and request/response mapping
- `internal/config`: Configuration loading and validation
- `cmd/server`: Application entry point

## Prerequisites

- Go 1.22+
- Docker (for local containerized testing)
- GCP project with Firestore and Cloud Storage enabled (for production)
- Clerk account (for production authentication)

## Configuration

Copy `.env.example` to `.env.local` and configure:

```bash
cp .env.example .env.local
```

### Local Development (No Auth, In-Memory Storage)

```env
PORT=8080
DATASTORE=memory
AUTH_MODE=noop
```

### Production (Clerk Auth, Firestore)

```env
PORT=8080
GCP_PROJECT_ID=your-project-id
DATASTORE=firestore
AUTH_MODE=clerk
CLERK_JWKS_URL=https://your-domain.clerk.accounts.dev/.well-known/jwks.json
CLERK_ISSUER=https://your-domain.clerk.accounts.dev
GCS_BUCKET=your-productivity-images-bucket
```

## Running Locally

### Direct Execution

```bash
go run ./cmd/server
```

### With Docker

Build the image:

```bash
docker build -f Dockerfile -t activity-service:latest ..
```

Run the container:

```bash
docker run -p 8080:8080 \
  --env-file .env.local \
  activity-service:latest
```

## API Endpoints

See `api/openapi.yaml` for complete API documentation.

### Health Check

```bash
curl http://localhost:8080/healthz
```

### Create Productivity Entry

```bash
curl -X POST http://localhost:8080/v1/productivities \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "category": "Deep Work",
    "timeConsumedMinutes": 90,
    "cycleMode": "Pomodoro",
    "description": "Finished feature implementation",
    "mood": "focused"
  }'
```

### List Productivity Entries (Current Month)

```bash
curl http://localhost:8080/v1/productivities \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### List with Filters

```bash
curl "http://localhost:8080/v1/productivities?month=2025-10&page=1&pageSize=20" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### Get Single Entry

```bash
curl http://localhost:8080/v1/productivities/{id} \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### Delete Entry

```bash
curl -X DELETE http://localhost:8080/v1/productivities/{id} \
  -H "Authorization: Bearer YOUR_TOKEN"
```

## Testing with Noop Auth

When `AUTH_MODE=noop`, the bearer token is treated as the user ID:

```bash
curl http://localhost:8080/v1/productivities \
  -H "Authorization: Bearer test-user-123"
```

## Deploying to Google Cloud Run

### Build and Push

```bash
gcloud builds submit --tag gcr.io/YOUR_PROJECT_ID/activity-service
```

### Deploy

```bash
gcloud run deploy activity-service \
  --image gcr.io/YOUR_PROJECT_ID/activity-service \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --set-env-vars="GCP_PROJECT_ID=YOUR_PROJECT_ID,DATASTORE=firestore,AUTH_MODE=clerk" \
  --set-env-vars="CLERK_JWKS_URL=https://your-domain.clerk.accounts.dev/.well-known/jwks.json" \
  --set-env-vars="CLERK_ISSUER=https://your-domain.clerk.accounts.dev" \
  --set-env-vars="GCS_BUCKET=your-bucket-name"
```

## Image Upload Flow

Images are handled via Cloud Storage signed URLs:

1. **Frontend** requests a signed upload URL from backend
2. **Backend** generates signed URL with upload permissions
3. **Frontend** uploads image directly to Cloud Storage
4. **Frontend** submits productivity entry with the GCS object URL

This keeps image data out of the service and leverages CDN for delivery.

## Development

Format code:

```bash
go fmt ./...
```

Run tests:

```bash
go test ./...
```

Lint:

```bash
golangci-lint run
```

## License

Proprietary - FocusNest

