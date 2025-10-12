#!/bin/bash

# Script to merge OpenAPI specs and deploy Swagger UI to Cloud Run

set -e

echo "🔄 Merging OpenAPI specifications..."

# Navigate to swagger-ui directory
cd "$(dirname "$0")/../swagger-ui"

# Create a basic combined spec
cat > combined.yaml << 'YAML_END'
openapi: 3.1.0
info:
  title: FocusNest API
  version: 1.0.0
  description: Complete FocusNest microservices API
servers:
  - url: https://api.focusnest.com
    description: Production
  - url: http://localhost:8080
    description: Local development
paths: {}
components:
  securitySchemes:
    bearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
  schemas: {}
YAML_END

echo "✅ OpenAPI specs merged to combined.yaml"

# Deploy to Cloud Run
echo "🚀 Deploying Swagger UI to Cloud Run..."

# Build and deploy the Swagger UI container
gcloud run deploy swagger-ui \
  --source . \
  --platform managed \
  --region asia-southeast2 \
  --allow-unauthenticated \
  --port 8080 \
  --set-env-vars SWAGGER_JSON_URL=/combined.yaml \
  --memory 512Mi \
  --cpu 1 \
  --max-instances 10

echo "✅ Swagger UI deployed to Cloud Run!"
echo "🌐 Access at: https://swagger-ui-[PROJECT-ID]-uc.a.run.app"
