# [DOMAIN] Load Test Plan

> **Domain:** [DOMAIN_NAME]
> **Version:** 1.0
> **Last Updated:** YYYY-MM-DD

---

## Overview

Brief description of what this domain does and what aspects we're testing.

---

## Test Objectives

1. **Performance:** Validate [specific metrics]
2. **Reliability:** Ensure [specific behavior] under load
3. **Scalability:** Verify system handles [specific load]

---

## Target Metrics

| Metric | Target | Threshold |
|--------|--------|-----------|
| p95 Latency | < XXXms | < YYYms |
| p99 Latency | < XXXms | < YYYms |
| Max Throughput | XXX RPS | - |
| Error Rate | < 0.1% | < 1% |

---

## Test Scenarios

### 1. Smoke Test

**Purpose:** Quick validation that basic functionality works

**Configuration:**
- VUs: 1-5
- Duration: 30s - 1m
- Rate: Limited

**Script:** `scripts/quick_test.js`

**Success Criteria:**
- [ ] All requests return 2xx/4xx (no 5xx)
- [ ] Auth pipeline works
- [ ] No unexpected errors

---

### 2. Normal Load

**Purpose:** Simulate expected production traffic

**Configuration:**
- VUs: XX
- Duration: Xm
- Rate: XX requests/sec

**Script:** `scripts/normal_load_test.js`

**Success Criteria:**
- [ ] p95 latency < target
- [ ] Error rate < target
- [ ] No memory leaks
- [ ] CPU < 80%

---

### 3. High Load

**Purpose:** Test system at peak expected traffic

**Configuration:**
- VUs: XX
- Duration: Xm
- Rate: XX requests/sec

**Script:** `scripts/high_load_test.js`

**Success Criteria:**
- [ ] p95 latency < target
- [ ] Error rate < target
- [ ] Degradation is graceful

---

### 4. Stress Test

**Purpose:** Find breaking point

**Configuration:**
- Ramp: X → XX VUs over Xm
- Duration: Xm
- Rate: Unlimited

**Script:** `scripts/stress_test.js`

**Success Criteria:**
- [ ] Identify breaking point
- [ ] System recovers when load stops
- [ ] No data corruption

---

### 5. Soak Test

**Purpose:** Test stability over extended period

**Configuration:**
- VUs: XX
- Duration: Xh
- Rate: XX requests/sec

**Script:** `scripts/soak_test.js`

**Success Criteria:**
- [ ] No memory leaks
- [ ] Stable response times
- [ ] Connection pool stays healthy

---

## Test Data

### Prerequisites

- [ ] Test accounts created
- [ ] Test data seeded (auctions, listings, etc.)
- [ ] Database in known state

### Cleanup

- [ ] Test data removed after run
- [ ] Test accounts reset

---

## Dependencies

| Service | Required For | Notes |
|---------|--------------|-------|
| Firebase Auth | All tests | Valid tokens needed |
| Database | All tests | Test DB recommended |
| Redis | Rate limiting | Flush before tests |

---

## Run Commands

```bash
# From backend/tests/load/[domain]/

# Quick smoke test
k6 run scripts/quick_test.js

# Normal load
k6 run scripts/normal_load_test.js

# High load
k6 run scripts/high_load_test.js

# Stress test
k6 run scripts/stress_test.js

# All tests (sequential)
npm run test:load:domain
```

---

## Monitoring

During tests, monitor:

- [ ] Backend logs (check for errors)
- [ ] Database connections
- [ ] Redis memory
- [ ] CPU/Memory usage
- [ ] Response times

---

## Failure Analysis

If tests fail, check:

1. **Auth:** Are tokens valid? Not expired?
2. **Test Data:** Do test entities exist?
3. **Dependencies:** Are all services running?
4. **Rate Limits:** Is protection layer blocking?
5. **Resources:** Is CPU/memory sufficient?

---

## References

- API Documentation: [link]
- Architecture Docs: [link]
- Related Issues: #[issue_numbers]
