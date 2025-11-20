# FocusNest Chatbot Service

Conversational assistant backend for FocusNest. Sessions, prompts, and assistant replies live in Firestore while Gemini (Vertex AI or API key mode) handles completions. The current beta ships purely over REST—streaming and pagination will arrive in a later iteration.

## Features (Beta Scope)

| Capability              | Endpoint(s)                                                                          | Notes                                                                |
| ----------------------- | ------------------------------------------------------------------------------------ | -------------------------------------------------------------------- |
| List all sessions       | `GET /v1/chatbot/sessions`                                                           | Returns every conversation with title + timestamps.                  |
| Session transcript      | `GET /v1/chatbot/sessions/{sessionID}`                                               | Includes metadata plus the latest 200 messages (oldest trimmed).     |
| Rename or delete a chat | `PATCH /v1/chatbot/sessions/{sessionID}` / `DELETE /v1/chatbot/sessions/{sessionID}` | Title edits and hard deletes.                                        |
| Chat via REST           | `POST /v1/chatbot/ask`                                                               | Creates/continues sessions; synchronous response (no streaming yet). |

All routes run behind the shared auth middleware and expect the gateway to inject `X-User-ID` for the authenticated subject.

### Beta Constraints & Roadmap

- **REST only**: we keep a synchronous request/response flow for launch. SSE/WebSocket streaming is on the backlog once stability is proven.
- **Pagination**: `GET /sessions` currently returns the full list. Add `pageSize`/`pageToken` later by extending the repository queries.
- **Context budgeting**: `CHATBOT_CONTEXT_MESSAGES` caps how many recent turns are sent to Gemini. System prompts (`cfg.LLM.ContextMessages`) are injected at runtime so the hidden role instructions never leak back to Firestore.

## Configuration

Environment variables (also see `internal/config/config.go`):

| Variable                            | Default            | Description                                                                      |
| ----------------------------------- | ------------------ | -------------------------------------------------------------------------------- |
| `PORT`                              | `8080`             | HTTP listen port.                                                                |
| `GCP_PROJECT_ID`                    | _none_             | Required GCP project for Firestore / Vertex.                                     |
| `AUTH_MODE`                         | `noop`             | `noop` or `clerk`. Additional Clerk vars required when set to `clerk`.           |
| `CLERK_JWKS_URL`                    | _none_             | Needed when `AUTH_MODE=clerk`.                                                   |
| `CLERK_ISSUER`                      | _none_             | Optional Clerk issuer for validation.                                            |
| `FIRESTORE_EMULATOR_HOST`           | _empty_            | Point to emulator (also switches database to `(default)`).                       |
| `GEMINI_API_KEY` / `GOOGLE_API_KEY` | _none_             | API key when not using Vertex.                                                   |
| `GOOGLE_GENAI_USE_VERTEXAI`         | `false`            | Toggle Vertex mode. Requires workload identity/ADC plus `GOOGLE_CLOUD_LOCATION`. |
| `GEMINI_MODEL`                      | `gemini-2.5-flash` | Model name passed to the assistant.                                              |
| `CHATBOT_CONTEXT_MESSAGES`          | `16`               | Max recent turns kept when building the prompt.                                  |
| `CHATBOT_MAX_OUTPUT_TOKENS`         | `512`              | Gemini output cap.                                                               |

## Running Locally

```bash
# inside focusnest-services/chatbot-service
export GCP_PROJECT_ID=focusnest-dev
export AUTH_MODE=noop
export FIRESTORE_EMULATOR_HOST=localhost:8081
export GEMINI_API_KEY=your-key

# run with Go
GOOGLE_APPLICATION_CREDENTIALS=/path/to/key.json \
  go run ./cmd/server
```

Docker example:

```bash
cd focusnest-services
docker build -t focusnest/chatbot-service:local -f chatbot-service/Dockerfile .
docker run --rm -p 8080:8080 \
  -v "$(pwd)/service-account.json":/key.json:ro \
  -e GOOGLE_APPLICATION_CREDENTIALS=/key.json \
  -e GCP_PROJECT_ID=focusnest-dev \
  -e AUTH_MODE=noop \
  -e GOOGLE_GENAI_USE_VERTEXAI=true \
  -e GOOGLE_CLOUD_LOCATION=asia-southeast2 \
  focusnest/chatbot-service:local
```

## REST API Reference

All responses are JSON. Include `X-User-ID` (or `x-user-id`) header on every call.

### `GET /v1/chatbot/sessions`

Lists every session.

```jsonc
{
  "sessions": [
    {
      "session_id": "abc123",
      "title": "Weekly planning",
      "updated_at": "2025-11-20T06:30:00Z",
      "created_at": "2025-11-18T14:03:00Z"
    }
  ]
}
```

### `GET /v1/chatbot/sessions/{sessionID}`

Returns the session metadata plus the most recent 200 messages (oldest turns are trimmed server-side to cap payload size).

```jsonc
{
  "session": {
    "session_id": "abc123",
    "title": "Weekly planning"
  },
  "messages": [
    { "role": "user", "content": "help me plan" },
    { "role": "assistant", "content": "Sure, let's start..." }
  ]
}
```

### `POST /v1/chatbot/ask`

Send a user question. Leave `session_id` empty to start a new conversation.

```json
{
  "session_id": "abc123",
  "question": "What should I focus on today?"
}
```

```jsonc
{
  "session_id": "abc123",
  "assistant_message": "Let's prioritize your top three OKRs..."
}
```

### `PATCH /v1/chatbot/sessions/{sessionID}`

Rename a chat thread.

```json
{
  "title": "Marketing retro"
}
```

Response: `{ "status": "updated" }`

### `DELETE /v1/chatbot/sessions/{sessionID}`

Deletes the session (no body, returns HTTP 204).

## Future Enhancements

- Streaming responses via Server-Sent Events or WebSocket while keeping REST fallback.
- Optional pagination & filtering on the sessions endpoint.
- Background summarization to store preview text per session.
- Request metrics and Firestore latency tracking for better alerting.
