import 'package:go_router/go_router.dart';
import 'package:hishumi/core/src/router/modules/base_module.dart';
import 'package:hishumi/core/src/router/route_paths.dart';
import 'package:hishumi/domains/commerce/transaction/checkout/checkout.dart';
import 'package:hishumi/domains/finance/transaction/payment/presentation/screens/payment_webview_screen.dart';

/// Checkout Module - Transaction flow routes
///
/// Handles:
/// - Direct buy checkout flow
/// - Order creation (Checkout ends here → Order Detail owns payment)
/// - Payment redirect (payment WebView)
/// - Payment result verification
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
        final forSaleId = state.pathParameters['forSaleId']!;
        final productId = state.uri.queryParameters['product_id'];

        // Chat commerce context - optional query parameters
        final negotiationId = state.uri.queryParameters['negotiation_id'];

        // Auction checkout context — buy-now OR bid-win. `bid_win=1` marks
        // the winner completing the settlement window: the order is created
        // WITHOUT a payment method (chosen at Order Detail).
        final auctionId = state.uri.queryParameters['auction_id'];
        final bidWin = state.uri.queryParameters['bid_win'] == '1';

        // **SHIPPING QUOTE FIX:** Shipping quote ID from seller's manual quote
        final shippingQuoteId = state.uri.queryParameters['shipping_quote_id'];

        // Conversation scope for the manual shipping quote (required when a
        // quote is used): the chat that produced the quote.
        final chatId = state.uri.queryParameters['chat_id'];

        return CheckoutScreen(
          productId: productId,
          forSaleId: forSaleId,
          negotiationId: negotiationId,
          auctionId: auctionId,
          bidWin: bidWin,
          shippingQuoteId: shippingQuoteId,
          chatId: chatId,
        );
      },
    ),

    // Payment Result route - Post-payment status check
    GoRoute(
      path: RoutePaths.paymentResult,
      name: RouteNames.paymentResult,
      builder: (context, state) {
        final orderId = state.pathParameters['orderId']!;
        final orderNumber = state.extra as String?;
        return PaymentResultScreen(orderId: orderId, orderNumber: orderNumber);
      },
    ),

    // Payment WebView - SINGLE CANONICAL PAYMENT PRESENTATION SURFACE.
    // Payment URLs are presented exclusively inside Labuda's internal WebView.
    // External-browser payment navigation is obsolete and must not be reintroduced.
    // Payment completion remains backend-authoritative (PaymentResultNotifier).
    GoRoute(
      path: RoutePaths.paymentWebview,
      name: RouteNames.paymentWebview,
      builder: (context, state) {
        final url = state.uri.queryParameters['url'] ?? '';
        final orderId = state.uri.queryParameters['orderId'];
        return PaymentWebviewScreen(
          paymentUrl: Uri.decodeComponent(url),
          orderId: orderId,
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
