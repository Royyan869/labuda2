#!/bin/bash

# =============================================================================
# STUCK EVENT RECOVERY TEST SCRIPT
# =============================================================================
#
# This script demonstrates how to manually test the outbox worker's
# self-healing mechanism for stuck events.
#
# SCENARIO:
# 1. Create a fake event in 'processing' status
# 2. Wait for worker to recover it (within configured check interval)
# 3. Verify the event was recovered and processed
#
# =============================================================================

set -e

echo "============================================================================"
echo "STUCK EVENT RECOVERY TEST"
echo "============================================================================"
echo ""

# Configuration
DB_NAME="labuda"
DB_USER="postgres"
TIMEOUT_MINUTES=5

echo "Step 1: Creating fake stuck event in 'processing' status..."
echo ""

SQL_CREATE_STUCK_EVENT=$(cat <<EOF
-- Create a fake event that appears to be stuck
INSERT INTO outbox (
    id,
    aggregate_type,
    aggregate_id,
    event_type,
    payload,
    status,
    retry_count,
    next_attempt_at,
    idempotency_key,
    created_at,
    updated_at
) VALUES (
    gen_random_uuid(),
    'test',
    gen_random_uuid(),
    'test.stuck_event',
    '{"test": "payload"}'::jsonb,
    'processing',
    0,
    NOW() - INTERVAL '10 minutes',
    'test.stuck_event.manual_test',
    NOW() - INTERVAL '10 minutes',
    NOW() - INTERVAL '10 minutes'
);
EOF
)

echo "$SQL_CREATE_STUCK_EVENT"
echo ""

psql -U "$DB_USER" -d "$DB_NAME" <<EOF
$SQL_CREATE_STUCK_EVENT
EOF

echo "✓ Stuck event created"
echo ""

echo "Step 2: Checking for stuck events..."
echo ""

SQL_CHECK_STUCK=$(cat <<EOF
SELECT
    id,
    event_type,
    status,
    retry_count,
    updated_at,
    NOW() - updated_at as time_since_update
FROM outbox
WHERE status = 'processing'
  AND updated_at < NOW() - INTERVAL '5 minutes'
ORDER BY updated_at DESC
LIMIT 10;
EOF
)

echo "$SQL_CHECK_STUCK"
echo ""

psql -U "$DB_USER" -d "$DB_NAME" <<EOF
$SQL_CHECK_STUCK
EOF

echo ""
echo "✓ Stuck events displayed above"
echo ""

echo "Step 3: Waiting for worker recovery..."
echo ""
echo "The outbox worker should automatically recover this event"
echo "within the configured check interval (default: 1 minute)."
echo ""
echo "Waiting 90 seconds for recovery..."
echo ""

sleep 90

echo "Step 4: Verifying event recovery..."
echo ""

SQL_VERIFY_RECOVERY=$(cat <<EOF
-- Check if event was recovered (status should be 'pending' or 'succeeded')
SELECT
    id,
    event_type,
    status,
    retry_count,
    updated_at,
    next_attempt_at
FROM outbox
WHERE idempotency_key = 'test.stuck_event.manual_test'
ORDER BY updated_at DESC
LIMIT 1;
EOF
)

echo "$SQL_VERIFY_RECOVERY"
echo ""

psql -U "$DB_USER" -d "$DB_NAME" <<EOF
$SQL_VERIFY_RECOVERY
EOF

echo ""
echo "✓ Recovery check complete"
echo ""

echo "============================================================================"
echo "TEST COMPLETE"
echo "============================================================================"
echo ""
echo "If the event status is 'pending' or 'succeeded', the recovery worked!"
echo "If the event status is still 'processing', check:"
echo "  1. Is the outbox worker running?"
echo "  2. Is the stuck_event_check_interval configured correctly?"
echo "  3. Is the processing_timeout longer than the event's stuck duration?"
echo ""
