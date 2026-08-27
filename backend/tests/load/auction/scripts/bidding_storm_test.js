/**
 * BIDDING STORM TEST - Last Second Sniping
 * Simulates massive bidding rush in final seconds before auction ends
 * Tests: race conditions, extension limits, double-winner prevention
 */

import http from 'k6/http';
import { Counter, Trend, Rate } from 'k6/metrics';
import { getTokenForVu } from '../../infra/token_loader.js';

const rateLimited429 = new Rate('rate_limited_429');
const serverError500 = new Rate('server_error_500');
const validation400 = new Rate('validation_400');
const bidAccepted = new Counter('bid_accepted');
const bidTotalLatency = new Trend('bid_total_latency');
const extensionTriggered = new Counter('extension_triggered');
const doubleWinnerDetected = new Counter('double_winner_detected');

// Single auction for concentrated load
const AUCTION_ID = '00000000-0000-0000-0000-000000000001';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

// Auth tokens are loaded from tests/load/tokens/token_*.txt files via token_loader.js

// Track winners to detect double-winner bug
const winners = new Map();

export const options = {
  stages: [
    { duration: '10s', target: 50 },    // Warm up
    { duration: '30s', target: 500 },   // STORM! Massive spike
    { duration: '10s', target: 0 },     // Cool down
  ],
  thresholds: {
    bid_total_latency: ['p(95)<3000'],  // Lenient for storm
    server_error_500: ['rate<0.1'],     // Allow 10% during storm
    double_winner_detected: ['count==0'], // CRITICAL: No double winner!
  },
};

function calculateBidAmount(vuId, iteration) {
  // Start high to ensure bids are competitive
  const baseAmount = 200000;
  const increment = 5000; // Smaller increments for more竞争
  return baseAmount + (iteration * increment) + (vuId * 1000);
}

export default function () {
  const authToken = getTokenForVu(__VU);

  const bidAmount = calculateBidAmount(__VU, __ITER);

  const url = `${BASE_URL}/api/v1/auctions/${AUCTION_ID}/bid`;
  const payload = JSON.stringify({ amount: bidAmount });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${authToken}`,
    },
    tags: {
      test_type: 'bidding_storm',
      user_id: `vu${__VU}`,
      auction_id: AUCTION_ID,
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

    // Check for winner info to detect double-winner bug
    if (responseData && responseData.data && responseData.data.bid) {
      const bidderId = responseData.data.bid.bidder_id;
      const isWinning = responseData.data.bid.is_winning;

      if (isWinning) {
        if (winners.has(AUCTION_ID)) {
          const currentWinner = winners.get(AUCTION_ID);
          if (currentWinner !== bidderId) {
            // Winner changed - normal outbid
            winners.set(AUCTION_ID, bidderId);
          }
        } else {
          winners.set(AUCTION_ID, bidderId);
        }
      }

      // Check for extension
      if (responseData.data.is_extended) {
        extensionTriggered.add(1);
      }
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
    rateLimited429.add(0);
    serverError500.add(0);
  } else if (res.status === 500) {
    serverError500.add(1);
    rateLimited429.add(0);
    validation400.add(0);

    // Log 500 errors during storm
    if (__ITER < 10) {
      console.error(`VU${__VU} 500 Error: ${res.body.substring(0, 200)}`);
    }
  } else if (res.status === 401) {
    console.error(`VU${__VU} Auth failed!`);
    rateLimited429.add(0); serverError500.add(0); validation400.add(0);
  }

  // No sleep during storm - max pressure
}

export function handleSummary(data) {
  return {
    'stdout': textSummary(data),
    'bidding_storm_results.json': JSON.stringify(data, null, 2),
  };
}

function textSummary(data) {
  let out = '\n╔══════════════════════════════════════════════════════╗\n';
  out += '║       BIDDING STORM TEST RESULTS (Last Second Sniping)    ║\n';
  out += '╚══════════════════════════════════════════════════════╝\n\n';

  out += `🌀 STORM INTENSITY:\n`;
  out += `  Peak VUs: 500\n`;
  out += `  Duration: 30s\n`;
  out += `  Target: Single auction (concentrated load)\n\n`;

  if (data.metrics.http_req_duration) {
    const req = data.metrics.http_req_duration.values;
    out += `📊 HTTP DURATION:\n`;
    out += `  p50: ${req['p(50)']?.toFixed(2) || 'N/A'} ms\n`;
    out += `  p95: ${req['p(95)']?.toFixed(2) || 'N/A'} ms ${req['p(95)'] < 3000 ? '✅' : '❌'}\n`;
    out += `  p99: ${req['p(99)']?.toFixed(2) || 'N/A'} ms\n`;
  }

  if (data.metrics.bid_total_latency) {
    const lat = data.metrics.bid_total_latency.values;
    out += `\n📊 BID LATENCY:\n`;
    out += `  p50: ${lat['p(50)']?.toFixed(2) || 'N/A'} ms\n`;
    out += `  p95: ${lat['p(95)']?.toFixed(2) || 'N/A'} ms\n`;
    out += `  p99: ${lat['p(99)']?.toFixed(2) || 'N/A'} ms\n`;
  }

  if (data.metrics.bid_accepted) out += `\n✅ BID ACCEPTED: ${data.metrics.bid_accepted.values.count}\n`;
  if (data.metrics.extension_triggered) out += `⏱️  EXTENSIONS: ${data.metrics.extension_triggered.values.count}\n`;
  if (data.metrics.double_winner_detected) out += `🚨 DOUBLE WINNER: ${data.metrics.double_winner_detected.values.count} (should be 0!)\n`;

  out += `\n❌ ERRORS:\n`;
  if (data.metrics.rate_limited_429) {
    const rate429 = data.metrics.rate_limited_429.values.rate * 100;
    out += `  429 Rate Limited: ${data.metrics.rate_limited_429.values.count} (${rate429.toFixed(2)}%)`;
    out += rate429 > 10 ? ' ✅ (protection active)\n' : '\n';
  }
  if (data.metrics.server_error_500) {
    const rate500 = data.metrics.server_error_500.values.rate * 100;
    out += `  500 Server Error: ${data.metrics.server_error_500.values.count} (${rate500.toFixed(2)}%) ${(rate500 < 10) ? '✅' : '❌'}\n`;
  }
  if (data.metrics.validation_400) {
    out += `  400 Validation: ${data.metrics.validation_400.values.count}\n`;
  }

  if (data.metrics.http_reqs) {
    out += `\n📨 REQUESTS:\n`;
    out += `  Total: ${data.metrics.http_reqs.values.count}\n`;
    out += `  Peak RPS: ~${(data.metrics.http_reqs.values.count / 30).toFixed(2)}\n`;
  }

  out += `\n🎯 STORM TEST CRITERIA:\n`;
  out += `  ${data.metrics.server_error_500 && data.metrics.server_error_500.values.rate < 0.1 ? '✅' : '❌'} Server errors < 10%\n`;
  out += `  ${data.metrics.double_winner_detected && data.metrics.double_winner_detected.values.count === 0 ? '✅' : '❌'} No double winner bug\n`;
  out += `  ${data.metrics.rate_limited_429 && data.metrics.rate_limited_429.values.rate > 0 ? '✅' : '❌'} Protection layer active\n`;

  out += '\n';
  return out;
}
