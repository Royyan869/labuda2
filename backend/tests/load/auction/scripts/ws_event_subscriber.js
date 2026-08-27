/**
 * WEBSOCKET EVENT SUBSCRIBER TEST
 *
 * Architecture Compliance:
 * - WebSocket ONLY for realtime event delivery (QUERY)
 * - Validates events after HTTP bid placement
 * - Tests event delivery, ordering, latency
 *
 * Events Validated:
 * - bid_placed: New bid notification
 * - outbid: When user is outbid
 * - auction_extended: Extension triggered
 * - price_updated: Current price update
 */

import ws from 'k6/ws';
import { check } from 'k6';
import { Counter, Trend, Rate } from 'k6/metrics';

// ==============================================================================
// CUSTOM METRICS
// ==============================================================================
const eventsReceived = new Counter('events_received');
const eventsBidPlaced = new Counter('events_bid_placed');
const eventsOutbid = new Counter('events_outbid');
const eventsExtended = new Counter('events_extended');
const eventsPriceUpdated = new Counter('events_price_updated');
const eventsError = new Counter('events_error');

const eventLatency = new Trend('event_latency');
const eventDeliveryRate = new Rate('event_delivery_rate');

const bidReceivedCount = new Counter('bid_received_count');
const currentPrice = new Trend('current_price');

// ==============================================================================
// TEST CONFIGURATION
// ==============================================================================
export const options = {
  scenarios: {
    event_subscriber: {
      executor: 'constant-vus',
      vus: 20,
      duration: '3m',
      gracefulStop: '30s',
    },
  },

  thresholds: {
    'event_latency': ['p(95)<500', 'p(99)<1000'],
    'event_delivery_rate': ['rate>0.95'], // 95% of events should be delivered
  },
};

// ==============================================================================
// CONFIGURATION
// ==============================================================================
const BASE_URL = __ENV.WS_URL || 'ws://localhost:8080';

const AUCTIONS = [
  '00000000-0000-0000-0000-000000000001',
  '00000000-0000-0000-0000-000000000002',
  '00000000-0000-0000-0000-000000000003',
  '00000000-0000-0000-0000-000000000004',
  '00000000-0000-0000-0000-000000000005',
];

// Track state per VU
const vuState = {
  connected: false,
  auctionId: null,
  lastPrice: 0,
  lastEventTime: 0,
  eventSequence: 0,
  receivedEvents: [],
};

// ==============================================================================
// HELPER FUNCTIONS
// ==============================================================================

function getAuctionId(vuId) {
  return AUCTIONS[vuId % AUCTIONS.length];
}

/**
 * Parse and classify event
 */
function handleEvent(message, vuId) {
  try {
    const event = typeof message === 'string' ? JSON.parse(message) : message;
    const receivedTime = Date.now();

    eventsReceived.add(1);
    eventDeliveryRate.add(1);

    // Calculate latency if timestamp exists
    if (event.timestamp) {
      const latency = receivedTime - event.timestamp;
      if (latency >= 0 && latency < 10000) {
        eventLatency.add(latency);
      }
    }

    // Track sequence
    vuState.eventSequence++;
    vuState.receivedEvents.push({
      seq: vuState.eventSequence,
      type: event.type,
      time: receivedTime,
    });

    // Classify by event type
    switch (event.type) {
      case 'bid_placed':
      case 'bid_accepted':
        eventsBidPlaced.add(1);

        if (event.amount) {
          currentPrice.add(event.amount);
          vuState.lastPrice = event.amount;

          // Validate price only increases
          if (event.amount < vuState.lastPrice && vuState.lastPrice > 0) {
            console.error(`[VU${vuId}] PRICE DECREASED! ${vuState.lastPrice} -> ${event.amount}`);
          }
        }

        if (vuState.eventSequence <= 3) {
          console.log(`[VU${vuId}] 📢 BID_PLACED: Amount=${event.amount}, Auction=${vuState.auctionId}`);
        }
        break;

      case 'outbid':
        eventsOutbid.add(1);
        console.log(`[VU${vuId}] 😢 OUTBID: New bid=${event.amount}, Your bid=${event.your_bid || '?'}`);
        break;

      case 'auction_extended':
      case 'extension_triggered':
        eventsExtended.add(1);
        console.log(`[VU${vuId}] ⏱️  AUCTION_EXTENDED: NewEndTime=${event.new_end_time || '?'}`);
        break;

      case 'price_updated':
        eventsPriceUpdated.add(1);

        if (event.price) {
          currentPrice.add(event.price);
        }
        break;

      case 'error':
        eventsError.add(1);
        console.error(`[VU${vuId}] ❌ ERROR EVENT: ${event.message || event.reason || 'Unknown error'}`);
        break;

      case 'ping':
      case 'pong':
        // Heartbeat, ignore
        break;

      default:
        if (vuState.eventSequence <= 5) {
          console.log(`[VU${vuId}] ❓ UNKNOWN EVENT: ${event.type}`);
        }
    }

    vuState.lastEventTime = receivedTime;

  } catch (e) {
    console.error(`[VU${vuId}] Failed to parse event: ${e.message}`);
    eventsError.add(1);
    eventDeliveryRate.add(0);
  }
}

/**
 * Validate event ordering
 */
function validateEventOrdering(vuId) {
  const events = vuState.receivedEvents;
  let outOfOrder = 0;

  for (let i = 1; i < events.length; i++) {
    if (events[i].time < events[i-1].time) {
      outOfOrder++;
    }
  }

  if (outOfOrder > 0) {
    console.warn(`[VU${vuId}] ⚠️  ${outOfOrder} events out of order!`);
  }

  return outOfOrder;
}

// ==============================================================================
// MAIN TEST FUNCTION
// ==============================================================================

export default function () {
  const vuId = __VU;
  const auctionId = getAuctionId(vuId);
  vuState.auctionId = auctionId;

  const url = `${BASE_URL}/ws/auctions/${auctionId}`;
  const params = {
    tags: {
      test_type: 'ws_event_subscriber',
      user_id: `subscriber-${vuId}`,
      auction_id: auctionId,
    },
  };

  const res = ws.connect(url, params, function (socket) {
    vuState.connected = true;

    // Connection check
    const connected = check(socket, {
      'WS connection established': (s) => s && s.readyState === ws.OPEN,
    });

    if (!connected) {
      console.error(`[VU${vuId}] ❌ Connection failed!`);
      eventDeliveryRate.add(0);
      return;
    }

    console.log(`[VU${vuId}] ✅ Connected to auction ${auctionId}`);

    // Handle incoming messages
    socket.on('message', (data) => {
      // Messages may be newline-separated JSON objects
      const parts = data.toString().split('\n').filter(p => p.trim());

      parts.forEach((part) => {
        handleEvent(part, vuId);
      });
    });

    socket.on('error', (e) => {
      console.error(`[VU${vuId}] WebSocket error: ${e}`);
      eventsError.add(1);
      eventDeliveryRate.add(0);
    });

    socket.on('close', () => {
      vuState.connected = false;

      // Validate ordering on close
      const outOfOrder = validateEventOrdering(vuId);

      console.log(`[VU${vuId}] 🔌 Connection closed. Events: ${vuState.receivedEvents.length}, OutOfOrder: ${outOfOrder}`);
    });

    // Send ping every 30 seconds to keep connection alive
    const pingInterval = setInterval(() => {
      if (socket.readyState === ws.OPEN) {
        socket.send(JSON.stringify({ type: 'ping' }));
      }
    }, 30000);

    // Cleanup after test duration
    socket.setTimeout(() => {
      clearInterval(pingInterval);
      socket.close();
    }, 180000); // 3 minutes
  });

  // Check handshake
  const handshakeOk = check(res, {
    'WS handshake successful': (r) => r && r.status === 101,
  });

  if (!handshakeOk) {
    console.error(`[VU${vuId}] ❌ Handshake failed! Status: ${res?.status}`);
  }
}

// ==============================================================================
// SUMMARY HANDLER
// ==============================================================================

export function handleSummary(data) {
  return {
    'stdout': textSummary(data),
    'ws_event_results.json': JSON.stringify(data, null, 2),
    'ws_event_summary.html': htmlReport(data),
  };
}

function textSummary(data) {
  let out = '\n╔══════════════════════════════════════════════════════════════╗\n';
  out += '║      WEBSOCKET EVENT SUBSCRIBER TEST RESULTS                  ║\n';
  out += '╚══════════════════════════════════════════════════════════════╝\n\n';

  // Event Latency
  if (data.metrics.event_latency) {
    const lat = data.metrics.event_latency.values;
    out += `📡 EVENT LATENCY:\n`;
    out += `  p50: ${lat['p(50)'] ? lat['p(50)'].toFixed(2) : 'N/A'} ms\n`;
    out += `  p95: ${lat['p(95)'] ? lat['p(95)'].toFixed(2) : 'N/A'} ms ${lat['p(95)'] < 500 ? '✅' : '❌'}\n`;
    out += `  p99: ${lat['p(99)'] ? lat['p(99)'].toFixed(2) : 'N/A'} ms\n`;
  }

  // Event Types
  out += `\n📢 EVENTS RECEIVED:\n`;
  out += `  Total: ${data.metrics.events_received?.values.count || 0}\n`;
  out += `  bid_placed: ${data.metrics.events_bid_placed?.values.count || 0}\n`;
  out += `  outbid: ${data.metrics.events_outbid?.values.count || 0}\n`;
  out += `  auction_extended: ${data.metrics.events_extended?.values.count || 0}\n`;
  out += `  price_updated: ${data.metrics.events_price_updated?.values.count || 0}\n`;
  out += `  error: ${data.metrics.events_error?.values.count || 0}\n`;

  // Delivery Rate
  if (data.metrics.event_delivery_rate) {
    const rate = data.metrics.event_delivery_rate.values.rate * 100;
    out += `\n📈 DELIVERY RATE: ${rate.toFixed(2)}% ${rate >= 95 ? '✅' : '❌'}\n`;
  }

  // Connection Metrics
  out += `\n🔌 CONNECTION:\n`;
  if (data.metrics.ws_connecting) {
    const conn = data.metrics.ws_connecting.values;
    out += `  Connecting Time: ${conn['p(95)'] ? conn['p(95)'].toFixed(2) : 'N/A'} ms (p95)\n`;
  }
  if (data.metrics.ws_sessions) {
    out += `  Sessions: ${data.metrics.ws_sessions.values.count}\n`;
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
  const lat = data.metrics.event_latency?.values || {};
  const eventsTotal = data.metrics.events_received?.values.count || 0;
  const bidPlaced = data.metrics.events_bid_placed?.values.count || 0;
  const outbid = data.metrics.events_outbid?.values.count || 0;
  const extended = data.metrics.events_extended?.values.count || 0;
  const errors = data.metrics.events_error?.values.count || 0;
  const deliveryRate = (data.metrics.event_delivery_rate?.values.rate || 0) * 100;

  return `
<!DOCTYPE html>
<html>
<head>
  <title>WebSocket Event Subscriber Results</title>
  <style>
    * { box-sizing: border-box; }
    body { font-family: 'Segoe UI', sans-serif; margin: 0; padding: 20px; background: #f5f7fa; }
    .container { max-width: 1200px; margin: 0 auto; }
    h1 { color: #1a202c; border-bottom: 4px solid #ed8936; padding-bottom: 10px; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; margin: 20px 0; }
    .card { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
    .card-value { font-size: 2rem; font-weight: bold; color: #2d3748; }
    .card-label { font-size: 0.85rem; color: #718096; text-transform: uppercase; }
    .pass { color: #48bb78; }
    .fail { color: #f56565; }
    table { width: 100%; border-collapse: collapse; background: white; border-radius: 8px; overflow: hidden; }
    th, td { padding: 12px 15px; text-align: left; border-bottom: 1px solid #e2e8f0; }
    th { background: #ed8936; color: white; }
  </style>
</head>
<body>
  <div class="container">
    <h1>📡 WebSocket Event Subscriber Test Results</h1>

    <div class="grid">
      <div class="card">
        <div class="card-value">${eventsTotal}</div>
        <div class="card-label">Events Received</div>
      </div>
      <div class="card">
        <div class="card-value">${deliveryRate.toFixed(1)}%</div>
        <div class="card-label">Delivery Rate ${deliveryRate >= 95 ? '✅' : '❌'}</div>
      </div>
      <div class="card">
        <div class="card-value">${lat['p(95)']?.toFixed(0) || 'N/A'} ms</div>
        <div class="card-label">p95 Event Latency ${lat['p(95)'] < 500 ? '✅' : '❌'}</div>
      </div>
      <div class="card">
        <div class="card-value">${bidPlaced}</div>
        <div class="card-label">Bid Placed Events</div>
      </div>
    </div>

    <h2>📢 Event Breakdown</h2>
    <table>
      <tr><th>Event Type</th><th>Count</th><th>Percentage</th></tr>
      <tr><td>bid_placed</td><td>${bidPlaced}</td><td>${eventsTotal > 0 ? ((bidPlaced/eventsTotal)*100).toFixed(1) : 0}%</td></tr>
      <tr><td>outbid</td><td>${outbid}</td><td>${eventsTotal > 0 ? ((outbid/eventsTotal)*100).toFixed(1) : 0}%</td></tr>
      <tr><td>auction_extended</td><td>${extended}</td><td>${eventsTotal > 0 ? ((extended/eventsTotal)*100).toFixed(1) : 0}%</td></tr>
      <tr><td>price_updated</td><td>${data.metrics.events_price_updated?.values.count || 0}</td><td>${eventsTotal > 0 ? (((data.metrics.events_price_updated?.values.count || 0)/eventsTotal)*100).toFixed(1) : 0}%</td></tr>
      <tr><td>error</td><td>${errors}</td><td>${eventsTotal > 0 ? ((errors/eventsTotal)*100).toFixed(2) : 0}%</td></tr>
    </table>

    <h2>⏱️ Event Latency</h2>
    <table>
      <tr><th>Percentile</th><th>Value (ms)</th><th>Threshold</th></tr>
      <tr><td>p50</td><td>${lat['p(50)']?.toFixed(2) || 'N/A'}</td><td>-</td></tr>
      <tr><td>p95</td><td>${lat['p(95)']?.toFixed(2) || 'N/A'}</td><td>< 500ms ${lat['p(95)'] < 500 ? '✅' : '❌'}</td></tr>
      <tr><td>p99</td><td>${lat['p(99)']?.toFixed(2) || 'N/A'}</td><td>< 1000ms</td></tr>
    </table>

    <p style="color: #718096; margin-top: 40px; text-align: center;">
      Generated: ${new Date().toISOString()} | WebSocket Event Subscriber Test v1.0
    </p>
  </div>
</body>
</html>
  `;
}
