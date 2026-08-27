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
/// - Order creation handoff (via order API with pricing token)
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
/// 1. User views listing detail / accepts negotiation / accepts seller quote
/// 2. Preview API called → returns pricing + pricingToken (10 min expiry)
/// 3. Navigate to CheckoutScreen with pricing token
/// 4. Review order details (backend-calculated pricing)
/// 5. Click "Buat Pesanan" → CreateOrder API with pricing token
/// 6. Backend validates token, creates order, returns paymentUrl
/// 7. Open payment URL with url_launcher
/// 8. Navigate to PaymentResultScreen to check payment status
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
