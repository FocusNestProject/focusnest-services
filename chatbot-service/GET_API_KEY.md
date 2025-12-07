# How to Get Gemini API Access

## Option 1: Vertex AI with Service Account (Recommended for Production)

This is the recommended approach - no API keys needed, just a service account JSON file.

### Step 1: Create a Service Account in Google Cloud

1. **Go to Google Cloud Console**
   - Visit: https://console.cloud.google.com/
   - Select or create a project

2. **Enable Required APIs**
   - Go to "APIs & Services" > "Library"
   - Search for and enable:
     - "Vertex AI API"
     - "Generative Language API" (if needed)

3. **Create Service Account**
   - Go to "IAM & Admin" > "Service Accounts"
   - Click "Create Service Account"
   - Name: `chatbot-service` (or any name you prefer)
   - Description: "Service account for FocusNest chatbot"
   - Click "Create and Continue"

4. **Grant Permissions**
   - Role: Select "Vertex AI User" (or "AI Platform User")
   - Click "Continue" then "Done"

5. **Create and Download JSON Key**
   - Click on the service account you just created
   - Go to "Keys" tab
   - Click "Add Key" > "Create new key"
   - Select "JSON" format
   - Click "Create" - the JSON file will download automatically
   - **Save this file securely!** (e.g., `service-account.json`)

### Step 2: Use the Service Account

**For Docker Compose:**
```yaml
chatbot-service:
  environment:
    GOOGLE_GENAI_USE_VERTEXAI: "true"
    GOOGLE_CLOUD_LOCATION: "us-central1"  # or your preferred region
    GCP_PROJECT_ID: "your-project-id"
    GOOGLE_APPLICATION_CREDENTIALS: "/path/to/service-account.json"
  volumes:
    - ./service-account.json:/path/to/service-account.json:ro
```

**For Local Development:**
```bash
export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account.json"
export GOOGLE_GENAI_USE_VERTEXAI="true"
export GOOGLE_CLOUD_LOCATION="us-central1"
export GCP_PROJECT_ID="your-project-id"
```

**Benefits:**
- ✅ No API key to manage
- ✅ Better security (IAM-based)
- ✅ Production-ready
- ✅ Can set up fine-grained permissions
- ✅ Works with GCP billing and quotas

---

## Option 2: Google AI Studio (Easiest - Free Tier Available)

1. **Go to Google AI Studio**
   - Visit: https://aistudio.google.com/
   - Sign in with your Google account

2. **Get Your API Key**
   - Click on "Get API Key" in the left sidebar
   - Or go directly to: https://aistudio.google.com/app/apikey
   - Click "Create API Key"
   - Select an existing Google Cloud project or create a new one
   - Copy the API key immediately (you won't be able to see it again!)

3. **Set it in your environment**
   ```bash
   export GEMINI_API_KEY="your-api-key-here"
   ```
   
   Or add to your `.env` file:
   ```
   GEMINI_API_KEY=your-api-key-here
   ```

## Option 2: Google Cloud Console (For Production/Vertex AI)

If you want to use Vertex AI (recommended for production):

1. **Go to Google Cloud Console**
   - Visit: https://console.cloud.google.com/
   - Select or create a project

2. **Enable the Vertex AI API**
   - Go to "APIs & Services" > "Library"
   - Search for "Vertex AI API"
   - Click "Enable"

3. **Create a Service Account**
   - Go to "IAM & Admin" > "Service Accounts"
   - Click "Create Service Account"
   - Give it a name (e.g., "chatbot-service")
   - Grant it the "Vertex AI User" role
   - Click "Done"

4. **Create and Download JSON Key**
   - Click on the service account you just created
   - Go to "Keys" tab
   - Click "Add Key" > "Create new key"
   - Select "JSON" format
   - Download the JSON file

5. **Use the Service Account**
   - Mount the JSON file in Docker or set the path:
   ```bash
   export GOOGLE_APPLICATION_CREDENTIALS="/path/to/service-account.json"
   export GOOGLE_GENAI_USE_VERTEXAI="true"
   export GOOGLE_CLOUD_LOCATION="us-central1"
   export GCP_PROJECT_ID="your-project-id"
   ```

## Quick Test

After setting your API key, test it:

```bash
# Test with the script
./chatbot-service/test-assistant.sh

# Or test directly with curl
curl -X POST http://localhost:8082/v1/chatbot/ask \
  -H "Content-Type: application/json" \
  -H "X-User-ID: test-user" \
  -d '{"question": "How can I stay focused?"}'
```

## Notes

- **Free Tier**: Google AI Studio offers a free tier with generous limits
- **Rate Limits**: Free tier has rate limits, but should be fine for development
- **Security**: Never commit API keys to git! Use environment variables or secrets management
- **Model Availability**: Some models may require enabling specific APIs in Google Cloud Console

