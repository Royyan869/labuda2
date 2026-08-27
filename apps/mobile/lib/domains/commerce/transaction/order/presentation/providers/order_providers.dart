import 'dart:async';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/order/data/order_providers.dart';

// Note: dart:async is kept for the StreamProvider types
// Note: orderRepositoryProvider is imported from data/order_providers.dart

// =============================================================================
// REFRESH TRIGGER PROVIDERS
// =============================================================================
// These providers allow manual refresh of order data when screens are opened

/// Refresh trigger for single order - use this to force a refetch
final orderRefreshTriggerProvider = StreamProvider.autoDispose
    .family<void, String>((ref, orderId) {
      // This provider exists only to be invalidated/refetched to trigger refresh
      return const Stream.empty();
    });

/// Refresh trigger for order list - use this to force a refetch
final orderListRefreshTriggerProvider = StreamProvider.autoDispose<void>((ref) {
  // This provider exists only to be invalidated/refetched to trigger refresh
  return const Stream.empty();
});

// =============================================================================
// ORDER LIST SCREEN PROVIDERS
// =============================================================================

/// Provider for watching seller orders with real-time updates
/// Uses named parameters to match existing code in order_list_screen.dart
/// Delegates to repository's watchSellerOrders stream which handles polling
final _watchSellerOrdersProviderFamily =
    StreamProvider.family<
      List<Order>,
      ({String sellerId, OrderStatus? status})
    >((ref, params) {
      final repository = ref.watch(orderRepositoryProvider);

      // Watch refresh trigger to force refetch when needed
      ref.watch(orderListRefreshTriggerProvider);

      // Use repository's existing stream implementation
      return repository.watchSellerOrders(
        WatchOrdersParams(userId: params.sellerId, status: params.status),
      );
    });

/// Proxy function for watchSellerOrdersProvider with named parameters
StreamProvider<List<Order>> watchSellerOrdersProvider({
  required String sellerId,
  OrderStatus? status,
}) {
  return _watchSellerOrdersProviderFamily((sellerId: sellerId, status: status));
}

/// Provider for watching buyer orders with real-time updates
/// Uses named parameters to match existing code in order_list_screen.dart
/// Delegates to repository's watchBuyerOrders stream which handles polling
final _watchBuyerOrdersProviderFamily =
    StreamProvider.family<List<Order>, ({String buyerId, OrderStatus? status})>(
      (ref, params) {
        final repository = ref.watch(orderRepositoryProvider);

        // Watch refresh trigger to force refetch when needed
        ref.watch(orderListRefreshTriggerProvider);

        // Use repository's existing stream implementation
        return repository.watchBuyerOrders(
          WatchOrdersParams(userId: params.buyerId, status: params.status),
        );
      },
    );

/// Proxy function for watchBuyerOrdersProvider with named parameters
StreamProvider<List<Order>> watchBuyerOrdersProvider({
  required String buyerId,
  OrderStatus? status,
}) {
  return _watchBuyerOrdersProviderFamily((buyerId: buyerId, status: status));
}

/// One-shot recent seller orders provider for dashboard summary cards.
///
/// Uses the same canonical seller orders endpoint but resolves once so the
/// dashboard section can render loading, empty, error, or data without getting
/// stuck behind the polling stream lifecycle.
final recentSellerOrdersProvider = FutureProvider.autoDispose
    .family<List<Order>, String>((ref, sellerId) async {
      final repository = ref.watch(orderRepositoryProvider);
      final result = await repository.getSellerOrders(
        GetOrdersParams(userId: sellerId, limit: 3),
      );

      if (result.isError) {
        throw Exception(result.error ?? 'Failed to load recent orders');
      }

      return result.data ?? <Order>[];
    });

/// Provider for watching a single order with real-time updates
/// Delegates to repository's watchOrder stream which handles polling
/// Refreshes immediately when provider is recreated (screen opened)
final watchOrderProvider = StreamProvider.family<Order, String>((ref, orderId) {
  final repository = ref.watch(orderRepositoryProvider);

  // Watch refresh trigger to force refetch when needed
  ref.watch(orderRefreshTriggerProvider(orderId));

  // Use repository's existing stream implementation (handles polling internally)
  return repository.watchOrder(orderId);
});

/// Provider for fetching refunds by order ID
/// Currently returns empty list - backend integration pending
final refundsByOrderProvider =
    StreamProvider.family<List<RefundRequest>, String>((ref, orderId) {
      final repository = ref.watch(orderRepositoryProvider);
      final orderStream = repository.watchOrder(orderId);
      return orderStream.map((order) {
        final activeRefund = order.activeRefund;
        if (activeRefund == null) return <RefundRequest>[];
        return <RefundRequest>[activeRefund];
      });
    });

// =============================================================================
// ORDER PREVIEW PROVIDER
// =============================================================================

/// Provider for previewing order pricing before order creation
/// This provider calls POST /orders/preview to get backend-calculated pricing
/// The pricing from this provider should be used for checkout validation
/// instead of listing.price
final orderPreviewProvider =
    FutureProvider.family<PreviewOrderResult, PreviewOrderParams>((
      ref,
      params,
    ) async {
      final repository = ref.watch(orderRepositoryProvider);
      final result = await repository.previewOrder(params);

      if (result.isError || result.data == null) {
        throw Exception(result.error ?? 'Failed to preview order');
      }

      return result.data!;
    });
