# FocusNest Microservices

FocusNest is a suite of Go microservices that run on Firestore, Cloud Storage, and Clerk-based authentication. Each service owns a dedicated bounded context and exposes HTTP endpoints that are aggregated by the Gateway API.

This README combines the architectural overview with the endpoint-level API reference so you no longer have to jump into `docs/` for day-to-day work.

## Architecture snapshot

| Service          | Responsibility                                                                  | Primary Endpoints                               |
| ---------------- | ------------------------------------------------------------------------------- | ----------------------------------------------- |
| Focus Service    | Capture productivity sessions with optional image uploads and pagination.       | `/v1/productivities`, `/v1/productivities/{id}` |
| Progress Service | Generate analytics summaries and streak metrics from captured productivities.   | `/v1/progress/summary`, `/v1/progress/streak/*` |
| User Service     | Persist user profiles plus derived metadata (streaks, counters).                | `/v1/users/me`                                  |
| Chatbot Service  | Stores multi-chat history and calls Gemini 2.5 Flash for productivity coaching. | `/v1/chatbot/*`                                 |
| Gateway API      | Authenticates public traffic and routes to downstream services.                 | `/v1/*` proxy routes                            |

## Repository layout

```
focusnest-services/
├── activity-service/       # Future habit loops
├── analytics-service/      # Long-form insights (WIP)
├── auth-gateway/           # Edge auth helper
├── chatbot-service/        # Productivity coach + history
├── focus-service/          # Productivity capture API
├── gateway-api/            # Public entry point and routing
├── media-service/          # Image utilities
├── notification-service/   # Push/email fanout
├── postman/                # API collection
├── progress-service/       # Analytics + streaks
├── scripts/                # E2E helpers
├── session-service/        # Session persistence
├── shared-libs/            # Auth/logging/router packages
├── user-service/           # Profiles + metadata
├── webhook-service/        # Outbound webhooks
└── mk/, docker-compose.yml, Makefile, go.work, etc.
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

This section captures the endpoint-level contract for the core backend services. All endpoints are authenticated behind the Gateway API in production. When calling services directly in local development, pass the `X-User-ID` header.

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

| Field           | Type                                                                           | Required | Notes                                                               |
| --------------- | ------------------------------------------------------------------------------ | -------- | ------------------------------------------------------------------- |
| `activity_name` | string                                                                         | Yes      | Trimmed server-side; non-empty                                      |
| `time_elapsed`  | int (seconds)                                                                  | Yes      | Must be > 0                                                         |
| `num_cycle`     | int                                                                            | Yes      | Must be > 0                                                         |
| `time_mode`     | enum (`Pomodoro`, `Deep Work`, `Quick Focus`, `Free Timer`, `Other`)           | Yes      | Case-sensitive                                                      |
| `category`      | enum (`Work`, `Study`, `Read`, `Journal`, `Cook`, `Workout`, `Music`, `Other`) | Yes      | Case-sensitive                                                      |
| `description`   | string                                                                         | No       | ≤ 2000 characters                                                   |
| `mood`          | enum (`Fokus`, `Semangat`, `Biasa Aja`, `Capek`, `Burn Out`, `Mengantuk`)      | No       | Case-sensitive                                                      |
| `start_time`    | string (RFC3339)                                                               | Yes      | UTC timestamp (`YYYY-MM-DDTHH:MM:SSZ`)                              |
| `end_time`      | string (RFC3339)                                                               | Yes      | Must be ≥ `start_time`                                              |
| `image`         | file (`.jpg`, `.jpeg`, `.png`)                                                 | No       | Multipart field named `image`; stored in Cloud Storage              |
| `image_url`     | string (URL)                                                                   | No       | HTTPS link to an existing image; not used when `image` file present |

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

| Field           | Type                                                                           | Required | Notes                                     |
| --------------- | ------------------------------------------------------------------------------ | -------- | ----------------------------------------- |
| `activity_name` | string                                                                         | No       | When supplied, trimmed server-side        |
| `time_elapsed`  | int (seconds)                                                                  | No       | Must be > 0                               |
| `num_cycle`     | int                                                                            | No       | Must be > 0                               |
| `time_mode`     | enum (`Pomodoro`, `Deep Work`, `Quick Focus`, `Free Timer`, `Other`)           | No       | Case-sensitive                            |
| `category`      | enum (`Work`, `Study`, `Read`, `Journal`, `Cook`, `Workout`, `Music`, `Other`) | No       | Case-sensitive                            |
| `description`   | string                                                                         | No       | ≤ 2000 characters                         |
| `mood`          | enum (`Fokus`, `Semangat`, `Biasa Aja`, `Capek`, `Burn Out`, `Mengantuk`)      | No       | Case-sensitive                            |
| `start_time`    | string (RFC3339)                                                               | No       | UTC timestamp                             |
| `end_time`      | string (RFC3339)                                                               | No       | Must be ≥ `start_time` when both provided |
| `image`         | file (`.jpg`, `.jpeg`, `.png`)                                                 | No       | Multipart field named `image`             |
| `image_url`     | string (URL)                                                                   | No       | HTTPS link for an existing image          |

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

| Field       | Type                            | Required | Notes                                            |
| ----------- | ------------------------------- | -------- | ------------------------------------------------ |
| `full_name` | string                          | No       | Trimmed server-side; empty string allowed        |
| `username`  | string                          | No       | Lower-level service enforces uniqueness; trimmed |
| `bio`       | string                          | No       | Trimmed; recommend ≤ 2000 characters             |
| `birthdate` | string (`YYYY-MM-DD`) or `null` | No       | Provide ISO date to set value or `null` to clear |

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

### Chatbot Service (`/v1/chatbot`)

The chatbot keeps a complete history per user: every profile can open multiple chats (sessions) and each session records the ordered dialogue between the user (`role: user`) and the assistant (`role: assistant`). All endpoints require the `X-User-ID` header and respond in either English or Bahasa Indonesia based on the latest user input. When a prompt falls outside productivity/focus topics, the assistant replies with a boundaries message instead of answering. Gemini 2.5 Flash is used whenever a prompt remains in scope—with a capped context window to keep costs predictable.

#### Gemini configuration

| Variable                            | Required | Notes                                                                                                   |
| ----------------------------------- | -------- | ------------------------------------------------------------------------------------------------------- |
| `GEMINI_API_KEY` / `GOOGLE_API_KEY` | Yes\*    | One of these must be set when `GOOGLE_GENAI_USE_VERTEXAI=false`. Provide an API key with Gemini access. |
| `GOOGLE_GENAI_USE_VERTEXAI`         | No       | Set to `true` to route through Vertex AI using application default credentials. Defaults to false.      |
| `GOOGLE_CLOUD_LOCATION`             | Yes\*    | Required whenever `GOOGLE_GENAI_USE_VERTEXAI=true`. Example: `asia-southeast2`.                         |
| `GCP_PROJECT_ID`                    | Yes      | Already required by the service; reused for Vertex API calls.                                           |

`GOOGLE_APPLICATION_CREDENTIALS` must point to a service-account JSON file when running locally with Vertex (unless your ADC context already has the role). When the Vertex flag is disabled the chatbot falls back to the standard Gemini API using `GEMINI_API_KEY`.

#### `GET /v1/chatbot/sessions`

Lists every chat session for the caller (sorted by `updated_at` descending) and includes `id`, `title`, and timestamps. This is the lightweight list to populate a sidebar.

#### `GET /v1/chatbot/history`

Returns every session (most recent first) together with its full message log when you need to hydrate everything in one request.

#### `GET /v1/chatbot/sessions/{sessionID}`

Fetches a single session plus its messages. Use this to lazily hydrate one chat thread.

#### `PATCH /v1/chatbot/sessions/{sessionID}`

Updates the stored title. Titles default to a summary of the very first message, so this endpoint lets users rename it later.

| Field   | Type   | Required | Notes                                                                 |
| ------- | ------ | -------- | --------------------------------------------------------------------- |
| `title` | string | Yes      | Trimmed server-side; must be non-empty and typically ≤ 120 characters |

#### `DELETE /v1/chatbot/sessions/{sessionID}`

Deletes the session document and every dialog entry belonging to it.

#### `POST /v1/chatbot/ask`

Request body:

```json
{
  "session_id": "optional-existing-id",
  "question": "How can I stay focused for 30 minutes?"
}
```

| Field        | Type   | Required | Notes                                                                             |
| ------------ | ------ | -------- | --------------------------------------------------------------------------------- |
| `session_id` | string | No       | Existing chat session ID (UUID). Leave blank to auto-create a new session + title |
| `question`   | string | Yes      | Trimmed server-side; must be non-empty and stay within productivity/focus topics  |

- Omit `session_id` to start a new chat; the service derives a title from the first prompt and returns the generated ID.
- The assistant stores the user prompt, builds context from the most recent messages (respecting the configured window), and calls Gemini 2.5 Flash for a productivity-focused response. If the topic is out of bounds, it replies with the boundaries message instead of calling the model.
- Responses mirror the user language (English or Bahasa Indonesia) and contain two to three actionable steps.

Response payload:

```json
{
  "session_id": "session-id",
  "assistant_message": {
    "role": "assistant",
    "content": "Here’s a productivity check-in for \"weekly report\" (Work)…"
  },
  "messages": [
    /* entire conversation including the new reply */
  ]
}
```
