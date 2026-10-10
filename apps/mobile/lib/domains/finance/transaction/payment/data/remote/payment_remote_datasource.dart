/// Payment Remote Datasource
///
/// API-based datasource for payment operations.
/// All HTTP calls to Go backend are isolated here.
library;

import 'package:hishumi/core/api/api.dart';
import 'package:hishumi/core/common/result.dart';
import '../dto/payment_dto.dart';

/// Payment remote datasource
class PaymentRemoteDatasource extends BaseApiRepository {
  PaymentRemoteDatasource(super.apiClient, {super.logger});

  // ========================================
  // Payment Operations
  // ========================================

  /// Create a new payment
  Future<Result<PaymentIntentDto>> createPayment(
    CreatePaymentRequestDto request,
  ) async {
    return executeRequest(
      () => apiClient.post('/payments', data: request.toJson()),
      parser: (data) => PaymentIntentDto.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Get payment by ID
  Future<Result<PaymentDto>> getPayment(String paymentId) async {
    return executeRequest(
      () => apiClient.get('/payments/$paymentId'),
      parser: (data) => PaymentDto.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Get the enabled canonical payment methods for an order, each already
  /// carrying the backend-calculated buyer payment fee and total.
  ///
  /// PASS_18V: backend is sole authority — this must be called before
  /// createPayment so the buyer can choose a method and see its real fee.
  Future<Result<PaymentMethodOptionsDto>> getPaymentMethods(
    String orderId,
  ) async {
    return executeRequest(
      () => apiClient.get(
        '/payments/methods',
        queryParameters: {'order_id': orderId},
      ),
      parser: (data) =>
          PaymentMethodOptionsDto.fromJson(data as Map<String, dynamic>),
    );
  }

  /// Get the canonical PRE-ORDER payment pricing for a pricing token, each
  /// method carrying the backend-calculated buyer fee and FINAL payable amount.
  ///
  /// Read-only: no order is created and the token is not consumed. This is the
  /// checkout authority for choosing a method and knowing the final total
  /// before order creation.
  Future<Result<PreOrderPaymentPricingDto>> getPreOrderPaymentMethods(
    String pricingToken, {
    bool useCoins = false,
  }) async {
    return executeRequest(
      () => apiClient.get(
        '/payments/pre-order-methods',
        queryParameters: {
          'pricing_token': pricingToken,
          'use_coins': useCoins.toString(),
        },
      ),
      parser: (data) =>
          PreOrderPaymentPricingDto.fromJson(data as Map<String, dynamic>),
    );
  }

  /// On-demand payment status sync (POST /payments/:id/sync).
  ///
  /// Asks the backend to run its canonical gateway-inquiry → settle →
  /// domain-finalization pipeline for THIS payment right now, instead of only
  /// re-reading the local row. This is what "Cek status pembayaran" should
  /// call: a webhook cannot reach a non-public backend, and the discovery
  /// worker only scans pending payments after its eligibility age.
  ///
  /// Returns the post-sync status projection:
  /// - `status`: payments.status after the sync (pending/settlement/capture/...)
  /// - `provider_state`: gateway state observed by the inquiry
  /// - `settled`: true when the row is settlement/capture after the sync
  /// - `mutated`: true when THIS call produced the transition
  Future<Result<Map<String, dynamic>>> syncPayment(String paymentId) async {
    return executeRequest(
      () => apiClient.post('/payments/$paymentId/sync'),
      parser: (data) => data as Map<String, dynamic>,
    );
  }
}
