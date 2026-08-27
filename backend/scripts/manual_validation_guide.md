# 🔍 Negotiation → Chat Integration - Manual Validation Guide

## ❗ PURPOSE

Validate that the negotiation → chat event integration is safe for production by testing idempotency, race conditions, and outbox stability.

---

## 🧪 TEST 1: SINGLE FLOW VALIDATION

### Objective
Verify: 1 negotiation → 1 chat room, 1 proposal → 1 message

### Steps

#### 1.1 Create Test Data
```sql
-- Create test users
INSERT INTO users (id, username, email) VALUES
('test-buyer-001', 'test_buyer_001', 'buyer001@test.com'),
('test-seller-001', 'test_seller_001', 'seller001@test.com')
ON CONFLICT (id) DO NOTHING;
```

#### 1.2 Create Negotiation Session
```bash
curl -X POST http://localhost:8080/api/v1/negotiations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "listing_id": "test-listing-001",
    "buyer_id": "test-buyer-001",
    "seller_id": "test-seller-001",
    "initial_price": 100000,
    "note": "Test negotiation for validation"
  }'
```

#### 1.3 Wait for Event Processing
```bash
# Wait 5 seconds for outbox worker to process events
sleep 5
```

#### 1.4 Verify Chat Room Created
```sql
-- Check for negotiation chat room
SELECT
    id,
    room_type,
    created_at
FROM chat_rooms
WHERE room_type = 'negotiation'
  AND participant_a = 'test-buyer-001'
  AND participant_b = 'test-seller-001'
ORDER BY created_at DESC
LIMIT 1;
```

**Expected Result:** Exactly 1 row

#### 1.5 Verify Initial Proposal Message
```sql
-- Check for initial proposal message
SELECT
    id,
    room_id,
    sender_id,
    message_type,
    body,
    attachment->>'type' as attachment_type,
    attachment->>'session_id' as session_id,
    created_at
FROM chat_messages
WHERE room_id = (
    SELECT id FROM chat_rooms
    WHERE room_type = 'negotation'
      AND participant_a = 'test-buyer-001'
      AND participant_b = 'test-seller-001'
    ORDER BY created_at DESC LIMIT 1
)
ORDER BY created_at ASC
LIMIT 1;
```

**Expected Result:** Exactly 1 row with:
- `message_type = 'negotiation_proposal'`
- `attachment->>'type' = 'negotiation_proposal'`
- `attachment->>'session_id'` matches your negotiation session

### ✅ PASS Criteria
- [ ] Exactly 1 chat room created
- [ ] Exactly 1 initial proposal message
- [ ] Room type is 'negotiation'
- [ ] Message type is 'negotiation_proposal'

---

## 🧪 TEST 2: RETRY SAFETY VALIDATION

### Objective
Verify: Duplicate events don't create duplicates

### Steps

#### 2.1 Get Existing Data
```sql
-- Count current rooms and messages
SELECT
    (SELECT COUNT(*) FROM chat_rooms WHERE room_type = 'negotiation') as room_count,
    (SELECT COUNT(*) FROM chat_messages WHERE message_type = 'negotiation_proposal') as message_count;
```

#### 2.2 Insert Duplicate Event
```sql
-- Manually insert duplicate negotiation.started event
INSERT INTO outbox_messages (
    aggregate_type,
    aggregate_id,
    event_type,
    payload,
    status,
    retry_count,
    next_attempt_at,
    created_at
) VALUES (
    'negotiation',
    'test-session-001', -- Use your actual session ID
    'negotiation.started',
    '{
        "session_id": "test-session-001",
        "resource_type": "listing",
        "resource_id": "test-listing-001",
        "buyer_id": "test-buyer-001",
        "seller_id": "test-seller-001",
        "initial_price": 100000,
        "note": "Test negotiation for validation"
    }',
    'pending',
    0,
    NOW(),
    NOW()
);
```

#### 2.3 Wait for Processing
```bash
sleep 5
```

#### 2.4 Verify No Duplicates
```sql
-- Count again
SELECT
    (SELECT COUNT(*) FROM chat_rooms WHERE room_type = 'negotiation') as room_count,
    (SELECT COUNT(*) FROM chat_messages WHERE message_type = 'negotiation_proposal') as message_count;
```

**Expected Result:** Same counts as step 2.1

#### 2.5 Check Idempotency Logs
```bash
# Check logs for idempotency handling
grep "Initial proposal message already exists, treating as success" /path/to/logs
```

**Expected Result:** Log message found

### ✅ PASS Criteria
- [ ] No new chat room created
- [ ] No new proposal message created
- [ ] Logs show idempotency handling

---

## 🧪 TEST 3: PARALLEL SAFETY VALIDATION

### Objective
Verify: Concurrent requests don't create duplicates

### Steps

#### 3.1 Prepare Parallel Test Script
```bash
#!/bin/bash
# parallel_test.sh

SESSION_ID="test-parallel-001"
BUYER_ID="test-buyer-002"
SELLER_ID="test-seller-002"

# Launch 5 parallel requests
for i in {1..5}; do
    curl -X POST http://localhost:8080/api/v1/negotiations \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer YOUR_TOKEN" \
      -d "{
        \"listing_id\": \"test-listing-002\",
        \"buyer_id\": \"$BUYER_ID\",
        \"seller_id\": \"$SELLER_ID\",
        \"initial_price\": 100000,
        \"note\": \"Parallel test $i\"
      }" &
done

wait
echo "All parallel requests completed"
```

#### 3.2 Run Parallel Test
```bash
chmod +x parallel_test.sh
./parallel_test.sh
```

#### 3.3 Wait for Processing
```bash
sleep 10
```

#### 3.4 Verify Single Room
```sql
-- Check rooms created
SELECT COUNT(*) as room_count
FROM chat_rooms
WHERE room_type = 'negotiation'
  AND participant_a = 'test-buyer-002'
  AND participant_b = 'test-seller-002';
```

**Expected Result:** Exactly 1

#### 3.5 Verify Single Message
```sql
-- Check messages created
SELECT COUNT(*) as message_count
FROM chat_messages cm
JOIN chat_rooms cr ON cm.room_id = cr.id
WHERE cr.room_type = 'negotiation'
  AND cr.participant_a = 'test-buyer-002'
  AND cr.participant_b = 'test-seller-002'
  AND cm.message_type = 'negotiation_proposal';
```

**Expected Result:** Exactly 1

#### 3.6 Check for Constraint Violations
```bash
# Check logs for unique constraint violations
grep "unique constraint" /path/to/logs | grep -i "chat_rooms"
```

**Expected Result:** Constraint violation logs present (protection working)

### ✅ PASS Criteria
- [ ] Only 1 chat room created
- [ ] Only 1 proposal message created
- [ ] Database constraints enforced uniqueness

---

## 🧪 TEST 4: OUTBOX STABILITY VALIDATION

### Objective
Verify: No backlog, no stuck events

### Steps

#### 4.1 Check Outbox Statistics
```sql
-- Overall statistics
SELECT
    COUNT(*) as total_events,
    COUNT(*) FILTER (WHERE status = 'pending') as pending,
    COUNT(*) FILTER (WHERE status = 'processing') as processing,
    COUNT(*) FILTER (WHERE status = 'succeeded') as succeeded,
    COUNT(*) FILTER (WHERE status = 'failed') as failed,
    COUNT(*) FILTER (WHERE status = 'dead_letter') as dead_letter
FROM outbox_messages;
```

**Expected Result:**
- `pending = 0` (or very low, < 10)
- `processing = 0` (none stuck)
- `succeeded > 0` (events being processed)
- `failed = 0` (or very low, < 5)

#### 4.2 Check for Stuck Events
```sql
-- Find events stuck in 'processing' for > 5 minutes
SELECT
    id,
    event_type,
    status,
    retry_count,
    created_at,
    updated_at,
    NOW() - updated_at as stuck_duration
FROM outbox_messages
WHERE status = 'processing'
  AND updated_at < NOW() - INTERVAL '5 minutes';
```

**Expected Result:** 0 rows

#### 4.3 Check Event Age
```sql
-- Find pending events older than 1 minute
SELECT
    id,
    event_type,
    status,
    created_at,
    NOW() - created_at as waiting_duration
FROM outbox_messages
WHERE status = 'pending'
  AND created_at < NOW() - INTERVAL '1 minute'
ORDER BY created_at ASC
LIMIT 10;
```

**Expected Result:** 0 rows (or events are being processed)

#### 4.4 Check Negotiation Events Specifically
```sql
-- Check negotiation event processing
SELECT
    event_type,
    COUNT(*) as count,
    COUNT(*) FILTER (WHERE status = 'succeeded') as succeeded,
    COUNT(*) FILTER (WHERE status = 'failed') as failed,
    COUNT(*) FILTER (WHERE status = 'pending') as pending,
    AVG(EXTRACT(EPOCH FROM (updated_at - created_at))) as avg_processing_seconds
FROM outbox_messages
WHERE event_type LIKE 'negotiation.%'
GROUP BY event_type;
```

**Expected Result:**
- High success rate (> 95%)
- Low pending count
- Processing time < 5 seconds

#### 4.5 Check Worker Logs
```bash
# Check worker is processing events
grep "Processing outbox batch" /path/to/logs | tail -20

# Check for successful processing
grep "Event processed successfully" /path/to/logs | tail -20

# Check for errors
grep "Failed to process event" /path/to/logs | tail -20
```

**Expected Result:**
- Worker logs show regular processing
- Success messages present
- No error messages

### ✅ PASS Criteria
- [ ] No pending events backlog
- [ ] No stuck events in processing
- [ ] High success rate (> 95%)
- [ ] Worker logs show healthy processing

---

## 🧪 TEST 5: NEGOTIATION → CHAT END-TO-END

### Objective
Verify: Complete flow works correctly

### Steps

#### 5.1 Create Full Negotiation Flow
```bash
# 1. Start negotiation
SESSION_RESPONSE=$(curl -X POST http://localhost:8080/api/v1/negotiations \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "listing_id": "test-listing-e2e",
    "initial_price": 100000
  }')

SESSION_ID=$(echo $SESSION_RESPONSE | jq -r '.id')

# 2. Wait for chat room creation
sleep 3

# 3. Send counter-proposal
curl -X POST http://localhost:8080/api/v1/negotiations/$SESSION_ID/proposals \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -d '{
    "price": 90000
  }'

# 4. Wait for message creation
sleep 3
```

#### 5.2 Verify Complete Flow
```sql
-- Check room exists
SELECT id, room_type FROM chat_rooms
WHERE room_type = 'negotiation'
LIMIT 1;

-- Check initial proposal exists
SELECT id, message_type, attachment->>'proposal_sequence' as seq
FROM chat_messages
WHERE room_id = (SELECT id FROM chat_rooms WHERE room_type = 'negotiation' LIMIT 1)
  AND attachment->>'type' = 'negotiation_proposal'
  AND attachment->>'proposal_sequence' = '1';

-- Check counter-proposal exists
SELECT id, message_type, attachment->>'proposal_sequence' as seq
FROM chat_messages
WHERE room_id = (SELECT id FROM chat_rooms WHERE room_type = 'negotiation' LIMIT 1)
  AND attachment->>'type' = 'negotiation_proposal'
  AND attachment->>'proposal_sequence' = '2';
```

**Expected Result:** All queries return 1 row

#### 5.3 Verify Chat Room UI
```bash
# Get chat messages via API
curl -X GET http://localhost:8080/api/v1/chat/rooms/{room_id}/messages \
  -H "Authorization: Bearer YOUR_TOKEN"
```

**Expected Result:** Both proposal messages present with correct attachments

### ✅ PASS Criteria
- [ ] Chat room created automatically
- [ ] Initial proposal message present
- [ ] Counter-proposal message present
- [ ] Proposal sequences correct (1, 2)
- [ ] Chat UI shows messages correctly

---

## 📊 FINAL VALIDATION CHECKLIST

### TEST RESULTS SUMMARY

```plaintext
[ ] TEST 1: Single Flow - PASS / FAIL
[ ] TEST 2: Retry Safety - PASS / FAIL
[ ] TEST 3: Parallel Safety - PASS / FAIL
[ ] TEST 4: Outbox Stability - PASS / FAIL
[ ] TEST 5: End-to-End Flow - PASS / FAIL
```

### FINAL OUTPUT

```plaintext
CHAT_ROOM_DUPLICATE: NO ✅ / YES ❌
MESSAGE_DUPLICATE: NO ✅ / YES ❌
OUTBOX_STABLE: YES ✅ / NO ❌
RACE_SAFE: YES ✅ / NO ❌
```

### PRODUCTION READINESS

```plaintext
IF ALL TESTS PASS:
  CHAT INTEGRATION = AMAN UNTUK PRODUCTION ✅

IF ANY TEST FAILS:
  CHAT INTEGRATION = PERLU REVIEW ❌
  - Review failed tests
  - Fix issues
  - Re-run validation
```

---

## 🔄 CLEANUP

### Remove Test Data
```sql
-- Delete test negotiations
DELETE FROM negotiation_sessions WHERE id LIKE 'test-%';

-- Delete test chat messages
DELETE FROM chat_messages WHERE room_id IN (
    SELECT id FROM chat_rooms WHERE participant_a LIKE 'test-%'
);

-- Delete test chat rooms
DELETE FROM chat_rooms WHERE participant_a LIKE 'test-%';

-- Delete test users
DELETE FROM users WHERE id LIKE 'test-%';

-- Delete test outbox events
DELETE FROM outbox_messages WHERE aggregate_id LIKE 'test-%';
```

---

## 📞 SUPPORT

If tests fail or unexpected behavior occurs:

1. **Check Logs:**
   - Worker logs: `/var/log/outbox-worker.log`
   - Application logs: `/var/log/application.log`

2. **Database State:**
   - Run outbox statistics queries
   - Check for constraint violations

3. **Event Status:**
   - Query outbox_messages table
   - Check event retry counts

4. **Common Issues:**
   - Worker not running: Start outbox worker
   - Database constraints failing: Check schema
   - Events not processing: Check worker logs
