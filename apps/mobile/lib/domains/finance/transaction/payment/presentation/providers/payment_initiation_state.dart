/// Payment Initiation State
///
/// States for payment initiation flow with safety guards.
/// Prevents duplicate payment initiation and provides clear error states.
///
/// NOTE: there is deliberately NO client-side idempotency key here. POST
/// /api/v1/payments is idempotent server-side (order + active-payment reuse)
/// and the backend binds no client key for it.
library;

import 'package:equatable/equatable.dart';
import 'package:labuda/domains/finance/transaction/payment/domain/entities/payment_intent.dart';

/// Payment initiation state with safety mechanisms
class PaymentInitiationState extends Equatable {
  /// Whether a payment initiation is in progress
  /// IMMEDIATE LOCK - set synchronously to prevent double-tap
  final bool isInitiating;

  /// The payment intent created after successful initiation
  final PaymentIntent? intent;

  /// Error message if initiation failed
  final String? error;

  /// Whether payment has been initiated (for preventing re-initiation)
  final bool isInitiated;

  /// Timestamp of last initiation attempt (for rate limiting)
  final DateTime? lastInitiatedAt;

  const PaymentInitiationState({
    this.isInitiating = false,
    this.intent,
    this.error,
    this.isInitiated = false,
    this.lastInitiatedAt,
  });

  /// Initial state
  factory PaymentInitiationState.initial() {
    return const PaymentInitiationState();
  }

  /// Success state after successful initiation
  factory PaymentInitiationState.success({required PaymentIntent intent}) {
    return PaymentInitiationState(
      intent: intent,
      isInitiated: true,
      lastInitiatedAt: DateTime.now(),
    );
  }

  /// Error state after failed initiation
  factory PaymentInitiationState.failure({required String error}) {
    return PaymentInitiationState(
      error: error,
      lastInitiatedAt: DateTime.now(),
    );
  }

  /// Check if payment initiation is allowed
  /// Returns false if already initiating or already initiated
  bool get canInitiate => !isInitiating && !isInitiated;

  /// Check if enough time has passed since last attempt (5 second cooldown)
  bool get hasCooldownPassed {
    if (lastInitiatedAt == null) return true;
    final elapsed = DateTime.now().difference(lastInitiatedAt!);
    return elapsed.inSeconds >= 5;
  }

  PaymentInitiationState copyWith({
    bool? isInitiating,
    PaymentIntent? intent,
    String? error,
    bool? isInitiated,
    DateTime? lastInitiatedAt,
  }) {
    return PaymentInitiationState(
      isInitiating: isInitiating ?? this.isInitiating,
      intent: intent ?? this.intent,
      error: error,
      isInitiated: isInitiated ?? this.isInitiated,
      lastInitiatedAt: lastInitiatedAt ?? this.lastInitiatedAt,
    );
  }

  PaymentInitiationState clearError() {
    return copyWith(error: '');
  }

  PaymentInitiationState reset() {
    return const PaymentInitiationState();
  }

  @override
  List<Object?> get props => [
    isInitiating,
    intent,
    error,
    isInitiated,
    lastInitiatedAt,
  ];
}
