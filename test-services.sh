#!/bin/bash

# FocusNest Services Health Check Script

set -e

echo "🔍 Testing FocusNest Services..."

# Function to test endpoint
test_endpoint() {
    local name=$1
    local url=$2
    local expected_status=${3:-200}
    
    echo -n "Testing $name... "
    
    if response=$(curl -s -o /dev/null -w "%{http_code}" "$url" 2>/dev/null); then
        if [ "$response" = "$expected_status" ]; then
            echo "✅ OK ($response)"
        else
            echo "❌ FAILED (got $response, expected $expected_status)"
        fi
    else
        echo "❌ FAILED (connection error)"
    fi
}

# Test all services
echo "🏥 Health Checks:"
test_endpoint "Auth Gateway" "http://localhost:8080/healthz"
test_endpoint "Activity Service" "http://localhost:8081/healthz"
test_endpoint "User Service" "http://localhost:8082/healthz"
test_endpoint "Session Service" "http://localhost:8083/healthz"
test_endpoint "Media Service" "http://localhost:8084/healthz"
test_endpoint "Analytics Service" "http://localhost:8085/healthz"
test_endpoint "Webhook Service" "http://localhost:8086/healthz"

echo ""
echo "📊 API Endpoints:"
echo "   Auth Gateway:     http://localhost:8080"
echo "   Activity Service: http://localhost:8081"
echo "   OpenAPI Docs:     http://localhost:8081/openapi.yaml"

echo ""
echo "🎯 Test API calls:"
echo "   # Test productivity list (requires auth in production)"
echo "   curl -H 'Authorization: Bearer your-token' http://localhost:8080/v1/productivities"
echo ""
echo "   # Test chatbot ask (requires auth in production)"
echo "   curl -X POST -H 'Content-Type: application/json' -H 'Authorization: Bearer your-token' \\"
echo "        -d '{\"message\":\"Hello\"}' http://localhost:8080/v1/chatbot/ask"

echo ""
echo "✅ Health check complete!"
