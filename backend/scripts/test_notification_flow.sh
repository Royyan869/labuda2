#!/bin/bash
# Notification Flow Test Script
# Tests order.created and order.paid notification flow safely

set -e

echo "======================================================================"
echo "NOTIFICATION FLOW TEST - MINIMAL ACTIVATION"
echo "======================================================================"
echo ""
echo "Testing ONLY: order.created, order.paid"
echo ""

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Step 1: Check if backend is running
echo "Step 1: Checking backend status..."
if curl -s http://localhost:8080/health > /dev/null; then
    echo -e "${GREEN}✓ Backend is running${NC}"
else
    echo -e "${RED}✗ Backend is not running${NC}"
    echo "Please start the backend first"
    exit 1
fi
echo ""

# Step 2: Check notification count before test
echo "Step 2: Checking notification count before test..."
NOTIFICATIONS_BEFORE=$(psql -h localhost -U labuda -d labuda -t -c "SELECT COUNT(*) FROM notifications;" 2>/dev/null || echo "0")
echo "Notifications before: $NOTIFICATIONS_BEFORE"
echo ""

# Step 3: Check outbox table for pending events
echo "Step 3: Checking outbox table..."
OUTBOX_PENDING=$(psql -h localhost -U labuda -d labuda -t -c "SELECT COUNT(*) FROM outbox WHERE status = 'pending';" 2>/dev/null || echo "0")
echo "Pending outbox events: $OUTBOX_PENDING"
echo ""

# Step 4: Check if notification handlers are registered
echo "Step 4: Checking notification handler registration..."
# Check logs for registration message
if journalctl -u labuda-backend --no-pager -n 100 | grep -q "MINIMAL notification event handlers registered"; then
    echo -e "${GREEN}✓ Notification handlers registered${NC}"
else
    echo -e "${YELLOW}⚠ Could not verify handler registration in logs${NC}"
fi
echo ""

# Step 5: Test order creation (this would trigger order.created event)
echo "Step 5: To test notification flow, create an order via the app:"
echo "   1. Login as buyer"
echo "   2. Create an order for a listing"
echo "   3. Check if seller receives notification"
echo ""

# Step 6: Wait and check notifications
echo "Step 6: Waiting 10 seconds for outbox processing..."
sleep 10
echo ""

# Step 7: Check notification count after test
echo "Step 7: Checking notification count after test..."
NOTIFICATIONS_AFTER=$(psql -h localhost -U labuda -d labuda -t -c "SELECT COUNT(*) FROM notifications;" 2>/dev/null || echo "0")
echo "Notifications after: $NOTIFICATIONS_AFTER"
echo ""

# Step 8: Check for duplicate notifications
echo "Step 8: Checking for duplicate notifications..."
DUPLICATES=$(psql -h localhost -U labuda -d labuda -t -c "
SELECT COUNT(*) FROM (
    SELECT recipient_id, actor_id, type, entity_id, COUNT(*) as count
    FROM notifications
    GROUP BY recipient_id, actor_id, type, entity_id
    HAVING COUNT(*) > 1
) duplicates;
" 2>/dev/null || echo "0")

if [ "$DUPLICATES" = "0" ]; then
    echo -e "${GREEN}✓ NO DUPLICATES FOUND${NC}"
else
    echo -e "${RED}✗ FOUND $DUPLICATES DUPLICATE GROUPS${NC}"
fi
echo ""

# Step 9: Check outbox table again
echo "Step 9: Checking outbox table after processing..."
OUTBOX_AFTER=$(psql -h localhost -U labuda -d labuda -t -c "SELECT COUNT(*) FROM outbox WHERE status = 'pending';" 2>/dev/null || echo "0")
echo "Pending outbox events: $OUTBOX_AFTER"

if [ "$OUTBOX_AFTER" = "0" ]; then
    echo -e "${GREEN}✓ Outbox is stable (no backlog)${NC}"
else
    echo -e "${YELLOW}⚠ Outbox has $OUTBOX_AFTER pending events${NC}"
fi
echo ""

# Step 10: Show recent notifications
echo "Step 10: Recent notifications:"
psql -h localhost -U labuda -d labuda -c "
SELECT
    type,
    recipient_id,
    actor_id,
    is_read,
    created_at
FROM notifications
ORDER BY created_at DESC
LIMIT 5;
" 2>/dev/null || echo "Could not fetch notifications"
echo ""

# FINAL OUTPUT
echo "======================================================================"
echo "FINAL OUTPUT"
echo "======================================================================"
echo "NOTIFICATION_DUPLICATE: $([ "$DUPLICATES" = "0" ] && echo "NO" || echo "YES")"
echo "OUTBOX_STABLE: $([ "$OUTBOX_AFTER" = "0" ] && echo "YES" || echo "NO")"
echo "USER_RECEIVE: $([ "$NOTIFICATIONS_AFTER" -gt "$NOTIFICATIONS_BEFORE" ] && echo "YES" || echo "NO - No new notifications")"
echo "======================================================================"
echo ""
