# Firebase Token Generation for Load Testing

This directory stores valid Firebase ID tokens for load testing.

## IMPORTANT: Use Valid Firebase Tokens Only!

**DO NOT use dummy tokens.** Firebase Auth verifies JWT signatures. Tokens must be real Firebase ID tokens with 3 segments (header.payload.signature).

## Token Format

Valid Firebase ID Token format:
```
eyJhbGciOiJSUzI1NiIsImtpZCI6I...<header>.<payload>.<signature>
```

A valid token has exactly 3 parts when split by `.`.

## How to Generate Tokens

### Method 1: From Flutter App (Recommended)

1. Add this helper function in your Flutter app during development:

```dart
import 'package:firebase_auth/firebase_auth.dart';

// Add this to a debug screen or dev tools
Future<void> printIdTokens() async {
  final user = FirebaseAuth.instance.currentUser;

  if (user != null) {
    final idToken = await user.getIdToken();
    final idTokenResult = await user.getIdTokenResult();

    print('=== FIREBASE ID TOKEN ===');
    print('Token: $idToken');
    print('Issuer: ${idTokenResult.claims['iss']}');
    print('Subject: ${idTokenResult.claims['sub']}');
    print('Email: ${idTokenResult.claims['email']}');
    print('Expired: ${idTokenResult.expirationTime}');
    print('========================');
  } else {
    print('No user logged in');
  }
}
```

2. Login with different test accounts (5+ accounts recommended)
3. Call `printIdTokens()` for each account
4. Copy each token to a file: `token_1.txt`, `token_2.txt`, etc.

### Method 2: Using Firebase Admin SDK (Node.js)

```javascript
const admin = require('firebase-admin');

// Initialize with your service account
admin.initializeApp({
  credential: admin.credential.cert('./service-account-key.json')
});

// Create custom tokens for test users
async function createTestTokens() {
  const testUsers = [
    'load-test-user-1@example.com',
    'load-test-user-2@example.com',
    'load-test-user-3@example.com',
    'load-test-user-4@example.com',
    'load-test-user-5@example.com',
  ];

  for (let i = 0; i < testUsers.length; i++) {
    const customToken = await admin.auth().createCustomToken(testUsers[i]);
    // Note: Custom tokens need to be exchanged for ID tokens on client
    console.log(`${i + 1}: ${customToken}`);
  }
}

createTestTokens();
```

### Method 3: Manual Token Extraction via Browser DevTools

1. Open your web app in browser
2. Login to Firebase Auth
3. Open DevTools → Application → Local Storage
4. Look for Firebase auth data
5. Find the `accessToken` or `idToken` field
6. Copy the token value

## Token Storage

Create files in this directory:
```
tests/load/tokens/
├── token_1.txt   (contains Firebase ID token for user 1)
├── token_2.txt   (contains Firebase ID token for user 2)
├── token_3.txt   (contains Firebase ID token for user 3)
├── token_4.txt   (contains Firebase ID token for user 4)
└── token_5.txt   (contains Firebase ID token for user 5)
...
```

**Each file should contain ONLY the token string, no newlines or spaces.**

## Token Requirements

For proper load testing, create at least:
- **5 tokens** for quick/medium tests (50 VUs)
- **10 tokens** for high tests (100 VUs)
- **50+ tokens** for stress tests (500-1000 VUs)

Tokens are distributed round-robin across VUs, so more tokens = better user simulation.

## Token Expiration

Firebase ID tokens expire in **1 hour**. For long-running tests:
- Refresh tokens before each test run
- Or use a script that auto-refreshes expired tokens

## Verification

Verify a token is valid:

```bash
# Check token has 3 segments (JWT format)
cat token_1.txt | awk -F. '{print NF}'
# Should output: 3

# Decode token (optional, for inspection)
# Using jwt.io or jwt-cli
```

## Running Tests

After placing tokens:

```bash
# Quick smoke test
node tests/load/bidding/quick_test.js

# Medium load test (50 VUs, 3 min)
k6 run tests/load/bidding/bidding_medium_test.js

# High load test (100 VUs, 5 min)
k6 run tests/load/bidding/bidding_high_test.js

# Stress test (ramp to 1000 VUs)
k6 run tests/load/bidding/bidding_stress_test.js

# Full HTTP bid test
k6 run tests/load/bidding/http_bid_test.js
```

## Troubleshooting

### 401 Unauthorized Errors

**Cause**: Invalid or expired tokens

**Solution**:
1. Verify token format has 3 segments
2. Check token hasn't expired (1 hour lifetime)
3. Ensure backend Firebase project matches token issuer

### "No Valid Tokens Found" Error

**Cause**: No token files exist or all are invalid format

**Solution**:
1. Create `token_1.txt` with valid Firebase ID token
2. Ensure file contains only the token string (no quotes, no newlines)

## Security Notes

- **NEVER commit real tokens to git**
- Add `tests/load/tokens/*.txt` to `.gitignore`
- Use test accounts only, never production user tokens
- Rotate test tokens regularly

## .gitignore

Ensure your `.gitignore` includes:
```
tests/load/tokens/*.txt
!tests/load/tokens/.gitkeep
```
