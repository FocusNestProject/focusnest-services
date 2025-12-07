#!/bin/bash
# Quick setup script for Vertex AI with service account
# This helps you set up Vertex AI for the chatbot service

set -e

echo "Vertex AI Setup for Chatbot Service"
echo "===================================="
echo ""

# Check if service account file exists
if [ -f "./service-account.json" ]; then
    echo "✓ Found service-account.json"
    
    # Extract project ID
    PROJECT_ID=$(grep -o '"project_id"[[:space:]]*:[[:space:]]*"[^"]*"' ./service-account.json | head -1 | cut -d'"' -f4)
    
    if [ -n "$PROJECT_ID" ]; then
        echo "✓ Project ID: $PROJECT_ID"
    fi
    
    echo ""
    echo "To use Vertex AI, set these environment variables:"
    echo ""
    echo "  export GOOGLE_APPLICATION_CREDENTIALS=\"$(pwd)/service-account.json\""
    echo "  export GOOGLE_GENAI_USE_VERTEXAI=\"true\""
    echo "  export GOOGLE_CLOUD_LOCATION=\"us-central1\""
    echo "  export GCP_PROJECT_ID=\"$PROJECT_ID\""
    echo ""
    echo "Or update docker-compose.yml with:"
    echo "  volumes:"
    echo "    - ./service-account.json:/root/service-account.json:ro"
    echo "  environment:"
    echo "    GOOGLE_GENAI_USE_VERTEXAI: \"true\""
    echo "    GOOGLE_CLOUD_LOCATION: \"us-central1\""
    echo "    GCP_PROJECT_ID: \"$PROJECT_ID\""
    echo "    GOOGLE_APPLICATION_CREDENTIALS: \"/root/service-account.json\""
else
    echo "✗ service-account.json not found"
    echo ""
    echo "To create a service account:"
    echo ""
    echo "1. Go to: https://console.cloud.google.com/"
    echo "2. Select or create a project"
    echo "3. Enable 'Vertex AI API' in APIs & Services > Library"
    echo "4. Go to IAM & Admin > Service Accounts"
    echo "5. Click 'Create Service Account'"
    echo "6. Name it (e.g., 'chatbot-service')"
    echo "7. Grant role: 'Vertex AI User'"
    echo "8. Create a JSON key and download it"
    echo "9. Save it as 'service-account.json' in this directory"
    echo ""
    echo "Then run this script again!"
    echo ""
    echo "See GET_API_KEY.md for detailed step-by-step instructions"
fi

