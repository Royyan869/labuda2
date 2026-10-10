/// Seller Remote Datasource
///
/// API-based datasource for seller data - isolated from domain.
library;

import 'package:dio/dio.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/user/preference/seller/data/datasources/seller_api_datasource.dart';
import 'package:hishumi/domains/user/preference/seller/data/models/api/seller_api_models.dart';

import '../dto/seller_dto.dart';

/// Seller Remote Datasource
///
/// Wraps API calls and returns DTOs. Domain entities don't know about API.
class SellerRemoteDatasource {
  final ApiClient _apiClient;
  final SellerApiDatasource _sellerApiDatasource;

  SellerRemoteDatasource({required ApiClient apiClient, ILoggerService? logger})
    : _apiClient = apiClient,
      _sellerApiDatasource = SellerApiDatasource(
        apiClient: apiClient,
        logger: logger,
      );

  // ============================================
  // ANALYTICS
  // ============================================

  /// Get seller analytics (30-day read projection).
  /// GET /seller/analytics
  Future<Map<String, dynamic>> getAnalytics() async {
    try {
      final response = await _apiClient.get('/seller/analytics');
      return response.data['data'] as Map<String, dynamic>;
    } on ApiException catch (e) {
      throw Exception('API error: ${e.message}');
    } catch (e) {
      throw Exception('Failed to get analytics: $e');
    }
  }

  /// Get performance metrics
  Future<Map<String, dynamic>> getPerformance(String sellerId) async {
    try {
      final response = await _apiClient.get('/seller/performance');
      return response.data['data'] as Map<String, dynamic>;
    } on ApiException catch (e) {
      throw Exception('API error: ${e.message}');
    } catch (e) {
      throw Exception('Failed to get performance: $e');
    }
  }

  // ============================================
  // EARNINGS
  // ============================================

  /// Get earnings from existing API datasource
  Future<SellerEarningsApiModel> getEarnings(String sellerId) async {
    return await _sellerApiDatasource.getEarnings(sellerId);
  }

  /// Get earnings for specific seller
  Future<EarningsDto> getSellerEarnings(String sellerId) async {
    try {
      final response = await _apiClient.get('/seller/earnings');
      final data = response.data['data'] as Map<String, dynamic>;
      return EarningsDto.fromJson(data);
    } on ApiException catch (e) {
      throw Exception('API error: ${e.message}');
    } catch (e) {
      throw Exception('Failed to get earnings: $e');
    }
  }

  // ============================================
  // SUBSCRIPTION
  // ============================================

  /// Get subscription status
  Future<Map<String, dynamic>> getSubscription(String sellerId) async {
    try {
      final response = await _apiClient.get('/seller/subscription');
      return response.data['data'] as Map<String, dynamic>;
    } on ApiException catch (e) {
      throw Exception('API error: ${e.message}');
    } catch (e) {
      throw Exception('Failed to get subscription: $e');
    }
  }

  // ============================================
  // WITHDRAWAL
  // ============================================

  /// Request a withdrawal
  /// POST /api/v1/withdraw
  Future<Map<String, dynamic>> requestWithdraw(int amount) async {
    try {
      final response = await _apiClient.post(
        '/withdraw',
        data: {'amount': amount},
      );

      final data = response.data['data'] as Map<String, dynamic>?;
      if (data == null) {
        throw Exception('No data in response');
      }

      return data;
    } on ApiException catch (e) {
      throw Exception('API error: ${e.message}');
    } catch (e) {
      throw Exception('Failed to request withdrawal: $e');
    }
  }

  /// Get withdrawal history
  /// GET /api/v1/withdraw/history
  ///
  /// [limit] controls how many records to fetch (default 100).
  /// [offset] is the 0-based record offset for pagination (default 0).
  Future<Map<String, dynamic>> getWithdrawHistory({
    int limit = 100,
    int offset = 0,
  }) async {
    try {
      final response = await _apiClient.get(
        '/withdraw/history',
        queryParameters: {'limit': limit, 'offset': offset},
      );

      final data = response.data['data'] as Map<String, dynamic>?;
      if (data == null) {
        throw Exception('No data in response');
      }

      return data;
    } on ApiException catch (e) {
      throw Exception('API error: ${e.message}');
    } catch (e) {
      throw Exception('Failed to get withdrawal history: $e');
    }
  }

  /// Perform seller onboarding
  /// POST /seller/onboarding
  ///
  /// Rethrows the underlying [ApiException] so callers can inspect the
  /// backend's machine-readable error code (e.g. `MISSING_REQUIREMENTS`
  /// with `requires_verification` in `details`, or 403 `EMAIL_VERIFICATION_REQUIRED`,
  /// `ACCOUNT_SUSPENDED`, `ACCOUNT_BANNED`) and surface an actionable message
  /// to the user instead of a generic failure.
  Future<void> performOnboarding(
    String storeName, {
    String? storeImageUrl,
  }) async {
    try {
      final response = await _apiClient.post(
        '/seller/onboarding',
        data: {
          'store_name': storeName,
          if (storeImageUrl != null) 'store_image_url': storeImageUrl,
        },
      );
      _throwIfApiError(response);
    } on DioException catch (e) {
      if (e.error is ApiException) {
        throw e.error as ApiException;
      }
      rethrow;
    }
  }

  /// Get canonical seller profile
  /// GET /seller/profile
  Future<Map<String, dynamic>> getSellerProfile() async {
    try {
      final response = await _apiClient.get('/seller/profile');
      _throwIfApiError(response);
      final data = response.data['data'] as Map<String, dynamic>?;
      if (data == null) throw Exception('No data in response');
      return data;
    } on DioException catch (e) {
      if (e.error is ApiException) throw e.error as ApiException;
      rethrow;
    }
  }

  /// Update canonical seller profile
  /// PATCH /seller/profile
  Future<Map<String, dynamic>> updateSellerProfile({
    String? storeName,
    String? storeImageUrl,
  }) async {
    try {
      final response = await _apiClient.patch(
        '/seller/profile',
        data: {
          if (storeName != null) 'store_name': storeName,
          if (storeImageUrl != null) 'store_image_url': storeImageUrl,
        },
      );
      _throwIfApiError(response);
      final data = response.data['data'] as Map<String, dynamic>?;
      if (data == null) throw Exception('No data in response');
      return data;
    } on DioException catch (e) {
      if (e.error is ApiException) throw e.error as ApiException;
      rethrow;
    }
  }

  /// Get the canonical enabled payment methods for a seller subscription
  /// payment.
  /// GET /seller/subscription/payment-methods
  ///
  /// Each option already carries the backend-calculated payment-method fee F
  /// and the resulting gross A + F for the active subscription principal A.
  ///
  /// PMF-02: every payment flow carries a payment-method fee, so the seller must
  /// explicitly select a method before initiation. The backend is the sole fee
  /// authority — the client never computes a fee or a gross amount, it only
  /// renders the numbers returned here.
  ///
  /// Rethrows [ApiException] so callers can handle specific error codes:
  /// - `NO_ACTIVE_CONFIG` (503): No subscription config available
  Future<SellerSubscriptionPaymentMethodsDto>
  getSubscriptionPaymentMethods() async {
    try {
      final response = await _apiClient.get(
        '/seller/subscription/payment-methods',
      );
      _throwIfApiError(response);

      final data = response.data['data'] as Map<String, dynamic>?;
      if (data == null) {
        throw Exception('No data in response');
      }

      return SellerSubscriptionPaymentMethodsDto.fromJson(data);
    } on DioException catch (e) {
      if (e.error is ApiException) {
        throw e.error as ApiException;
      }
      rethrow;
    }
  }

  /// Initiate subscription payment via Midtrans Snap.
  /// POST /seller/subscription/initiate
  ///
  /// [paymentMethodCode] is REQUIRED and must be one of the codes returned by
  /// [getSubscriptionPaymentMethods]. The backend looks the method up, validates
  /// it is enabled, and calculates the payment-method fee itself (PMF-02) — the
  /// client sends only the code.
  ///
  /// Returns a map containing:
  /// - `payment_id`: UUID of the created payment
  /// - `payment_url`: Midtrans Snap redirect URL
  /// - `gross_amount`: Rupiah integer the gateway will charge (A + F)
  /// - `expired_at`: ISO 8601 payment expiry timestamp
  ///
  /// Rethrows [ApiException] so callers can handle specific error codes:
  /// - `NO_ACTIVE_CONFIG` (503): No subscription config available
  /// - `TOO_EARLY_RENEWAL` (409): Subscription still active
  /// - `MISSING_REQUIREMENTS` (400): Onboarding incomplete
  /// - `BAD_REQUEST` (400): Unknown or disabled payment_method_code
  Future<Map<String, dynamic>> initiateSubscriptionPayment({
    required String paymentMethodCode,
  }) async {
    try {
      final response = await _apiClient.post(
        '/seller/subscription/initiate',
        data: {'payment_method_code': paymentMethodCode},
      );
      _throwIfApiError(response);

      final data = response.data['data'] as Map<String, dynamic>?;
      if (data == null) {
        throw Exception('No data in response');
      }

      return data;
    } on DioException catch (e) {
      if (e.error is ApiException) {
        throw e.error as ApiException;
      }
      rethrow;
    }
  }

  void _throwIfApiError(Response<dynamic> response) {
    final statusCode = response.statusCode ?? 200;
    if (statusCode < 400) return;

    final data = response.data;
    String message = 'Request failed';
    String? code;
    dynamic details;

    if (data is Map<String, dynamic>) {
      final error = data['error'];
      if (error is Map<String, dynamic>) {
        message = error['message']?.toString() ?? message;
        code = error['code']?.toString();
        details = error['details'];
      } else if (data['message'] is String) {
        message = data['message'] as String;
      }
    } else if (data is String && data.isNotEmpty) {
      message = data;
    }

    throw ApiExceptionFactory.fromStatusCode(
      statusCode,
      message,
      code: code,
      details: details,
    );
  }
}
