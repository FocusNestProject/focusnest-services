#!/bin/bash
# Quick deploy script for chatbot service
set -e

PROJECT_ID="${GCP_PROJECT_ID:-focusnest-470308}"
REGION="${GOOGLE_CLOUD_LOCATION:-us-central1}"
SERVICE_NAME="chatbot-service"

echo "🚀 Building and deploying $SERVICE_NAME..."
echo "   Project: $PROJECT_ID"
echo "   Region: $REGION"
echo ""

# Build and push
echo "📦 Building Docker image..."
gcloud builds submit --config=$SERVICE_NAME/cloudbuild.yaml

# Deploy to Cloud Run
echo "🚢 Deploying to Cloud Run..."
gcloud run deploy $SERVICE_NAME \
  --image gcr.io/$PROJECT_ID/$SERVICE_NAME \
  --platform managed \
  --region $REGION \
  --allow-unauthenticated \
  --set-env-vars="GOOGLE_GENAI_USE_VERTEXAI=true,GOOGLE_CLOUD_LOCATION=$REGION,GCP_PROJECT_ID=$PROJECT_ID" \
  --memory=512Mi \
  --cpu=1 \
  --timeout=60 \
  --max-instances=10

echo ""
echo "✅ Deployment complete!"
echo ""
echo "Get the service URL:"
gcloud run services describe $SERVICE_NAME --region $REGION --format="value(status.url)"

