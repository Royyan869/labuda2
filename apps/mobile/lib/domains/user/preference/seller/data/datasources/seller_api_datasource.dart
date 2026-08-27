/// Seller API Datasource
///
/// API-based datasource for seller data
library;

import 'package:labuda/core/api/api.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/domains/user/preference/seller/data/models/api/seller_api_models.dart';

/// Seller API Datasource
///
/// Handles HTTP operations for seller-related data
class SellerApiDatasource {
  final ApiClient _apiClient;
  final ILoggerService? _logger;

  SellerApiDatasource({required ApiClient apiClient, ILoggerService? logger})
    : _apiClient = apiClient,
      _logger = logger;

  // ============================================
  // DASHBOARD STATS
  // ============================================

  /// Get dashboard stats from API
  Future<Map<String, dynamic>> getDashboardStats(String sellerId) async {
    try {
      _logger?.info('Fetching dashboard stats for seller: $sellerId').ignore();
      final response = await _apiClient.get('/seller/dashboard');
      return response.data['data'] as Map<String, dynamic>? ??
          _emptyDashboardStats();
    } on ApiException catch (e) {
      _logger?.error('API error in getDashboardStats: ${e.message}').ignore();
      rethrow;
    } catch (e) {
      _logger?.error('Failed to get dashboard stats: $e').ignore();
      rethrow;
    }
  }

  Map<String, dynamic> _emptyDashboardStats() {
    return {
      'total_orders': 0,
      'pending_orders': 0,
      'processing_orders': 0,
      'completed_orders': 0,
      'cancelled_orders': 0,
      'refunded_orders': 0,
      'problematic_orders': 0,
      'total_revenue': 0.0,
      'pending_revenue': 0.0,
      'refunded_revenue': 0.0,
      'total_collections': 0,
      'active_collections': 0,
      'sold_collections': 0,
      'total_auctions': 0,
      'active_auctions': 0,
    };
  }

  // ============================================
  // ANALYTICS
  // ============================================

  /// Get analytics from API
  Future<SellerAnalyticsApiModel> getAnalytics({
    String? sellerId,
    int days = 30,
  }) async {
    try {
      _logger?.info('Fetching analytics for seller: $sellerId').ignore();
      final path = '/seller/analytics';
      final response = await _apiClient.get(
        path,
        queryParameters: {'days': days},
      );
      final data = response.data['data'] as Map<String, dynamic>?;

      if (data == null) {
        return _emptyAnalytics();
      }

      return SellerAnalyticsApiModel(
        totalSales: (data['total_sales'] as num?)?.toDouble() ?? 0.0,
        totalOrders: data['total_orders'] as int? ?? 0,
        averageOrderValue:
            (data['average_order_value'] as num?)?.toDouble() ?? 0.0,
        salesData: _parseSalesData(data['sales_data'] as List?),
        calculatedAt:
            DateTime.tryParse(data['calculated_at'] as String? ?? '') ??
            DateTime.now(),
      );
    } on ApiException catch (e) {
      _logger?.error('API error in getAnalytics: ${e.message}').ignore();
      return _emptyAnalytics();
    } catch (e) {
      _logger?.error('Failed to get analytics: $e').ignore();
      return _emptyAnalytics();
    }
  }

  SellerAnalyticsApiModel _emptyAnalytics() {
    return SellerAnalyticsApiModel(
      totalSales: 0.0,
      totalOrders: 0,
      averageOrderValue: 0.0,
      salesData: const [],
      calculatedAt: DateTime.now(),
    );
  }

  List<SalesDataPointApiModel> _parseSalesData(List? data) {
    if (data == null) return const [];
    return data
        .whereType<Map<String, dynamic>>()
        .map((e) => SalesDataPointApiModel.fromJson(e))
        .toList();
  }

  /// Get performance metrics
  Future<Map<String, dynamic>> getPerformance(String sellerId) async {
    try {
      _logger?.info('Fetching performance for seller: $sellerId').ignore();
      final response = await _apiClient.get('/seller/performance');
      return response.data['data'] as Map<String, dynamic>? ??
          _emptyPerformance();
    } on ApiException catch (e) {
      _logger?.error('API error in getPerformance: ${e.message}').ignore();
      rethrow;
    } catch (e) {
      _logger?.error('Failed to get performance: $e').ignore();
      rethrow;
    }
  }

  Map<String, dynamic> _emptyPerformance() {
    return {
      'satisfactionScore': 0.0,
      'totalReviews': 0,
      'fiveStarReviews': 0,
      'fourStarReviews': 0,
      'threeStarReviews': 0,
      'twoStarReviews': 0,
      'oneStarReviews': 0,
      'avgResponseTime': 0.0,
      'fulfillmentRate': 0.0,
      'onTimeDeliveryRate': 0.0,
      'completedOrders': 0,
      'failedOrders': 0,
      'avgProcessingTime': 0.0,
      'returnRate': 0.0,
      'repeatCustomerRate': 0.0,
      'updatedAt': DateTime.now().toIso8601String(),
    };
  }

  // ============================================
  // EARNINGS
  // ============================================

  /// Get earnings from API
  /// Matches backend GET /api/v1/seller/earnings response
  Future<SellerEarningsApiModel> getEarnings(String sellerId) async {
    try {
      _logger?.info('Fetching earnings for seller: $sellerId').ignore();
      final response = await _apiClient.get('/seller/earnings');
      final data = response.data['data'] as Map<String, dynamic>?;

      if (data == null) {
        return _emptyEarnings();
      }

      return SellerEarningsApiModel(
        availableBalance:
            (data['available_balance'] as num?)?.toDouble() ?? 0.0,
        pendingBalance: (data['pending_balance'] as num?)?.toDouble() ?? 0.0,
        totalWithdrawn: (data['total_withdrawn'] as num?)?.toDouble() ?? 0.0,
        totalEarned: (data['total_earned'] as num?)?.toDouble() ?? 0.0,
        withdrawalFeeAmount:
            (data['withdrawal_fee_amount'] as num?)?.toDouble() ?? 0.0,
        grossPayable: (data['gross_payable'] as num?)?.toDouble(),
        activeDisputeFreeze: (data['active_dispute_freeze'] as num?)
            ?.toDouble(),
        withdrawableBalance: (data['withdrawable_balance'] as num?)?.toDouble(),
      );
    } on ApiException catch (e) {
      _logger?.error('API error in getEarnings: ${e.message}').ignore();
      return _emptyEarnings();
    } catch (e) {
      _logger?.error('Failed to get earnings: $e').ignore();
      return _emptyEarnings();
    }
  }

  SellerEarningsApiModel _emptyEarnings() {
    return SellerEarningsApiModel(
      availableBalance: 0.0,
      pendingBalance: 0.0,
      totalWithdrawn: 0.0,
      totalEarned: 0.0,
      withdrawalFeeAmount: 0.0,
    );
  }

  // ============================================
  // ACTIVITY
  // ============================================

  /// Get recent activity
  Future<List<Map<String, dynamic>>> getRecentActivity(
    String sellerId, {
    int limit = 10,
  }) async {
    throw UnsupportedError(
      'GET /seller/activity is not available in canonical backend routes.',
    );
  }

  /// Get activity history with filter
  Future<List<Map<String, dynamic>>> getActivityHistory(
    String sellerId, {
    String? filterType,
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
      _logger?.info('Fetching subscription for seller: $sellerId').ignore();
      final response = await _apiClient.get('/seller/subscription');
      return response.data['data'] as Map<String, dynamic>? ??
          _emptySubscription();
    } on ApiException catch (e) {
      _logger?.error('API error in getSubscription: ${e.message}').ignore();
      rethrow;
    } catch (e) {
      _logger?.error('Failed to get subscription: $e').ignore();
      rethrow;
    }
  }

  Map<String, dynamic> _emptySubscription() {
    final now = DateTime.now();
    return {
      'is_active': false,
      'yearly_fee': 0.0,
      'start_date': now.toIso8601String(),
      'expiry_date': now.toIso8601String(),
      'status': 'expired',
      'payment_id': '',
      'created_at': now.toIso8601String(),
    };
  }
}
