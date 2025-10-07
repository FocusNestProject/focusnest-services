# FocusNest Services

Production-ready microservices architecture for the FocusNest productivity tracking application.

## 🏗️ Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Frontend      │    │  Gateway API    │    │  Activity       │
│   (Expo App)    │───▶│  (Port 8080)    │───▶│  Service        │
│                 │    │  Main Entry     │    │  (Port 8081)    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
                              │
                              ├───▶ User Service (Port 8082)
                              ├───▶ Session Service (Port 8083)
                              ├───▶ Media Service (Port 8084)
                              ├───▶ Analytics Service (Port 8085)
                              └───▶ Webhook Service (Port 8086)
```

## 🚀 Services

| Service | Port | Purpose |
|---------|------|---------|
| **Gateway API** | `8080` | Main entry point with authentication and routing |
| **Activity Service** | `8081` | Productivity tracking, chatbot, analytics, user profiles |
| **User Service** | `8082` | User management |
| **Session Service** | `8083` | Session management |
| **Media Service** | `8084` | File/media handling |
| **Analytics Service** | `8085` | Analytics and reporting |
| **Webhook Service** | `8086` | Webhook handling |
| **Firebase Emulator** | `8088` | Firestore database (development) |

## 🔧 Quick Start

### Prerequisites
- Docker & Docker Compose
- Go 1.22+ (for development)

### Environment Setup
1. Copy environment template:
   ```bash
   cp env.example .env
   ```

2. Configure your Clerk authentication:
   ```bash
   # Edit .env with your Clerk values
   CLERK_JWKS_URL=https://your-app-name.clerk.accounts.dev/.well-known/jwks.json
   CLERK_AUDIENCE=your-application-id
   CLERK_ISSUER=https://your-app-name.clerk.accounts.dev
   ```

### Running Services
```bash
# Start all services
docker-compose up -d

# Or use the helper script
./start.sh

# Test services
./test-services.sh
```

### API Endpoints
- **Main Gateway**: `http://localhost:8080`
- **Health Check**: `http://localhost:8080/healthz`

## 📱 Frontend Integration

```javascript
// Your Expo app configuration
const API_BASE_URL = 'http://localhost:8080';

// Example API call
const response = await fetch(`${API_BASE_URL}/v1/productivities`, {
  headers: {
    'Authorization': 'Bearer YOUR_JWT_TOKEN',
    'Content-Type': 'application/json'
  }
});
```

## 🏗️ Development

### Project Structure
```
├── gateway-api/          # Main API gateway
├── activity-service/     # Core business logic
├── user-service/         # User management
├── session-service/      # Session handling
├── media-service/        # File/media processing
├── analytics-service/    # Analytics and reporting
├── webhook-service/      # Webhook handling
├── shared-libs/         # Shared libraries
├── docker-compose.yml   # Service orchestration
└── env.example          # Environment template
```

### Building Services
```bash
# Build specific service
docker-compose build gateway-api

# Build all services
docker-compose build
```

## 🔐 Authentication

The system uses **Clerk** for JWT-based authentication:
- JWT tokens are verified at the gateway level
- User context is automatically injected into downstream services
- All protected routes require valid authentication

## 📊 Database

- **Development**: Firebase Emulator (Firestore)
- **Production**: Google Cloud Firestore
- **Configuration**: See `FIRESTORE_SETUP.md`

## 🚀 Production Deployment

For production deployment, see:
- `README_DOCKER.md` - Docker deployment guide
- `FIRESTORE_SETUP.md` - Database setup

## 📋 API Documentation

- **OpenAPI Specs**: Available in each service's `api/` directory
- **Postman Collection**: `postman/FocusNest.postman_collection.json`

## 🛠️ Scripts

- `start.sh` - Start all services
- `test-services.sh` - Test service health
- `scripts/e2e.sh` - End-to-end testing

## 📝 License

This project is part of the FocusNest productivity tracking application.