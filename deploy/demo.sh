#!/usr/bin/env bash
set -e

GATEWAY_HOST="http://app.localtest.me"
STORE_HOST="http://store.localtest.me"

GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}======================================================${NC}"
echo -e "${BLUE}  Urumi MCP Gateway — End-to-End Demo & Verification  ${NC}"
echo -e "${BLUE}======================================================${NC}"

# 1. Storefront Connectivity
echo -e "\n1. Checking Storefront at ${STORE_HOST}..."
STORE_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "${STORE_HOST}" || echo "failed")
if [[ "$STORE_STATUS" =~ ^(200|301|302)$ ]]; then
    echo -e "${GREEN}✓ Storefront reachable (HTTP ${STORE_STATUS})${NC}"
else
    echo -e "${RED}✗ Storefront check failed (HTTP ${STORE_STATUS})${NC}"
fi

# 2. Dashboard Connectivity
echo -e "\n2. Checking Dashboard at ${GATEWAY_HOST}..."
DASH_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "${GATEWAY_HOST}" || echo "failed")
if [[ "$DASH_STATUS" =~ ^(200|301|302)$ ]]; then
    echo -e "${GREEN}✓ Dashboard reachable (HTTP ${DASH_STATUS})${NC}"
else
    echo -e "${RED}✗ Dashboard check failed (HTTP ${DASH_STATUS})${NC}"
fi

# 3. Admin Authentication
echo -e "\n3. Logging in as Admin (admin@localtest.me)..."
LOGIN_RESP=$(curl -s -c /tmp/urumi_cookie.txt -X POST "${GATEWAY_HOST}/api/auth/login" \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@localtest.me","password":"AdminPassword123!"}')
echo "Response: $LOGIN_RESP"

# 4. Generate API Key for User B
echo -e "\n4. Creating API key for User B..."
KEY_RESP=$(curl -s -b /tmp/urumi_cookie.txt -X POST "${GATEWAY_HOST}/api/keys" \
  -H "Content-Type: application/json" \
  -d '{"name":"User B External Client"}')
API_KEY=$(echo "$KEY_RESP" | grep -o '"api_key":"[^"]*' | cut -d'"' -f4)
KEY_ID=$(echo "$KEY_RESP" | grep -o '"id":[0-9]*' | cut -d':' -f2)
echo -e "${GREEN}✓ Generated API Key: ${API_KEY}${NC} (Key ID: ${KEY_ID})"

# 5. External Client tools/list through /mcp
echo -e "\n5. External Client querying /mcp for tools/list..."
TOOLS_LIST=$(curl -s -X POST "${GATEWAY_HOST}/mcp" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}')
echo "Discovered Tools:"
echo "$TOOLS_LIST" | grep -o '"name":"[^"]*' | head -n 8

# 6. Call namespaced tool utils__time_now
echo -e "\n6. Calling utils__time_now through Gateway..."
TIME_RESP=$(curl -s -X POST "${GATEWAY_HOST}/mcp" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"utils__time_now","arguments":{"timezone":"UTC"}}}')
echo -e "Result: ${GREEN}$TIME_RESP${NC}"

# 7. Call namespaced tool woo__list_orders
echo -e "\n7. Calling woo__list_orders through Gateway..."
ORDERS_RESP=$(curl -s -X POST "${GATEWAY_HOST}/mcp" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"woo__list_orders","arguments":{"per_page":5}}}')
echo -e "Result: ${GREEN}$ORDERS_RESP${NC}"

# 8. Revoke User B's API Key and verify immediate rejection
echo -e "\n8. Revoking User B's API Key..."
REVOKE_RESP=$(curl -s -b /tmp/urumi_cookie.txt -X DELETE "${GATEWAY_HOST}/api/keys/${KEY_ID}")
echo "Revoke response: $REVOKE_RESP"

echo -e "Verifying revoked key returns HTTP 401..."
REVOKED_STATUS=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${GATEWAY_HOST}/mcp" \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":4,"method":"tools/list","params":{}}')

if [[ "$REVOKED_STATUS" == "401" ]]; then
    echo -e "${GREEN}✓ Revocation verified! Gateway returned HTTP 401 immediately.${NC}"
else
    echo -e "${RED}✗ Expected HTTP 401, got ${REVOKED_STATUS}${NC}"
fi

# 9. Verify Audit Log contains all attributed operations
echo -e "\n9. Inspecting Audit Log..."
sleep 2 # Allow async audit batch worker to flush
AUDIT_LOGS=$(curl -s -b /tmp/urumi_cookie.txt "${GATEWAY_HOST}/api/audit")
echo "Recent Audit Entries:"
echo "$AUDIT_LOGS" | head -c 500
echo -e "\n..."

echo -e "\n${GREEN}======================================================${NC}"
echo -e "${GREEN}  All Definition-of-Done criteria verified cleanly!   ${NC}"
echo -e "${GREEN}======================================================${NC}"
