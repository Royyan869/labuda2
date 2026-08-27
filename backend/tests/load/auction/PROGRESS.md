# Auction Load Test Progress

> **Last Updated:** 2026-02-07
> **Status:** 🟡 IN PROGRESS

---

## Quick Status

| Feature | Status | Notes | Last Tested |
|---------|--------|-------|-------------|
| Auth pipeline | ✅ PASS | Firebase Auth integration works | 2026-02-07 |
| UserLookupMiddleware | ✅ PASS | Firebase UID → DB UUID conversion OK | 2026-02-07 |
| Bid single (sequential) | ✅ PASS | Single user bidding works | 2026-02-07 |
| Multi-user load test | 🔴 BLOCKED | Needs valid Firebase tokens | 2026-02-07 |
| Rate limiting (429) | 🟡 PARTIAL | Protection layer exists, needs validation | - |
| Bid cooldown (600ms) | 🟡 PARTIAL | Implemented, needs load validation | - |
| Auction extension | 🟢 TODO | Extension trigger on last-minute bids | - |
| Concurrent bidding | 🔴 BLOCKED | Storm test needs tokens | - |

---

## Test Coverage

### ✅ Completed

- [x] **Auth Pipeline Validation**
  - Firebase ID token verification works
  - Backend correctly validates JWT format
  - Invalid tokens return 401

- [x] **UserLookupMiddleware**
  - Firebase UID mapped to DB UUID (10000000-xxxx format)
  - bidder_id correctly populated in responses

- [x] **Single User Bidding**
  - Quick smoke test passes with valid auth
  - Bid accepted returns 201/200
  - Insufficient funds returns 400

### 🔴 Blocked

- [ ] **Multi-User Load Tests** (medium, high, stress)
  - **Blocker:** No valid Firebase tokens available
  - **Impact:** All k6 scripts fail with 401 Unauthorized
  - **Workaround:** None - requires real Firebase tokens
  - **Next Action:** Generate tokens via Flutter app login

  **Files affected:**
  - `scripts/bidding_medium_test.js` (50 VUs)
  - `scripts/bidding_high_test.js` (100 VUs)
  - `scripts/bidding_stress_test.js` (ramp to 1000 VUs)
  - `scripts/bidding_storm_test.js` (500 VUs spike)

### 🟡 In Progress

- [ ] **Token Infrastructure**
  - token_loader.js created
  - Tests need valid Firebase ID tokens (3-segment JWT)
  - Token verification script available: `../../infra/verify_tokens.js`

### 📋 Planned

- [ ] **Rate Limiting Validation**
  - Verify 429 responses under heavy load
  - Confirm 600ms cooldown enforced per user

- [ ] **Auction Extension**
  - Test extension trigger on bids near expiry
  - Validate max extension limit

- [ ] **Concurrent Bidding (Storm Test)**
  - Last-second sniping scenario
  - Double-winner prevention check

- [ ] **Soak Test**
  - Extended duration (1h+) stability
  - Memory leak detection

---

## Known Issues

| ID | Description | Impact | Status |
|----|-------------|--------|--------|
| #AUTH-001 | Dummy tokens (`sufficient_token_1`) not valid JWT | All load tests fail 401 | Blocked |
| #AUTH-002 | Need 5+ real Firebase test accounts | Cannot run multi-VU tests | Open |

---

## Test Results Summary

| Test Name | Date | Result | Key Metrics |
|-----------|------|--------|-------------|
| quick_test (manual token) | 2026-02-07 | ✅ Pass | Auth: OK, Bid: 201 |
| medium_load (dummy tokens) | 2026-02-07 | ❌ Fail | 55,000+ requests → 401 |

---

## Configuration

- **Base URL:** `http://localhost:8080`
- **Test Auctions:** 10 pre-configured UUIDs
- **Auth:** Firebase ID tokens from `tests/load/tokens/token_*.txt`
- **Token Loader:** `../../infra/token_loader.js`

---

## Next Steps (Priority Order)

1. **Generate Valid Firebase Tokens** (BLOCKER)
   ```bash
   # 1. Login via Flutter app
   # 2. Export idToken:
   await FirebaseAuth.instance.currentUser?.getIdToken()
   # 3. Save to tests/load/tokens/token_1.txt, etc.
   # 4. Verify:
   node backend/tests/load/infra/verify_tokens.js
   ```

2. **Run Medium Load Test**
   ```bash
   k6 run backend/tests/load/auction/scripts/bidding_medium_test.js
   ```

3. **Run High Load Test**
   ```bash
   k6 run backend/tests/load/auction/scripts/bidding_high_test.js
   ```

4. **Run Stress Test**
   ```bash
   k6 run backend/tests/load/auction/scripts/bidding_stress_test.js
   ```

5. **Run Storm Test**
   ```bash
   k6 run backend/tests/load/auction/scripts/bidding_storm_test.js
   ```

---

## Changelog

| Date | Change | Author |
|------|--------|--------|
| 2026-02-07 | Initial setup, migrated scripts from tests/load/bidding/ | Claude |
| 2026-02-07 | Created token_loader.js, updated imports | Claude |
| 2026-02-07 | Identified auth token blocker | Claude |
