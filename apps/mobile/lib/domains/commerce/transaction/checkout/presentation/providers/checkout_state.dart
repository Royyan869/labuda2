/// Checkout State
library;

import 'package:equatable/equatable.dart';
import 'package:labuda/domains/commerce/transaction/checkout/domain/entities/checkout_response.dart';

/// Checkout state for state management
class CheckoutState extends Equatable {
  final bool isLoading;
  final String? error;

  /// Machine-readable code for [error], when the failure came from a known
  /// API contract (e.g. `EMAIL_VERIFICATION_REQUIRED`). Null when the error
  /// is transport-level or untagged.
  final String? errorCode;
  final CheckoutResponse? response;
  final bool isCreatingOrder;

  /// Idempotency key for the current checkout attempt
  ///
  /// Generated once per checkout submission and preserved through retries.
  /// Uses UUID v4 format to ensure uniqueness and prevent duplicate orders.
  final String? idempotencyKey;

  const CheckoutState({
    this.isLoading = false,
    this.error,
    this.errorCode,
    this.response,
    this.isCreatingOrder = false,
    this.idempotencyKey,
  });

  CheckoutState copyWith({
    bool? isLoading,
    String? error,
    String? errorCode,
    CheckoutResponse? response,
    bool? isCreatingOrder,
    String? idempotencyKey,
  }) {
    return CheckoutState(
      isLoading: isLoading ?? this.isLoading,
      error: error,
      errorCode: errorCode,
      response: response ?? this.response,
      isCreatingOrder: isCreatingOrder ?? this.isCreatingOrder,
      idempotencyKey: idempotencyKey ?? this.idempotencyKey,
    );
  }

  @override
  List<Object?> get props => [
    isLoading,
    error,
    errorCode,
    response,
    isCreatingOrder,
    idempotencyKey,
  ];
}
