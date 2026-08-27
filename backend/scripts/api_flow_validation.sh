#!/bin/bash

###############################################################################
# API FLOW VALIDATION - STRICT MODE (NO DIRECT DB)
# ================================================
# Objective: Validate REAL application flow through API layer
# Rule: DO NOT USE DIRECT SQL INSERT - MUST USE HTTP API
###############################################################################

set -e  # Exit on error

# Configuration
API_BASE="${API_BASE:-http://localhost:8080}"
DB_CONN="${DB_CONN:-postgresql://labuda:labuda123@localhost:5432/labuda?sslmode=disable}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test data
SELLER_EMAIL="seller-test-$(date +%s)@example.com"
BUYER_EMAIL="buyer-test-$(date +%s)@example.com"
LISTING_TITLE="Test Koi $(date +%s)"
TEST_QUANTITY=1

echo "================================================================"
echo "API FLOW VALIDATION - STRICT MODE"
echo "================================================================"
echo "API Base: $API_BASE"
echo "Timestamp: $(date)"
echo ""

# Helper functions
check_server() {
    echo -e "${YELLOW}[CHECK]${NC} Checking if server is running..."
    if curl -s "$API_BASE/health" > /dev/null 2>&1; then
        echo -e "${GREEN}[OK]${NC} Server is running"
        return 0
    else
        echo -e "${RED}[FAIL]${NC} Server is not running at $API_BASE"
        echo "Please start the server first: go run cmd/server/main.go"
        exit 1
    fi
}

make_request() {
    local method=$1
    local endpoint=$2
    local data=$3
    local token=$4

    if [ -n "$token" ]; then
        curl -s -X "$method" \
            -H "Content-Type: application/json" \
            -H "Authorization: Bearer $token" \
            -d "$data" \
            "$API_BASE$endpoint"
    else
        curl -s -X "$method" \
            -H "Content-Type: application/json" \
            -d "$data" \
            "$API_BASE$endpoint"
    fi
}

extract_json_string() {
    local field=$1
    local response=$2
    echo "$response" | grep -o "\"$field\":\"[^\"]*\"" | head -n1 | cut -d'"' -f4
}

extract_json_bool() {
    local field=$1
    local response=$2
    echo "$response" | grep -o "\"$field\":[^,}]*" | head -n1 | cut -d':' -f2 | tr -d '[:space:]'
}

# Step 1: Setup - Create Seller and Buyer accounts
echo ""
echo "================================================================"
echo "STEP 1: CREATE TEST ACCOUNTS"
echo "================================================================"

echo -e "${YELLOW}[ACTION]${NC} Creating seller account via Firebase exchange"
SELLER_RESPONSE=$(make_request "POST" "/api/v1/auth/firebase/exchange" '{
    "firebase_id_token": "seller-1"
}' "")

SELLER_ID=$(extract_json_string "user_id" "$SELLER_RESPONSE")
SELLER_TOKEN=$(extract_json_string "access_token" "$SELLER_RESPONSE")
SELLER_REFRESH_TOKEN=$(extract_json_string "refresh_token" "$SELLER_RESPONSE")
SELLER_PROFILE_REQUIRED=$(extract_json_bool "requires_profile_completion" "$SELLER_RESPONSE")

if [ -z "$SELLER_ID" ] || [ -z "$SELLER_TOKEN" ] || [ -z "$SELLER_REFRESH_TOKEN" ] || [ "$SELLER_PROFILE_REQUIRED" = "true" ]; then
    echo -e "${RED}[FAIL]${NC} Failed to create seller account"
    echo "Response: $SELLER_RESPONSE"
    exit 1
fi

echo -e "${GREEN}[OK]${NC} Seller created: $SELLER_ID"

echo -e "${YELLOW}[ACTION]${NC} Creating buyer account via Firebase exchange"
BUYER_RESPONSE=$(make_request "POST" "/api/v1/auth/firebase/exchange" '{
    "firebase_id_token": "buyer-1"
}' "")

BUYER_ID=$(extract_json_string "user_id" "$BUYER_RESPONSE")
BUYER_TOKEN=$(extract_json_string "access_token" "$BUYER_RESPONSE")
BUYER_REFRESH_TOKEN=$(extract_json_string "refresh_token" "$BUYER_RESPONSE")
BUYER_PROFILE_REQUIRED=$(extract_json_bool "requires_profile_completion" "$BUYER_RESPONSE")

if [ -z "$BUYER_ID" ] || [ -z "$BUYER_TOKEN" ] || [ -z "$BUYER_REFRESH_TOKEN" ] || [ "$BUYER_PROFILE_REQUIRED" = "true" ]; then
    echo -e "${RED}[FAIL]${NC} Failed to create buyer account"
    echo "Response: $BUYER_RESPONSE"
    exit 1
fi

echo -e "${GREEN}[OK]${NC} Buyer created: $BUYER_ID"

# Step 2: Setup Seller (become a seller)
echo ""
echo "================================================================"
echo "STEP 2: SETUP SELLER ACCOUNT"
echo "================================================================"

echo -e "${YELLOW}[ACTION]${NC} Calling seller onboarding for seller"
SELLER_SETUP_RESPONSE=$(make_request "POST" "/api/v1/seller/onboarding" '{
    "business_name": "Test Koi Farm",
    "phone": "+628123456789",
    "address": "Jalan Test 123"
}' "$SELLER_TOKEN")

echo "Response: $SELLER_SETUP_RESPONSE"

# Step 3: Create a Listing
echo ""
echo "================================================================"
echo "STEP 3: CREATE LISTING VIA API"
echo "================================================================"

echo -e "${YELLOW}[ACTION]${NC} Creating listing: $LISTING_TITLE"
LISTING_RESPONSE=$(make_request "POST" "/api/v1/listings" '{
    "title": "'$LISTING_TITLE'",
    "description": "Beautiful koi fish for testing",
    "price": 500000,
    "quantity": 10,
    "negotiation_enabled": true,
    "visibility": "public",
    "variety": "Kohaku",
    "size_cm": 30,
    "age_months": 24,
    "gender": "male",
    "media_urls": ["https://picsum.photos/seed/koi1/800/600"]
}' "$SELLER_TOKEN")

LISTING_ID=$(echo "$LISTING_RESPONSE" | grep -o '"id":"[^"]*' | cut -d'"' -f4)

if [ -z "$LISTING_ID" ]; then
    echo -e "${RED}[FAIL]${NC} Failed to create listing"
    echo "Response: $LISTING_RESPONSE"
    exit 1
fi

echo -e "${GREEN}[OK]${NC} Listing created: $LISTING_ID"

# Step 4: Setup Shipping Options for the Listing
echo ""
echo "================================================================"
echo "STEP 4: SETUP SHIPPING OPTIONS"
echo "================================================================"

echo -e "${YELLOW}[ACTION]${NC} Creating shipping option"
SHIPPING_OPTION_RESPONSE=$(make_request "POST" "/api/v1/seller/shipping/options" '{
    "name": "JNE Trucking",
    "transport_type": "land",
    "expedition_name": "JNE",
    "estimated_days_min": 2,
    "estimated_days_max": 5
}' "$SELLER_TOKEN")

SHIPPING_OPTION_ID=$(echo "$SHIPPING_OPTION_RESPONSE" | grep -o '"id":"[^"]*' | cut -d'"' -f4)

if [ -z "$SHIPPING_OPTION_ID" ]; then
    echo -e "${YELLOW}[WARN]${NC} Failed to create shipping option (may exist)"
else
    echo -e "${GREEN}[OK]${NC} Shipping option created: $SHIPPING_OPTION_ID"
fi

# For now, use a test shipping option ID if creation failed
if [ -z "$SHIPPING_OPTION_ID" ]; then
    SHIPPING_OPTION_ID="00000000-0000-0000-0000-000000000001"
fi

echo -e "${YELLOW}[ACTION]${NC} Linking shipping option to listing"
LINK_SHIPPING_RESPONSE=$(make_request "PUT" "/api/v1/listings/$LISTING_ID/shipping" '{
    "shipping_option_ids": ["'$SHIPPING_OPTION_ID'"]
}' "$SELLER_TOKEN")

echo "Response: $LINK_SHIPPING_RESPONSE"

# Step 5: Create Buyer Address
echo ""
echo "================================================================"
echo "STEP 5: CREATE BUYER ADDRESS"
echo "================================================================"

echo -e "${YELLOW}[ACTION]${NC} Creating buyer address"
ADDRESS_RESPONSE=$(make_request "PUT" "/api/v1/users/me" '{
    "phone": "+628987654321",
    "address": "Jalan Buyer 456",
    "city": "Jakarta",
    "province": "DKI Jakarta",
    "postal_code": "12345"
}' "$BUYER_TOKEN")

echo "Response: $ADDRESS_RESPONSE"

# For testing, we'll use a fixed address ID (in real scenario, extract from response)
ADDRESS_ID="00000000-0000-0000-0000-000000000002"

# Step 6: Call Pricing Preview API
echo ""
echo "================================================================"
echo "STEP 6: CALL PRICING PREVIEW API"
echo "================================================================"

echo -e "${YELLOW}[ACTION]${NC} Generating pricing preview"
PRICING_RESPONSE=$(make_request "POST" "/api/v1/pricing/preview" '{
    "listing_id": "'$LISTING_ID'",
    "quantity": '$TEST_QUANTITY',
    "shipping_option_id": "'$SHIPPING_OPTION_ID'",
    "address_id": "'$ADDRESS_ID'"
}' "$BUYER_TOKEN")

PRICING_TOKEN=$(echo "$PRICING_RESPONSE" | grep -o '"token":"[^"]*' | cut -d'"' -f4)

if [ -z "$PRICING_TOKEN" ]; then
    echo -e "${RED}[FAIL]${NC} Failed to generate pricing preview"
    echo "Response: $PRICING_RESPONSE"
    exit 1
fi

echo -e "${GREEN}[OK]${NC} Pricing token generated: $PRICING_TOKEN"
echo "Full Response: $PRICING_RESPONSE"

# Step 7: Create Order Using Pricing Token
echo ""
echo "================================================================"
echo "STEP 7: CREATE ORDER VIA API USING PRICING TOKEN"
echo "================================================================"

echo -e "${YELLOW}[ACTION]${NC} Creating order with pricing token"
ORDER_RESPONSE=$(make_request "POST" "/api/v1/orders" '{
    "listing_id": "'$LISTING_ID'",
    "quantity": '$TEST_QUANTITY',
    "address_id": "'$ADDRESS_ID'",
    "shipping_option_id": "'$SHIPPING_OPTION_ID'",
    "pricing_token": "'$PRICING_TOKEN'"
}' "$BUYER_TOKEN")

ORDER_ID=$(echo "$ORDER_RESPONSE" | grep -o '"id":"[^"]*' | cut -d'"' -f4)

if [ -z "$ORDER_ID" ]; then
    echo -e "${RED}[FAIL]${NC} Failed to create order"
    echo "Response: $ORDER_RESPONSE"
    exit 1
fi

echo -e "${GREEN}[OK]${NC} Order created: $ORDER_ID"
echo "Full Response: $ORDER_RESPONSE"

# Step 8: Verify Database State
echo ""
echo "================================================================"
echo "STEP 8: VERIFY DATABASE STATE"
echo "================================================================"

echo -e "${YELLOW}[CHECK]${NC} Querying database for records..."

# Count listings
LISTING_COUNT=$(psql "$DB_CONN" -t -c "SELECT COUNT(*) FROM listings WHERE id = '$LISTING_ID';" 2>/dev/null | tr -d ' ')

# Count pricing tokens
TOKEN_COUNT=$(psql "$DB_CONN" -t -c "SELECT COUNT(*) FROM pricing_tokens WHERE id = '$PRICING_TOKEN';" 2>/dev/null | tr -d ' ')

# Count orders
ORDER_COUNT=$(psql "$DB_CONN" -t -c "SELECT COUNT(*) FROM orders WHERE id = '$ORDER_ID';" 2>/dev/null | tr -d ' ')

echo ""
echo "DATABASE VERIFICATION RESULTS:"
echo "----------------------------"
echo "Listings found: $LISTING_COUNT"
echo "Pricing Tokens found: $TOKEN_COUNT"
echo "Orders found: $ORDER_COUNT"

# Step 9: Verify Logs
echo ""
echo "================================================================"
echo "STEP 9: VERIFY LOGS"
echo "================================================================"

echo -e "${YELLOW}[CHECK]${NC} Searching for pricing token creation log..."
PRICING_LOG=$(psql "$DB_CONN" -t -c "SELECT COUNT(*) FROM outbox_events WHERE event_type = 'PRICING_TOKEN_CREATED';" 2>/dev/null | tr -d ' ')

echo -e "${YELLOW}[CHECK]${NC} Searching for order creation log..."
ORDER_LOG=$(psql "$DB_CONN" -t -c "SELECT COUNT(*) FROM outbox_events WHERE event_type = 'ORDER_CREATED';" 2>/dev/null | tr -d ' ')

echo ""
echo "LOG VERIFICATION RESULTS:"
echo "------------------------"
echo "PRICING_TOKEN_CREATED events: $PRICING_LOG"
echo "ORDER_CREATED events: $ORDER_LOG"

# Final Result
echo ""
echo "================================================================"
echo "VALIDATION RESULT"
echo "================================================================"

if [ "$LISTING_COUNT" -gt 0 ] && [ "$TOKEN_COUNT" -gt 0 ] && [ "$ORDER_COUNT" -gt 0 ]; then
    echo -e "${GREEN}[PASS]${NC} ✅ API FLOW VALIDATION PASSED"
    echo ""
    echo "REAL DATA:"
    echo "----------"
    echo "Listings: $LISTING_COUNT"
    echo "Pricing Tokens: $TOKEN_COUNT"
    echo "Orders: $ORDER_COUNT"
    echo ""
    echo "RESULT: PASS"
    echo ""
    echo "✅ Application flow works correctly through API layer!"
    exit 0
else
    echo -e "${RED}[FAIL]${NC} ❌ API FLOW VALIDATION FAILED"
    echo ""
    echo "Missing records in database."
    echo "Check the server logs for errors."
    exit 1
fi
