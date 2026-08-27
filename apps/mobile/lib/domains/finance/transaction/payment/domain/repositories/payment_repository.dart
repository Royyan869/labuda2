/// Payment Repository Interface
///
/// Pure Dart interface - no implementation details.
/// Defines contract for payment operations.
library;

import 'package:labuda/core/core.dart' as core;
import '../entities/payment.dart';
import '../entities/payment_intent.dart';
import '../entities/payment_method.dart';
import '../failures/payment_failure.dart';

/// Result type for repository operations
class RepositoryResult<T> {
  final T? data;
  final PaymentFailure? failure;

  const RepositoryResult._({this.data, this.failure});

  /// Create success result
  factory RepositoryResult.success(T data) {
    return RepositoryResult._(data: data);
  }

  /// Create failure result
  factory RepositoryResult.failure(PaymentFailure failure) {
    return RepositoryResult._(failure: failure);
  }

  /// Check if result is successful
  bool get isSuccess => data != null && failure == null;

  /// Check if result is failure
  bool get isFailure => failure != null;

  /// Get data or throw
  T get dataOrThrow {
    if (data != null) return data!;
    throw failure ?? const UnknownFailure('Unknown error');
  }

  /// Fold pattern - handle both success and failure
  R fold<R>(
    R Function(T data) onSuccess,
    R Function(PaymentFailure failure) onFailure,
  ) {
    if (data != null) {
      return onSuccess(data as T);
    }
    return onFailure(failure!);
  }

  /// Map data to new type
  RepositoryResult<R> map<R>(R Function(T data) mapper) {
    if (data != null) {
      try {
        return RepositoryResult.success(mapper(data as T));
      } catch (e) {
        return RepositoryResult.failure(UnknownFailure(e.toString()));
      }
    }
    return RepositoryResult.failure(failure!);
  }
}

/// Payment repository interface
abstract class PaymentRepository {
  /// Create a new payment
  Future<RepositoryResult<PaymentIntent>> createPayment(
    CreatePaymentRequest request,
  );

  /// Get payment by ID
  Future<RepositoryResult<Payment>> getPayment(String paymentId);

  /// Get the enabled canonical payment methods for [orderId], each with the
  /// backend-calculated buyer payment fee and total (PASS_18V). Call this
  /// before createPayment so the buyer can choose a method.
  Future<RepositoryResult<List<PaymentMethodOption>>> getPaymentMethodOptions(
    String orderId,
  );

  /// Get available payment methods
  List<PaymentMethod> getAvailablePaymentMethods();

  /// Calculate fee for a payment method
  ///
  /// Backend authority – DISPLAY ONLY, do not calculate on client.
  /// Fees come from backend via PriceSnapshot.
  @Deprecated('Backend authority – use PriceSnapshot from backend instead')
  double calculateFee(core.PaymentChannel channel, double amount);

  /// Calculate total with fee
  ///
  /// Backend authority – DISPLAY ONLY, do not calculate on client.
  /// Total amounts come from backend via PriceSnapshot.
  @Deprecated('Backend authority – use PriceSnapshot from backend instead')
  double calculateTotal(core.PaymentChannel channel, double amount);
}
