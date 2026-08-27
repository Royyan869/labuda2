# Load Testing

Modular load testing infrastructure for Labuda backend.

## Structure

```
backend/tests/load/
├── .templates/          # PROGRESS.md and TEST_PLAN.md templates
├── infra/               # Shared utilities
│   ├── token_loader.js  # Firebase token loader for k6
│   └── verify_tokens.js # Token verification script
│
├── auction/             # Auction/bidding load tests
│   ├── scripts/         # k6 test scripts
│   ├── results/         # Test reports (generated)
│   ├── PROGRESS.md      # Current status tracker
│   └── TEST_PLAN.md     # Test plan document
│
├── payment/             # Payment load tests (TODO)
│   ├── scripts/
│   ├── results/
│   ├── PROGRESS.md
│   └── TEST_PLAN.md
│
└── README.md            # This file
```

## Quick Start

### 1. Prerequisites

- **k6** installed: `brew install k6` or `choco install k6`
- **Backend** running: `go run cmd/server/main.go`
- **Firebase tokens** (see Token Setup below)

### 2. Token Setup

Load tests require valid Firebase ID tokens:

```bash
# Verify token setup
node backend/tests/load/infra/verify_tokens.js

# If no tokens, follow instructions in:
# tests/load/tokens/README.md
```

### 3. Run Tests

```bash
# From project root

# Quick smoke test (1 user)
node backend/tests/load/auction/scripts/quick_test.js

# Medium load (50 VUs, 3 min)
k6 run backend/tests/load/auction/scripts/bidding_medium_test.js

# High load (100 VUs, 5 min)
k6 run backend/tests/load/auction/scripts/bidding_high_test.js

# Stress test (ramp to 1000 VUs)
k6 run backend/tests/load/auction/scripts/bidding_stress_test.js

# Storm test (last-second sniping)
k6 run backend/tests/load/auction/scripts/bidding_storm_test.js
```

## Domain Organization

Each domain (auction, payment, etc.) has its own:

- **PROGRESS.md** - Current status, blocked items, next steps
- **TEST_PLAN.md** - Test objectives, scenarios, success criteria
- **scripts/** - K6 test files
- **results/** - Generated test reports

This allows:
- ✅ Work to be paused and resumed without losing context
- ✅ Each domain to have its own tracker
- ✅ Multiple developers to work on different domains
- ✅ Clear visibility into what's blocked and why

## Templates

When adding a new domain:

1. Copy templates:
   ```bash
   cp backend/tests/load/.templates/PROGRESS_TEMPLATE.md backend/tests/load/[domain]/PROGRESS.md
   cp backend/tests/load/.templates/TEST_PLAN_TEMPLATE.md backend/tests/load/[domain]/TEST_PLAN.md
   ```

2. Fill in the templates with domain-specific info

3. Create scripts and update PROGRESS.md as you go

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `BASE_URL` | `http://localhost:8080` | Backend API URL |
| `TOKEN_DIR` | `../../../../tests/load/tokens/` | Firebase token directory |
| `AUTH_TOKEN` | (from file) | Single auth token for Node.js scripts |

### Target Metrics

| Metric | Target | Threshold |
|--------|--------|-----------|
| p95 Latency | < 1200ms | < 2000ms |
| p99 Latency | < 2000ms | < 3000ms |
| Error Rate | < 0.01% | < 0.1% |
| 429 Rate | > 0% | Protection active |

## Monitoring During Tests

While tests are running, monitor:

```bash
# Backend logs
tail -f backend/logs/server.log

# System resources
htop

# Database connections
psql -c "SELECT count(*) FROM pg_stat_activity;"
```

## Troubleshooting

### 401 Unauthorized

**Cause:** Invalid or missing Firebase tokens

**Solution:**
```bash
# Verify tokens exist and are valid format
node backend/tests/load/infra/verify_tokens.js

# Regenerate tokens if expired (1 hour lifetime)
```

### Connection Refused

**Cause:** Backend not running

**Solution:**
```bash
cd backend
go run cmd/server/main.go
```

### All Requests Return 429

**Cause:** Rate limiting protection active

**This is expected behavior** - it means protection is working correctly.

### No Results Generated

**Cause:** K6 output directory issue

**Solution:** Run from domain directory:
```bash
cd backend/tests/load/auction
k6 run scripts/bidding_medium_test.js
```

## Contributing

When adding new tests:

1. **Use token_loader** for auth:
   ```javascript
   import { getTokenForVu } from '../../infra/token_loader.js';
   const authToken = getTokenForVu(__VU);
   ```

2. **Update PROGRESS.md** with status
3. **Add to TEST_PLAN.md** if new scenario
4. **Check results/** into .gitignore

## References

- [K6 Documentation](https://k6.io/docs/)
- [Firebase Auth Tokens](https://firebase.google.com/docs/auth/admin/verify-id-tokens)
- [Backend Architecture](../../ARCHITECTURE.md)
