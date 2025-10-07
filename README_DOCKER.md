# 🐳 FocusNest Docker Setup

Complete guide to run FocusNest microservices using Docker.

## 🚀 Quick Start

### 1. Setup Environment
```bash
# Copy environment template
cp env.example .env

# Edit configuration (optional for development)
nano .env
```

### 2. Start Services
```bash
# Option 1: Use the startup script
./start.sh

# Option 2: Manual start
docker-compose up -d
```

### 3. Test Services
```bash
# Check if all services are healthy
./test-services.sh
```

## 📋 Environment Configuration

### Required Environment Variables

I need you to provide the following environment variables for production setup:

#### 🔐 Clerk Authentication (Required for Production)
```bash
CLERK_JWKS_URL=https://your-clerk-instance.clerk.accounts.dev/.well-known/jwks.json
CLERK_ISSUER=https://your-clerk-instance.clerk.accounts.dev
```

#### 🔥 Firebase/Firestore (Required for Database)
```bash
GCP_PROJECT_ID=your-firebase-project-id
```

### Development vs Production

| Mode | AUTH_MODE | DATASTORE | Authentication Required |
|------|-----------|-----------|------------------------|
| **Development** | `noop` | `memory` | ❌ No |
| **Production** | `clerk` | `firestore` | ✅ Yes |

## 🏗️ Service Architecture

```
┌─────────────────┐    ┌──────────────────┐
│   Auth Gateway  │────│  Activity Service│
│   (Port 8080)   │    │   (Port 8081)    │
└─────────────────┘    └──────────────────┘
         │                       │
         ├───────────────────────┼───────────────────────┐
         │                       │                       │
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│  User Service  │    │ Session Service │    │  Media Service  │
│  (Port 8082)   │    │  (Port 8083)    │    │  (Port 8084)    │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐    ┌─────────────────┐
                    │Analytics Service│    │Webhook Service │
                    │  (Port 8085)    │    │  (Port 8086)    │
                    └─────────────────┘    └─────────────────┘
                                 │
                    ┌─────────────────┐
                    │Firebase Emulator│
                    │  (Port 8088)    │
                    └─────────────────┘
```

## 🔧 Configuration Options

### Development Configuration
```bash
# .env file for development
AUTH_MODE=noop
DATASTORE=memory
GCP_PROJECT_ID=focusnest-dev
```

### Production Configuration
```bash
# .env file for production
AUTH_MODE=clerk
DATASTORE=firestore
GCP_PROJECT_ID=your-production-project-id
CLERK_JWKS_URL=https://your-clerk-instance.clerk.accounts.dev/.well-known/jwks.json
CLERK_ISSUER=https://your-clerk-instance.clerk.accounts.dev
```

## 📊 API Endpoints

### Main Entry Point
- **Auth Gateway**: http://localhost:8080
- **OpenAPI Documentation**: http://localhost:8080/openapi.yaml

### Service Endpoints
- **Activity Service**: http://localhost:8081
- **User Service**: http://localhost:8082
- **Session Service**: http://localhost:8083
- **Media Service**: http://localhost:8084
- **Analytics Service**: http://localhost:8085
- **Webhook Service**: http://localhost:8086

### Key API Routes
```
# Productivity Tracking
GET    /v1/productivities          # List productivity entries
POST   /v1/productivities          # Create productivity entry
GET    /v1/productivities/{id}     # Get specific entry
DELETE /v1/productivities/{id}     # Delete entry

# Chatbot
GET    /v1/chatbot                 # List chat sessions
POST   /v1/chatbot                # Create chat session
POST   /v1/chatbot/ask            # Ask chatbot
GET    /v1/chatbot/{id}           # Get chat session
DELETE /v1/chatbot/{id}           # Delete chat session

# Analytics
GET    /v1/analytics/progress     # Get progress analytics
GET    /v1/analytics/streak       # Get streak analytics
GET    /v1/analytics/categories   # Get category breakdown

# User Profile
GET    /v1/users/profile          # Get user profile
POST   /v1/users/profile          # Create user profile
PUT    /v1/users/profile          # Update user profile
DELETE /v1/users/profile          # Delete user profile
```

## 🛠️ Management Commands

### Start Services
```bash
# Start all services
docker-compose up -d

# Start specific service
docker-compose up -d activity-service
```

### View Logs
```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f activity-service
```

### Stop Services
```bash
# Stop all services
docker-compose down

# Stop and remove volumes
docker-compose down -v
```

### Rebuild Services
```bash
# Rebuild all services
docker-compose build --no-cache

# Rebuild specific service
docker-compose build --no-cache activity-service
```

## 🧪 Testing

### Health Checks
```bash
# Test all services
./test-services.sh

# Manual health check
curl http://localhost:8080/healthz
```

### API Testing
```bash
# Test productivity endpoint (development mode)
curl http://localhost:8081/v1/productivities

# Test with authentication (production mode)
curl -H "Authorization: Bearer your-jwt-token" http://localhost:8080/v1/productivities
```

## 🔍 Troubleshooting

### Common Issues

1. **Port Conflicts**
   ```bash
   # Check if ports are in use
   netstat -tulpn | grep :8080
   
   # Kill process using port
   sudo kill -9 $(lsof -t -i:8080)
   ```

2. **Docker Build Issues**
   ```bash
   # Clean Docker cache
   docker system prune -f
   
   # Rebuild without cache
   docker-compose build --no-cache
   ```

3. **Environment Variables**
   ```bash
   # Check environment variables
   docker-compose config
   
   # Verify .env file
   cat .env
   ```

### Debug Commands
```bash
# Enter container
docker-compose exec activity-service sh

# View container logs
docker-compose logs activity-service

# Check container status
docker-compose ps
```

## 📝 Environment Variables Reference

| Variable | Description | Required | Default | Example |
|----------|-------------|----------|---------|---------|
| `CLERK_JWKS_URL` | Clerk JWKS endpoint | Production | - | `https://clerk.example.com/.well-known/jwks.json` |
| `CLERK_ISSUER` | Clerk JWT issuer | Production | - | `https://clerk.example.com` |
| `GCP_PROJECT_ID` | Firebase Project ID | Firestore | `focusnest-dev` | `my-firebase-project` |
| `AUTH_MODE` | Authentication mode | No | `noop` | `clerk` or `noop` |
| `DATASTORE` | Data storage backend | No | `memory` | `firestore` or `memory` |

## 🚀 Production Deployment

### Prerequisites
1. **Clerk Account**: Set up authentication
2. **GCP Project**: Configure Firestore database
3. **Domain**: Set up your domain and SSL

### Deployment Steps
1. Configure production environment variables
2. Set up Clerk authentication
3. Configure GCP Firestore
4. Deploy using Docker Compose or Kubernetes
5. Set up monitoring and logging

## 📞 Support

For issues and questions:
- Check the logs: `docker-compose logs -f`
- Test services: `./test-services.sh`
- Review configuration: `cat .env`
