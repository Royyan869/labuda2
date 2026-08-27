/**
 * HYBRID SCENARIO TEST - HTTP Bid + WebSocket Event Validation
 *
 * Architecture Compliance:
 * - Bidding via HTTP POST /api/v1/auctions/:id/bid (COMMAND)
 * - WebSocket for realtime event confirmation (QUERY)
 * - End-to-end validation: Bid -> Event -> Price Update
 *
 * Flow per VU:
 * 1. Connect WebSocket to auction
 * 2. Send HTTP POST bid
 * 3. Wait for WebSocket event confirmation
 * 4. Validate price increased
 * 5. Handle 429 cooldown
 */

import http from 'k6/http';
import ws from 'k6/ws';
import { check, sleep } from 'k6';
import { Counter, Trend, Rate } from 'k6/metrics';
import { getTokenForVu } from '../../infra/token_loader.js';

// ==============================================================================
// CUSTOM METRICS
// ==============================================================================
// HTTP Bid Metrics
const httpBidSent = new Counter('http_bid_sent');
const httpBidAccepted = new Counter('http_bid_accepted');
const httpBid429 = new Counter('http_bid_429');
const httpBidError = new Counter('http_bid_error');
const httpBidLatency = new Trend('http_bid_latency');

// WebSocket Event Metrics
const wsEventReceived = new Counter('ws_event_received');
const wsBidEventReceived = new Counter('ws_bid_event_received');
const wsEventLatency = new Trend('ws_event_latency');
const wsBidMatchCount = new Counter('ws_bid_match_count');

// End-to-End Metrics
const e2eBidSuccess = new Counter('e2e_bid_success');
const e2eBidLatency = new Trend('e2e_bid_latency');
const e2ePriceValidation = new Rate('e2e_price_validation');

// State tracking
const vuState = new Map();

// ==============================================================================
// TEST CONFIGURATION
// ==============================================================================
export const options = {
  scenarios: {
    hybrid_bidding: {
      executor: 'constant-vus',
      vus: 30,
      duration: '3m',
      gracefulStop: '30s',
      tags: { test_type: 'hybrid' },
    },
  },

  thresholds: {
    'http_bid_latency': ['p(95)<1200'],
    'ws_event_latency': ['p(95)<500'],
    'e2e_bid_latency': ['p(95)<2000'], // End-to-end including WS roundtrip
    'e2e_price_validation': ['rate>0.95'], // 95% of bids should show price increase
  },
};

// ==============================================================================
// CONFIGURATION
// ==============================================================================
const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const WS_URL = __ENV.WS_URL || 'ws://localhost:8080';
const API_BASE = `${BASE_URL}/api/v1`;

const AUCTIONS = [
  '00000000-0000-0000-0000-000000000001',
  '00000000-0000-0000-0000-000000000002',
  '00000000-0000-0000-0000-000000000003',
  '00000000-0000-0000-0000-000000000004',
  '00000000-0000-0000-0000-000000000005',
];

// Firebase ID tokens are loaded from tests/load/tokens/token_*.txt files via token_loader.js

const AUCTION_BASE_BIDS = {
  [AUCTIONS[0]]: 1000,
  [AUCTIONS[1]]: 500,
  [AUCTIONS[2]]: 2000,
  [AUCTIONS[3]]: 1500,
  [AUCTIONS[4]]: 800,
};

const BID_INCREMENT = 50;
const COOLDOWN_MS = 600;

// ==============================================================================
// VU STATE MANAGEMENT
// ==============================================================================

function getVUState(vuId) {
  if (!vuState.has(vuId)) {
    vuState.set(vuId, {
      auctionId: null,
      wsConnected: false,
      wsSocket: null,
      lastPrice: 0,
      currentBidAmount: 0,
      pendingBid: null, // Track bid waiting for confirmation
      eventQueue: [], // Queue events while waiting for confirmation
      bidCounter: 0,
      matchCount: 0,
      authToken: null,
    });
  }
  return vuState.get(vuId);
}

function getAuctionId(vuId) {
  return AUCTIONS[vuId % AUCTIONS.length];
}

function getAuthToken(vuId) {
  return getTokenForVu(vuId);
}

function calculateBidAmount(auctionId, vuId, bidCounter) {
  const baseBid = AUCTION_BASE_BIDS[auctionId] || 1000;
  return baseBid + (bidCounter * BID_INCREMENT) + (vuId * 10);
}

// ==============================================================================
// WEBSOCKET MESSAGE HANDLER
// ==============================================================================

function handleWSMessage(message, state, vuId) {
  try {
    const event = typeof message === 'string' ? JSON.parse(message) : message;
    const receivedTime = Date.now();

    wsEventReceived.add(1);

    // Track event latency
    if (event.timestamp) {
      const latency = receivedTime - event.timestamp;
      if (latency >= 0 && latency < 10000) {
        wsEventLatency.add(latency);
      }
    }

    // Handle bid events
    if (event.type === 'bid_placed' || event.type === 'bid_accepted') {
      wsBidEventReceived.add(1);

      const newPrice = event.amount || event.price || event.current_price;
      const oldPrice = state.lastPrice;

      // Update price
      if (newPrice && newPrice > oldPrice) {
        state.lastPrice = newPrice;

        // Validate price increased
        e2ePriceValidation.add(1);

        // Check if this matches our pending bid
        if (state.pendingBid && state.pendingBid.amount === newPrice) {
          const e2eLatency = receivedTime - state.pendingBid.sentTime;
          e2eBidLatency.add(e2eLatency);
          e2eBidSuccess.add(1);
          wsBidMatchCount.add(1);

          state.matchCount++;
          state.pendingBid = null;

          if (state.bidCounter <= 3 || state.matchCount <= 3) {
            console.log(`[VU${vuId}] ✅ BID->EVENT MATCH! Amount: ${newPrice}, E2E: ${e2eLatency}ms`);
          }
        } else {
          // Bid from another user
          if (state.bidCounter <= 5) {
            console.log(`[VU${vuId}] 📢 Other's bid: ${newPrice} (was ${oldPrice})`);
          }
        }
      }
    }

    // Handle outbid
    if (event.type === 'outbid') {
      console.log(`[VU${vuId}] 😢 OUTBID! New winner bid: ${event.amount}`);
    }

    // Handle extension
    if (event.type === 'auction_extended' || event.type === 'extension_triggered') {
      console.log(`[VU${vuId}] ⏱️  AUCTION EXTENDED!`);
    }

  } catch (e) {
    console.error(`[VU${vuId}] Failed to parse WS message: ${e.message}`);
  }
}

// ==============================================================================
// MAIN TEST FUNCTION
// ==============================================================================

export default function () {
  const vuId = __VU;
  const state = getVUState(vuId);

  // Initialize on first iteration
  if (!state.auctionId) {
    state.auctionId = getAuctionId(vuId);
    state.authToken = getAuthToken(vuId);
    state.currentBidAmount = calculateBidAmount(state.auctionId, vuId, 0);
    state.lastPrice = AUCTION_BASE_BIDS[state.auctionId] || 1000;

    // Connect WebSocket first
    connectWebSocket(vuId, state);

    // Wait for connection to establish
    sleep(1);
  }

  // Skip if WebSocket not connected
  if (!state.wsConnected) {
    sleep(1);
    return;
  }

  // Send HTTP bid
  const bidAmount = calculateBidAmount(state.auctionId, vuId, state.bidCounter);
  const bidSentTime = Date.now();

  const url = `${API_BASE}/auctions/${state.auctionId}/bid`;
  const payload = JSON.stringify({ amount: bidAmount });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${state.authToken}`,
    },
    tags: {
      test_type: 'hybrid',
      user_id: `hybrid-${vuId}`,
      auction_id: state.auctionId,
    },
  };

  const res = http.post(url, payload, params);

  httpBidSent.add(1);
  httpBidLatency.add(res.timings.duration || 0);

  // Track pending bid for event matching
  if (res.status === 201 || res.status === 200) {
    httpBidAccepted.add(1);
    state.pendingBid = {
      amount: bidAmount,
      sentTime: bidSentTime,
    };
    state.bidCounter++;
    state.currentBidAmount = bidAmount;

    if (state.bidCounter <= 5) {
      console.log(`[VU${vuId}] 💰 Bid sent: ${bidAmount}, Status: ${res.status}`);
    }
  } else if (res.status === 429) {
    httpBid429.add(1);

    if (state.bidCounter <= 3) {
      console.log(`[VU${vuId}] ⏸️  Rate limited (429), waiting...`);
    }

    // Back off on rate limit
    sleep(2);
    return;
  } else {
    httpBidError.add(1);
    e2ePriceValidation.add(0);

    if (state.bidCounter <= 3) {
      console.log(`[VU${vuId}] ❌ Bid failed: ${res.status} - ${res.body?.substring(0, 100)}`);
    }
  }

  // Wait for event confirmation (with timeout)
  const maxWaitTime = 2000; // 2 seconds max
  const checkInterval = 100;
  let waited = 0;
  let eventReceived = false;

  while (waited < maxWaitTime) {
    if (state.pendingBid === null) {
      // Event received and matched
      eventReceived = true;
      break;
    }
    sleep(checkInterval / 1000);
    waited += checkInterval;
  }

  if (!eventReceived && state.pendingBid) {
    // Timeout waiting for event
    if (state.bidCounter <= 5) {
      console.log(`[VU${vuId}] ⏱️  Event timeout for bid ${state.pendingBid.amount}`);
    }
    state.pendingBid = null;
    e2ePriceValidation.add(0);
  }

  // Respect cooldown
  sleep(Math.max(0.6, COOLDOWN_MS / 1000));
}

// ==============================================================================
// WEBSOCKET CONNECTION
// ==============================================================================

function connectWebSocket(vuId, state) {
  const url = `${WS_URL}/ws/auctions/${state.auctionId}`;
  const params = {
    tags: {
      test_type: 'hybrid',
      user_id: `hybrid-${vuId}`,
      auction_id: state.auctionId,
    },
  };

  // Note: In k6, WS connection is established asynchronously
  // We'll track connection state via the message handler
  state.wsConnected = false;

  // Store reference for message handler
  const res = ws.connect(url, params, function (socket) {
    state.wsSocket = socket;
    state.wsConnected = true;

    socket.on('message', (data) => {
      const parts = data.toString().split('\n').filter(p => p.trim());
      parts.forEach((part) => handleWSMessage(part, state, vuId));
    });

    socket.on('error', (e) => {
      console.error(`[VU${vuId}] WS error: ${e}`);
      state.wsConnected = false;
    });

    socket.on('close', () => {
      state.wsConnected = false;
      console.log(`[VU${vuId}] WS closed. Total bids: ${state.bidCounter}, Matches: ${state.matchCount}`);
    });

    // Keep alive for test duration
    socket.setTimeout(() => {
      socket.close();
    }, 200000); // 200 seconds
  });

  const handshakeOk = check(res, {
    'WS handshake successful': (r) => r && r.status === 101,
  });

  if (!handshakeOk) {
    console.error(`[VU${vuId}] WS handshake failed! Status: ${res?.status}`);
    state.wsConnected = false;
  }
}

// ==============================================================================
// SUMMARY HANDLER
// ==============================================================================

export function handleSummary(data) {
  return {
    'stdout': textSummary(data),
    'hybrid_results.json': JSON.stringify(data, null, 2),
    'hybrid_summary.html': htmlReport(data),
  };
}

function textSummary(data) {
  let out = '\n╔══════════════════════════════════════════════════════════════╗\n';
  out += '║         HYBRID SCENARIO TEST RESULTS                         ║\n';
  out += '║    (HTTP Bid + WebSocket Event Validation)                  ║\n';
  out += '╚══════════════════════════════════════════════════════════════╝\n\n';

  // HTTP Metrics
  out += `🌐 HTTP BIDDING:\n`;
  out += `  Bids Sent: ${data.metrics.http_bid_sent?.values.count || 0}\n`;
  out += `  Bids Accepted: ${data.metrics.http_bid_accepted?.values.count || 0}\n`;
  out += `  Rate Limited (429): ${data.metrics.http_bid_429?.values.count || 0}\n`;
  out += `  Errors: ${data.metrics.http_bid_error?.values.count || 0}\n`;

  if (data.metrics.http_bid_latency) {
    const lat = data.metrics.http_bid_latency.values;
    out += `  Latency p95: ${lat['p(95)']?.toFixed(2) || 'N/A'} ms ${lat['p(95)'] < 1200 ? '✅' : '❌'}\n`;
  }

  // WebSocket Metrics
  out += `\n📡 WEBSOCKET EVENTS:\n`;
  out += `  Events Received: ${data.metrics.ws_event_received?.values.count || 0}\n`;
  out += `  Bid Events: ${data.metrics.ws_bid_event_received?.values.count || 0}\n`;

  if (data.metrics.ws_event_latency) {
    const lat = data.metrics.ws_event_latency.values;
    out += `  Event Latency p95: ${lat['p(95)']?.toFixed(2) || 'N/A'} ms ${lat['p(95)'] < 500 ? '✅' : '❌'}\n`;
  }

  // End-to-End Metrics
  out += `\n🔄 END-TO-END VALIDATION:\n`;
  const e2eSuccess = data.metrics.e2e_bid_success?.values.count || 0;
  const bidMatch = data.metrics.ws_bid_match_count?.values.count || 0;
  out += `  Bid->Event Matches: ${bidMatch}\n`;
  out += `  E2E Success: ${e2eSuccess}\n`;

  if (data.metrics.e2e_bid_latency) {
    const lat = data.metrics.e2e_bid_latency.values;
    out += `  E2E Latency p95: ${lat['p(95)']?.toFixed(2) || 'N/A'} ms ${lat['p(95)'] < 2000 ? '✅' : '❌'}\n`;
  }

  if (data.metrics.e2e_price_validation) {
    const rate = data.metrics.e2e_price_validation.values.rate * 100;
    out += `  Price Validation Rate: ${rate.toFixed(2)}% ${rate >= 95 ? '✅' : '❌'}\n`;
  }

  // Calculate match rate
  const bidsSent = data.metrics.http_bid_sent?.values.count || 0;
  if (bidsSent > 0) {
    const matchRate = ((bidMatch / bidsSent) * 100).toFixed(2);
    out += `  Event Match Rate: ${matchRate}%\n`;
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
  const httpLat = data.metrics.http_bid_latency?.values || {};
  const wsLat = data.metrics.ws_event_latency?.values || {};
  const e2eLat = data.metrics.e2e_bid_latency?.values || {};
  const validationRate = (data.metrics.e2e_price_validation?.values.rate || 0) * 100;

  const bidsSent = data.metrics.http_bid_sent?.values.count || 0;
  const bidsAccepted = data.metrics.http_bid_accepted?.values.count || 0;
  const bidMatches = data.metrics.ws_bid_match_count?.values.count || 0;
  const rate429 = data.metrics.http_bid_429?.values.count || 0;

  return `
<!DOCTYPE html>
<html>
<head>
  <title>Hybrid Scenario Test Results</title>
  <style>
    * { box-sizing: border-box; }
    body { font-family: 'Segoe UI', sans-serif; margin: 0; padding: 20px; background: #f5f7fa; }
    .container { max-width: 1200px; margin: 0 auto; }
    h1 { color: #1a202c; border-bottom: 4px solid #805ad5; padding-bottom: 10px; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; margin: 20px 0; }
    .card { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
    .card-value { font-size: 2rem; font-weight: bold; color: #2d3748; }
    .card-label { font-size: 0.85rem; color: #718096; text-transform: uppercase; }
    .pass { color: #48bb78; }
    .fail { color: #f56565; }
    table { width: 100%; border-collapse: collapse; background: white; border-radius: 8px; overflow: hidden; }
    th, td { padding: 12px 15px; text-align: left; border-bottom: 1px solid #e2e8f0; }
    th { background: #805ad5; color: white; }
    .flow { background: white; padding: 20px; border-radius: 8px; margin: 20px 0; }
    .flow-step { display: inline-block; padding: 10px 20px; background: #e2e8f0; border-radius: 20px; margin: 5px; }
  </style>
</head>
<body>
  <div class="container">
    <h1>🔄 Hybrid Scenario Test Results</h1>
    <p style="color: #718096;">HTTP Bid + WebSocket Event Validation</p>

    <div class="flow">
      <span class="flow-step">1️⃣ HTTP POST Bid</span>
      →
      <span class="flow-step">2️⃣ WS Subscribe</span>
      →
      <span class="flow-step">3️⃣ Event Confirmation</span>
      →
      <span class="flow-step">4️⃣ Price Validation</span>
    </div>

    <div class="grid">
      <div class="card">
        <div class="card-value">${bidsSent}</div>
        <div class="card-label">HTTP Bids Sent</div>
      </div>
      <div class="card">
        <div class="card-value">${bidsAccepted}</div>
        <div class="card-label">Bids Accepted</div>
      </div>
      <div class="card">
        <div class="card-value">${bidMatches}</div>
        <div class="card-label">Bid->Event Matches</div>
      </div>
      <div class="card">
        <div class="card-value">${validationRate.toFixed(1)}%</div>
        <div class="card-label">Price Validation ${validationRate >= 95 ? '✅' : '❌'}</div>
      </div>
    </div>

    <h2>🌐 HTTP Bidding Metrics</h2>
    <table>
      <tr><th>Metric</th><th>Value</th><th>Status</th></tr>
      <tr><td>Bids Sent</td><td>${bidsSent}</td><td>-</td></tr>
      <tr><td>Bids Accepted</td><td>${bidsAccepted}</td><td>✅</td></tr>
      <tr><td>Rate Limited (429)</td><td>${rate429}</td><td>${rate429 > 0 ? '✅ Protection' : '⚠️ None'}</td></tr>
      <tr><td>HTTP Latency p95</td><td>${httpLat['p(95)']?.toFixed(0) || 'N/A'} ms</td><td>${httpLat['p(95)'] < 1200 ? '✅' : '❌'}</td></tr>
    </table>

    <h2>📡 WebSocket Event Metrics</h2>
    <table>
      <tr><th>Metric</th><th>Value</th><th>Status</th></tr>
      <tr><td>Events Received</td><td>${data.metrics.ws_event_received?.values.count || 0}</td><td>-</td></tr>
      <tr><td>Bid Events</td><td>${data.metrics.ws_bid_event_received?.values.count || 0}</td><td>-</td></tr>
      <tr><td>Event Latency p95</td><td>${wsLat['p(95)']?.toFixed(0) || 'N/A'} ms</td><td>${wsLat['p(95)'] < 500 ? '✅' : '❌'}</td></tr>
    </table>

    <h2>🔄 End-to-End Metrics</h2>
    <table>
      <tr><th>Metric</th><th>Value</th><th>Threshold</th></tr>
      <tr><td>E2E Latency p95</td><td>${e2eLat['p(95)']?.toFixed(0) || 'N/A'} ms</td><td>< 2000ms ${e2eLat['p(95)'] < 2000 ? '✅' : '❌'}</td></tr>
      <tr><td>Price Validation Rate</td><td>${validationRate.toFixed(2)}%</td><td>> 95% ${validationRate >= 95 ? '✅' : '❌'}</td></tr>
      <tr><td>Event Match Rate</td><td>${bidsSent > 0 ? ((bidMatches/bidsSent)*100).toFixed(2) : 0}%</td><td>Target: > 80%</td></tr>
    </table>

    <p style="color: #718096; margin-top: 40px; text-align: center;">
      Generated: ${new Date().toISOString()} | Hybrid Scenario Test v1.0
    </p>
  </div>
</body>
</html>
  `;
}
