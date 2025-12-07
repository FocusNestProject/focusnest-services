# FocusNest Services Handbook

Centralized API contract and runbook for every service that lives in this monorepo. All public traffic flows through the Gateway API, but each downstream service exposes a documented REST surface for local testing and internal integration.

## Monorepo map

| Service          | Path                | Responsibility                                                  | Primary endpoints                               |
| ---------------- | ------------------- | --------------------------------------------------------------- | ----------------------------------------------- |
| Focus Service    | `focus-service/`    | CRUD for productivity sessions, attachment uploads, pagination. | `/v1/productivities`, `/v1/productivities/{id}` |
| Progress Service | `progress-service/` | Derived analytics (summaries, streaks, distributions).          | `/v1/progress/summary`, `/v1/progress/streak/*` |
| User Service     | `user-service/`     | Profile data plus derived metadata (streak counters, totals).   | `/v1/users/me`                                  |
| Chatbot Service  | `chatbot-service/`  | Multi-session productivity coach backed by Gemini / Vertex.     | `/v1/chatbot/*`                                 |
| Gateway API      | `gateway-api/`      | Public entry point that handles Clerk auth and request routing. | `/v1/*` proxy surface                           |
| Shared Libraries | `shared-libs/`      | Common auth, logging, DTO, and server scaffolding.              | Imported modules                                |

## Common requirements

- **Auth:** In production everything sits behind Gateway API + Clerk JWT validation. When calling a service directly (local dev, port-forward, etc.) include `X-User-ID` or `x-user-id` with the authenticated subject.
- **Dates & times:** Date-only fields use `YYYY-MM-DD`. Timestamps are RFC3339 with UTC (`2025-11-20T07:00:00Z`).
- **Environments:** Each service honors `AUTH_MODE=noop` for local hacking. Firestore clients need `GCP_PROJECT_ID` plus either emulator vars or ADC credentials.
- **Testing:** Run `go test ./...` from any service folder (or repo root) before pushing. Go 1.24+ and Docker are required for parity builds.

---

## Service API reference

### Focus Service — `/v1/productivities`

Allowed values:

- `category`: `Work`, `Study`, `Read`, `Journal`, `Cook`, `Workout`, `Music`, `Other`
- `time_mode`: `Pomodoro`, `Deep Work`, `Quick Focus`, `Free Timer`, `Other`
- `mood` (optional): `Fokus`, `Semangat`, `Biasa Aja`, `Capek`, `Burn Out`, `Mengantuk`

#### `GET /v1/productivities`

| Query        | Type   | Notes                         |
| ------------ | ------ | ----------------------------- |
| `page_size`  | int    | Default 20, max 100           |
| `page_token` | string | Cursor from previous response |
| `month`      | int    | 1-12; requires `year`         |
| `year`       | int    | 1970-2100; requires `month`   |

Response skeleton:

```jsonc
{
  "items": [
    {
      "id": "entry-id",
      "activity_name": "Deep focus block",
      "category": "Work",
      "time_elapsed": 1500,
      "num_cycle": 3,
      "time_mode": "Pomodoro",
      "start_time": "2025-11-19T02:04:00Z",
      "end_time": "2025-11-19T02:29:00Z",
      "image": "https://signed-url"
    }
  ],
  "next_page_token": "opaque-cursor",
  "total_items": 42
}
```

#### `POST /v1/productivities`

Accepts JSON or `multipart/form-data` (binary field named `image`). Required fields are bold.

| Field               | Type          | Notes              |
| ------------------- | ------------- | ------------------ |
| **`activity_name`** | string        | Trimmed, non-empty |
| **`time_elapsed`**  | int (seconds) | > 0                |
| **`num_cycle`**     | int           | > 0                |

# FocusNest Services Handbook

Centralized API contract and runbook for every service in this monorepo. All public traffic flows through the Gateway API, but each downstream service exposes a REST surface for internal integrations, Postman testing, and local development.

## Monorepo map

| Service          | Path                | Responsibility                                                  | Primary endpoints                               |
| ---------------- | ------------------- | --------------------------------------------------------------- | ----------------------------------------------- |
| Focus Service    | `focus-service/`    | CRUD for productivity sessions, attachment uploads, pagination. | `/v1/productivities`, `/v1/productivities/{id}` |
| Progress Service | `progress-service/` | Derived analytics (summaries, streaks, distributions).          | `/v1/progress/summary`, `/v1/progress/streak/*` |
| User Service     | `user-service/`     | Profile data plus derived metadata (streak counters, totals).   | `/v1/users/me`                                  |
| Chatbot Service  | `chatbot-service/`  | Multi-session productivity coach backed by Gemini / Vertex.     | `/v1/chatbot/*`                                 |
| Gateway API      | `gateway-api/`      | Public entry point that handles Clerk auth and request routing. | `/v1/*` proxy surface                           |
| Shared Libraries | `shared-libs/`      | Common auth, logging, DTO, and server scaffolding.              | Imported modules                                |

## Common requirements

- **Auth:** In production everything sits behind Gateway API + Clerk JWT validation. When calling a service directly (local dev, port-forward, etc.) include `X-User-ID` with the authenticated subject (case-insensitive).
- **Dates & times:** Date-only fields use `YYYY-MM-DD`. Timestamps are RFC3339 with UTC (`2025-11-20T07:00:00Z`). Most analytics are anchored to the Asia/Jakarta timezone when defaults are needed.
- **Environments:** Each service honors `AUTH_MODE=noop` for local hacking. Firestore clients need `GCP_PROJECT_ID` plus either emulator variables or ADC credentials (`GOOGLE_APPLICATION_CREDENTIALS`).
- **Testing:** Run `go test ./...` from any service folder (or repo root) before pushing. Go 1.24+ and Docker are required for parity builds.

---

## Service API reference

### Focus Service — `/v1/productivities`

Allowed values:

- `category`: `Work`, `Study`, `Read`, `Journal`, `Cook`, `Workout`, `Music`, `Other`
- `time_mode`: `Pomodoro`, `Deep Work`, `Quick Focus`, `Free Timer`, `Other`
- `mood` (optional): `Fokus`, `Semangat`, `Biasa Aja`, `Capek`, `Burn Out`, `Mengantuk`

#### `GET /v1/productivities`

| Query        | Type   | Notes                         |
| ------------ | ------ | ----------------------------- |
| `page_size`  | int    | Default 20, max 100           |
| `page_token` | string | Cursor from previous response |
| `month`      | int    | 1-12; requires `year`         |
| `year`       | int    | 1970-2100; requires `month`   |

Response skeleton:

```jsonc
{
  "items": [
    {
      "id": "entry-id",
      "activity_name": "Deep focus block",
      "category": "Work",
      "time_elapsed": 1500,
      "num_cycle": 3,
      "time_mode": "Pomodoro",
      "start_time": "2025-11-19T02:04:00Z",
      "end_time": "2025-11-19T02:29:00Z",
      "image": "https://signed-url"
    }
  ],
  "next_page_token": "opaque-cursor",
  "total_items": 42
}
```

#### `POST /v1/productivities`

Accepts JSON or `multipart/form-data` (binary field named `image`). Required fields are bold.

| Field               | Type                           | Notes                               |
| ------------------- | ------------------------------ | ----------------------------------- |
| **`activity_name`** | string                         | Trimmed, non-empty                  |
| **`time_elapsed`**  | int (seconds)                  | > 0                                 |
| **`num_cycle`**     | int                            | > 0                                 |
| **`time_mode`**     | enum                           | Use allowed list                    |
| **`category`**      | enum                           | Use allowed list                    |
| `description`       | string                         | ≤ 2000 chars                        |
| `mood`              | enum                           | Optional                            |
| **`start_time`**    | RFC3339 timestamp              | UTC                                 |
| **`end_time`**      | RFC3339 timestamp              | Must be ≥ `start_time`              |
| `image`             | file (`.jpg`, `.jpeg`, `.png`) | Multipart only                      |
| `image_url`         | string                         | HTTPS link when using remote assets |

#### `PATCH /v1/productivities/{id}`

Same shape as `POST`; all fields optional but at least one mutation (or a new `image`) must be supplied. Values are validated against the enums above.

#### `GET /v1/productivities/{id}`

Returns the full entry with all fields captured at creation time. The `image` field is automatically rewritten to a signed URL when the stored value is a Cloud Storage path.

#### `DELETE /v1/productivities/{id}`

Soft-delete; repeated deletes return `404`.

---

### Progress Service — `/v1/progress`

#### `GET /v1/progress/summary`

| Query            | Type                                      | Notes                            |
| ---------------- | ----------------------------------------- | -------------------------------- |
| `range`          | enum (`week`, `month`, `3months`, `year`) | Default `week`                   |
| `category`       | string                                    | Optional filter                  |
| `reference_date` | `YYYY-MM-DD`                              | Defaults to today (Asia/Jakarta) |

Response:

```jsonc
{
  "range": "week",
  "reference_date": "2025-11-20T00:00:00Z",
  "time_distribution": [
    { "label": "Mon", "time_elapsed": 120 },
    { "label": "Tue", "time_elapsed": 95 }
  ],
  "total_filtered_time": 420,
  "total_time_frame": 580,
  "total_sessions": 12,
  "most_productive_hour_start": "2025-11-20T01:00:00Z",
  "most_productive_hour_end": "2025-11-20T02:00:00Z"
}
```

#### Streak endpoints

All streak endpoints return `days`, an ordered list with:

```jsonc
{
  "date": "2025-11-19",
  "day": "Wednesday",
  "status": "done" // or "skipped" / "upcoming"
}
```

- `GET /v1/progress/streak/monthly?date=YYYY-MM-DD` — derives `month`/`year` from the optional anchor date. Defaults to current month in Asia/Jakarta. Response includes `total_streak` (longest all-time) and `current_streak` for the displayed month.
- `GET /v1/progress/streak/weekly?date=YYYY-MM-DD` — snapshots the ISO week containing `date` (defaults to current week). Response includes ISO week label and streak metadata.
- `GET /v1/progress/streak/current` — examines the trailing 30-day window ending today.

---

### User Service — `/v1/users/me`

#### `GET /v1/users/me`

```jsonc
{
  "user_id": "uid-123",
  "full_name": "Focus Nest",
  "bio": "building calm productivity",
  "birthdate": "1996-09-14",
  "metadata": {
    "longest_streak": 12,
    "total_productivities": 48,
    "total_sessions": 48,
    "total_cycle": 180
  },
  "created_at": "2025-11-19T09:10:11Z",
  "updated_at": "2025-11-19T10:00:00Z"
}
```

If the profile does not exist yet, the service returns default values (blank strings, `null` birthdate) while still computing metadata from the user's productivities.

#### `PATCH /v1/users/me`

| Field       | Type                   | Notes                                            |
| ----------- | ---------------------- | ------------------------------------------------ |
| `full_name` | string                 | Optional, trimmed                                |
| `username`  | string                 | Optional, trimmed, unique at product level       |
| `bio`       | string                 | Optional, trimmed                                |
| `birthdate` | `YYYY-MM-DD` or `null` | Provide ISO date to set value or `null` to clear |

Body must be JSON (max 64 KB). Unknown fields are rejected. Empty strings are allowed, but `username` uniqueness is enforced upstream.

Metadata fields returned (read-only): `longest_streak`, `total_productivities`, `total_sessions`, and `total_cycle` (sum of `num_cycle` across all non-deleted productivities).

---

### Chatbot Service — `/v1/chatbot`

The chatbot keeps a complete history per user: every profile can open multiple sessions and each session records the ordered dialogue between the user (`role: user`) and the assistant (`role: assistant`). Gemini 2.5 Flash is used whenever a prompt remains within productivity/focus topics, with localization (English or Bahasa Indonesia) mirroring the most recent user input.

#### Gemini configuration

| Variable                            | Required | Notes                                                                         |
| ----------------------------------- | -------- | ----------------------------------------------------------------------------- |
| `GEMINI_API_KEY` / `GOOGLE_API_KEY` | Yes\*    | Provide one when `GOOGLE_GENAI_USE_VERTEXAI=false`.                           |
| `GOOGLE_GENAI_USE_VERTEXAI`         | No       | Set `true` to route through Vertex AI. Defaults to `false`.                   |
| `GOOGLE_CLOUD_LOCATION`             | Yes\*    | Required whenever `GOOGLE_GENAI_USE_VERTEXAI=true` (e.g., `asia-southeast2`). |
| `GCP_PROJECT_ID`                    | Yes      | Shared requirement across services.                                           |
| `GOOGLE_APPLICATION_CREDENTIALS`    | Yes\*    | Needed when using Vertex locally unless ADC already configured.               |

#### Endpoints

- `GET /v1/chatbot/sessions` — Lists every chat session for the caller (`id`, `title`, timestamps).
- `GET /v1/chatbot/history` — Returns every session together with its message log.
- `GET /v1/chatbot/sessions/{sessionID}` — Fetches a single session plus up to the 200 most recent messages (older turns are trimmed server-side).
- `PATCH /v1/chatbot/sessions/{sessionID}` — Body `{ "title": "Marketing retro" }`. Title is required and trimmed.
- `DELETE /v1/chatbot/sessions/{sessionID}` — Removes the session document and every dialog entry belonging to it (HTTP `204`).
- `POST /v1/chatbot/ask` — Body `{ "session_id": "optional-existing-id", "question": "How can I stay focused for 30 minutes?" }`. Omit `session_id` to start a new chat; the service derives a title from the first prompt and returns the generated session ID together with the assistant reply.

Responses include the saved assistant message and, when necessary, the trimmed context window that was sent to Gemini.

---

### Gateway API — `/v1/*`

Acts as the only public surface:

- Validates Clerk JWTs (production) or propagates noop auth (`AUTH_MODE=noop`) for local development.
- Injects `X-User-ID` before proxying to downstream services (`FOCUS_URL`, `PROGRESS_URL`, `CHATBOT_URL`, `USER_URL`).
- Exposes `/healthz` for Cloud Run load balancers and local smoke tests.
- Adds a consistent `requestId` header that downstream services log via `shared-libs/logging`.

---

## Tooling & workflows

- `go work sync` keeps local modules aligned with the multi-module workspace.
- `docker build -t focusnest/<service>:local -f <service>/Dockerfile .` produces Cloud Run–compatible images (static `CGO_ENABLED=0` binaries).
- `scripts/e2e.sh` can smoke-test critical flows end-to-end (focus → progress → user).
- Shared middleware (`shared-libs/server`) ships with logging, panic recovery, and CORS helpers.

## Contributing checklist

1. Branch from `main` and keep changes scoped to one service when possible.
2. Update this README whenever you add or change an endpoint.
3. Run `go test ./...` (or service-specific tests) before committing.
4. Push and open a PR—GitHub Actions will build, lint, and deploy to staging.

#### `GET /v1/progress/summary`

| Query            | Type                                      | Notes                            |
| ---------------- | ----------------------------------------- | -------------------------------- |
| `range`          | enum (`week`, `month`, `3months`, `year`) | Default `week`                   |
| `category`       | string                                    | Optional filter                  |
| `reference_date` | `YYYY-MM-DD`                              | Defaults to today (Asia/Jakarta) |

Response:

```jsonc
{
  "range": "week",
  "reference_date": "2025-11-20T00:00:00Z",
  "time_distribution": [
    { "label": "Mon", "time_elapsed": 120 },
    { "label": "Tue", "time_elapsed": 95 }
  ],
  "total_filtered_time": 420,
  "total_time_frame": 580,
  "total_sessions": 12,
  "most_productive_hour_start": "2025-11-20T01:00:00Z",
  "most_productive_hour_end": "2025-11-20T02:00:00Z"
}
```

#### `GET /v1/progress/streak/monthly`

`?date=YYYY-MM-DD` anchors the calendar month. Response includes `days[]`, `current_streak`, and `total_streak`.

#### `GET /v1/progress/streak/weekly`

`?date=YYYY-MM-DD` anchors the ISO week. Response mirrors monthly but for seven days.

#### `GET /v1/progress/streak/current`

Looks at the trailing 30 days ending today.

---

### User Service — `/v1/users/me`

#### `GET /v1/users/me`

```jsonc
{
  "user_id": "uid-123",
  "bio": "building calm productivity",
  "birthdate": "1996-09-14",
  "metadata": {
    "longest_streak": 12,
    "total_productivities": 48,
    "total_sessions": 48,
    "total_cycle": 180
  },
  "created_at": "2025-11-19T09:10:11Z",
  "updated_at": "2025-11-19T10:00:00Z"
}
```

If the profile does not exist, defaults are returned (blank strings, `null` birthdate) while metadata is still computed.

#### `PATCH /v1/users/me`

| Field       | Type                   | Notes             |
| ----------- | ---------------------- | ----------------- |
| `bio`       | string                 | Optional, trimmed |
| `birthdate` | `YYYY-MM-DD` or `null` | Set or clear date |

Any combination may be provided. Response echoes the updated profile.

---

### Chatbot Service — `/v1/chatbot`

- Firestore database defaults to `focusnest-prod`; emulator runs use `(default)`.
- Context window is capped via `CHATBOT_CONTEXT_MESSAGES` (default 16). Hidden system prompts never hit Firestore.
- Responses are localized (English/Bahasa) according to the most recent user input, and out-of-scope prompts return the boundaries message instead of hitting Gemini.

#### `GET /v1/chatbot/sessions`

List every chat session (`id`, `title`, `created_at`, `updated_at`).

#### `GET /v1/chatbot/sessions/{sessionID}`

Returns the session metadata plus the most recent **200** messages (older turns are trimmed server-side):

```jsonc
{
  "session": { "id": "abc", "title": "Weekly planning" },
  "messages": [
    { "role": "user", "content": "help me plan" },
    { "role": "assistant", "content": "Let's prioritize…" }
  ]
}
```

#### `POST /v1/chatbot/ask`

Request body:

```json
{
  "session_id": "optional-existing-id",
  "question": "How can I stay focused for 30 minutes?"
}
```

| Field        | Type   | Notes                                   |
| ------------ | ------ | --------------------------------------- |
| `session_id` | string | Leave empty to auto-create a session    |
| `question`   | string | Required, trimmed, productivity-focused |

Response:

```jsonc
{
  "session_id": "abc",
  "assistant_message": {
    "role": "assistant",
    "content": "Here’s how to stay on track for the next 30 minutes…"
  }
}
```

Fetch the transcript via `GET /sessions/{id}` if you need the full log.

#### `PATCH /v1/chatbot/sessions/{sessionID}`

Body:

```json
{ "title": "Marketing retro" }
```

Title is required and trimmed.

#### `DELETE /v1/chatbot/sessions/{sessionID}`

Removes the session and all associated messages (HTTP `204`).

---
