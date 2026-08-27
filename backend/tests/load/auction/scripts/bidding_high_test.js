/**
 * BIDDING HIGH LOAD TEST - 100 VUs, 5 minutes
 * HTTP REST API bidding with auth, cooldown, and multi-auction
 */

import { check } from 'k6';
import http from 'k6/http';
import { Counter, Trend, Rate } from 'k6/metrics';
import { getTokenForVu } from '../../infra/token_loader.js';

// Custom Metrics for Bidding
const rateLimited429 = new Rate('rate_limited_429');
const serverError500 = new Rate('server_error_500');
const validation400 = new Rate('validation_400');
const bidAccepted = new Counter('bid_accepted');
const bidInsufficientFunds = new Counter('bid_insufficient_funds');
const bidTotalLatency = new Trend('bid_total_latency');
const extensionTriggered = new Counter('extension_triggered');

// Test Auctions
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
  vus: 100,
  duration: '5m',
  thresholds: {
    bid_total_latency: ['p(95)<1200'],
    server_error_500: ['rate<0.0001'],
    http_req_duration: ['p(95)<1000'],
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
    tags: {
      test_type: 'bidding_high',
      user_id: `vu${__VU}`,
      auction_id: auctionId,
    },
  };

  const startTime = Date.now();
  const res = http.post(url, payload, params);
  const latency = Date.now() - startTime;

  bidTotalLatency.add(latency);

  let responseData = null;
  try {
    if (res.body) responseData = JSON.parse(res.body);
  } catch (e) {}

  if (res.status === 201 || res.status === 200) {
    bidAccepted.add(1);
    if (responseData && responseData.data && responseData.data.is_extended) {
      extensionTriggered.add(1);
    }
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
      const msg = responseData.error.message.toLowerCase();
      if (msg.includes('insufficient') || msg.includes('balance') || msg.includes('coins')) {
        bidInsufficientFunds.add(1);
      }
    }
    rateLimited429.add(0);
    serverError500.add(0);
  } else if (res.status === 500) {
    serverError500.add(1);
    rateLimited429.add(0);
    validation400.add(0);
  } else if (res.status === 401) {
    console.error(`VU${__VU} Auth failed!`);
    rateLimited429.add(0);
    serverError500.add(0);
    validation400.add(0);
  }

  if (__ITER < 2) {
    const statusText = res.status === 201 ? 'ACCEPTED' : res.status === 429 ? 'RATE_LIMITED' : `ERR_${res.status}`;
    console.log(`VU${__VU} Iter${__ITER} Auction:${auctionId.slice(-4)} Amount:${bidAmount} → ${statusText}`);
  }

  __SJAZZIC_SLEEP = 0.3;
}

export function handleSummary(data) {
  return {
    'stdout': textSummary(data),
    'bidding_high_results.json': JSON.stringify(data, null, 2),
  };
}

function textSummary(data) {
  let out = '\n╔══════════════════════════════════════════════════════╗\n';
  out += '║     BIDDING HIGH LOAD TEST RESULTS (100 VUs, 5m)       ║\n';
  out += '╚══════════════════════════════════════════════════════╝\n\n';

  if (data.metrics.http_req_duration) {
    const req = data.metrics.http_req_duration.values;
    out += `📊 HTTP REQUEST DURATION:\n`;
    out += `  p50: ${req['p(50)'] ? req['p(50)'].toFixed(2) : 'N/A'} ms\n`;
    out += `  p95: ${req['p(95)'] ? req['p(95)'].toFixed(2) : 'N/A'} ms ${req['p(95)'] < 1000 ? '✅' : '❌'}\n`;
    out += `  p99: ${req['p(99)'] ? req['p(99)'].toFixed(2) : 'N/A'} ms\n`;
  }

  if (data.metrics.bid_total_latency) {
    const lat = data.metrics.bid_total_latency.values;
    out += `\n📊 BID LATENCY:\n`;
    out += `  p50: ${lat['p(50)'] ? lat['p(50)'].toFixed(2) : 'N/A'} ms\n`;
    out += `  p95: ${lat['p(95)'] ? lat['p(95)'].toFixed(2) : 'N/A'} ms ${lat['p(95)'] < 1200 ? '✅' : '❌'}\n`;
    out += `  p99: ${lat['p(99)'] ? lat['p(99)'].toFixed(2) : 'N/A'} ms\n`;
  }

  if (data.metrics.bid_accepted) out += `\n✅ BID ACCEPTED: ${data.metrics.bid_accepted.values.count}\n`;
  if (data.metrics.bid_insufficient_funds) out += `💸 INSUFFICIENT FUNDS: ${data.metrics.bid_insufficient_funds.values.count}\n`;
  if (data.metrics.extension_triggered) out += `⏱️  EXTENSION TRIGGERED: ${data.metrics.extension_triggered.values.count}\n`;

  out += `\n❌ ERRORS:\n`;
  if (data.metrics.rate_limited_429) {
    const rate429 = data.metrics.rate_limited_429.values.rate * 100;
    out += `  429 Rate Limited: ${data.metrics.rate_limited_429.values.count} (${rate429.toFixed(2)}%)\n`;
  }
  if (data.metrics.server_error_500) {
    const rate500 = data.metrics.server_error_500.values.rate * 100;
    out += `  500 Server Error: ${data.metrics.server_error_500.values.count} (${rate500.toFixed(4)}%) ${rate500 < 0.01 ? '✅' : '❌'}\n`;
  }
  if (data.metrics.validation_400) {
    const rate400 = data.metrics.validation_400.values.rate * 100;
    out += `  400 Validation: ${data.metrics.validation_400.values.count} (${rate400.toFixed(2)}%)\n`;
  }

  if (data.metrics.http_reqs) {
    out += `\n📨 REQUESTS: ${data.metrics.http_reqs.values.count} (${(data.metrics.http_reqs.values.count / 300).toFixed(2)} RPS)\n`;
  }

  out += '\n';
  return out;
}
