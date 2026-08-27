/// Seller Remote Datasource
///
/// API-based datasource for seller data - isolated from domain.
library;

import 'dart:async';

import 'package:dio/dio.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/utils/polling_monitor.dart';
import 'package:labuda/domains/user/preference/seller/data/datasources/seller_api_datasource.dart';
import 'package:labuda/domains/user/preference/seller/data/models/api/seller_api_models.dart';

import '../../domain/entities/seller_activity.dart';
import '../dto/seller_dto.dart';

/// Seller Remote Datasource
///
/// Wraps API calls and returns DTOs. Domain entities don't know about API.
class SellerRemoteDatasource {
  final ApiClient _apiClient;
  final SellerApiDatasource _sellerApiDatasource;
  final ILoggerService? _logger;

  // Polling monitors for tracking subscription status
  final Map<String, PollingMonitor> _subscriptionMonitors = {};

  SellerRemoteDatasource({required ApiClient apiClient, ILoggerService? logger})
    : _apiClient = apiClient,
      _sellerApiDatasource = SellerApiDatasource(
        apiClient: apiClient,
        logger: logger,
      ),
      _logger = logger;

  // ============================================
  // DASHBOARD STATS
  // ============================================

  /// Get dashboard stats from API
  Future<DashboardStatsDto> getDashboardStats(String sellerId) async {
    try {
      final response = await _apiClient.get('/seller/dashboard');

      final data = response.data['data'] as Map<String, dynamic>?;

      if (data == null) {
        throw Exception('No data in response');
      }

      return DashboardStatsDto.fromJson(data);
    } on ApiException catch (e) {
      throw Exception('API error: ${e.message}');
    } catch (e) {
      throw Exception('Failed to get dashboard stats: $e');
    }
  }

  // ============================================
  // ANALYTICS
  // ============================================

  /// Get analytics from existing API datasource
  Future<SellerAnalyticsApiModel> getAnalytics() async {
    return await _sellerApiDatasource.getAnalytics();
  }

  /// Get sales trend data
  Future<List<Map<String, dynamic>>> getSalesTrendData({
    required String sellerId,
    int days = 30,
  }) async {
    throw UnsupportedError(
      'GET /seller/analytics/sales-trend is not available in canonical backend routes.',
    );
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
  // ACTIVITY
  // ============================================

  /// Get recent activity
  Future<List<ActivityItemDto>> getRecentActivity(
    String sellerId, {
    int limit = 10,
  }) async {
    throw UnsupportedError(
      'GET /seller/activity is not available in canonical backend routes.',
    );
  }

  /// Get activity history with filter
  Future<List<ActivityItemDto>> getActivityHistory(
    String sellerId, {
    ActivityType? filterType,
    int limit = 100,
  }) async {
    throw UnsupportedError(
      'GET /seller/activity/history is not available in canonical backend routes.',
    );
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

  /// Stream subscription status via polling
  ///
  /// SOURCE OF TRUTH: PostgreSQL (Backend API /users/{id}/subscription)
  /// Uses polling with 30s interval ± jitter to avoid server thundering herd.
  /// Includes backoff on error: 15s -> 30s -> 90s max.
  Stream<Map<String, dynamic>> watchSubscription(String sellerId) {
    // Create monitor for this seller if not exists
    if (!_subscriptionMonitors.containsKey(sellerId) && _logger != null) {
      _subscriptionMonitors[sellerId] = PollingMonitor(
        logger: _logger,
        domain: PollingDomain.subscription,
        operationId: sellerId,
        config: PollingBackoffConfig.subscription,
      );
    }

    final monitor = _subscriptionMonitors[sellerId];

    // Create a controller that supports dynamic interval adjustment
    final controller = StreamController<Map<String, dynamic>>.broadcast();

    // Polling function with monitoring and backoff
    Future<void> poll() async {
      if (controller.isClosed) return;

      try {
        if (monitor != null) {
          await monitor.monitor(() async {
            final data = await getSubscription(sellerId);
            if (!controller.isClosed) {
              controller.add(data);
            }
          });
        } else {
          // Fallback without monitoring if logger not provided
          final data = await getSubscription(sellerId);
          if (!controller.isClosed) {
            controller.add(data);
          }
        }
      } catch (e) {
        // Return error map - let repository handle error state
        if (!controller.isClosed) {
          controller.add({'error': true, 'message': e.toString()});
        }
      }

      // Schedule next poll with current interval
      if (!controller.isClosed) {
        final interval =
            monitor?.getCurrentInterval() ?? const Duration(seconds: 30);
        Timer(interval, poll);
      }
    }

    // Start polling on listen
    controller.onListen = () {
      poll();
    };

    // Clean up on cancel
    controller.onCancel = () {
      _subscriptionMonitors.remove(sellerId);
    };

    return controller.stream;
  }

  /// Get polling status for a subscription (for UI/debugging)
  Map<String, dynamic>? getSubscriptionPollingStatus(String sellerId) {
    final monitor = _subscriptionMonitors[sellerId];
    return monitor?.getStatusSummary();
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
  Future<void> performOnboarding(String storeName) async {
    try {
      final response = await _apiClient.post(
        '/seller/onboarding',
        data: {'store_name': storeName},
      );
      _throwIfApiError(response);
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
  /// Returns a map containing:
  /// - `payment_id`: UUID of the created payment
  /// - `payment_url`: Midtrans Snap redirect URL
  /// - `gross_amount`: Payment amount in cents
  /// - `expired_at`: ISO 8601 payment expiry timestamp
  ///
  /// Rethrows [ApiException] so callers can handle specific error codes:
  /// - `NO_ACTIVE_CONFIG` (503): No subscription config available
  /// - `TOO_EARLY_RENEWAL` (409): Subscription still active
  /// - `MISSING_REQUIREMENTS` (400): Onboarding incomplete
  Future<Map<String, dynamic>> initiateSubscriptionPayment() async {
    try {
      final response = await _apiClient.post('/seller/subscription/initiate');
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
