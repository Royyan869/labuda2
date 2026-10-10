/// Payment Repository Interface
///
/// Pure Dart interface - no implementation details.
/// Defines contract for payment operations.
library;

import 'package:hishumi/core/common/result.dart';
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
  /// backend-calculated buyer payment fee and total (PASS_18V). This is the
  /// POST-ORDER disclosure (retry / order detail).
  Future<Result<List<PaymentMethodOption>>> getPaymentMethodOptions(
    String orderId,
  );

  /// Get the canonical PRE-ORDER payment pricing for [pricingToken]: every
  /// enabled method with the backend-computed buyer payment fee and FINAL
  /// payable amount. Read-only — no order is created and the token is not
  /// consumed. This is the checkout authority for choosing a method and knowing
  /// the final total before "Buat Pesanan".
  Future<Result<PreOrderPaymentPricing>> getPreOrderPaymentPricing(
    String pricingToken, {
    bool useCoins,
  });
}
