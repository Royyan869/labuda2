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

/// Payment cannot be processed in its current state.
///
/// HTTP 409 — backend `INVALID_PAYMENT_STATUS` (`paymentrepo.ErrInvalidStatusTransition`).
/// The payment domain's only payment-specific rejection code.
const String invalidPaymentStatus = 'INVALID_PAYMENT_STATUS';

/// A payment reference is required.
///
/// HTTP 400 — backend `REFERENCE_REQUIRED` (`paymentrepo.ErrReferenceIDRequired`).
const String referenceRequired = 'REFERENCE_REQUIRED';

// ============================================================================
// TRANSPORT-LEVEL CODES
//
// These codes are NOT backend `code` values — no HTTP response ever carried
// them. They are produced by the mobile API layer itself, one per
// `DioExceptionType`, and they are the ONLY machine-readable identity of a
// request that failed to produce a usable HTTP envelope. Consumers must never
// substring-match the accompanying human message to tell these apart.
// ============================================================================

/// The device could not complete the socket/handshake to the backend host.
///
/// `DioExceptionType.connectionError`. Distinct from the device having no
/// network at all — see [backendUnreachable] copy in the error interceptor.
const String backendUnreachable = 'BACKEND_UNREACHABLE';

/// The connection was established but stalled past the configured timeout.
///
/// `DioExceptionType.connectionTimeout` / `sendTimeout` / `receiveTimeout` /
/// `transformTimeout`.
const String requestTimeout = 'TIMEOUT';

/// The transport reported a plain network failure — `DioExceptionType.unknown`
/// carrying a `SocketException`, i.e. the request never reached the backend.
const String networkError = 'NETWORK_ERROR';

/// TLS handshake failed — `DioExceptionType.badCertificate`.
const String sslError = 'SSL_ERROR';

/// The request was cancelled by the caller — `DioExceptionType.cancel`.
/// Not a failure to show the user; callers abort deliberately.
const String requestCancelled = 'CANCELLED';

/// A transport failure the API layer could not classify further.
///
/// Honest "we do not know" — the previous behaviour of inventing a plausible
/// failure kind from message text is exactly what these constants replace.
const String unknownError = 'UNKNOWN_ERROR';

/// Whether [code] is one of the transport-level codes above.
///
/// This is the single mobile authority for "the failure happened before any
/// HTTP envelope existed". Callers that need to react to transport failure
/// (retryable? show a connectivity message? degrade instead of failing?) must
/// use this predicate on `Result.errorCode` — never a message-text match.
///
/// [unknownError] is deliberately NOT included: it means "the API layer could
/// not classify this", which is not the same claim as "this was a transport
/// failure". Callers that treat it as transport are guessing.
bool isTransportFailureCode(String? code) {
  switch (code) {
    case backendUnreachable:
    case requestTimeout:
    case networkError:
    case sslError:
    case requestCancelled:
      return true;
  }
  return false;
}

/// Market authority required.
///
/// HTTP 403 — backend enforced active seller subscription / market authority
/// gate. This is the mobile canonical identity for the backend error code
/// `MARKET_AUTHORITY_REQUIRED` produced by `response.MarketAuthorityRequired(...)`
/// (HTTP 403 + code `MARKET_AUTHORITY_REQUIRED`).
///
/// Distinct from generic `FORBIDDEN` and from `COMMERCE_RESTRICTED`.
const String marketAuthorityRequired = 'MARKET_AUTHORITY_REQUIRED';
