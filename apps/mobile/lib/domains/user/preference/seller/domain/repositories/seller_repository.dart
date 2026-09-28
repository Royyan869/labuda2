/// Seller Repository Interface
///
/// Pure Dart interface - no Firebase/Flutter dependencies.
library;

import '../entities/seller_dashboard.dart';
import '../entities/seller_analytics.dart';
import '../entities/seller_earnings.dart';
import '../entities/seller_activity.dart';
import '../entities/seller_subscription.dart';
import '../entities/withdrawal.dart';
import 'package:labuda/core/common/result.dart';

/// Seller Repository Interface
///
/// Aggregates all seller-related operations.
abstract class SellerRepository {
  // ============================================
  // DASHBOARD STATS
  // ============================================

  /// Get seller dashboard statistics
  Future<Result<SellerDashboardStats>> getDashboardStats(
    String sellerId,
  );

  // ============================================
  // ANALYTICS
  // ============================================

  /// Get seller analytics for a specific period
  Future<Result<SellerAnalytics>> getAnalytics({
    required String sellerId,
    required AnalyticsPeriod period,
    required DateTime startDate,
    required DateTime endDate,
  });

  /// Get seller performance metrics
  Future<Result<SellerPerformance>> getPerformance(String sellerId);

  /// Get sales trend data points for charts
  Future<Result<List<SalesDataPoint>>> getSalesTrendData({
    required String sellerId,
    int days = 30,
  });

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
  // ACTIVITY
  // ============================================

  /// Get recent activity for seller
  Future<Result<List<RecentActivityItem>>> getRecentActivity(
    String sellerId, {
    int limit = 10,
  });

  /// Get activity history with optional filter
  Future<Result<List<RecentActivityItem>>> getActivityHistory(
    ActivityHistoryParams params, {
    int limit = 100,
  });

  // ============================================
  // SUBSCRIPTION
  // ============================================

  /// Get seller subscription status
  Future<Result<SellerSubscription>> getSubscription(String sellerId);

  /// Stream seller subscription for real-time updates
  Stream<SellerSubscription?> watchSubscription(String sellerId);

  // ============================================
  // WITHDRAWAL
  // ============================================

  /// Request a withdrawal
  /// Returns a WithdrawResult containing the withdrawal ID and status
  Future<Result<WithdrawResult>> requestWithdraw(
    WithdrawRequest request,
  );

  /// Get withdrawal history
  ///
  /// [limit] controls how many records to fetch (default 100).
  /// [offset] is the 0-based record offset for pagination (default 0).
  Future<Result<List<Withdrawal>>> getWithdrawHistory({
    int limit = 100,
    int offset = 0,
  });
}
