/// Canonical API error code constants.
///
/// Backend returns these as the `code` field inside the error envelope:
/// ```json
/// { "success": false, "error": { "code": "COMMERCE_RESTRICTED", "message": "..." } }
/// ```
///
/// Every mobile layer that branches on an error code MUST reference these
/// constants instead of raw string literals. This file is the single
/// authority for mobile error code identity.
library;

/// User's commerce activity is restricted by the backend governance layer.
///
/// HTTP 403 — backend enforced via `commercegov.IsUserRestricted(...)`.
/// This is NOT an account suspension/ban. The user can still browse and
/// use non-commerce features.
const String commerceRestricted = 'COMMERCE_RESTRICTED';

/// User's email has not been verified.
///
/// HTTP 403 — backend enforced email verification gate.
const String emailVerificationRequired = 'EMAIL_VERIFICATION_REQUIRED';

/// BNR (Buyer Not Rated) auction restriction.
///
/// HTTP 403 — user previously won an auction but did not complete payment.
const String bnrAuctionRestricted = 'BNR_AUCTION_RESTRICTED';
