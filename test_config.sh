#!/bin/bash

# Test script for Genspark2API Configuration Management
# This script tests the new configuration management endpoints

BASE_URL="http://localhost:7055"
ADMIN_KEY="admin123"  # You can set this via ADMIN_KEY environment variable

echo "🧪 Testing Genspark2API Configuration Management"
echo "=============================================="

# Function to make HTTP requests
make_request() {
    local method=$1
    local endpoint=$2
    local data=$3
    local expected_status=$4
    
    echo "Testing: $method $endpoint"
    
    if [ "$method" = "GET" ]; then
        response=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL$endpoint" \
            -H "X-Admin-Key: $ADMIN_KEY" \
            -H "Content-Type: application/json")
    else
        response=$(curl -s -w "\n%{http_code}" -X $method "$BASE_URL$endpoint" \
            -H "X-Admin-Key: $ADMIN_KEY" \
            -H "Content-Type: application/json" \
            -d "$data")
    fi
    
    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')
    
    if [ "$http_code" = "$expected_status" ]; then
        echo "✅ Success: HTTP $http_code"
        echo "Response: $body"
    else
        echo "❌ Failed: Expected $expected_status, got $http_code"
        echo "Response: $body"
    fi
    echo "---"
}

# Test 1: Get current configuration
echo "Test 1: Get Current Configuration"
make_request "GET" "/admin/config" "" "200"

# Test 2: Update a configuration value
echo "Test 2: Update Configuration"
update_data='{
    "key": "RateLimitRPS",
    "value": 120,
    "description": "Increased rate limit for testing"
}'
make_request "PUT" "/admin/config" "$update_data" "200"

# Test 3: Get configuration history
echo "Test 3: Get Configuration History"
make_request "GET" "/admin/config/history?limit=10" "" "200"

# Test 4: Reset configuration to defaults
echo "Test 4: Reset Configuration"
reset_data='{
    "description": "Reset to defaults for testing"
}'
make_request "POST" "/admin/config/reset" "$reset_data" "200"

# Test 5: Test without admin key (should fail)
echo "Test 5: Test without Admin Key (should fail)"
echo "Testing: GET /admin/config without admin key"
response=$(curl -s -w "\n%{http_code}" -X GET "$BASE_URL/admin/config" \
    -H "Content-Type: application/json")
http_code=$(echo "$response" | tail -n1)
body=$(echo "$response" | sed '$d')

if [ "$http_code" = "401" ]; then
    echo "✅ Success: Correctly rejected without admin key (HTTP 401)"
    echo "Response: $body"
else
    echo "❌ Failed: Expected 401, got $http_code"
    echo "Response: $body"
fi
echo "---"

echo "🎉 Configuration Management Tests Completed!"
echo ""
echo "📋 Summary:"
echo "- Configuration endpoints are now available at /admin/*"
echo "- Admin authentication via X-Admin-Key header"
echo "- Environment variable ADMIN_KEY can be set for admin access"
echo ""
echo "🔧 Configuration Endpoints:"
echo "  GET    /admin/config         - Get current configuration"
echo "  PUT    /admin/config         - Update configuration"
echo "  GET    /admin/config/history - Get configuration history"
echo "  POST   /admin/config/reset   - Reset to defaults"