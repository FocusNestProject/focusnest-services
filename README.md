# FocusNest Microservices

A comprehensive productivity tracking platform built with Go microservices, Google Cloud Firestore, and Cloud Storage.

## 🏗️ Architecture

- **Gateway API**: Main entry point with authentication
- **Focus Service**: Productivity tracking with image uploads
- **Progress Service**: Analytics and streak tracking
- **Chatbot Service**: AI-powered productivity assistance
- **User Service**: User profile management

## 🚀 Quick Start

### Prerequisites

- Go 1.24+
- Docker
- Google Cloud SDK (for production)
- Service Account Key (for Firestore/Storage access)

### Option 1: Local Development (Memory Storage)

For quick testing without Google Cloud setup:

```bash
# Clone the repository
git clone https://github.com/FocusNestProject/focusnest-services.git
cd focusnest-services

# Build and run focus service
cd focus-service
docker build -t focus-service-local -f Dockerfile .
docker run -p 8080:8080 \
  -e DATA_STORE=memory \
  -e AUTH_MODE=noop \
  focus-service-local

# Test the service
curl -H "X-user-id: test_user_123" \
  "http://localhost:8080/health"
```

### Option 2: Production Setup (Google Cloud)

For full functionality with Firestore and Cloud Storage:

```bash
# Set up Google Cloud credentials
export GOOGLE_APPLICATION_CREDENTIALS="path/to/service-account-key.json"
export GCP_PROJECT_ID="your-project-id"

# Build and run with Firestore
cd focus-service
docker build -t focus-service-local -f Dockerfile .
docker run -p 8080:8080 \
  -e DATA_STORE=firestore \
  -e AUTH_MODE=noop \
  -e GCP_PROJECT_ID=$GCP_PROJECT_ID \
  -v $GOOGLE_APPLICATION_CREDENTIALS:/app/credentials.json \
  focus-service-local
```

## 📋 Service-by-Service Setup

### 1. Focus Service (Productivity Tracking)

**Build:**
```bash
cd focus-service
docker build -t focus-service-local -f Dockerfile .
```

**Run with Memory Storage:**
```bash
docker run -p 8080:8080 \
  -e DATA_STORE=memory \
  -e AUTH_MODE=noop \
  focus-service-local
```

**Run with Firestore:**
```bash
docker run -p 8080:8080 \
  -e DATA_STORE=firestore \
  -e AUTH_MODE=noop \
  -e GCP_PROJECT_ID=your-project-id \
  -v /path/to/service-account.json:/app/credentials.json \
  focus-service-local
```

**Test Endpoints:**
```bash
# Health check
curl -H "X-user-id: test_user_123" \
  "http://localhost:8080/health"

# Create productivity entry
curl -X POST -H "X-user-id: test_user_123" \
  -F "category=Work" \
  -F "time_mode=Pomodoro" \
  -F "elapsed_ms=1500000" \
  -F "image=@/path/to/image.jpg" \
  "http://localhost:8080/v1/productivities"

# List productivities
curl -H "X-user-id: test_user_123" \
  "http://localhost:8080/v1/productivities"
```

### 2. Progress Service (Analytics & Streaks)

**Build:**
```bash
cd progress-service
docker build -t progress-service-local -f Dockerfile .
```

**Run:**
```bash
docker run -p 8081:8080 \
  -e DATA_STORE=memory \
  -e AUTH_MODE=noop \
  progress-service-local
```

**Test Streak Endpoints:**
```bash
# Monthly streak
curl -H "X-user-id: test_user_123" \
  "http://localhost:8081/v1/streaks/month?month=10&year=2025"

# Weekly streak
curl -H "X-user-id: test_user_123" \
  "http://localhost:8081/v1/streaks/week"

# Current streak
curl -H "X-user-id: test_user_123" \
  "http://localhost:8081/v1/streaks/current"
```

### 3. Chatbot Service

**Build:**
```bash
cd chatbot-service
docker build -t chatbot-service-local -f Dockerfile .
```

**Run:**
```bash
docker run -p 8082:8080 \
  -e DATA_STORE=memory \
  -e AUTH_MODE=noop \
  chatbot-service-local
```

### 4. User Service

**Build:**
```bash
cd user-service
docker build -t user-service-local -f Dockerfile .
```

**Run:**
```bash
docker run -p 8083:8080 \
  -e DATA_STORE=memory \
  -e AUTH_MODE=noop \
  user-service-local
```

### 5. Gateway API

**Build:**
```bash
cd gateway-api
docker build -t gateway-api-local -f Dockerfile .
```

**Run:**
```bash
docker run -p 8084:8080 \
  -e AUTH_MODE=noop \
  -e FOCUS_URL=http://localhost:8080 \
  -e PROGRESS_URL=http://localhost:8081 \
  -e CHATBOT_URL=http://localhost:8082 \
  -e USER_URL=http://localhost:8083 \
  gateway-api-local
```

## 🔧 Environment Variables

### Common Variables
- `DATA_STORE`: `memory` or `firestore`
- `AUTH_MODE`: `noop` (for testing) or `clerk` (for production)
- `PORT`: Service port (default: 8080)

### Google Cloud Variables
- `GCP_PROJECT_ID`: Your Google Cloud project ID
- `GOOGLE_APPLICATION_CREDENTIALS`: Path to service account key
- `FIRESTORE_EMULATOR_HOST`: For local Firestore emulator

### Service-Specific Variables
- `CLERK_JWKS_URL`: Clerk JWKS endpoint (for production auth)
- `CLERK_ISSUER`: Clerk issuer URL
- `CLERK_AUDIENCE`: Clerk audience

## 🧪 Testing

### Authentication
All services use `X-user-id` header for testing:
```bash
curl -H "X-user-id: test_user_123" \
  "http://localhost:8080/health"
```

### Image Upload Testing
```bash
# Create productivity with image
curl -X POST -H "X-user-id: test_user_123" \
  -F "category=Work" \
  -F "time_mode=Pomodoro" \
  -F "elapsed_ms=1500000" \
  -F "image=@/path/to/your/image.jpg" \
  "http://localhost:8080/v1/productivities"
```

### Streak Testing
```bash
# Test monthly streak
curl -H "X-user-id: test_user_123" \
  "http://localhost:8081/v1/streaks/month?month=10&year=2025"

# Test weekly streak
curl -H "X-user-id: test_user_123" \
  "http://localhost:8081/v1/streaks/week?date=2025-10-15"
```

## 🚀 Production Deployment

### Cloud Run Deployment
The project includes GitHub Actions workflows for automatic deployment:

1. **Push to main branch** triggers deployment
2. **All services** deploy to Cloud Run
3. **Swagger UI** is automatically deployed for API documentation

### Access Points
- **Gateway API**: `https://gateway-api-[PROJECT-ID]-uc.a.run.app`
- **Swagger UI**: `https://swagger-ui-[PROJECT-ID]-uc.a.run.app`
- **Focus Service**: `https://focus-service-[PROJECT-ID]-uc.a.run.app`
- **Progress Service**: `https://progress-service-[PROJECT-ID]-uc.a.run.app`

## 📚 API Documentation

### Swagger UI
Access the interactive API documentation at:
- **Local**: `http://localhost:8089` (if running docker-compose)
- **Production**: `https://swagger-ui-[PROJECT-ID]-uc.a.run.app`

### Key Endpoints

#### Focus Service
- `POST /v1/productivities` - Create productivity entry with image
- `GET /v1/productivities` - List productivities
- `GET /v1/productivities/{id}` - Get specific productivity
- `DELETE /v1/productivities/{id}` - Delete productivity

#### Progress Service
- `GET /v1/streaks/month` - Monthly streak data
- `GET /v1/streaks/week` - Weekly streak data
- `GET /v1/streaks/current` - Current running streak

## 🔒 Security Features

- **Signed URLs**: 24-hour expiry for image access
- **UUID Generation**: Standardized IDs throughout
- **Private Storage**: GCS bucket with signed URL access
- **Authentication**: Clerk JWT validation (production)

## 🛠️ Development

### Local Development with Docker Compose
```bash
# Start all services locally
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

### Code Formatting
```bash
# Format all Go code
find . -name "*.go" -exec go fmt {} \;

# Or use the workspace
go work sync
```

## 📁 Project Structure

```
focusnest-services/
├── focus-service/          # Productivity tracking
├── progress-service/       # Analytics & streaks
├── chatbot-service/        # AI assistance
├── user-service/           # User management
├── gateway-api/            # API gateway
├── shared-libs/            # Common libraries
├── swagger-ui/             # API documentation
└── .github/workflows/      # CI/CD pipelines
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests and formatting
5. Submit a pull request

## 📄 License

This project is licensed under the MIT License - see the LICENSE file for details.