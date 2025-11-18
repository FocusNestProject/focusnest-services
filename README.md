# FocusNest Microservices

FocusNest is a suite of Go microservices that run on Firestore, Cloud Storage, and Clerk-based authentication. Each service owns a dedicated bounded context and exposes HTTP endpoints that are aggregated by the Gateway API.

This README combines the architectural overview with the endpoint-level API reference so you no longer have to jump into `docs/` for day-to-day work.

## Architecture snapshot

| Service          | Responsibility                                                                | Primary Endpoints                               | Current Status |
| ---------------- | ----------------------------------------------------------------------------- | ----------------------------------------------- | -------------- |
| Focus Service    | Capture productivity sessions with optional image uploads and pagination.     | `/v1/productivities`, `/v1/productivities/{id}` | Audited ✔︎     |
| Progress Service | Generate analytics summaries and streak metrics from captured productivities. | `/v1/progress/summary`, `/v1/progress/streak/*` | Audited ✔︎     |
| User Service     | Persist user profiles plus derived metadata (streaks, counters).              | `/v1/users/me`                                  | Audited ✔︎     |
| Chatbot Service  | AI assistant consuming downstream APIs.                                       | `/v1/chat/*`                                    | Pending audit  |
| Gateway API      | Authenticates public traffic and routes to downstream services.               | `/v1/*` proxy routes                            | Pending audit  |

## Repository layout

```
focusnest-services/
├── focus-service/        # Productivity capture API
├── progress-service/     # Analytics and streak calculations
├── user-service/         # Profile storage and metadata
├── gateway-api/          # Public entry point
├── chatbot-service/      # Conversational assistant (pending audit)
├── shared-libs/          # Authentication, logging, router helpers
├── scripts/              # E2E and operational tooling
└── mk/                   # Shared make targets
```

## Getting started

### Prerequisites

- Go 1.24 or newer
- Docker (for local builds and Cloud Run parity)
- Google Cloud SDK (when targeting Firestore/Storage)
- Service account credentials with Firestore and Storage permissions (for cloud-backed runs)

### Quick start (memory-backed service)

```bash
# Clone and sync workspace
git clone https://github.com/FocusNestProject/focusnest-services.git
cd focusnest-services
go work sync

# Run focus-service against in-memory storage
cd focus-service
PORT=8080 DATA_STORE=memory AUTH_MODE=noop go run ./cmd/server
```

Hit the health endpoint with the `X-User-ID` header during local testing:

```bash
curl -H "X-User-ID: test_user" http://localhost:8080/health
```

### Running against Firestore

```bash
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account.json"
export GCP_PROJECT_ID="your-project-id"

cd focus-service
DATA_STORE=firestore AUTH_MODE=noop GCP_PROJECT_ID=$GCP_PROJECT_ID \
  go run ./cmd/server
```

Use the same environment variables for progress-service and user-service whenever you need Firestore-backed persistence. The focus-service also needs bucket settings defined in `internal/storage` for image uploads.

### Docker workflow

```bash
# Build and run a single service
docker build -t focus-service-local -f focus-service/Dockerfile focus-service

docker run -p 8080:8080 \
  -e DATA_STORE=memory \
  -e AUTH_MODE=noop \
  focus-service-local
```

Use `docker-compose up` from the repo root to launch multiple services with shared configuration. To tear everything down, run `docker-compose down`.

### Testing and formatting

```bash
go test ./...
go fmt ./...
```

Only packages with tests will execute code, but the command ensures every service compiles.

### Deployment overview

- GitHub Actions workflows build and deploy every service to Cloud Run on pushes to `main`.
- Gateway API routes client traffic and enforces Clerk JWT verification.
- Set `FOCUS_URL`, `PROGRESS_URL`, `CHATBOT_URL`, and `USER_URL` on the gateway to point at the deployed services.

### Security notes

- Gateway API is the only public entry point; downstream services still validate `X-User-ID` for staged environments.
- Focus Service image uploads are private and surfaced via 24-hour signed URLs.
- `shared-libs/auth` centralizes Clerk JWKS fetching to avoid divergent validation logic.

### Contributing

1. Fork the repository and create a feature branch.
2. Implement your change, run `go test ./...` and `go fmt ./...`.
3. Submit a pull request detailing the affected services and any new environment variables.

## API Reference

This section captures the endpoint-level contract for the three audited backend services (Focus, Progress, and User). All endpoints are authenticated behind the Gateway API in production. When calling services directly in local development, pass the `X-User-ID` header.

### Conventions

- **Headers**: `X-User-ID` (case-insensitive) is required on every request.
- **Dates**: Unless stated otherwise, use ISO-8601 strings in the form `YYYY-MM-DD` for date-only fields and RFC3339 timestamps for date-time fields.
- **Pagination**: Focus Service uses cursor-based pagination with `page_size` (1-100) and `page_token` query parameters.
- **Enums**: Enumerated values are case-sensitive unless explicitly noted.

### Focus Service (`/v1/productivities`)

#### Allowed values

- **Categories**: `Work`, `Study`, `Read`, `Journal`, `Cook`, `Workout`, `Music`, `Other`
- **Time modes**: `Pomodoro`, `Deep Work`, `Quick Focus`, `Free Timer`, `Other`
- **Moods** (optional): `Fokus`, `Semangat`, `Biasa Aja`, `Capek`, `Burn Out`, `Mengantuk`

#### `GET /v1/productivities`

| Query param  | Type   | Notes                                                    |
| ------------ | ------ | -------------------------------------------------------- |
| `page_size`  | int    | Defaults to 20, max 100                                  |
| `page_token` | string | Cursor from previous response                            |
| `month`      | int    | Optional filter, must be 1-12. Requires `year` to be set |
| `year`       | int    | Optional filter (1970-2100). Requires `month` to be set  |

Response payload:

```jsonc
{
  "items": [
    {
      "id": "entry-id",
      "image": "https://signed-url",
      "category": "Work",
      "time_elapsed": 1500,
      "num_cycle": 3,
      "time_mode": "Pomodoro",
      "start_time": "2025-11-19T02:04:00Z"
    }
  ],
  "next_page_token": "opaque-cursor",
  "total_items": 42
}
```

#### `POST /v1/productivities`

- Supports JSON or `multipart/form-data`. When using multipart, the binary field is named `image` and an optional `image_url` string can be provided instead.
- Required fields: `activity_name`, `time_elapsed` (seconds), `num_cycle`, `time_mode`, `category`, `start_time`, `end_time`.
- `description` is capped at 2000 characters. `mood` must be one of the allowed values when provided.
- `start_time` and `end_time` must be RFC3339 timestamps (`2025-11-19T02:04:00Z`). `end_time` must be greater than or equal to `start_time`.

Example JSON payload:

```json
{
  "activity_name": "Deep focus block",
  "time_elapsed": 1500,
  "num_cycle": 3,
  "time_mode": "Pomodoro",
  "category": "Work",
  "description": "Wrote status updates",
  "mood": "Semangat",
  "start_time": "2025-11-19T02:04:00Z",
  "end_time": "2025-11-19T02:29:00Z"
}
```

#### `PATCH /v1/productivities/{id}`

- Accepts JSON or multipart (same rules as `POST`).
- At least one mutable field must be supplied unless an image file is uploaded.
- `time_mode`, `category`, and `mood` values are validated against the allowed lists.
- Provide either a new binary `image` upload or an `image_url`, not both.

#### `GET /v1/productivities/{id}`

Returns the full entry with all fields captured at creation time. The `image` field is automatically rewritten to a signed URL when the stored value is a Cloud Storage path.

#### `DELETE /v1/productivities/{id}`

Soft-deletes the entry for the authenticated user. Repeated deletions on the same ID return `404`.

### Progress Service (`/v1/progress`)

#### Summary endpoint — `GET /v1/progress/summary`

| Query param      | Type   | Notes                                                   |
| ---------------- | ------ | ------------------------------------------------------- |
| `range`          | enum   | `week`, `month`, `3months`, `year` (defaults to `week`) |
| `category`       | string | Optional filter (case-insensitive)                      |
| `reference_date` | date   | ISO `YYYY-MM-DD`. Defaults to "today" in Asia/Jakarta   |

Response fields:

- `range`: Echoes the requested summary window.
- `reference_date`: Anchor day for the calculation (UTC timestamp string).
- `total_filtered_time`: Minutes spent in the filtered category (or all categories when blank).
- `time_distribution`: Array of `{ label, time_elapsed }` buckets whose shape depends on the range (weekday labels for `week`, weeks for `month`, months for `3months`, and quarters for `year`).
- `total_sessions`: Number of sessions matching the category filter.
- `total_time_frame`: Minutes spent across all categories within the window.
- `most_productive_hour_start` / `most_productive_hour_end`: UTC timestamps delimiting the most productive hour during the window (null when insufficient data).

#### Streak endpoints

All streak endpoints return `days`, an ordered list with:

```jsonc
{
  "date": "2025-11-19",
  "day": "Wednesday",
  "status": "done" // or "skipped" / "upcoming"
}
```

- `GET /v1/progress/streak/monthly?date=YYYY-MM-DD` — derives `month` and `year` from the optional `date`. Defaults to current month in Asia/Jakarta. Response also includes `total_streak` (longest all-time streak) and `current_streak` for the month.
- `GET /v1/progress/streak/weekly?date=YYYY-MM-DD` — Snapshots the ISO week containing `date` (defaults to current week). Response includes ISO week label (`week`), `total_streak`, and `current_streak`.
- `GET /v1/progress/streak/current` — Examines the trailing 30-day window ending today.

### User Service (`/v1/users/me`)

#### `GET /v1/users/me`

Returns the persisted profile merged with derived metadata:

```json
{
  "user_id": "uid-123",
  "full_name": "Focus Nest",
  "username": "focusnest",
  "bio": "building calm productivity",
  "birthdate": "1996-09-14",
  "metadata": {
    "longest_streak": 12,
    "total_productivities": 48,
    "total_sessions": 48
  },
  "created_at": "2025-11-19T09:10:11Z",
  "updated_at": "2025-11-19T10:00:00Z"
}
```

If a profile document does not exist yet, the service returns default values (blank strings, `null` birthdate) while still computing metadata from the user's productivities.

#### `PATCH /v1/users/me`

- Body must be JSON (max 64 KB) and may include any combination of `full_name`, `username`, `bio`, and `birthdate`.
- Unknown fields are rejected.
- `birthdate` accepts an ISO `YYYY-MM-DD` string or explicit `null` to clear the stored value.
- All string fields are trimmed server-side. Empty strings are allowed, but `username` uniqueness is enforced at the product level (outside this service).

Example payload:

```json
{
  "full_name": "Focus Nest",
  "username": "focusnest",
  "bio": "building calm productivity",
  "birthdate": "1996-09-14"
}
```

Both endpoints compute metadata on the fly: `total_productivities` counts non-deleted productivity documents, `total_sessions` mirrors the same count (reserved for future divergence), and `longest_streak` is calculated in the Asia/Jakarta timezone.
