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
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';

/// Seller Repository Interface
///
/// Aggregates all seller-related operations.
abstract class SellerRepository {
  // ============================================
  // DASHBOARD STATS
  // ============================================

  /// Get seller dashboard statistics
  Future<RepositoryResult<SellerDashboardStats>> getDashboardStats(
    String sellerId,
  );

  // ============================================
  // ANALYTICS
  // ============================================

  /// Get seller analytics for a specific period
  Future<RepositoryResult<SellerAnalytics>> getAnalytics({
    required String sellerId,
    required AnalyticsPeriod period,
    required DateTime startDate,
    required DateTime endDate,
  });

  /// Get seller performance metrics
  Future<RepositoryResult<SellerPerformance>> getPerformance(String sellerId);

  /// Get sales trend data points for charts
  Future<RepositoryResult<List<SalesDataPoint>>> getSalesTrendData({
    required String sellerId,
    int days = 30,
  });

  // ============================================
  // EARNINGS
  // ============================================

  /// Get seller earnings data
  Future<RepositoryResult<SellerEarnings>> getEarnings(String sellerId);

  /// Get earnings breakdown by period
  Future<RepositoryResult<SellerEarnings>> getEarningsBreakdown({
    required String sellerId,
    required DateTime startDate,
    required DateTime endDate,
  });

  /// Get withdrawal history
  Future<RepositoryResult<List<WithdrawalRecord>>> getWithdrawalHistory({
    required String sellerId,
    int limit = 20,
    int offset = 0,
  });

  // ============================================
  // ACTIVITY
  // ============================================

  /// Get recent activity for seller
  Future<RepositoryResult<List<RecentActivityItem>>> getRecentActivity(
    String sellerId, {
    int limit = 10,
  });

  /// Get activity history with optional filter
  Future<RepositoryResult<List<RecentActivityItem>>> getActivityHistory(
    ActivityHistoryParams params, {
    int limit = 100,
  });

  // ============================================
  // SUBSCRIPTION
  // ============================================

  /// Get seller subscription status
  Future<RepositoryResult<SellerSubscription>> getSubscription(String sellerId);

  /// Stream seller subscription for real-time updates
  Stream<SellerSubscription?> watchSubscription(String sellerId);

  // ============================================
  // WITHDRAWAL
  // ============================================

  /// Request a withdrawal
  /// Returns a WithdrawResult containing the withdrawal ID and status
  Future<RepositoryResult<WithdrawResult>> requestWithdraw(
    WithdrawRequest request,
  );

  /// Get withdrawal history
  ///
  /// [limit] controls how many records to fetch (default 100).
  /// [offset] is the 0-based record offset for pagination (default 0).
  Future<RepositoryResult<List<Withdrawal>>> getWithdrawHistory({
    int limit = 100,
    int offset = 0,
  });
}
