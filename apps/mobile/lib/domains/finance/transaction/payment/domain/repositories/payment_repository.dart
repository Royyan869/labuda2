/// Payment Repository Interface
///
/// Pure Dart interface - no implementation details.
/// Defines contract for payment operations.
library;

import 'package:labuda/core/common/result.dart';
import '../entities/payment.dart';
import '../entities/payment_intent.dart';

/// Payment repository interface
abstract class PaymentRepository {
  /// Create a new payment
  Future<Result<PaymentIntent>> createPayment(
    CreatePaymentRequest request,
  );

  /// Get payment by ID
  Future<Result<Payment>> getPayment(String paymentId);

  /// Get the enabled canonical payment methods for [orderId], each with the
  /// backend-calculated buyer payment fee and total (PASS_18V). Call this
  /// before createPayment so the buyer can choose a method.
  Future<Result<List<PaymentMethodOption>>> getPaymentMethodOptions(
    String orderId,
  );

}
