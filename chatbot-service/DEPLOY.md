# Deploying Chatbot Service

## Option 1: Google Cloud Build (Recommended)

### Build and Push Image

```bash
# From the project root
gcloud builds submit --config=chatbot-service/cloudbuild.yaml
```

This will:
1. Build the Docker image
2. Push it to `gcr.io/$PROJECT_ID/chatbot-service`
3. Make it available for Cloud Run deployment

### Deploy to Cloud Run

```bash
gcloud run deploy chatbot-service \
  --image gcr.io/focusnest-470308/chatbot-service \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --set-env-vars="GOOGLE_GENAI_USE_VERTEXAI=true,GOOGLE_CLOUD_LOCATION=us-central1,GCP_PROJECT_ID=focusnest-470308" \
  --set-secrets="GOOGLE_APPLICATION_CREDENTIALS=service-account-key:latest"
```

Or if you want to use a service account secret:

```bash
# First, create a secret from your service account JSON
gcloud secrets create service-account-key \
  --data-file=service-account.json \
  --replication-policy="automatic"

# Then deploy with the secret
gcloud run deploy chatbot-service \
  --image gcr.io/focusnest-470308/chatbot-service \
  --platform managed \
  --region us-central1 \
  --allow-unauthenticated \
  --set-env-vars="GOOGLE_GENAI_USE_VERTEXAI=true,GOOGLE_CLOUD_LOCATION=us-central1,GCP_PROJECT_ID=focusnest-470308" \
  --set-secrets="GOOGLE_APPLICATION_CREDENTIALS=service-account-key:latest"
```

## Option 2: Manual Docker Push

### Build and Tag

```bash
# Build the image
docker build -t gcr.io/focusnest-470308/chatbot-service -f chatbot-service/Dockerfile .

# Authenticate with GCR
gcloud auth configure-docker

# Push the image
docker push gcr.io/focusnest-470308/chatbot-service
```

### Deploy to Cloud Run

```bash
gcloud run deploy chatbot-service \
  --image gcr.io/focusnest-470308/chatbot-service \
  --platform managed \
  --region us-central1
```

## Option 3: Git Push (CI/CD)

If you have GitHub Actions or similar CI/CD set up:

1. **Commit your changes:**
   ```bash
   git add chatbot-service/
   git commit -m "feat: improve chatbot responses and add Vertex AI support"
   git push origin your-branch
   ```

2. **Open a PR** - The CI/CD pipeline should automatically build and deploy

## Environment Variables for Production

Make sure to set these in Cloud Run:

| Variable | Value | Notes |
|----------|-------|-------|
| `GOOGLE_GENAI_USE_VERTEXAI` | `true` | Use Vertex AI |
| `GOOGLE_CLOUD_LOCATION` | `us-central1` | Your GCP region |
| `GCP_PROJECT_ID` | `focusnest-470308` | Your project ID |
| `GOOGLE_APPLICATION_CREDENTIALS` | `/secrets/service-account.json` | Path to service account (if using secrets) |
| `PORT` | `8080` | Port the service listens on |
| `AUTH_MODE` | `clerk` | Use Clerk in production (not `noop`) |
| `CLERK_JWKS_URL` | `...` | Clerk JWKS URL |
| `CLERK_ISSUER` | `...` | Clerk issuer |

## Quick Deploy Script

Save this as `deploy.sh`:

```bash
#!/bin/bash
set -e

PROJECT_ID="focusnest-470308"
REGION="us-central1"
SERVICE_NAME="chatbot-service"

echo "Building and deploying $SERVICE_NAME..."

# Build and push
gcloud builds submit --config=$SERVICE_NAME/cloudbuild.yaml

# Deploy to Cloud Run
gcloud run deploy $SERVICE_NAME \
  --image gcr.io/$PROJECT_ID/$SERVICE_NAME \
  --platform managed \
  --region $REGION \
  --allow-unauthenticated \
  --set-env-vars="GOOGLE_GENAI_USE_VERTEXAI=true,GOOGLE_CLOUD_LOCATION=$REGION,GCP_PROJECT_ID=$PROJECT_ID"

echo "Deployment complete!"
```

Make it executable and run:
```bash
chmod +x deploy.sh
./deploy.sh
```

