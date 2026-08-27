// =============================================================================
// QUICK SMOKE TEST - 1 USER, MINIMAL BIDS
// =============================================================================
// Purpose: Validate auth pipeline and bid placement works
// Usage: node backend/tests/load/auction/scripts/quick_test.js
// =============================================================================

const http = require('http');

const CONFIG = {
    BASE_URL: 'http://localhost:8080',
    AUCTION_ID: '00000000-0000-0000-0000-000000000001',
    AUTH_TOKEN: process.env.AUTH_TOKEN || '',
    NUM_BIDS: 5,
    BID_AMOUNT_START: 130000,
    BID_INCREMENT: 10000
};

// Read token from file if not in env
if (!CONFIG.AUTH_TOKEN) {
    try {
        const fs = require('fs');
        // Try to read first token from the new tokens directory
        const tokenPath = 'tests/load/tokens/token_1.txt';
        CONFIG.AUTH_TOKEN = fs.readFileSync(tokenPath, 'utf8').trim();
    } catch (e) {
        console.error('❌ No auth token found. Set AUTH_TOKEN env var or create tests/load/tokens/token_1.txt');
        console.error('   Each token file should contain a valid Firebase ID token.');
        process.exit(1);
    }
}

const metrics = {
    bids_sent: 0,
    bids_received: [],
    errors: [],
    status_codes: {},
    start_time: Date.now()
};

function placeBid(amount) {
    return new Promise((resolve) => {
        const data = JSON.stringify({ amount });

        const options = {
            hostname: 'localhost',
            port: 8080,
            path: `/api/v1/auctions/${CONFIG.AUCTION_ID}/bid`,
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'Authorization': `Bearer ${CONFIG.AUTH_TOKEN}`,
                'Content-Length': data.length
            }
        };

        const req = http.request(options, (res) => {
            let body = '';

            res.on('data', (chunk) => {
                body += chunk;
            });

            res.on('end', () => {
                const statusCode = res.statusCode;
                metrics.status_codes[statusCode] = (metrics.status_codes[statusCode] || 0) + 1;

                if (statusCode === 201 || statusCode === 200) {
                    try {
                        const response = JSON.parse(body);
                        metrics.bids_received.push({
                            amount,
                            status: statusCode,
                            bidder_id: response.data?.bid?.bidder_id,
                            is_winning: response.data?.bid?.is_winning,
                            response_time: Date.now() - metrics.start_time
                        });
                    } catch (e) {
                        metrics.errors.push({ amount, status: statusCode, error: 'Parse error', body: body.substring(0, 100) });
                    }
                } else {
                    try {
                        const response = JSON.parse(body);
                        metrics.errors.push({
                            amount,
                            status: statusCode,
                            error: response.error?.message || response.message || 'Unknown error'
                        });
                    } catch (e) {
                        metrics.errors.push({ amount, status: statusCode, error: 'Parse error', body: body.substring(0, 100) });
                    }
                }
                resolve();
            });
        });

        req.on('error', (error) => {
            metrics.errors.push({ amount, error: error.message });
            resolve();
        });

        req.write(data);
        req.end();
    });
}

async function runSmokeTest() {
    console.log('============================================================================');
    console.log('SMOKE TEST - 1 USER');
    console.log('============================================================================');
    console.log(`Auction ID: ${CONFIG.AUCTION_ID}`);
    console.log(`Bids to send: ${CONFIG.NUM_BIDS}`);
    console.log(`Auth Token: ${CONFIG.AUTH_TOKEN.substring(0, 50)}...`);
    console.log('============================================================================\n');

    // First verify auction is accessible
    console.log('[1/2] Verifying auction state...');
    try {
        const checkRes = await new Promise((resolve) => {
            http.get(`http://localhost:8080/api/v1/auctions/${CONFIG.AUCTION_ID}`, (res) => {
                let body = '';
                res.on('data', (chunk) => { body += chunk; });
                res.on('end', () => {
                    resolve({ status: res.statusCode, body });
                });
            }).on('error', (e) => resolve({ error: e.message }));
        });

        if (checkRes.error || checkRes.status !== 200) {
            console.error(`❌ Auction check failed: ${checkRes.error || 'HTTP ' + checkRes.status}`);
            process.exit(1);
        }

        const auctionData = JSON.parse(checkRes.body);
        console.log(`  ✅ Auction: ${auctionData.data.title}`);
        console.log(`  ✅ Status: ${auctionData.data.status}`);
        console.log(`  ✅ Can Bid: ${auctionData.data.can_bid}`);
        console.log(`  ✅ Min Bid: ${auctionData.data.minimum_bid}`);
        console.log(`  ✅ Current Highest: ${auctionData.data.current_highest_bid}\n`);

        if (!auctionData.data.can_bid) {
            console.error('❌ Auction is not accepting bids!');
            process.exit(1);
        }

        // Update starting bid based on current minimum
        CONFIG.BID_AMOUNT_START = auctionData.data.minimum_bid;
    } catch (e) {
        console.error(`❌ Error checking auction: ${e.message}`);
        process.exit(1);
    }

    // Second send bids (auth is verified by bid response)
    console.log('[2/2] Sending bids...');
    const bids = [];
    for (let i = 0; i < CONFIG.NUM_BIDS; i++) {
        bids.push(CONFIG.BID_AMOUNT_START + (i * CONFIG.BID_INCREMENT));
    }

    for (const amount of bids) {
        metrics.bids_sent++;
        await placeBid(amount);
        // Small delay between bids
        await new Promise(r => setTimeout(r, 100));
        process.stdout.write(`  Bid ${metrics.bids_sent}/${CONFIG.NUM_BIDS} (${amount})...\r`);
    }

    console.log('\n');

    // Print results
    printResults();
}

function printResults() {
    const duration = Date.now() - metrics.start_time;

    console.log('============================================================================');
    console.log('SMOKE TEST RESULTS');
    console.log('============================================================================');
    console.log(`Duration: ${duration}ms`);
    console.log(`\n📊 BIDS SENT: ${metrics.bids_sent}`);
    console.log(`✅ BIDS RECEIVED: ${metrics.bids_received.length}`);
    console.log(`❌ ERRORS: ${metrics.errors.length}`);

    console.log('\n📋 STATUS CODES:');
    Object.entries(metrics.status_codes).forEach(([code, count]) => {
        console.log(`  ${code}: ${count}`);
    });

    if (metrics.bids_received.length > 0) {
        console.log('\n✅ SUCCESSFUL BIDS:');
        metrics.bids_received.forEach((bid, i) => {
            console.log(`  ${i + 1}. Amount ${bid.amount}: ${bid.is_winning ? 'WINNING' : 'OUTBID'} | bidder_id: ${bid.bidder_id} (${bid.response_time}ms)`);
        });
    }

    if (metrics.errors.length > 0) {
        console.log('\n❌ ERRORS:');
        metrics.errors.slice(0, 5).forEach((err, i) => {
            console.log(`  ${i + 1}. Amount ${err.amount}: ${err.status} - ${err.error}`);
        });
        if (metrics.errors.length > 5) {
            console.log(`  ... and ${metrics.errors.length - 5} more`);
        }
    }

    // Validation
    console.log('\n============================================================================');
    console.log('VALIDATION');
    console.log('============================================================================');

    const issues = [];

    if (metrics.bids_sent === 0) {
        issues.push('❌ No bids sent');
    } else {
        console.log('✅ Bids sent: ' + metrics.bids_sent);
    }

    if (metrics.bids_received.length === 0) {
        issues.push('❌ No bids received');
    } else {
        console.log('✅ Bids received: ' + metrics.bids_received.length);
    }

    // Verify auth pipeline worked - check for DB UUID in bidder_id
    const hasValidUUID = metrics.bids_received.some(b =>
        b.bidder_id && b.bidder_id.startsWith('10000000-')
    );
    if (hasValidUUID) {
        console.log('✅ UserLookupMiddleware: Firebase UID → DB UUID conversion works');
        console.log(`   bidder_id: ${metrics.bids_received[0].bidder_id}`);
    } else {
        issues.push('❌ UserLookupMiddleware: No DB UUID found in responses');
    }

    const hasAuthError = metrics.errors.some(e =>
        e.error?.includes('User not authenticated') ||
        e.error?.includes('not authenticated') ||
        e.status === 401
    );
    if (hasAuthError) {
        issues.push('❌ Auth errors detected');
    } else {
        console.log('✅ No auth errors');
    }

    const hasAuctionError = metrics.errors.some(e =>
        e.error?.includes('auction is not accepting bids')
    );
    if (hasAuctionError) {
        issues.push('❌ Auction not accepting bids errors');
    } else {
        console.log('✅ No auction state errors');
    }

    const has200 = metrics.status_codes['200'] > 0 || metrics.status_codes['201'] > 0;
    const has400 = metrics.status_codes['400'] > 0;
    const has429 = metrics.status_codes['429'] > 0;

    console.log('\n📋 RESPONSE PATTERNS:');
    console.log(`  200/201 Accepted: ${has200 ? '✅' : '❌'}`);
    console.log(`  400 Validation: ${has400 ? '✅' : '⚠️ '}`);
    console.log(`  429 Rate Limited: ${has429 ? '✅' : '⚠️ '}`);

    console.log('\n============================================================================');
    if (issues.length === 0 && has200) {
        console.log('✅ SMOKE TEST PASSED!');
        console.log('============================================================================\n');
        process.exit(0);
    } else {
        console.log('❌ SMOKE TEST FAILED!');
        console.log('============================================================================\n');
        process.exit(1);
    }
}

runSmokeTest().catch(err => {
    console.error('Fatal error:', err);
    process.exit(1);
});
