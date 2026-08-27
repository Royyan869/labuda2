/// Payment Remote Datasource
///
/// API-based datasource for payment operations.
/// All HTTP calls to Go backend are isolated here.
library;

import 'package:labuda/core/api/api.dart';
import 'package:labuda/core/common/result.dart';
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
}
