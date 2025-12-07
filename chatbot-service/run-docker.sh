#!/bin/bash
# Run chatbot service with Docker
# Usage: ./run-docker.sh

set -e

# Default values
IMAGE_NAME="${CHATBOT_IMAGE:-chatbot-service-local}"
PORT="${PORT:-8082}"
GCP_PROJECT_ID="${GCP_PROJECT_ID:-focusnest-470308}"
LOCATION="${GOOGLE_CLOUD_LOCATION:-us-central1}"
SERVICE_ACCOUNT="${SERVICE_ACCOUNT:-./service-account.json}"

# Check if service account file exists
if [ ! -f "$SERVICE_ACCOUNT" ]; then
    echo "Error: Service account file not found: $SERVICE_ACCOUNT"
    echo ""
    echo "To create a service account:"
    echo "  1. Go to https://console.cloud.google.com/iam-admin/serviceaccounts"
    echo "  2. Create a service account with 'Vertex AI User' role"
    echo "  3. Download the JSON key"
    echo "  4. Save it as: $SERVICE_ACCOUNT"
    echo ""
    echo "Or set SERVICE_ACCOUNT environment variable to point to your file"
    exit 1
fi

echo "Starting chatbot service..."
echo "  Image: $IMAGE_NAME"
echo "  Port: $PORT"
echo "  Project: $GCP_PROJECT_ID"
echo "  Location: $LOCATION"
echo "  Service Account: $SERVICE_ACCOUNT"
echo ""

docker run --rm -p ${PORT}:8080 \
  -v "$(pwd)/$SERVICE_ACCOUNT:/key.json:ro" \
  -e GOOGLE_APPLICATION_CREDENTIALS=/key.json \
  -e GOOGLE_GENAI_USE_VERTEXAI=true \
  -e GOOGLE_CLOUD_LOCATION=${LOCATION} \
  -e PORT=8080 \
  -e GCP_PROJECT_ID=${GCP_PROJECT_ID} \
  -e AUTH_MODE=noop \
  ${IMAGE_NAME}

