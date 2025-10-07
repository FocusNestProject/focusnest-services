#!/bin/bash

# FocusNest Swagger UI Startup Script

set -e

echo "🚀 Starting FocusNest API Documentation..."

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker first."
    exit 1
fi

# Navigate to swagger-ui directory
cd swagger-ui

# Start Swagger UI
echo "📚 Starting Swagger UI..."
docker-compose up -d

# Wait for service to be ready
echo "⏳ Waiting for Swagger UI to be ready..."
sleep 5

# Check if service is running
if curl -s http://localhost:8089 > /dev/null; then
    echo "✅ Swagger UI is running!"
    echo ""
    echo "🌐 API Documentation:"
    echo "   📖 Swagger UI: http://localhost:8089"
    echo ""
    echo "🔗 Service Endpoints:"
    echo "   🚪 Gateway API:     http://localhost:8080"
    echo "   📊 Activity API:    http://localhost:8081"
    echo "   👤 User API:       http://localhost:8082"
    echo "   🔐 Session API:    http://localhost:8083"
    echo "   📁 Media API:      http://localhost:8084"
    echo "   📈 Analytics API:  http://localhost:8085"
    echo "   🔗 Webhook API:    http://localhost:8086"
    echo ""
    echo "🧪 Test the APIs:"
    echo "   curl http://localhost:8080/healthz"
    echo "   curl http://localhost:8081/healthz"
    echo ""
    echo "📱 For your frontend:"
    echo "   API_BASE_URL = 'http://localhost:8080'"
    echo ""
    echo "🛑 To stop Swagger UI:"
    echo "   cd swagger-ui && docker-compose down"
else
    echo "❌ Failed to start Swagger UI"
    echo "Check the logs: docker-compose logs"
    exit 1
fi
