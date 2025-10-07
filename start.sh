#!/bin/bash

# FocusNest Docker Startup Script

set -e

echo "🚀 Starting FocusNest Microservices..."

# Check if .env file exists
if [ ! -f .env ]; then
    echo "📝 Creating .env file from template..."
    cp env.example .env
    echo "⚠️  Please edit .env file with your configuration before running again"
    echo "   For development, you can use the default values"
    exit 1
fi

# Load environment variables
source .env

echo "🔧 Configuration:"
echo "   AUTH_MODE: ${AUTH_MODE:-noop}"
echo "   DATASTORE: ${DATASTORE:-memory}"
echo "   GCP_PROJECT_ID: ${GCP_PROJECT_ID:-focusnest-dev}"

# Build and start services
echo "🏗️  Building Docker images..."
docker-compose build

echo "🚀 Starting services..."
docker-compose up -d

echo "⏳ Waiting for services to be ready..."
sleep 10

echo "✅ Services started successfully!"
echo ""
echo "🌐 Available endpoints:"
echo "   Auth Gateway:     http://localhost:8080"
echo "   Activity Service: http://localhost:8081"
echo "   User Service:     http://localhost:8082"
echo "   Session Service:  http://localhost:8083"
echo "   Media Service:    http://localhost:8084"
echo "   Analytics:        http://localhost:8085"
echo "   Webhook Service:  http://localhost:8086"
echo "   Firebase Emulator: http://localhost:8088"
echo ""
echo "📊 View logs: docker-compose logs -f"
echo "🛑 Stop services: docker-compose down"
echo ""
echo "🎉 Happy coding!"
