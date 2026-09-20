import 'package:dio/dio.dart';
import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';

import '../dto/dto_barrel.dart';
import '../models/api/order_api_models.dart'
    show
        OrderApiResponse,
        OrderListApiResponse,
        OrderFilterParams,
        RefundFilterParams,
        CheckDeliveryApiResponse,
        CheckDeliveryApiRequest;
import 'order_remote_datasource.dart';
import '../../domain/entities/order_params.dart';

class OrderApiException implements Exception {
  final String message;
  final String? code;
  final Map<String, dynamic>? details;

  const OrderApiException(this.message, {this.code, this.details});

  @override
  String toString() => code == null ? message : '[$code] $message';
}

/// Order API Datasource Implementation
///
/// Implements OrderRemoteDatasource interface
/// Returns DTOs directly, throws Exception on error
class OrderApiDatasourceImpl implements OrderRemoteDatasource {
  final ApiClient _apiClient;
  final ILoggerService? _logger;

  OrderApiDatasourceImpl(this._apiClient, {ILoggerService? logger})
    : _logger = logger;

  // Helper: Execute request and return data or throw
  Future<T> _executeRequest<T>(
    Future<Response<dynamic>> Function() request, {
    required T Function(dynamic data) parser,
  }) async {
    try {
      final response = await request();
      final data = response.data;

      if (data is Map<String, dynamic>) {
        // Handle standard API response with success field
        if (data['success'] == false && data['error'] != null) {
          final error = data['error'] as Map<String, dynamic>?;
          final detailsRaw = error?['details'];
          throw OrderApiException(
            error?['message']?.toString() ?? 'Request failed',
            code: error?['code']?.toString(),
            details: detailsRaw is Map<String, dynamic> ? detailsRaw : null,
          );
        }

        // Parse the data field if available, otherwise use entire response
        final parsedData = data['data'] ?? data;
        return parser(parsedData);
      }

      // Direct data (not wrapped in standard format)
      return parser(data);
    } on DioException catch (e) {
      final exception = _apiClient.extractException(e);
      _logger?.error(
        'API request failed: ${exception.message}',
        extra: {'code': exception.code, 'statusCode': exception.statusCode},
      );
      throw OrderApiException(
        exception.message,
        code: exception.code,
        details: exception.details is Map<String, dynamic>
            ? exception.details as Map<String, dynamic>
            : null,
      );
    } catch (e, stackTrace) {
      _logger?.error('Unexpected error: $e', stackTrace: stackTrace);
      if (e is OrderApiException) rethrow;
      throw OrderApiException(e.toString());
    }
  }

  // ========================================
  // Order CRUD Operations
  // ========================================

  @override
  Future<OrderApiResponse> getOrder(String orderId) async {
    return _executeRequest(
      () => _apiClient.get('/orders/$orderId'),
      parser: (data) =>
          OrderApiResponse.fromJson(data as Map<String, dynamic>),
    );
  }

  @override
  Future<OrderListApiResponse> listMyOrders({OrderFilterParams? params}) async {
    final query = <String, dynamic>{
      'role': 'buyer',
      'limit': params?.pageSize ?? 20,
    };
    if (params?.status != null) {
      query['status'] = params!.toQueryParams()['status'];
    }
    return _executeRequest(
      () => _apiClient.get('/orders', queryParameters: query),
      parser: (data) => _parseOrderListResponse(data),
    );
  }

  @override
  Future<OrderListApiResponse> listSellerOrders({
    OrderFilterParams? params,
  }) async {
    final query = <String, dynamic>{
      'role': 'seller',
      'limit': params?.pageSize ?? 20,
    };
    if (params?.status != null) {
      query['status'] = params!.toQueryParams()['status'];
    }
    return _executeRequest(
      () => _apiClient.get('/orders', queryParameters: query),
      parser: (data) => _parseOrderListResponse(data),
    );
  }

  OrderListApiResponse _parseOrderListResponse(dynamic data) {
    if (data is Map<String, dynamic>) {
      return OrderListApiResponse.fromJson(data);
    }
    if (data is List) {
      return OrderListApiResponse.fromJson({'orders': data});
    }
    return OrderListApiResponse.fromJson({'orders': const []});
  }

  // ========================================
  // Order Lifecycle Operations
  // ========================================

  @override
  Future<OrderApiResponse> shipOrder(
    String orderId,
    MarkAsShippedParams params,
  ) async {
    final idempotencyKey =
        '${orderId}_ship_${DateTime.now().millisecondsSinceEpoch}';
    return _executeRequest(
      () => _apiClient.post(
        '/orders/$orderId/ship',
        data: params.toJson(),
        options: Options(headers: {'Idempotency-Key': idempotencyKey}),
      ),
      parser: (_) => OrderApiResponse.fromJson({'id': orderId}),
    );
  }

  @override
  Future<OrderApiResponse> completeOrder(String orderId) async {
    final idempotencyKey =
        '${orderId}_complete_${DateTime.now().millisecondsSinceEpoch}';
    return _executeRequest(
      () => _apiClient.post(
        '/orders/$orderId/complete',
        options: Options(headers: {'Idempotency-Key': idempotencyKey}),
      ),
      parser: (_) => OrderApiResponse.fromJson({'id': orderId}),
    );
  }

  @override
  Future<OrderApiResponse> cancelOrder(String orderId) async {
    final idempotencyKey =
        '${orderId}_cancel_${DateTime.now().millisecondsSinceEpoch}';
    return _executeRequest(
      () => _apiClient.post(
        '/orders/$orderId/cancel',
        options: Options(headers: {'Idempotency-Key': idempotencyKey}),
      ),
      parser: (_) => OrderApiResponse.fromJson({'id': orderId}),
    );
  }

  // ========================================
  // Refund Operations
  // ========================================

  @override
  Future<RefundDto> requestRefund(
    String orderId,
    CreateRefundDto request,
  ) async {
    final idempotencyKey =
        '${orderId}_refund_${DateTime.now().millisecondsSinceEpoch}';
    return _executeRequest(
      () => _apiClient.post(
        '/orders/$orderId/refund',
        data: request.toJson(),
        options: Options(headers: {'Idempotency-Key': idempotencyKey}),
      ),
      parser: (data) => RefundDto.fromJson(data as Map<String, dynamic>),
    );
  }

  @override
  Future<RefundDto?> getRefundByOrderId(String orderId) async {
    throw UnsupportedError(
      'GET /orders/:id/refunds is not supported by backend contract.',
    );
  }

  @override
  Future<RefundDto> getRefund(String refundId) async {
    throw UnsupportedError(
      'GET /refunds/:id is not supported by backend contract.',
    );
  }

  @override
  Future<RefundListDto> listMyRefunds({RefundFilterParams? params}) async {
    throw UnsupportedError(
      'GET /refunds is not supported by backend contract.',
    );
  }

  @override
  Future<RefundListDto> listSellerRefunds({RefundFilterParams? params}) async {
    throw UnsupportedError(
      'GET /seller/refunds is not supported by backend contract.',
    );
  }

  // ========================================
  // Refund Decision Operations (H2-D1)
  // ========================================

  @override
  Future<RefundDto> approveRefund(String refundId, {String? notes}) async {
    return _executeRequest(
      () => _apiClient.post(
        '/refunds/$refundId/approve',
        data: notes != null ? {'notes': notes} : null,
      ),
      parser: (data) => RefundDto.fromJson(data as Map<String, dynamic>),
    );
  }

  @override
  Future<RefundDto> rejectRefund(String refundId, {String? notes}) async {
    return _executeRequest(
      () => _apiClient.post(
        '/refunds/$refundId/reject',
        data: notes != null ? {'notes': notes} : null,
      ),
      parser: (data) => RefundDto.fromJson(data as Map<String, dynamic>),
    );
  }

  @override
  Future<Map<String, dynamic>> escalateRefund(String refundId) async {
    return _executeRequest(
      () => _apiClient.post('/refunds/$refundId/escalate'),
      parser: (data) => data as Map<String, dynamic>,
    );
  }

  // ========================================
  // Shipping Operations
  // ========================================

  @override
  Future<CheckDeliveryApiResponse> checkDelivery(
    CheckDeliveryApiRequest request,
  ) async {
    return _executeRequest(
      () => _apiClient.post('/shipping/check', data: request.toJson()),
      parser: (data) =>
          CheckDeliveryApiResponse.fromJson(data as Map<String, dynamic>),
    );
  }

  // ========================================
  // Dispute Operations
  // ========================================

  @override
  Future<DisputeDto> createDispute(
    String orderId,
    CreateDisputeDto request,
  ) async {
    return _executeRequest(
      () => _apiClient.post('/orders/$orderId/dispute', data: request.toJson()),
      parser: (data) => DisputeDto.fromJson(data as Map<String, dynamic>),
    );
  }

  // ========================================
  // Pricing Preview Operations
  // ========================================

  @override
  Future<Map<String, dynamic>> fetchPricingPreview(
    Map<String, dynamic> body,
  ) async {
    return _executeRequest(
      () => _apiClient.post('/pricing/preview', data: body),
      parser: (data) => data as Map<String, dynamic>,
    );
  }

  // ========================================
  // Order Action Operations (Decision V2)
  // ========================================

  @override
  Future<void> extendOrderConfirmation(String orderId) async {
    // Generate idempotency key for this request
    final idempotencyKey =
        '${orderId}_${DateTime.now().millisecondsSinceEpoch}';

    try {
      await _apiClient.post(
        '/orders/$orderId/extend-confirmation',
        options: Options(headers: {'Idempotency-Key': idempotencyKey}),
      );
    } on DioException catch (e) {
      final exception = _apiClient.extractException(e);
      _logger?.error(
        'Failed to extend order confirmation: ${exception.message}',
        extra: {'code': exception.code, 'statusCode': exception.statusCode},
      );
      throw Exception(exception.message);
    } catch (e, stackTrace) {
      _logger?.error(
        'Unexpected error extending confirmation: $e',
        stackTrace: stackTrace,
      );
      throw Exception(e.toString());
    }
  }
}
