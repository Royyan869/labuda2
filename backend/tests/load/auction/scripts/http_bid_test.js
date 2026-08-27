/**
 * HTTP BIDDING LOAD TEST
 *
 * Architecture Compliance:
 * - Bidding via HTTP POST /api/v1/auctions/:id/bid (COMMAND)
 * - Measures actual bid processing latency
 * - Tests protection layer, rate limiting, balance validation
 *
 * Test Distribution:
 * - 70% users with sufficient balance (10K-50K coins)
 * - 30% users with insufficient balance (10-90 coins)
 * - 5 parallel auctions
 * - 600ms cooldown between bids per user
 */

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter, Trend, Rate } from 'k6/metrics';
import { getTokenForVu } from '../../infra/token_loader.js';

// ==============================================================================
// CUSTOM METRICS
// ==============================================================================
const bidAccepted = new Counter('bid_accepted');
const bidInsufficientFunds = new Counter('bid_insufficient_funds');
const bidRateLimited = new Counter('bid_rate_limited');
const bidValidationError = new Counter('bid_validation_error');
const bidServerError = new Counter('bid_server_error');

const bidTotalLatency = new Trend('bid_total_latency');
const bidProcessingTime = new Trend('bid_processing_time');

const rateLimited429 = new Rate('rate_limited_429');
const serverError500 = new Rate('server_error_500');

// ==============================================================================
// TEST CONFIGURATION
// ==============================================================================
export const options = {
  scenarios: {
    medium_load: {
      executor: 'constant-vus',
      vus: 50,
      duration: '3m',
      gracefulStop: '30s',
      tags: { test_type: 'medium' },
    },
    high_load: {
      executor: 'constant-vus',
      vus: 100,
      duration: '5m',
      startTime: '4m',
      gracefulStop: '30s',
      tags: { test_type: 'high' },
    },
    stress_test: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '1m', target: 50 },
        { duration: '2m', target: 100 },
        { duration: '2m', target: 200 },
        { duration: '1m', target: 100 },
        { duration: '1m', target: 0 },
      ],
      startTime: '10m',
      gracefulStop: '30s',
      tags: { test_type: 'stress' },
    },
  },

  thresholds: {
    'bid_total_latency': ['p(95)<1200', 'p(99)<2000'],
    'server_error_500': ['rate<0.0001'],
    'rate_limited_429': ['rate>0'], // Expected to have some rate limiting
  },
};

// ==============================================================================
// AUCTION & USER CONFIGURATION
// ==============================================================================
const AUCTIONS = [
  '00000000-0000-0000-0000-000000000001',
  '00000000-0000-0000-0000-000000000002',
  '00000000-0000-0000-0000-000000000003',
  '00000000-0000-0000-0000-000000000004',
  '00000000-0000-0000-0000-000000000005',
];

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_BASE = `${BASE_URL}/api/v1`;

// Firebase ID tokens are loaded from tests/load/tokens/token_*.txt files
// via token_loader.js module (SharedArray for efficient VU sharing)
// Token format: Valid Firebase JWT (3 segments: header.payload.signature)

// Base bid amounts per auction
const AUCTION_BASE_BIDS = {
  [AUCTIONS[0]]: 1000,
  [AUCTIONS[1]]: 500,
  [AUCTIONS[2]]: 2000,
  [AUCTIONS[3]]: 1500,
  [AUCTIONS[4]]: 800,
};

const BID_INCREMENT = 50;
const COOLDOWN_MS = 600; // Must respect 600ms cooldown

// ==============================================================================
// HELPER FUNCTIONS
// ==============================================================================

/**
 * Get auction ID for this VU (round-robin)
 */
function getAuctionId(vuId) {
  return AUCTIONS[vuId % AUCTIONS.length];
}

/**
 * Get user balance tier
 * First 70% get sufficient balance, last 30% get insufficient
 */
function getUserBalanceTier(vuId, totalVUs) {
  const sufficientThreshold = Math.floor(totalVUs * 0.7);
  return vuId <= sufficientThreshold ? 'sufficient' : 'insufficient';
}

/**
 * Get auth token for this VU (loaded from token files via token_loader)
 */
function getAuthToken(vuId) {
  return getTokenForVu(vuId);
}

/**
 * Calculate next bid amount with randomization
 */
function calculateNextBid(auctionId, vuId) {
  const baseBid = AUCTION_BASE_BIDS[auctionId] || 1000;
  const randomIncrement = Math.floor(Math.random() * 10) * BID_INCREMENT; // 0, 50, 100, ... 450
  return baseBid + randomIncrement + (vuId * 10); // Add VU offset for uniqueness
}

/**
 * Classify bid response
 */
function classifyBidResponse(res) {
  const status = res.status;

  // Track latency
  if (res.timings) {
    bidTotalLatency.add(res.timings.duration || 0);

    // Extract processing time from header if present
    const processingTime = res.headers['X-Processing-Time'] || res.headers['x-processing-time'];
    if (processingTime) {
      bidProcessingTime.add(parseFloat(processingTime));
    }
  }

  // Classify by status code
  if (status === 201 || status === 200) {
    bidAccepted.add(1);
    return { type: 'accepted', code: status };
  } else if (status === 429) {
    bidRateLimited.add(1);
    rateLimited429.add(1);
    return { type: 'rate_limited', code: status };
  } else if (status === 400 || status === 422) {
    bidValidationError.add(1);

    // Check for insufficient funds in response body
    try {
      const body = JSON.parse(res.body);
      if (body.error && (body.error.includes('insufficient') || body.error.includes('balance'))) {
        bidInsufficientFunds.add(1);
        return { type: 'insufficient_funds', code: status };
      }
    } catch (e) {}

    return { type: 'validation_error', code: status };
  } else if (status >= 500) {
    bidServerError.add(1);
    serverError500.add(1);
    return { type: 'server_error', code: status };
  }

  return { type: 'unknown', code: status };
}

// ==============================================================================
// MAIN TEST FUNCTION
// ==============================================================================

export default function () {
  const vuId = __VU;
  const auctionId = getAuctionId(vuId);
  const balanceTier = getUserBalanceTier(vuId, 50); // Use 50 as default for medium test
  const authToken = getAuthToken(vuId);
  const bidAmount = calculateNextBid(auctionId, vuId);

  // Build request
  const url = `${API_BASE}/auctions/${auctionId}/bid`;
  const payload = JSON.stringify({
    amount: bidAmount,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${authToken}`,
    },
    tags: {
      test_type: __ENV.SCENARIO_TYPE || 'medium',
      user_id: `user-${vuId}`,
      auction_id: auctionId,
      balance_tier: balanceTier,
    },
  };

  // Send bid request
  const startTime = Date.now();
  const res = http.post(url, payload, params);
  const endTime = Date.now();

  // Classify response
  const result = classifyBidResponse(res);

  // Log for debugging (sample only)
  if (result.type === 'server_error' || (result.type === 'rate_limited' && vuId < 5)) {
    console.log(`[VU${vuId}] ${result.type.toUpperCase()} - Status: ${result.code}, Auction: ${auctionId}, Bid: ${bidAmount}`);
  }

  // Track success/failure
  const success = check(res, {
    'bid request completed': (r) => r.status !== 0,
    'not a server error': (r) => r.status < 500,
  });

  // Respect cooldown (600ms as per system requirements)
  sleep(Math.max(0.6, COOLDOWN_MS / 1000 + (Math.random() * 0.2))); // 600-800ms
}

// ==============================================================================
// SUMMARY HANDLER
// ==============================================================================

export function handleSummary(data) {
  return {
    'stdout': textSummary(data),
    'http_bid_results.json': JSON.stringify(data, null, 2),
    'http_bid_summary.html': htmlReport(data),
  };
}

function textSummary(data) {
  let out = '\n╔══════════════════════════════════════════════════════════════╗\n';
  out += '║         HTTP BIDDING LOAD TEST RESULTS                         ║\n';
  out += '╚══════════════════════════════════════════════════════════════╝\n\n';

  // Bid Latency
  if (data.metrics.bid_total_latency) {
    const lat = data.metrics.bid_total_latency.values;
    out += `📊 BID TOTAL LATENCY:\n`;
    out += `  p50: ${lat['p(50)'] ? lat['p(50)'].toFixed(2) : 'N/A'} ms\n`;
    out += `  p95: ${lat['p(95)'] ? lat['p(95)'].toFixed(2) : 'N/A'} ms ${lat['p(95)'] < 1200 ? '✅' : '❌'}\n`;
    out += `  p99: ${lat['p(99)'] ? lat['p(99)'].toFixed(2) : 'N/A'} ms\n`;
    out += `  max: ${lat.max ? lat.max.toFixed(2) : 'N/A'} ms\n`;
  }

  // Processing Time
  if (data.metrics.bid_processing_time && data.metrics.bid_processing_time.values.count > 0) {
    const proc = data.metrics.bid_processing_time.values;
    out += `\n⚙️  SERVER PROCESSING TIME:\n`;
    out += `  p50: ${proc['p(50)'] ? proc['p(50)'].toFixed(2) : 'N/A'} ms\n`;
    out += `  p95: ${proc['p(95)'] ? proc['p(95)'].toFixed(2) : 'N/A'} ms\n`;
  }

  // Bid Results
  const accepted = data.metrics.bid_accepted?.values.count || 0;
  const insufficient = data.metrics.bid_insufficient_funds?.values.count || 0;
  const rateLimited = data.metrics.bid_rate_limited?.values.count || 0;
  const validationError = data.metrics.bid_validation_error?.values.count || 0;
  const serverError = data.metrics.bid_server_error?.values.count || 0;

  out += `\n✅ BID RESULTS:\n`;
  out += `  Accepted: ${accepted}\n`;
  out += `  Insufficient Funds: ${insufficient}\n`;
  out += `  Rate Limited (429): ${rateLimited}\n`;
  out += `  Validation Error (400): ${validationError}\n`;
  out += `  Server Error (500): ${serverError}\n`;

  // Calculate totals
  const totalBids = accepted + insufficient + rateLimited + validationError + serverError;
  if (totalBids > 0) {
    const successRate = ((accepted / totalBids) * 100).toFixed(2);
    const rejectRate = ((insufficient / totalBids) * 100).toFixed(2);
    const expectedRejectRate = 30; // 30% of users have insufficient balance

    out += `\n📈 ANALYSIS:\n`;
    out += `  Total Bids: ${totalBids}\n`;
    out += `  Success Rate: ${successRate}%\n`;
    out += `  Insufficient Reject Rate: ${rejectRate}% (Expected: ~${expectedRejectRate}%)\n`;
    out += `  Rate Limit Rate: ${((rateLimited / totalBids) * 100).toFixed(2)}%\n`;
    out += `  ${Math.abs(parseFloat(rejectRate) - expectedRejectRate) < 10 ? '✅ BALANCE DISTRIBUTION OK' : '⚠️  BALANCE DISTRIBUTION OFF'}\n`;
  }

  // Error Rates
  out += `\n❌ ERROR RATES:\n`;
  if (data.metrics.rate_limited_429) {
    const rate429 = data.metrics.rate_limited_429.values.rate * 100;
    out += `  429 Rate Limited: ${rate429.toFixed(2)}% ${rate429 > 0 ? '✅ (Protection Active)' : ''}\n`;
  }
  if (data.metrics.server_error_500) {
    const rate500 = data.metrics.server_error_500.values.rate * 100;
    out += `  500 Server Error: ${rate500.toFixed(4)}% ${rate500 < 0.01 ? '✅' : '❌'}\n`;
  }

  // HTTP Metrics
  if (data.metrics.http_req_duration) {
    const http = data.metrics.http_req_duration.values;
    out += `\n🌐 HTTP REQUESTS:\n`;
    out += `  Total: ${data.metrics.http_reqs?.values.count || 0}\n`;
    out += `  p50: ${http['p(50)'] ? http['p(50)'].toFixed(2) : 'N/A'} ms\n`;
    out += `  p95: ${http['p(95)'] ? http['p(95)'].toFixed(2) : 'N/A'} ms\n`;
  }

  // Threshold Results
  out += `\n🎯 THRESHOLD RESULTS:\n`;
  for (const [name, threshold] of Object.entries(data.thresholds || {})) {
    const status = threshold.ok ? '✅ PASS' : '❌ FAIL';
    out += `  ${status} - ${name}\n`;
  }

  out += '\n';
  return out;
}

function htmlReport(data) {
  const lat = data.metrics.bid_total_latency?.values || {};
  const accepted = data.metrics.bid_accepted?.values.count || 0;
  const insufficient = data.metrics.bid_insufficient_funds?.values.count || 0;
  const rateLimited = data.metrics.bid_rate_limited?.values.count || 0;
  const serverError = data.metrics.bid_server_error?.values.count || 0;
  const totalBids = accepted + insufficient + rateLimited + serverError;

  const successRate = totalBids > 0 ? ((accepted / totalBids) * 100).toFixed(2) : '0.00';
  const rate429 = (data.metrics.rate_limited_429?.values.rate || 0) * 100;
  const rate500 = (data.metrics.server_error_500?.values.rate || 0) * 100;

  return `
<!DOCTYPE html>
<html>
<head>
  <title>HTTP Bidding Load Test Results</title>
  <style>
    * { box-sizing: border-box; }
    body { font-family: 'Segoe UI', sans-serif; margin: 0; padding: 20px; background: #f5f7fa; }
    .container { max-width: 1200px; margin: 0 auto; }
    h1 { color: #1a202c; border-bottom: 4px solid #4299e1; padding-bottom: 10px; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; margin: 20px 0; }
    .card { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
    .card-value { font-size: 2rem; font-weight: bold; color: #2d3748; }
    .card-label { font-size: 0.85rem; color: #718096; text-transform: uppercase; }
    .pass { color: #48bb78; }
    .fail { color: #f56565; }
    table { width: 100%; border-collapse: collapse; background: white; border-radius: 8px; overflow: hidden; }
    th, td { padding: 12px 15px; text-align: left; border-bottom: 1px solid #e2e8f0; }
    th { background: #4299e1; color: white; }
  </style>
</head>
<body>
  <div class="container">
    <h1>🚀 HTTP Bidding Load Test Results</h1>

    <div class="grid">
      <div class="card">
        <div class="card-value">${accepted}</div>
        <div class="card-label">Bids Accepted</div>
      </div>
      <div class="card">
        <div class="card-value">${successRate}%</div>
        <div class="card-label">Success Rate</div>
      </div>
      <div class="card">
        <div class="card-value">${lat['p(95)']?.toFixed(0) || 'N/A'} ms</div>
        <div class="card-label">p95 Latency ${lat['p(95)'] < 1200 ? '✅' : '❌'}</div>
      </div>
      <div class="card">
        <div class="card-value">${rate429.toFixed(2)}%</div>
        <div class="card-label">Rate Limited (429)</div>
      </div>
    </div>

    <h2>📊 Bid Results Breakdown</h2>
    <table>
      <tr><th>Metric</th><th>Count</th><th>Rate</th><th>Status</th></tr>
      <tr><td>Bids Accepted</td><td>${accepted}</td><td>${successRate}%</td><td>✅</td></tr>
      <tr><td>Insufficient Funds</td><td>${insufficient}</td><td>${totalBids > 0 ? ((insufficient/totalBids)*100).toFixed(2) : 0}%</td><td>Expected (~30%)</td></tr>
      <tr><td>Rate Limited (429)</td><td>${rateLimited}</td><td>${totalBids > 0 ? ((rateLimited/totalBids)*100).toFixed(2) : 0}%</td><td>${rate429 > 0 ? '✅ Protection' : '⚠️ Low'}</td></tr>
      <tr><td>Server Error (500)</td><td>${serverError}</td><td>${rate500.toFixed(4)}%</td><td>${rate500 < 0.01 ? '✅' : '❌'}</td></tr>
    </table>

    <h2>⏱️ Latency Distribution</h2>
    <table>
      <tr><th>Percentile</th><th>Value (ms)</th><th>Threshold</th></tr>
      <tr><td>p50</td><td>${lat['p(50)']?.toFixed(2) || 'N/A'}</td><td>-</td></tr>
      <tr><td>p95</td><td>${lat['p(95)']?.toFixed(2) || 'N/A'}</td><td>< 1200ms ${lat['p(95)'] < 1200 ? '✅' : '❌'}</td></tr>
      <tr><td>p99</td><td>${lat['p(99)']?.toFixed(2) || 'N/A'}</td><td>< 2000ms</td></tr>
      <tr><td>max</td><td>${lat.max?.toFixed(2) || 'N/A'}</td><td>-</td></tr>
    </table>

    <p style="color: #718096; margin-top: 40px; text-align: center;">
      Generated: ${new Date().toISOString()} | HTTP Bidding Load Test v1.0
    </p>
  </div>
</body>
</html>
  `;
}
