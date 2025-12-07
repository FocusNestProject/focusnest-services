#!/bin/bash
# Test script to verify Gemini Assistant connection
# Usage: ./test-assistant.sh

set -e

echo "Testing Gemini Assistant Connection..."
echo "======================================"

# Check if using Vertex AI or API Key
if [ "$GOOGLE_GENAI_USE_VERTEXAI" = "true" ]; then
    echo "Mode: Vertex AI (Service Account)"
    echo "Project: ${GCP_PROJECT_ID:-$GOOGLE_CLOUD_PROJECT}"
    echo "Location: ${GOOGLE_CLOUD_LOCATION}"
    echo "Model: ${GEMINI_MODEL:-gemini-2.0-flash-exp}"
    
    if [ -z "$GOOGLE_APPLICATION_CREDENTIALS" ]; then
        echo "✗ GOOGLE_APPLICATION_CREDENTIALS not set"
        echo ""
        echo "To set up Vertex AI with service account:"
        echo "  1. Go to https://console.cloud.google.com/"
        echo "  2. Create a service account in IAM & Admin > Service Accounts"
        echo "  3. Grant it 'Vertex AI User' role"
        echo "  4. Create and download a JSON key"
        echo "  5. Set: export GOOGLE_APPLICATION_CREDENTIALS='/path/to/service-account.json'"
        echo ""
        echo "See GET_API_KEY.md for detailed instructions"
        exit 1
    else
        echo "Credentials path: $GOOGLE_APPLICATION_CREDENTIALS"
        if [ -f "$GOOGLE_APPLICATION_CREDENTIALS" ]; then
            echo "✓ Service account file exists"
            # Try to extract project ID from JSON if GCP_PROJECT_ID not set
            if [ -z "$GCP_PROJECT_ID" ] && [ -z "$GOOGLE_CLOUD_PROJECT" ]; then
                PROJECT_FROM_JSON=$(grep -o '"project_id"[[:space:]]*:[[:space:]]*"[^"]*"' "$GOOGLE_APPLICATION_CREDENTIALS" | head -1 | cut -d'"' -f4)
                if [ -n "$PROJECT_FROM_JSON" ]; then
                    echo "  Found project_id in JSON: $PROJECT_FROM_JSON"
                fi
            fi
        else
            echo "✗ Service account file not found at: $GOOGLE_APPLICATION_CREDENTIALS"
            exit 1
        fi
    fi
else
    echo "Mode: Gemini API (API Key)"
    echo "Model: ${GEMINI_MODEL:-gemini-2.0-flash-exp}"
    
    if [ -n "$GEMINI_API_KEY" ]; then
        echo "✓ GEMINI_API_KEY is set (length: ${#GEMINI_API_KEY})"
    elif [ -n "$GOOGLE_API_KEY" ]; then
        echo "✓ GOOGLE_API_KEY is set (length: ${#GOOGLE_API_KEY})"
    else
        echo "✗ No API key found (GEMINI_API_KEY or GOOGLE_API_KEY required)"
        echo ""
        echo "To get an API key:"
        echo "  1. Go to https://aistudio.google.com/app/apikey"
        echo "  2. Sign in with your Google account"
        echo "  3. Click 'Create API Key'"
        echo "  4. Copy the key and set it:"
        echo "     export GEMINI_API_KEY='your-key-here'"
        echo ""
        echo "Or use Vertex AI with service account (recommended):"
        echo "  Set GOOGLE_GENAI_USE_VERTEXAI=true and provide service account JSON"
        echo ""
        echo "See GET_API_KEY.md for detailed instructions"
        exit 1
    fi
fi

echo ""
echo "To test the assistant, run the chatbot service and check logs for:"
echo "  - Error messages with details about what failed"
echo ""
echo "Common issues:"
echo "  1. Model name might be incorrect - try: gemini-2.0-flash-exp or gemini-1.5-flash"
echo "  2. Vertex AI requires proper service account credentials"
echo "  3. Check that the model is enabled in your GCP project"
echo "  4. Verify location matches your project region (e.g., us-central1)"
echo ""
echo "For more help, see: GET_API_KEY.md"

