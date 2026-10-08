/// Checkout Feature Module
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// ARCHITECTURAL ROLE
/// ═══════════════════════════════════════════════════════════════════════════════
/// Checkout is an ORCHESTRATION LAYER, NOT a backend domain aggregate.
///
/// It coordinates:
/// - Pricing preview (via preview API → pricing token)
/// - Token validation (ensures pricing snapshot integrity)
/// - Order creation (via order API with pricing token) → hand off to Order Detail
///
/// Checkout owns ORDER CREATION ONLY. It never initiates a payment; payment is
/// an action against the created order, owned by the canonical Order Detail
/// surface ("Bayar Sekarang" = the backend decision action).
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// WHY NO DEDICATED CHECKOUT DOMAIN IN BACKEND?
/// ═══════════════════════════════════════════════════════════════════════════════
/// Checkout does NOT need its own domain because:
/// - Pricing is handled by the pricing service (token-based)
/// - Order creation is handled by the order domain
/// - Checkout is purely a UI coordination concern
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// COMMERCE FLOW
/// ═══════════════════════════════════════════════════════════════════════════════
/// 1. User views forSale detail or accepts negotiation, and
///    navigates to CheckoutScreen with the sale/product identity (NOT a token).
/// 2. CheckoutScreen calls the canonical pricing preview (`POST /pricing/preview`)
///    for the CURRENT inputs and receives pricing + pricingToken (10 min expiry).
///    The pricing is a backend snapshot; a local price is never checkout money.
/// 3. Review order details — rendered strictly from that applied preview, and
///    choose a payment method (`GET /payments/pre-order-methods`, backend fee)
///    that the order binds.
/// 4. Click "Buat Pesanan" → `POST /orders` with the pricing token. The backend
///    validates the token and returns the created ORDER (id, order_number,
///    status = pending_payment, canonical pricing snapshot) — NOT a payment URL.
/// 5. Navigate to the canonical Order Detail surface. Order Detail shows the
///    order status and owns the "Bayar Sekarang" action (backend decision).
/// 6. "Bayar Sekarang" → `POST /payments` → payment URL → internal WebView
///    (PaymentWebviewScreen), then the canonical payment-result/status flow.
///
/// Payment initiation is NOT part of checkout: order creation and payment
/// initiation are separate lifecycles with separate user-facing entry points.
///
/// ═══════════════════════════════════════════════════════════════════════════════
library;

// Domain exports
export 'domain/entities/checkout_request.dart';
export 'domain/entities/checkout_response.dart';

// Data exports (for provider dependencies)
export 'data/checkout_providers.dart';

// Presentation exports
export 'presentation/providers/checkout_provider.dart';
export 'presentation/providers/checkout_state.dart';
export 'presentation/screens/checkout_screen.dart';
export 'presentation/screens/payment_result_screen.dart';
