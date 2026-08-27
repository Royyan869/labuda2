/**
 * FIREBASE TOKEN LOADER FOR K6 LOAD TESTS
 *
 * This module loads valid Firebase ID tokens from files for authentication.
 *
 * PREREQUISITES:
 * 1. Create valid Firebase test accounts (5+ accounts recommended)
 * 2. Login via app and extract idToken
 * 3. Save tokens to tests/load/tokens/token_1.txt, token_2.txt, etc.
 *
 * TOKEN FORMAT:
 * - Each file should contain ONLY the idToken string (no newlines, spaces, quotes)
 * - Must be valid Firebase JWT (3 segments: header.payload.signature)
 *
 * HOW TO EXTRACT TOKEN FROM APP:
 * 1. Login to Firebase Auth in your Flutter app
 * 2. Get idToken: await FirebaseAuth.instance.currentUser?.getIdToken()
 * 3. Copy the token value (starts with "eyJhbGci...")
 * 4. Save to token_n.txt file
 */

import { SharedArray } from 'k6/data';

// ==============================================================================
// CONFIGURATION
// ==============================================================================

const TOKEN_DIR = __ENV.TOKEN_DIR || '../../../../tests/load/tokens/';
const TOKEN_COUNT = parseInt(__ENV.TOKEN_COUNT || '50');

// ==============================================================================
// TOKEN LOADING
// ==============================================================================

/**
 * Load all available Firebase ID tokens from token files
 * Uses SharedArray for efficient sharing across VUs
 */
const firebaseTokens = new SharedArray('firebase-tokens', function () {
  const tokens = [];
  const maxTokens = 100; // Safety limit

  for (let i = 1; i <= maxTokens; i++) {
    try {
      const filename = `${TOKEN_DIR}token_${i}.txt`;
      const data = open(filename);
      const token = data.trim();

      // Validate JWT format (3 segments)
      if (token && token.split('.').length === 3) {
        tokens.push(token);
      } else if (token) {
        console.warn(`Invalid JWT format in ${filename} (expected 3 segments)`);
      }
    } catch (e) {
      // File doesn't exist, stop loading
      break;
    }
  }

  if (tokens.length === 0) {
    throw new Error(
      'NO VALID TOKENS FOUND!\n' +
      'Please create token files in tests/load/tokens/ directory.\n' +
      'Format: token_1.txt, token_2.txt, etc.\n' +
      'Each file should contain a valid Firebase ID token.'
    );
  }

  console.log(`Loaded ${tokens.length} Firebase tokens from ${TOKEN_DIR}`);
  return tokens;
});

// ==============================================================================
// PUBLIC API
// ==============================================================================

/**
 * Get total number of available tokens
 */
export function getTokenCount() {
  return firebaseTokens.length;
}

/**
 * Get auth token for a specific VU using round-robin
 * @param {number} vuId - Virtual User ID
 * @returns {string} Firebase ID token
 */
export function getTokenForVu(vuId) {
  const index = vuId % firebaseTokens.length;
  return firebaseTokens[index];
}

/**
 * Get auth token with user classification
 * @param {number} vuId - Virtual User ID
 * @param {number} totalVUs - Total number of VUs
 * @returns {object} { token: string, tier: 'sufficient'|'insufficient' }
 */
export function getTokenWithTier(vuId, totalVUs) {
  const sufficientThreshold = Math.floor(totalVUs * 0.7);
  const tier = vuId <= sufficientThreshold ? 'sufficient' : 'insufficient';

  return {
    token: getTokenForVu(vuId),
    tier: tier,
    userId: `load-test-user-${vuId}`,
  };
}

/**
 * Get all tokens (for debugging)
 */
export function getAllTokens() {
  return firebaseTokens;
}

// Export tokens array for direct access
export { firebaseTokens };
