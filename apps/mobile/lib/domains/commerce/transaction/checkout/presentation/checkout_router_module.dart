import 'package:go_router/go_router.dart';
import 'package:labuda/core/src/router/modules/base_module.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'package:labuda/domains/commerce/transaction/checkout/checkout.dart';

/// Checkout Module - Transaction flow routes
///
/// Handles:
/// - Direct buy checkout flow
/// - Order creation
/// - Payment redirect
/// - Payment result verification
///
/// **CV2:** Added returnToChat support for chat-commerce continuity
class CheckoutModule extends BaseModule {
  @override
  String get moduleName => 'CheckoutModule';

  @override
  List<GoRoute> get routes => [
    // Checkout route - Direct buy flow
    GoRoute(
      path: RoutePaths.checkout,
      name: RouteNames.checkout,
      builder: (context, state) {
        final fixedPriceSaleId = state.pathParameters['fixedPriceSaleId']!;
        final productId = state.uri.queryParameters['product_id'];

        // Chat commerce context - optional query parameters
        final negotiationId = state.uri.queryParameters['negotiation_id'];

        // Auction checkout context - for winning bid or buy now
        final auctionId = state.uri.queryParameters['auction_id'];

        // **SHIPPING QUOTE FIX:** Shipping quote ID from seller's manual quote
        final shippingQuoteId = state.uri.queryParameters['shipping_quote_id'];

        // **CV2:** Chat return context - navigate back to chat after checkout
        final returnToChat = state.uri.queryParameters['return_to_chat'];

        return CheckoutScreen(
          productId: productId,
          fixedPriceSaleId: fixedPriceSaleId,
          negotiationId: negotiationId,
          auctionId: auctionId,
          shippingQuoteId: shippingQuoteId,
          returnToChat: returnToChat,
        );
      },
    ),

    // Payment Result route - Post-payment status check
    // **CV2:** Support returnToChat for chat-commerce continuity
    GoRoute(
      path: RoutePaths.paymentResult,
      name: RouteNames.paymentResult,
      builder: (context, state) {
        final orderId = state.pathParameters['orderId']!;
        final orderNumber = state.extra as String?;
        // **CV2:** Check for returnToChat in query parameters
        final returnToChat = state.uri.queryParameters['return_to_chat'];
        return PaymentResultScreen(
          orderId: orderId,
          orderNumber: orderNumber,
          returnToChat: returnToChat,
        );
      },
    ),
  ];

  @override
  Future<void> initialize() async {
    // Checkout module initialized - no special setup needed
  }

  @override
  void registerRoutes(List<GoRoute> mainRoutes) {
    mainRoutes.addAll(routes);
  }

  @override
  void dispose() {
    // No cleanup needed for Checkout module
  }
}
