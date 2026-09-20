/// Checkout Provider
///
/// State management for checkout flow using Riverpod 3.x
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/commerce/transaction/checkout/domain/entities/checkout_request.dart';
import 'package:labuda/domains/commerce/transaction/checkout/domain/entities/checkout_response.dart';
// Re-exports CheckoutException (defined next to the repository implementation).
import 'package:labuda/domains/commerce/transaction/checkout/data/checkout_providers.dart';
import 'package:labuda/domains/commerce/transaction/checkout/domain/usecases/checkout_usecase_providers.dart';
import 'package:labuda/domains/commerce/transaction/checkout/presentation/providers/checkout_state.dart';
import 'package:labuda/core/core.dart';

// =============================================================================
// CHECKOUT NOTIFIER
// =============================================================================

/// Checkout Notifier for managing checkout UI state
class CheckoutNotifier extends Notifier<CheckoutState> {
  ILoggerService? get _logger => ref.read(loggerServiceProvider);

  @override
  CheckoutState build() {
    return const CheckoutState();
  }

  /// Create order with the given checkout request
  ///
  /// Uses usecase for business logic and idempotency management.
  Future<CheckoutResponse?> createOrder(CheckoutRequest request) async {
    // Get idempotency key from state if available (retry scenario)
    final idempotencyKey = state.idempotencyKey;

    // Set loading state and preserve the idempotency key
    state = state.copyWith(
      isCreatingOrder: true,
      error: null,
      idempotencyKey: idempotencyKey,
    );

    try {
      // Use usecase for order creation
      final createOrderUseCase = ref.read(createOrderUseCaseProvider);
      final result = await createOrderUseCase(
        request,
        idempotencyKey: idempotencyKey,
      );

      if (result.isError) {
        state = state.copyWith(
          isCreatingOrder: false,
          error: result.error ?? 'Failed to create order',
          errorCode: result.errorCode,
        );
        return null;
      }

      final response = result.data!;
      _logger?.info('Order created successfully: ${response.orderId}');
      // Clear idempotency key on success
      state = state.copyWith(isCreatingOrder: false, idempotencyKey: null);
      return response;
    } on CheckoutException catch (e) {
      _logger?.error('Failed to create order: ${e.message}');
      // Preserve the structured error code so the UI can branch on
      // COMMERCE_RESTRICTED, EMAIL_VERIFICATION_REQUIRED, etc.
      state = state.copyWith(
        isCreatingOrder: false,
        error: e.userFriendlyMessage,
        errorCode: e.code,
      );
      return null;
    } catch (e, stackTrace) {
      _logger?.error('Failed to create order: $e', stackTrace: stackTrace);
      // Keep idempotency key for potential retry
      state = state.copyWith(isCreatingOrder: false, error: e.toString());
      return null;
    }
  }

  /// Clear error but preserve idempotency key for retry
  void clearError() {
    state = state.copyWith(error: null);
  }
}

// =============================================================================
// CHECKOUT STATE PROVIDER
// =============================================================================

/// Checkout State Provider
/// Usage: ref.watch(checkoutNotifierProvider) to get state
///        ref.read(checkoutNotifierProvider.notifier).createOrder(...) to create order
final checkoutNotifierProvider =
    NotifierProvider<CheckoutNotifier, CheckoutState>(CheckoutNotifier.new);
