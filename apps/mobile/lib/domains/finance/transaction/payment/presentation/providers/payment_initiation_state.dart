/// Payment Initiation State
///
/// States for payment initiation flow with safety guards.
/// Prevents duplicate payment initiation and provides clear error states.
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

  /// Idempotency key for the current payment initiation attempt
  /// Generated once per payment attempt and preserved through retries
  /// Uses UUID v4 format to ensure uniqueness and prevent duplicate payments
  final String? idempotencyKey;

  /// Whether payment has been initiated (for preventing re-initiation)
  final bool isInitiated;

  /// Timestamp of last initiation attempt (for rate limiting)
  final DateTime? lastInitiatedAt;

  const PaymentInitiationState({
    this.isInitiating = false,
    this.intent,
    this.error,
    this.idempotencyKey,
    this.isInitiated = false,
    this.lastInitiatedAt,
  });

  /// Initial state
  factory PaymentInitiationState.initial() {
    return const PaymentInitiationState();
  }

  /// Loading state during initiation
  factory PaymentInitiationState.initiating({required String idempotencyKey}) {
    return PaymentInitiationState(
      isInitiating: true,
      idempotencyKey: idempotencyKey,
      lastInitiatedAt: DateTime.now(),
    );
  }

  /// Success state after successful initiation
  factory PaymentInitiationState.success({
    required PaymentIntent intent,
    String? idempotencyKey,
  }) {
    return PaymentInitiationState(
      intent: intent,
      isInitiated: true,
      idempotencyKey: idempotencyKey,
      lastInitiatedAt: DateTime.now(),
    );
  }

  /// Error state after failed initiation
  factory PaymentInitiationState.failure({
    required String error,
    String? idempotencyKey,
  }) {
    return PaymentInitiationState(
      error: error,
      idempotencyKey: idempotencyKey,
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
    String? idempotencyKey,
    bool? isInitiated,
    DateTime? lastInitiatedAt,
    bool clearIdempotencyKey = false,
  }) {
    return PaymentInitiationState(
      isInitiating: isInitiating ?? this.isInitiating,
      intent: intent ?? this.intent,
      error: error,
      idempotencyKey: clearIdempotencyKey
          ? null
          : (idempotencyKey ?? this.idempotencyKey),
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
    idempotencyKey,
    isInitiated,
    lastInitiatedAt,
  ];
}
