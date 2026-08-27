#!/usr/bin/env node
/**
 * TOKEN VERIFICATION SCRIPT
 *
 * Verifies that Firebase ID tokens in tests/load/tokens/ are valid format.
 * Run this before load tests to ensure tokens are properly configured.
 */

const fs = require('fs');
const path = require('path');

const TOKEN_DIR = path.join(__dirname, 'tokens');
const MAX_TOKENS = 100;

function verifyTokenFormat(token) {
  // JWT should have 3 segments separated by dots
  const parts = token.split('.');

  if (parts.length !== 3) {
    return { valid: false, reason: `Invalid JWT format (has ${parts.length} segments, expected 3)` };
  }

  // Each segment should be base64url encoded (non-empty after removing padding)
  for (let i = 0; i < parts.length; i++) {
    if (parts[i].length === 0) {
      return { valid: false, reason: `Segment ${i + 1} is empty` };
    }
  }

  return { valid: true };
}

function decodeJWTPayload(token) {
  try {
    const parts = token.split('.');
    const payload = parts[1];
    // Add padding if needed
    const padded = payload + '='.repeat((4 - payload.length % 4) % 4);
    const decoded = Buffer.from(padded, 'base64').toString('utf-8');
    return JSON.parse(decoded);
  } catch (e) {
    return null;
  }
}

function isTokenExpired(payload) {
  if (!payload || !payload.exp) return false;
  const now = Math.floor(Date.now() / 1000);
  return payload.exp < now;
}

function main() {
  console.log('╔════════════════════════════════════════════════════════════╗');
  console.log('║         FIREBASE TOKEN VERIFICATION FOR LOAD TESTS         ║');
  console.log('╚════════════════════════════════════════════════════════════╝\n');

  const results = {
    total: 0,
    valid: 0,
    invalid: 0,
    expired: 0,
    tokens: [],
  };

  // Scan for token files
  for (let i = 1; i <= MAX_TOKENS; i++) {
    const tokenFile = path.join(TOKEN_DIR, `token_${i}.txt`);

    if (!fs.existsSync(tokenFile)) {
      break; // Stop at first missing file
    }

    results.total++;

    try {
      const token = fs.readFileSync(tokenFile, 'utf-8').trim();

      if (!token) {
        results.invalid++;
        results.tokens.push({
          file: `token_${i}.txt`,
          valid: false,
          reason: 'Empty file',
        });
        continue;
      }

      // Verify format
      const formatCheck = verifyTokenFormat(token);

      if (!formatCheck.valid) {
        results.invalid++;
        results.tokens.push({
          file: `token_${i}.txt`,
          valid: false,
          reason: formatCheck.reason,
        });
        continue;
      }

      // Decode and check expiration
      const payload = decodeJWTPayload(token);
      const expired = payload ? isTokenExpired(payload) : false;

      if (expired) {
        results.expired++;
        results.tokens.push({
          file: `token_${i}.txt`,
          valid: false,
          reason: 'Token expired',
          exp: new Date((payload.exp * 1000)).toISOString(),
        });
        continue;
      }

      results.valid++;
      results.tokens.push({
        file: `token_${i}.txt`,
        valid: true,
        email: payload?.email || 'N/A',
        exp: payload?.exp ? new Date((payload.exp * 1000)).toISOString() : 'N/A',
      });

    } catch (e) {
      results.invalid++;
      results.tokens.push({
        file: `token_${i}.txt`,
        valid: false,
        reason: `Read error: ${e.message}`,
      });
    }
  }

  // Print results
  console.log(`📂 Token Directory: ${TOKEN_DIR}`);
  console.log(`📊 Token Files Found: ${results.total}\n`);

  if (results.total === 0) {
    console.log('❌ NO TOKEN FILES FOUND!\n');
    console.log('Please create token files:');
    console.log('  1. Login to your Flutter app');
    console.log('  2. Extract Firebase ID token using getIdToken()');
    console.log('  3. Save to tests/load/tokens/token_1.txt, token_2.txt, etc.\n');
    console.log('See tests/load/tokens/README.md for detailed instructions.\n');
    process.exit(1);
  }

  console.log('─────────────────────────────────────────────────────────────');
  console.log('SUMMARY');
  console.log('─────────────────────────────────────────────────────────────\n');

  console.log(`  Valid Tokens:     ${results.valid} ✅`);
  console.log(`  Invalid Tokens:   ${results.invalid} ${results.invalid === 0 ? '✅' : '❌'}`);
  console.log(`  Expired Tokens:   ${results.expired} ${results.expired === 0 ? '✅' : '⚠️'}\n`);

  if (results.tokens.length > 0 && results.tokens.length <= 10) {
    console.log('─────────────────────────────────────────────────────────────');
    console.log('TOKEN DETAILS');
    console.log('─────────────────────────────────────────────────────────────\n');

    results.tokens.forEach((t, i) => {
      const status = t.valid ? '✅ VALID' : '❌ INVALID';
      const detail = t.valid ? `(${t.email})` : t.reason;
      console.log(`  ${i + 1}. ${t.file}: ${status} ${detail}`);
    });
    console.log('');
  }

  // Recommendations
  console.log('─────────────────────────────────────────────────────────────');
  console.log('RECOMMENDATIONS');
  console.log('─────────────────────────────────────────────────────────────\n');

  if (results.valid === 0) {
    console.log('❌ CRITICAL: No valid tokens available!');
    console.log('   Load tests will FAIL with 401 errors.\n');
    console.log('   Action required:');
    console.log('   1. Create test Firebase accounts');
    console.log('   2. Extract ID tokens from authenticated sessions');
    console.log('   3. Save to tests/load/tokens/token_*.txt files\n');
    process.exit(1);
  }

  if (results.expired > 0) {
    console.log(`⚠️  WARNING: ${results.expired} token(s) have expired.`);
    console.log('   Refresh expired tokens before running load tests.\n');
  }

  if (results.invalid > 0) {
    console.log(`⚠️  WARNING: ${results.invalid} token(s) are invalid format.`);
    console.log('   Ensure tokens are valid Firebase JWT (3 segments).\n');
  }

  // Load test recommendations
  const maxVUs = {
    quick: 1,
    medium: 50,
    high: 100,
    stress: 1000,
  };

  console.log('📊 Token Count vs Load Test Requirements:\n');

  for (const [test, vus] of Object.entries(maxVUs)) {
    const recommended = Math.min(vus, results.valid);
    const status = results.valid >= vus ? '✅' : results.valid >= recommended ? '⚠️' : '❌';
    console.log(`  ${status} ${test.padEnd(10)} ${vus.toString().padStart(4)} VUs  →  Need ${Math.min(vus, results.valid)} tokens`);
  }
  console.log('');

  if (results.valid >= 50) {
    console.log('✅ Token count is sufficient for all load tests!\n');
    process.exit(0);
  } else if (results.valid >= 10) {
    console.log('⚠️  Token count is sufficient for quick/medium tests only.');
    console.log('   For high/stress tests, create more tokens.\n');
    process.exit(0);
  } else {
    console.log('⚠️  Limited tokens available. Consider creating more for better test coverage.\n');
    process.exit(0);
  }
}

main();
