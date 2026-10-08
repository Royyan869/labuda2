/// Seller Repository Interface
///
/// Pure Dart interface - no Firebase/Flutter dependencies.
library;

import '../entities/seller_analytics_read.dart';
import '../entities/seller_performance.dart';
import '../entities/seller_earnings.dart';
import '../entities/seller_subscription.dart';
import '../entities/withdrawal.dart';
import 'package:labuda/core/common/result.dart';

/// Seller Repository Interface
///
/// Aggregates all seller-related operations.
abstract class SellerRepository {
  // ============================================
  // ANALYTICS
  // ============================================

  /// Get seller analytics (30-day read projection over Product View + sales).
  Future<Result<SellerAnalytics>> getAnalytics(String sellerId);

  // ============================================
  // PERFORMANCE
  // ============================================

  /// Get seller performance metrics
  Future<Result<SellerPerformance>> getPerformance(String sellerId);

  // ============================================
  // EARNINGS
  // ============================================

  /// Get seller earnings data
  Future<Result<SellerEarnings>> getEarnings(String sellerId);

  /// Get earnings breakdown by period
  Future<Result<SellerEarnings>> getEarningsBreakdown({
    required String sellerId,
    required DateTime startDate,
    required DateTime endDate,
  });

  /// Get withdrawal history
  Future<Result<List<WithdrawalRecord>>> getWithdrawalHistory({
    required String sellerId,
    int limit = 20,
    int offset = 0,
  });

  // ============================================
  // SUBSCRIPTION
  // ============================================

  /// Get seller subscription status
  Future<Result<SellerSubscription>> getSubscription(String sellerId);

  // ============================================
  // WITHDRAWAL
  // ============================================

  /// Request a withdrawal
  /// Returns a WithdrawResult containing the withdrawal ID and status
  Future<Result<WithdrawResult>> requestWithdraw(WithdrawRequest request);

  /// Get withdrawal history
  ///
  /// [limit] controls how many records to fetch (default 100).
  /// [offset] is the 0-based record offset for pagination (default 0).
  Future<Result<List<Withdrawal>>> getWithdrawHistory({
    int limit = 100,
    int offset = 0,
  });
}
