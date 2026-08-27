#!/bin/bash
# Quick Start Validation Script
# Run this to quickly validate the negotiation → chat integration

set -e

echo "🚀 Negotiation → Chat Integration - Quick Validation"
echo "================================================"
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check prerequisites
echo "📋 Checking prerequisites..."

# Check if psql is available
if ! command -v psql &> /dev/null; then
    echo -e "${RED}❌ psql not found. Please install PostgreSQL client.${NC}"
    exit 1
fi

# Check if curl is available
if ! command -v curl &> /dev/null; then
    echo -e "${RED}❌ curl not found. Please install curl.${NC}"
    exit 1
fi

echo -e "${GREEN}✅ All prerequisites met${NC}"
echo ""

# Load configuration
echo "⚙️  Configuration"
echo "================"
echo ""
echo "Enter database connection details:"
read -p "Database host [localhost]: " DB_HOST
DB_HOST=${DB_HOST:-localhost}
read -p "Database port [5432]: " DB_PORT
DB_PORT=${DB_PORT:-5432}
read -p "Database name [labuda]: " DB_NAME
DB_NAME=${DB_NAME:-labuda}
read -p "Database user: " DB_USER
read -p "API base URL [http://localhost:8080]: " API_URL
API_URL=${API_URL:-http://localhost:8080}

echo ""
echo -e "${GREEN}✅ Configuration loaded${NC}"
echo ""

# Quick health check
echo "🏥 System Health Check"
echo "===================="
echo ""

# Check outbox statistics
echo "Checking outbox table..."
OUTBOX_STATS=$(psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
SELECT
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE status = 'pending') as pending,
    COUNT(*) FILTER (WHERE status = 'processing') as processing,
    COUNT(*) FILTER (WHERE status = 'succeeded') as succeeded,
    COUNT(*) FILTER (WHERE status = 'failed') as failed
FROM outbox_messages;
")

echo "$OUTBOX_STATS"
echo ""

# Check if worker is processing
PROCESSING_COUNT=$(echo "$OUTBOX_STATS" | awk '{print $3}')
if [ "$PROCESSING_COUNT" -eq 0 ]; then
    echo -e "${GREEN}✅ No events stuck in processing${NC}"
else
    echo -e "${YELLOW}⚠️  $PROCESSING_COUNT events in processing state${NC}"
fi

echo ""
echo "================================================"
echo ""

# Menu for test selection
echo "🧪 VALIDATION TESTS"
echo "=================="
echo ""
echo "Select test to run:"
echo "1. TEST 1: Single Flow (1 negotiation → 1 chat room)"
echo "2. TEST 2: Retry Safety (duplicate event handling)"
echo "3. TEST 3: Parallel Safety (concurrent requests)"
echo "4. TEST 4: Outbox Stability (event processing)"
echo "5. Run all tests"
echo "6. Exit"
echo ""

read -p "Enter choice [1-6]: " CHOICE

case $CHOICE in
    1)
        echo ""
        echo "Running TEST 1: Single Flow Validation..."
        echo ""
        echo "📖 Please refer to scripts/manual_validation_guide.md"
        echo "   Section: TEST 1: SINGLE FLOW VALIDATION"
        echo ""
        read -p "Press Enter after completing the test..."

        # Run verification queries
        echo "Running verification queries..."
        psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f scripts/validation_queries.sql
        ;;
    2)
        echo ""
        echo "Running TEST 2: Retry Safety Validation..."
        echo ""
        echo "📖 Please refer to scripts/manual_validation_guide.md"
        echo "   Section: TEST 2: RETRY SAFETY VALIDATION"
        echo ""
        read -p "Press Enter after completing the test..."

        # Run verification queries
        echo "Running verification queries..."
        psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f scripts/validation_queries.sql
        ;;
    3)
        echo ""
        echo "Running TEST 3: Parallel Safety Validation..."
        echo ""
        echo "📖 Please refer to scripts/manual_validation_guide.md"
        echo "   Section: TEST 3: PARALLEL SAFETY VALIDATION"
        echo ""
        read -p "Press Enter after completing the test..."

        # Run verification queries
        echo "Running verification queries..."
        psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f scripts/validation_queries.sql
        ;;
    4)
        echo ""
        echo "Running TEST 4: Outbox Stability Validation..."
        echo ""

        # Run outbox stability queries
        echo "Running outbox stability queries..."
        psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << EOF
-- Outbox stability check
SELECT
    'Overall Statistics' as check_type,
    COUNT(*) as total_events,
    COUNT(*) FILTER (WHERE status = 'pending') as pending,
    COUNT(*) FILTER (WHERE status = 'processing') as processing,
    COUNT(*) FILTER (WHERE status = 'succeeded') as succeeded,
    COUNT(*) FILTER (WHERE status = 'failed') as failed
FROM outbox_messages;
EOF
        ;;
    5)
        echo ""
        echo "Running ALL tests..."
        echo ""
        echo "📖 Please follow scripts/manual_validation_guide.md"
        echo "   Run all tests in sequence"
        echo ""
        read -p "Press Enter after completing all tests..."

        # Run all verification queries
        echo "Running all verification queries..."
        psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f scripts/validation_queries.sql
        ;;
    6)
        echo "Exiting..."
        exit 0
        ;;
    *)
        echo "Invalid choice. Exiting."
        exit 1
        ;;
esac

echo ""
echo "================================================"
echo ""
echo "✅ Validation complete!"
echo ""
echo "📊 FINAL OUTPUT:"
echo "  CHAT_ROOM_DUPLICATE: NO (if TEST 1 passed)"
echo "  MESSAGE_DUPLICATE: NO (if TEST 2 passed)"
echo "  OUTBOX_STABLE: YES (if TEST 4 passed)"
echo "  RACE_SAFE: YES (if TEST 3 passed)"
echo ""
echo "If all tests passed:"
echo "  CHAT INTEGRATION = AMAN UNTUK PRODUCTION ✅"
echo ""
echo "If any test failed:"
echo "  Review the failed test"
echo "  Check logs for errors"
echo "  Fix issues and re-test"
echo ""
echo "================================================"
