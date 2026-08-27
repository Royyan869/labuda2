#!/bin/bash

# ============================================================================
# System Integration Validation - Master Script
# ============================================================================
# This script runs all validation flows and generates a final report
#
# Usage: ./run_all_flows.sh
# ============================================================================

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Database configuration
DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_USER=${DB_USER:-labuda}
DB_NAME=${DB_NAME:-labuda}

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")

# Create output directory
mkdir -p "$OUTPUT_DIR"

echo -e "${BLUE}============================================================================${NC}"
echo -e "${BLUE}SYSTEM INTEGRATION VALIDATION - MASTER SCRIPT${NC}"
echo -e "${BLUE}============================================================================${NC}"
echo ""
echo "Database: $DB_USER@$DB_HOST:$DB_PORT/$DB_NAME"
echo "Output: $OUTPUT_DIR/validation_$TIMESTAMP.log"
echo ""

# Test database connection
echo -e "${YELLOW}Testing database connection...${NC}"
if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT 1;" > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Database connection successful${NC}"
else
    echo -e "${RED}✗ Database connection failed${NC}"
    echo "Please check your database configuration and try again."
    exit 1
fi
echo ""

# Array of flows
FLOWS=(
    "flow1_negotiation_chat_order_notification.sql"
    "flow2_moderation_content_effect.sql"
    "flow3_moderation_listing_order_safety.sql"
    "flow4_retry_idempotency_validation.sql"
    "flow5_outbox_health_check.sql"
)

FLOW_NAMES=(
    "Negotiation → Chat → Order → Notification"
    "Moderation → Content Effect"
    "Moderation → Listing → Order Safety"
    "Retry & Idempotency Validation"
    "Outbox Health Check"
)

# Results array
declare -a FLOW_RESULTS

# Run each flow
for i in "${!FLOWS[@]}"; do
    FLOW_FILE="${FLOWS[$i]}"
    FLOW_NAME="${FLOW_NAMES[$i]}"

    echo -e "${BLUE}============================================================================${NC}"
    echo -e "${BLUE}RUNNING: ${FLOW_NAME}${NC}"
    echo -e "${BLUE}============================================================================${NC}"
    echo ""

    FLOW_OUTPUT="$OUTPUT_DIR/flow$(($i+1))_${TIMESTAMP}.log"

    if psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -f "$SCRIPT_DIR/$FLOW_FILE" > "$FLOW_OUTPUT" 2>&1; then
        FLOW_RESULTS[$i]="PASS"
        echo -e "${GREEN}✓ ${FLOW_NAME}: PASSED${NC}"
    else
        FLOW_RESULTS[$i]="FAIL"
        echo -e "${RED}✗ ${FLOW_NAME}: FAILED${NC}"
        echo "  Check $FLOW_OUTPUT for details"
    fi
    echo ""
done

# Generate final report
echo -e "${BLUE}============================================================================${NC}"
echo -e "${BLUE}FINAL VALIDATION REPORT${NC}"
echo -e "${BLUE}============================================================================${NC}"
echo ""

echo "Summary:"
echo ""

for i in "${!FLOW_NAMES[@]}"; do
    FLOW_NAME="${FLOW_NAMES[$i]}"
    RESULT="${FLOW_RESULTS[$i]}"

    if [ "$RESULT" = "PASS" ]; then
        echo -e "  ${GREEN}✓${NC} ${FLOW_NAME}: ${GREEN}PASSED${NC}"
    else
        echo -e "  ${RED}✗${NC} ${FLOW_NAME}: ${RED}FAILED${NC}"
    fi
done
echo ""

# Calculate overall result
ALL_PASSED=true
for result in "${FLOW_RESULTS[@]}"; do
    if [ "$result" != "PASS" ]; then
        ALL_PASSED=false
        break
    fi
done

echo -e "${BLUE}============================================================================${NC}"
if [ "$ALL_PASSED" = true ]; then
    echo -e "${GREEN}SYSTEM INTEGRATED: YES${NC}"
    echo -e "${GREEN}ALL FLOWS VALIDATED SUCCESSFULLY${NC}"
else
    echo -e "${RED}SYSTEM INTEGRATED: NO${NC}"
    echo -e "${RED}SOME FLOWS FAILED VALIDATION${NC}"
fi
echo -e "${BLUE}============================================================================${NC}"
echo ""

# Save final report
FINAL_REPORT="$OUTPUT_DIR/validation_report_$TIMESTAMP.txt"
{
    echo "SYSTEM INTEGRATION VALIDATION REPORT"
    echo "===================================="
    echo "Date: $(date)"
    echo "Database: $DB_USER@$DB_HOST:$DB_PORT/$DB_NAME"
    echo ""
    echo "Results:"
    echo ""
    for i in "${!FLOW_NAMES[@]}"; do
        FLOW_NAME="${FLOW_NAMES[$i]}"
        RESULT="${FLOW_RESULTS[$i]}"
        echo "  $FLOW_NAME: $RESULT"
    done
    echo ""
    echo "Overall: $(if [ "$ALL_PASSED" = true ]; then echo "PASS"; else echo "FAIL"; fi)"
} > "$FINAL_REPORT"

echo "Detailed logs saved to:"
echo "  $OUTPUT_DIR/"
echo ""
echo "Full report saved to:"
echo "  $FINAL_REPORT"
echo ""

# Exit with appropriate code
if [ "$ALL_PASSED" = true ]; then
    exit 0
else
    exit 1
fi
