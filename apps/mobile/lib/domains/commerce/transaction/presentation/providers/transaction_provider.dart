/// Transaction Provider
///
/// **DOMAIN:** Commerce → Transaction
/// **RESPONSIBILITY:** Order status operations for chat commerce context
/// **BOUNDARY:** Pure UI state management - business logic in use cases
///
/// **PHASE 2 EXTRACTION:** Moved from chat module to commerce domain
/// - Was: chat_order_provider.dart
/// - Now: transaction_provider.dart (commerce domain)
///
/// **CLEAN ARCHITECTURE:**
/// - Provider: UI state management only
/// - UseCase: All business logic
/// - Repository: Data access (injected via use case)
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/transaction/usecases/get_order_status_usecase.dart';
import 'package:labuda/domains/commerce/transaction/order/data/order_providers.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/entities/order_status.dart';
import 'package:labuda/core/common/types/payment_types.dart';

// ==============================================================================
// TRANSACTION STATE
// ==============================================================================

/// Transaction state
///
/// Holds order status information for commerce context.
/// No business logic - pure state container.
class TransactionState {
  final String? orderId;
  final OrderStatus? orderStatus;
  final PaymentStatus? paymentStatus;
  final bool isLoading;
  final String? error;

  const TransactionState({
    this.orderId,
    this.orderStatus,
    this.paymentStatus,
    this.isLoading = false,
    this.error,
  });

  TransactionState copyWith({
    String? orderId,
    OrderStatus? orderStatus,
    PaymentStatus? paymentStatus,
    bool? isLoading,
    String? error,
  }) {
    return TransactionState(
      orderId: orderId ?? this.orderId,
      orderStatus: orderStatus ?? this.orderStatus,
      paymentStatus: paymentStatus ?? this.paymentStatus,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }

  @override
  String toString() =>
      'TransactionState(orderId: $orderId, orderStatus: $orderStatus, paymentStatus: $paymentStatus, isLoading: $isLoading, error: $error)';
}

// ==============================================================================
// TRANSACTION NOTIFIER
// ==============================================================================

/// Transaction Notifier
///
/// **PURE UI STATE MANAGER** - Only handles UI state.
/// All business logic delegated to use cases.
///
/// **STATE ISOLATION:**
/// This provider maintains a SINGLE shared order state across all chats.
/// The most recently fetched order status will be displayed.
///
/// **DOMAIN BOUNDARY ENFORCEMENT:**
/// - ❌ NO direct repository calls
/// - ❌ NO business logic in provider
/// - ✅ ALL operations through use cases
class TransactionNotifier extends Notifier<TransactionState> {
  late final GetOrderStatusUseCase _getOrderStatusUseCase;

  @override
  TransactionState build() {
    // Initialize use case with repository
    final orderRepository = ref.watch(orderRepositoryProvider);
    _getOrderStatusUseCase = GetOrderStatusUseCase(orderRepository);

    return const TransactionState();
  }

  /// Fetch order status for a linked order
  ///
  /// **UI STATE ONLY:** No business logic here.
  /// All data fetching logic in use case.
  Future<void> fetchOrderStatus(String orderId) async {
    // Avoid duplicate fetches for same order
    if (state.orderId == orderId && !state.isLoading) {
      return;
    }

    state = state.copyWith(orderId: orderId, isLoading: true, error: null);

    try {
      // **CLEAN ARCHITECTURE:** Delegate to use case
      final result = await _getOrderStatusUseCase.executeWithPaymentStatus(
        orderId,
      );

      result.fold(
        (error) {
          // Don't show error in UI - just log it
          // User can still navigate to order detail for full info
          state = state.copyWith(isLoading: false, error: error.toString());
        },
        (Map<String, dynamic> data) {
          // Extract status values from the map
          // The use case already returns enum objects, use them directly
          final orderStatus = data['orderStatus'] as OrderStatus?;
          final paymentStatus = data['paymentStatus'] as PaymentStatus?;

          state = state.copyWith(
            orderStatus: orderStatus,
            paymentStatus: paymentStatus,
            isLoading: false,
          );
        },
      );
    } catch (e) {
      state = state.copyWith(isLoading: false, error: e.toString());
    }
  }

  /// Clear the current order state
  ///
  /// Called when navigating away from chat or when order link is removed.
  void clear() {
    state = const TransactionState();
  }

  /// Refresh order status
  ///
  /// User can manually refresh to get latest status.
  Future<void> refresh() async {
    if (state.orderId != null) {
      await fetchOrderStatus(state.orderId!);
    }
  }
}

// ==============================================================================
// PROVIDER DEFINITIONS
// ==============================================================================

/// Transaction Notifier Provider
///
/// Simple scoped provider (not family). All chats share the same order state,
/// which updates when opening different chats. This matches the actual usage
/// pattern (one chat visible at a time).
final transactionNotifierProvider =
    NotifierProvider<TransactionNotifier, TransactionState>(
      TransactionNotifier.new,
    );

/// Transaction State Provider
///
/// Simple alias to the base notifier state. The chatId parameter is kept
/// for API compatibility but not used for state isolation.
///
/// **USAGE:** Call this provider to get the current order state.
/// State will reflect the most recently fetched linked order.
final transactionStateProvider = Provider<TransactionState>((ref) {
  return ref.watch(transactionNotifierProvider);
});
