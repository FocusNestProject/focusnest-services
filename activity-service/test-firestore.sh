#!/bin/bash

# Test Activity Service with Firestore Connection
# This tests that the service account can write to and read from Firestore

BASE_URL="http://localhost:8080"
USER_TOKEN="test-firestore-user"

echo "🧪 Testing Activity Service with Firestore"
echo "==========================================="
echo ""

echo "1️⃣  Health Check"
curl -s "$BASE_URL/healthz" | python3 -m json.tool || cat
echo -e "\n"

echo "2️⃣  Create Entry (will be saved to Firestore)"
RESPONSE=$(curl -s -X POST "$BASE_URL/v1/productivities" \
  -H "Authorization: Bearer $USER_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "category": "Testing",
    "timeConsumedMinutes": 45,
    "cycleMode": "Deep Work",
    "description": "Testing Firestore connection with service account",
    "mood": "excited"
  }')

echo "$RESPONSE" | python3 -m json.tool || echo "$RESPONSE"
ENTRY_ID=$(echo "$RESPONSE" | python3 -c "import sys, json; print(json.load(sys.stdin).get('id', ''))" 2>/dev/null || echo "")
echo "Created ID: $ENTRY_ID"
echo -e "\n"

echo "3️⃣  List Entries (from Firestore)"
curl -s "$BASE_URL/v1/productivities" \
  -H "Authorization: Bearer $USER_TOKEN" | python3 -m json.tool || cat
echo -e "\n"

if [ -n "$ENTRY_ID" ]; then
  echo "4️⃣  Get Single Entry (from Firestore)"
  curl -s "$BASE_URL/v1/productivities/$ENTRY_ID" \
    -H "Authorization: Bearer $USER_TOKEN" | python3 -m json.tool || cat
  echo -e "\n"
  
  echo "5️⃣  Delete Entry"
  curl -s -X DELETE "$BASE_URL/v1/productivities/$ENTRY_ID" \
    -H "Authorization: Bearer $USER_TOKEN" \
    -w "\nHTTP Status: %{http_code}\n"
  echo -e "\n"
fi

echo "✅ Test Complete!"
echo ""
echo "📊 Check Firestore Console:"
echo "https://console.firebase.google.com/project/focusnest-470308/firestore/data"
echo ""
echo "Look for: users/$USER_TOKEN/productivities"
