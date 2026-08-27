/**
 * BIDDING STRESS LOAD TEST - Ramp 200 to 1000 VUs
 * Tests system limits and degradation under extreme load
 */

import http from 'k6/http';
import { Counter, Trend, Rate } from 'k6/metrics';
import { getTokenForVu } from '../../infra/token_loader.js';

const rateLimited429 = new Rate('rate_limited_429');
const serverError500 = new Rate('server_error_500');
const validation400 = new Rate('validation_400');
const bidAccepted = new Counter('bid_accepted');
const bidInsufficientFunds = new Counter('bid_insufficient_funds');
const bidTotalLatency = new Trend('bid_total_latency');
const extensionTriggered = new Counter('extension_triggered');

const AUCTIONS = [
  '00000000-0000-0000-0000-000000000001',
  '00000000-0000-0000-0000-000000000002',
  '00000000-0000-0000-0000-000000000003',
  '00000000-0000-0000-0000-000000000004',
  '00000000-0000-0000-0000-000000000005',
  '00000000-0000-0000-0000-000000000006',
  '00000000-0000-0000-0000-000000000007',
  '00000000-0000-0000-0000-000000000008',
  '00000000-0000-0000-0000-000000000009',
  '00000000-0000-0000-0000-000000000010',
];

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Auth tokens are loaded from tests/load/tokens/token_*.txt files via token_loader.js

export const options = {
  stages: [
    { duration: '30s', target: 200 },   // Ramp up to 200
    { duration: '1m', target: 500 },     // Ramp up to 500
    { duration: '1m', target: 1000 },    // Ramp up to 1000
    { duration: '30s', target: 0 },      // Ramp down
  ],
  thresholds: {
    bid_total_latency: ['p(95)<2000'],  // More lenient during stress
    server_error_500: ['rate<0.05'],     // Allow up to 5% during stress
  },
};

function getAuctionId(vuId) {
  return AUCTIONS[vuId % AUCTIONS.length];
}

function calculateBidAmount(vuId, iteration) {
  const baseAmount = 130000;
  const increment = 10000;
  const vuOffset = (vuId % 10) * increment;
  const iterOffset = (iteration % 5) * increment;
  return baseAmount + vuOffset + iterOffset;
}

export default function () {
  const authToken = getTokenForVu(__VU);

  const auctionId = getAuctionId(__VU);
  const bidAmount = calculateBidAmount(__VU, __ITER);

  const url = `${BASE_URL}/api/v1/auctions/${auctionId}/bid`;
  const payload = JSON.stringify({ amount: bidAmount });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${authToken}`,
    },
    tags: { test_type: 'bidding_stress', user_id: `vu${__VU}`, auction_id: auctionId },
  };

  const startTime = Date.now();
  const res = http.post(url, payload, params);
  const latency = Date.now() - startTime;

  bidTotalLatency.add(latency);

  let responseData = null;
  try { if (res.body) responseData = JSON.parse(res.body); } catch (e) {}

  if (res.status === 201 || res.status === 200) {
    bidAccepted.add(1);
    if (responseData && responseData.data && responseData.data.is_extended) extensionTriggered.add(1);
    rateLimited429.add(0);
    serverError500.add(0);
    validation400.add(0);
  } else if (res.status === 429) {
    rateLimited429.add(1);
    serverError500.add(0);
    validation400.add(0);
  } else if (res.status === 400) {
    validation400.add(1);
    if (responseData && responseData.error && responseData.error.message) {
      if (responseData.error.message.toLowerCase().includes('insufficient')) bidInsufficientFunds.add(1);
    }
    rateLimited429.add(0);
    serverError500.add(0);
  } else if (res.status === 500) {
    serverError500.add(1);
    rateLimited429.add(0);
    validation400.add(0);
  } else if (res.status === 401) {
    console.error(`VU${__VU} Auth failed!`);
    rateLimited429.add(0); serverError500.add(0); validation400.add(0);
  }

  __SJAZZIC_SLEEP = 0.2;
}

export function handleSummary(data) {
  return {
    'stdout': textSummary(data),
    'bidding_stress_results.json': JSON.stringify(data, null, 2),
  };
}

function textSummary(data) {
  let out = '\n╔══════════════════════════════════════════════════════╗\n';
  out += '║        BIDDING STRESS TEST RESULTS (ramp 200-1000)     ║\n';
  out += '╚══════════════════════════════════════════════════════╝\n\n';

  if (data.metrics.http_req_duration) {
    const req = data.metrics.http_req_duration.values;
    out += `📊 HTTP DURATION:\n  p50: ${req['p(50)']?.toFixed(2) || 'N/A'} ms\n`;
    out += `  p95: ${req['p(95)']?.toFixed(2) || 'N/A'} ms ${req['p(95)'] < 2000 ? '✅' : '❌'}\n`;
    out += `  p99: ${req['p(99)']?.toFixed(2) || 'N/A'} ms\n`;
  }

  if (data.metrics.bid_total_latency) {
    const lat = data.metrics.bid_total_latency.values;
    out += `\n📊 BID LATENCY:\n  p50: ${lat['p(50)']?.toFixed(2) || 'N/A'} ms\n`;
    out += `  p95: ${lat['p(95)']?.toFixed(2) || 'N/A'} ms ${lat['p(95)'] < 2000 ? '✅' : '❌'}\n`;
    out += `  p99: ${lat['p(99)']?.toFixed(2) || 'N/A'} ms\n`;
  }

  if (data.metrics.bid_accepted) out += `\n✅ BID ACCEPTED: ${data.metrics.bid_accepted.values.count}\n`;
  if (data.metrics.extension_triggered) out += `⏱️  EXTENSION: ${data.metrics.extension_triggered.values.count}\n`;

  out += `\n❌ ERRORS:\n`;
  if (data.metrics.rate_limited_429) {
    out += `  429: ${data.metrics.rate_limited_429.values.count} (${(data.metrics.rate_limited_429.values.rate * 100).toFixed(2)}%)\n`;
  }
  if (data.metrics.server_error_500) {
    out += `  500: ${data.metrics.server_error_500.values.count} (${(data.metrics.server_error_500.values.rate * 100).toFixed(4)}%) ${(data.metrics.server_error_500.values.rate < 0.05) ? '✅' : '❌'}\n`;
  }
  if (data.metrics.validation_400) {
    out += `  400: ${data.metrics.validation_400.values.count} (${(data.metrics.validation_400.values.rate * 100).toFixed(2)}%)\n`;
  }

  if (data.metrics.http_reqs) out += `\n📨 REQUESTS: ${data.metrics.http_reqs.values.count}\n`;

  out += '\n';
  return out;
}
