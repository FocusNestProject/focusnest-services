# 📚 FocusNest API Documentation

Complete API documentation for the FocusNest productivity tracking microservices.

## 🚀 Quick Access

### **Interactive API Documentation**
- **Swagger UI**: http://localhost:8089
- **Start Swagger**: `./start-swagger.sh`
- **Stop Swagger**: `cd swagger-ui && docker-compose down`

### **Service Endpoints**

| Service | Port | Health Check | Main Purpose |
|---------|------|--------------|--------------|
| **Gateway API** | `8080` | `http://localhost:8080/healthz` | Main entry point with authentication |
| **Activity Service** | `8081` | `http://localhost:8081/healthz` | Productivity tracking, chatbot, analytics |
| **User Service** | `8082` | `http://localhost:8082/healthz` | User profile management |
| **Session Service** | `8083` | `http://localhost:8083/healthz` | Session management |
| **Media Service** | `8084` | `http://localhost:8084/healthz` | File/media handling |
| **Analytics Service** | `8085` | `http://localhost:8085/healthz` | Analytics and reporting |
| **Webhook Service** | `8086` | `http://localhost:8086/healthz` | Webhook handling |
| **Firebase Emulator** | `8088` | `http://localhost:8088` | Firestore database |

## 🔐 Authentication

All protected endpoints require JWT authentication via Clerk:

```bash
# Add to your requests
Authorization: Bearer YOUR_JWT_TOKEN
```

## 📊 API Endpoints Overview

### **Productivity Management**
- `GET /v1/productivities` - List productivity entries
- `POST /v1/productivities` - Create productivity entry
- `GET /v1/productivities/{id}` - Get specific entry
- `DELETE /v1/productivities/{id}` - Delete entry

### **Chatbot Integration**
- `GET /v1/chatbot` - List chatbot sessions
- `POST /v1/chatbot` - Create chatbot session
- `POST /v1/chatbot/ask` - Ask chatbot
- `GET /v1/chatbot/{id}` - Get specific session
- `DELETE /v1/chatbot/{id}` - Delete session

### **Analytics & Insights**
- `GET /v1/analytics/progress` - Get progress analytics
- `GET /v1/analytics/streak` - Get streak analytics
- `GET /v1/analytics/categories` - Get category breakdown

### **User Profile**
- `GET /v1/users/profile` - Get user profile
- `POST /v1/users/profile` - Create user profile
- `PUT /v1/users/profile` - Update user profile
- `DELETE /v1/users/profile` - Delete user profile

## 🧪 Testing APIs

### **Health Checks**
```bash
# Test all services
./test-services.sh

# Individual health checks
curl http://localhost:8080/healthz
curl http://localhost:8081/healthz
```

### **API Testing (Development Mode)**
```bash
# Test productivity list (no auth required in development)
curl http://localhost:8081/v1/productivities

# Test chatbot ask (no auth required in development)
curl -X POST -H "Content-Type: application/json" \
     -d '{"message":"Hello"}' \
     http://localhost:8081/v1/chatbot/ask
```

### **API Testing (Production Mode)**
```bash
# Test with authentication
curl -H "Authorization: Bearer YOUR_JWT_TOKEN" \
     http://localhost:8080/v1/productivities

# Test chatbot with authentication
curl -X POST -H "Content-Type: application/json" \
     -H "Authorization: Bearer YOUR_JWT_TOKEN" \
     -d '{"message":"Hello"}' \
     http://localhost:8080/v1/chatbot/ask
```

## 📱 Frontend Integration

### **Configuration**
```javascript
// Your Expo app configuration
const API_CONFIG = {
  baseURL: 'http://localhost:8080',  // Gateway API
  // OR for direct access: 'http://localhost:8081'
  
  headers: {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${token}` // Add when implementing auth
  }
};
```

### **Example API Calls**
```javascript
// List productivity entries
const response = await fetch(`${API_CONFIG.baseURL}/v1/productivities`, {
  headers: API_CONFIG.headers
});

// Create productivity entry
const newEntry = await fetch(`${API_CONFIG.baseURL}/v1/productivities`, {
  method: 'POST',
  headers: API_CONFIG.headers,
  body: JSON.stringify({
    category: 'kerja',
    timeConsumed: 120,
    cycleCount: 4,
    cycleMode: 'pomodoro',
    description: 'Working on project',
    mood: 'focused'
  })
});

// Ask chatbot
const chatbotResponse = await fetch(`${API_CONFIG.baseURL}/v1/chatbot/ask`, {
  method: 'POST',
  headers: API_CONFIG.headers,
  body: JSON.stringify({
    message: 'How can I improve my productivity?'
  })
});
```

## 📋 Data Models

### **Productivity Entry**
```typescript
interface ProductivityEntry {
  id: string;
  userId: string;
  category: 'kerja' | 'belajar' | 'baca_buku' | 'journaling' | 'memasak' | 'olahraga' | 'lainnya';
  timeConsumed: number; // minutes
  cycleCount: number;
  cycleMode: 'pomodoro' | 'quick_focus' | 'free_timer' | 'custom_timer';
  description?: string;
  mood?: 'excited' | 'focused' | 'tired' | 'motivated' | 'stressed' | 'relaxed';
  imageUrl?: string;
  startedAt?: string; // ISO date
  endedAt?: string; // ISO date
  createdAt: string; // ISO date
  updatedAt: string; // ISO date
}
```

### **Chatbot Session**
```typescript
interface ChatbotSession {
  id: string;
  userId: string;
  title: string;
  messages: ChatMessage[];
  createdAt: string;
  updatedAt: string;
}

interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: string;
}
```

### **User Profile**
```typescript
interface UserProfile {
  id: string;
  userId: string;
  bio?: string;
  birthdate?: string; // YYYY-MM-DD
  backgroundImage?: string;
  createdAt: string;
  updatedAt: string;
}
```

## 🔧 Development Tools

### **Postman Collection**
- Import: `postman/FocusNest.postman_collection.json`
- Pre-configured requests for all endpoints

### **OpenAPI Specifications**
- Gateway API: `gateway-api/api/openapi.yaml`
- Activity Service: `activity-service/api/openapi.yaml`
- User Service: `user-service/api/openapi.yaml`
- Session Service: `session-service/api/openapi.yaml`
- Media Service: `media-service/api/openapi.yaml`
- Analytics Service: `analytics-service/api/openapi.yaml`
- Webhook Service: `webhook-service/api/openapi.yaml`

## 🚀 Production Deployment

For production deployment:
1. Update environment variables in `.env`
2. Configure Clerk authentication
3. Set up Firestore database
4. Deploy using Docker or cloud services

See `README_DOCKER.md` and `FIRESTORE_SETUP.md` for detailed instructions.

## 🆘 Troubleshooting

### **Services Not Starting**
```bash
# Check service status
docker-compose ps

# Check logs
docker-compose logs gateway-api
docker-compose logs activity-service
```

### **API Documentation Not Loading**
```bash
# Restart Swagger UI
cd swagger-ui && docker-compose down && docker-compose up -d
```

### **Authentication Issues**
- Ensure Clerk environment variables are set
- Check JWT token validity
- Verify token format: `Bearer YOUR_TOKEN`

## 📞 Support

For issues or questions:
1. Check the interactive Swagger UI: http://localhost:8089
2. Review service logs: `docker-compose logs`
3. Test individual endpoints: `./test-services.sh`
