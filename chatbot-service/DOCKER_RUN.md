# Running Chatbot Service with Docker

## Quick Start

### Build the image first:
```bash
cd chatbot-service
docker build -t chatbot-service-local -f Dockerfile ..
```

### Run with Vertex AI (Service Account):
```bash
docker run --rm -p 8082:8080 \
  -v $(pwd)/service-account.json:/key.json:ro \
  -e GOOGLE_APPLICATION_CREDENTIALS=/key.json \
  -e GOOGLE_GENAI_USE_VERTEXAI=true \
  -e GOOGLE_CLOUD_LOCATION=us-central1 \
  -e PORT=8080 \
  -e GCP_PROJECT_ID=focusnest-470308 \
  -e AUTH_MODE=noop \
  chatbot-service-local
```

### Or use the helper script:
```bash
# Make it executable
chmod +x run-docker.sh

# Run it
./run-docker.sh

# Or with custom values
GCP_PROJECT_ID=your-project PORT=8082 ./run-docker.sh
```

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GOOGLE_APPLICATION_CREDENTIALS` | Yes* | - | Path to service account JSON inside container |
| `GOOGLE_GENAI_USE_VERTEXAI` | No | `false` | Set to `true` to use Vertex AI |
| `GOOGLE_CLOUD_LOCATION` | Yes (if Vertex AI) | - | GCP region (e.g., `us-central1`) |
| `GCP_PROJECT_ID` | Yes | - | Your GCP project ID |
| `PORT` | No | `8080` | Port the service listens on |
| `AUTH_MODE` | No | `noop` | Authentication mode (`noop` for local dev) |
| `GEMINI_MODEL` | No | `gemini-2.0-flash-exp` | Model to use |
| `CHATBOT_CONTEXT_MESSAGES` | No | `32` | Number of messages in context window |
| `CHATBOT_MAX_OUTPUT_TOKENS` | No | `1024` | Max tokens in response |

*Required if using Vertex AI. For API key mode, use `GEMINI_API_KEY` instead.

## Alternative: Using Gemini API Key

If you prefer API key instead of service account:

```bash
docker run --rm -p 8082:8080 \
  -e GEMINI_API_KEY=your-api-key-here \
  -e GOOGLE_GENAI_USE_VERTEXAI=false \
  -e PORT=8080 \
  -e GCP_PROJECT_ID=focusnest-470308 \
  -e AUTH_MODE=noop \
  chatbot-service-local
```

## Testing

Once running, test the service:

```bash
# Health check (if you add a health endpoint)
curl http://localhost:8082/health

# Ask a question
curl -X POST http://localhost:8082/v1/chatbot/ask \
  -H "Content-Type: application/json" \
  -H "X-User-ID: test-user" \
  -d '{"question": "How can I stay focused?"}'
```

## Troubleshooting

### Service account not found
- Make sure the file exists: `ls -la service-account.json`
- Check the path in the volume mount matches the file location
- Verify the file is readable: `cat service-account.json | jq .project_id`

### Vertex AI errors
- Verify the service account has "Vertex AI User" role
- Check that Vertex AI API is enabled in your GCP project
- Ensure `GOOGLE_CLOUD_LOCATION` matches your project's region

### Model not found
- Try a different model: `-e GEMINI_MODEL=gemini-1.5-flash`
- Check available models in your GCP project

