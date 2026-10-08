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
