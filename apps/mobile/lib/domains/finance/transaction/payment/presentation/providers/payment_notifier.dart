/// Payment Notifier
///
/// Riverpod Notifier for payment state management.
/// Replaces UseCase classes - business logic lives here.
library;

import 'package:riverpod_annotation/riverpod_annotation.dart';
import '../../domain/entities/payment.dart';
import 'payment_state.dart';
import 'payment_providers.dart' show paymentRepositoryProvider;

part 'payment_notifier.g.dart';

// ============================================================================
// Payment Notifier
// ============================================================================

/// Payment notifier for state management
@riverpod
class PaymentNotifier extends _$PaymentNotifier {
  // Synchronous double-submit guard for financial operations
  bool _isCreatingPayment = false;

  @override
  PaymentState build() {
    return const PaymentState.initial();
  }

  /// Create a new payment
  Future<void> createPayment(CreatePaymentRequest request) async {
    // Synchronous guard - prevent double-tap
    if (_isCreatingPayment) return;
    _isCreatingPayment = true;

    state = const PaymentState.loading();
    final repo = ref.read(paymentRepositoryProvider);

    try {
      final result = await repo.createPayment(request);

      result.fold(
        (intent) => state = PaymentState.paymentCreated(intent),
        (failure) => state = PaymentState.error(failure.message),
      );
    } finally {
      // Always reset guard in finally
      _isCreatingPayment = false;
    }
  }

  /// Get payment by ID
  Future<void> getPayment(String paymentId) async {
    state = const PaymentState.loading();
    final repo = ref.read(paymentRepositoryProvider);

    final result = await repo.getPayment(paymentId);

    result.fold(
      (payment) => state = PaymentState.paymentLoaded(payment),
      (failure) => state = PaymentState.error(failure.message),
    );
  }

  /// Clear error
  void clearError() {
    state = const PaymentState.initial();
  }

  /// Reset state
  void reset() {
    state = const PaymentState.initial();
  }
}
