#!/usr/bin/env bash
set -euo pipefail

API=${API:-http://localhost:8080}
PRODUCT=${PRODUCT:-http://localhost:8081}
EMAIL="demo-$(date +%s)@example.com"
PASSWORD="StrongPass123"

echo "== register"
CUSTOMER_ID=$(curl -s -X POST "$API/api/v1/auth/register" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}" | jq -r .id)

echo "== login"
TOKEN=$(curl -s -X POST "$API/api/v1/auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASSWORD\"}" | jq -r .accessToken)

echo "== create order"
ORDER=$(curl -s -X POST "$API/api/v1/orders" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"customerId\":$CUSTOMER_ID,\"productId\":501,\"quantity\":2}")
echo "$ORDER" | jq .
ORDER_ID=$(echo "$ORDER" | jq -r .id)

echo "== get order"
curl -s "$API/api/v1/orders/$ORDER_ID" -H "Authorization: Bearer $TOKEN" | jq .

echo "== list orders"
curl -s "$API/api/v1/orders?page=1&pageSize=5" -H "Authorization: Bearer $TOKEN" | jq .

echo "== graphql"
curl -s -X POST "$API/graphql" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"query":"{ orders { total items { id status totalPrice } } }"}' | jq .

echo "== validation error"
curl -s -X POST "$API/api/v1/orders" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"customerId\":$CUSTOMER_ID,\"productId\":501,\"quantity\":0}" | jq .

echo "== forbidden: customer cannot confirm"
curl -s -X PUT "$API/api/v1/orders/$ORDER_ID/status" -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"status":"CONFIRMED"}' | jq .

if [[ -n "${ADMIN_PASSWORD:-}" ]]; then
  echo "== admin confirms"
  ADMIN_TOKEN=$(curl -s -X POST "$API/api/v1/auth/login" -H 'Content-Type: application/json' \
    -d "{\"email\":\"${ADMIN_EMAIL:-admin@example.com}\",\"password\":\"$ADMIN_PASSWORD\"}" | jq -r .accessToken)
  curl -s -X PUT "$API/api/v1/orders/$ORDER_ID/status" -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
    -d '{"status":"CONFIRMED"}' | jq .
fi

if [[ -n "${PRODUCT_SERVICE_TOKEN:-}" ]]; then
  echo "== resilience: product service outage"
  curl -s -X POST "$PRODUCT/chaos/on" -H "X-Internal-Token: $PRODUCT_SERVICE_TOKEN" | jq .
  for i in 1 2 3 4; do
    curl -s -o /dev/null -w "attempt $i -> HTTP %{http_code}\n" -X POST "$API/api/v1/orders" \
      -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
      -d "{\"customerId\":$CUSTOMER_ID,\"productId\":501,\"quantity\":1}"
  done
  curl -s -X POST "$PRODUCT/chaos/off" -H "X-Internal-Token: $PRODUCT_SERVICE_TOKEN" | jq .
fi

echo "== metrics"
curl -s "${METRICS:-http://localhost:9090}/metrics" | grep -E '^(http_requests_total|http_errors_total|db_errors_total|product_service_calls_total)' | head -20
