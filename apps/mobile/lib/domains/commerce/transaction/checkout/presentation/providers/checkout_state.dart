/// Checkout State
library;

import 'package:equatable/equatable.dart';

/// Checkout state for state management
///
/// Only the facts the checkout UI actually reads live here: the in-flight flag,
/// the error (message + machine-readable code) and the idempotency key that
/// must survive a retry. The created order/preview results are returned from the
/// notifier call and rendered by the screen — they are NOT duplicated into this
/// state, so there is a single authority for them.
class CheckoutState extends Equatable {
  final String? error;

  /// Machine-readable code for [error], when the failure came from a known
  /// API contract (e.g. `EMAIL_VERIFICATION_REQUIRED`). Null when the error
  /// is transport-level or untagged.
  final String? errorCode;
  final bool isCreatingOrder;

  /// Idempotency key for the current checkout attempt
  ///
  /// Generated once per checkout submission and preserved through retries.
  /// Uses UUID v4 format to ensure uniqueness and prevent duplicate orders.
  final String? idempotencyKey;

  const CheckoutState({
    this.error,
    this.errorCode,
    this.isCreatingOrder = false,
    this.idempotencyKey,
  });

  CheckoutState copyWith({
    String? error,
    String? errorCode,
    bool? isCreatingOrder,
    String? idempotencyKey,
  }) {
    return CheckoutState(
      error: error,
      errorCode: errorCode,
      isCreatingOrder: isCreatingOrder ?? this.isCreatingOrder,
      idempotencyKey: idempotencyKey ?? this.idempotencyKey,
    );
  }

  @override
  List<Object?> get props => [
    error,
    errorCode,
    isCreatingOrder,
    idempotencyKey,
  ];
}
