#!/bin/bash
# CacheStorm HTTP Admin API Examples
# The HTTP API runs on port 9090 by default

BASE_URL="http://localhost:9090"

echo "=== CacheStorm HTTP Admin API Examples ==="

echo ""
echo "1. Health Check"
echo "==============="
curl -s "$BASE_URL/health" | jq .

echo ""
echo "2. Server Info"
echo "=============="
curl -s "$BASE_URL/info"

echo ""
echo "3. Memory Statistics"
echo "===================="
curl -s "$BASE_URL/memory" | jq .

echo ""
echo "4. List All Keys"
echo "================"
curl -s "$BASE_URL/keys"

echo ""
echo "5. List All Tags"
echo "================"
curl -s "$BASE_URL/tags"

echo ""
echo "6. Server Statistics"
echo "===================="
curl -s "$BASE_URL/stats" | jq .

echo ""
echo "7. Current Configuration"
echo "========================"
curl -s "$BASE_URL/config"

echo ""
echo "8. Prometheus Metrics"
echo "====================="
curl -s "$BASE_URL/metrics"

echo ""
echo "9. Using with jq for pretty output"
echo "=================================="
# Assuming you have jq installed
curl -s "$BASE_URL/stats" | jq '.'

echo ""
echo "10. Check specific key via HTTP"
echo "==============================="
# First set a key via redis-cli
redis-cli -p 6380 SET http_test "Hello from HTTP"
# Then check via HTTP (if key endpoint exists)
curl -s "$BASE_URL/keys" | grep http_test

echo ""
echo "=== HTTP API Response Codes ==="
echo "200 - Success"
echo "404 - Not Found"
echo "500 - Internal Server Error"
echo "503 - Service Unavailable"
