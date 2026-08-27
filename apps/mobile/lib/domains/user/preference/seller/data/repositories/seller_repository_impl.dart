/// Seller Repository Implementation
///
/// Implements repository interface using remote datasource and mappers.
/// NO FALLBACK LOGIC - all data comes from backend API.
library;

import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';

import '../../domain/entities/seller_dashboard.dart';
import '../../domain/entities/seller_analytics.dart';
import '../../domain/entities/seller_earnings.dart';
import '../../domain/entities/seller_activity.dart';
import '../../domain/entities/seller_subscription.dart';
import '../../domain/entities/withdrawal.dart';
import '../../domain/repositories/seller_repository.dart';
import '../dto/withdraw_dto.dart';
import '../mappers/seller_mapper.dart';
import '../mappers/withdraw_mapper.dart';
import '../remote/seller_remote_datasource.dart';

/// Seller Repository Implementation
///
/// CONTEST DOMAIN REMOVED: Contest dependencies removed as Contest domain has been sunset
class SellerRepositoryImpl implements SellerRepository {
  final SellerRemoteDatasource _remoteDatasource;

  SellerRepositoryImpl({required SellerRemoteDatasource remoteDatasource})
    : _remoteDatasource = remoteDatasource;

  // ============================================
  // DASHBOARD STATS
  // ============================================

  @override
  Future<RepositoryResult<SellerDashboardStats>> getDashboardStats(
    String sellerId,
  ) async {
    try {
      final dto = await _remoteDatasource.getDashboardStats(sellerId);
      return RepositoryResult.success(DashboardStatsMapper.toEntity(dto));
    } catch (e) {
      return RepositoryResult.error('Failed to get dashboard stats: $e');
    }
  }

  // ============================================
  // ANALYTICS
  // ============================================

  @override
  Future<RepositoryResult<SellerAnalytics>> getAnalytics({
    required String sellerId,
    required AnalyticsPeriod period,
    required DateTime startDate,
    required DateTime endDate,
  }) async {
    try {
      final apiModel = await _remoteDatasource.getAnalytics();

      // Convert API model to entity
      final salesDataJson = await _remoteDatasource.getSalesTrendData(
        sellerId: sellerId,
        days: endDate.difference(startDate).inDays,
      );

      return RepositoryResult.success(
        SellerAnalyticsMapper.toEntity(
          sellerId: sellerId,
          period: period,
          currentPeriodData: {
            'sales': apiModel.totalSales,
            'orders': apiModel.totalOrders,
          },
          previousPeriodData: {
            'sales': 0.0, // TODO: implement previous period calculation
            'orders': 0, // TODO: implement previous period calculation
          },
          salesDataJson: salesDataJson,
          periodStart: startDate,
          periodEnd: endDate,
        ),
      );
    } catch (e) {
      return RepositoryResult.error('Failed to get analytics: $e');
    }
  }

  @override
  Future<RepositoryResult<SellerPerformance>> getPerformance(
    String sellerId,
  ) async {
    try {
      final perfJson = await _remoteDatasource.getPerformance(sellerId);
      return RepositoryResult.success(
        SellerPerformanceMapper.toEntity(sellerId: sellerId, json: perfJson),
      );
    } catch (e) {
      return RepositoryResult.error('Failed to get performance: $e');
    }
  }

  @override
  Future<RepositoryResult<List<SalesDataPoint>>> getSalesTrendData({
    required String sellerId,
    int days = 30,
  }) async {
    try {
      final salesJson = await _remoteDatasource.getSalesTrendData(
        sellerId: sellerId,
        days: days,
      );

      final dataPoints = salesJson
          .map((json) => SalesDataPointMapper.toEntity(json))
          .toList();

      return RepositoryResult.success(dataPoints);
    } catch (e) {
      return RepositoryResult.error('Failed to get sales trend: $e');
    }
  }

  // ============================================
  // EARNINGS
  // ============================================

  @override
  Future<RepositoryResult<SellerEarnings>> getEarnings(String sellerId) async {
    try {
      final apiModel = await _remoteDatasource.getEarnings(sellerId);
      final earnings = SellerEarningsMapper.fromApiModel(
        sellerId: sellerId,
        apiModel: apiModel,
      );
      return RepositoryResult.success(earnings);
    } catch (e) {
      return RepositoryResult.error('Failed to get earnings: $e');
    }
  }

  @override
  Future<RepositoryResult<SellerEarnings>> getEarningsBreakdown({
    required String sellerId,
    required DateTime startDate,
    required DateTime endDate,
  }) async {
    // For now, delegate to getEarnings
    // API can be enhanced to support period breakdown
    return getEarnings(sellerId);
  }

  @override
  Future<RepositoryResult<List<WithdrawalRecord>>> getWithdrawalHistory({
    required String sellerId,
    int limit = 20,
    int offset = 0,
  }) async {
    try {
      // GET /withdraw/history — auth-scoped, no sellerId param needed.
      final responseJson = await _remoteDatasource.getWithdrawHistory();
      final withdrawalsList = responseJson['withdrawals'] as List<dynamic>;

      final withdrawals = withdrawalsList.map((item) {
        final json = item as Map<String, dynamic>;
        return WithdrawalRecord(
          id: json['withdrawal_id'] as String,
          amount: (json['amount'] as num).toDouble(),
          feeAmount: (json['fee_amount'] as num?)?.toDouble() ?? 0.0,
          totalDebitAmount:
              (json['total_debit_amount'] as num?)?.toDouble() ??
              ((json['amount'] as num).toDouble() +
                  ((json['fee_amount'] as num?)?.toDouble() ?? 0.0)),
          withdrawalDate:
              DateTime.tryParse(
                json['requested_at'] as String? ??
                    json['created_at'] as String? ??
                    '',
              ) ??
              DateTime.fromMillisecondsSinceEpoch(0),
          status: _parseWithdrawalRecordStatus(
            json['status'] as String? ?? 'REQUESTED',
          ),
          bankAccount: null, // not included in /withdraw/history response
        );
      }).toList();

      return RepositoryResult.success(withdrawals);
    } catch (e) {
      return RepositoryResult.error('Failed to get withdrawal history: $e');
    }
  }

  WithdrawalRecordStatus _parseWithdrawalRecordStatus(String status) {
    switch (status.toUpperCase()) {
      case 'SETTLED':
      case 'COMPLETED':
        return WithdrawalRecordStatus.success;
      case 'FAILED':
      case 'FAILED_FINAL':
      case 'FAILED_RETRYABLE':
        return WithdrawalRecordStatus.failed;
      default:
        return WithdrawalRecordStatus.pending;
    }
  }

  // ============================================
  // ACTIVITY
  // ============================================

  @override
  Future<RepositoryResult<List<RecentActivityItem>>> getRecentActivity(
    String sellerId, {
    int limit = 10,
  }) async {
    try {
      final dtos = await _remoteDatasource.getRecentActivity(
        sellerId,
        limit: limit,
      );
      return RepositoryResult.success(
        dtos.map(ActivityItemMapper.toEntity).toList(),
      );
    } catch (e) {
      return RepositoryResult.error('Failed to get recent activity: $e');
    }
  }

  @override
  Future<RepositoryResult<List<RecentActivityItem>>> getActivityHistory(
    ActivityHistoryParams params, {
    int limit = 100,
  }) async {
    try {
      final dtos = await _remoteDatasource.getActivityHistory(
        params.sellerId,
        filterType: params.filterType,
        limit: limit,
      );
      return RepositoryResult.success(
        dtos.map(ActivityItemMapper.toEntity).toList(),
      );
    } catch (e) {
      return RepositoryResult.error('Failed to get activity history: $e');
    }
  }

  // ============================================
  // SUBSCRIPTION
  // ============================================

  @override
  Future<RepositoryResult<SellerSubscription>> getSubscription(
    String sellerId,
  ) async {
    try {
      final json = await _remoteDatasource.getSubscription(sellerId);

      final isActive =
          json['is_active'] as bool? ?? json['isActive'] as bool? ?? false;
      final yearlyFee =
          (json['yearly_fee'] as num?)?.toDouble() ??
          (json['yearlyFee'] as num?)?.toDouble() ??
          0.0;
      final startDateRaw =
          json['start_date'] as String? ??
          json['startDate'] as String? ??
          DateTime.now().toIso8601String();
      final expiryDateRaw =
          json['expiry_date'] as String? ??
          json['expiryDate'] as String? ??
          DateTime.now().toIso8601String();
      final paymentID =
          json['payment_id'] as String? ?? json['paymentId'] as String? ?? '';
      final createdAtRaw =
          json['created_at'] as String? ??
          json['createdAt'] as String? ??
          DateTime.now().toIso8601String();
      final lastRenewalRaw =
          json['last_renewal_date'] as String? ??
          json['lastRenewalDate'] as String?;

      return RepositoryResult.success(
        SellerSubscription(
          isActive: isActive,
          yearlyFee: yearlyFee,
          startDate: DateTime.parse(startDateRaw),
          expiryDate: DateTime.parse(expiryDateRaw),
          status: _parseSubscriptionStatus(json['status'] as String?),
          paymentId: paymentID,
          createdAt: DateTime.parse(createdAtRaw),
          lastRenewalDate: lastRenewalRaw != null
              ? DateTime.parse(lastRenewalRaw)
              : null,
        ),
      );
    } catch (e) {
      return RepositoryResult.error('Failed to get subscription: $e');
    }
  }

  @override
  Stream<SellerSubscription?> watchSubscription(String sellerId) {
    // SOURCE OF TRUTH: PostgreSQL (Backend API /users/{id}/subscription)
    // Uses polling via datasource - NO Firestore fallback
    return _remoteDatasource.watchSubscription(sellerId).map((data) {
      // Handle error from datasource
      if (data.containsKey('error') && data['error'] == true) {
        return SellerSubscription.empty();
      }

      return SellerSubscription(
        isActive:
            data['is_active'] as bool? ?? data['isActive'] as bool? ?? false,
        yearlyFee:
            (data['yearly_fee'] as num?)?.toDouble() ??
            (data['yearlyFee'] as num?)?.toDouble() ??
            0.0,
        startDate: data['start_date'] != null
            ? DateTime.parse(data['start_date'] as String)
            : data['startDate'] != null
            ? DateTime.parse(data['startDate'] as String)
            : DateTime.now(),
        expiryDate: data['expiry_date'] != null
            ? DateTime.parse(data['expiry_date'] as String)
            : data['expiryDate'] != null
            ? DateTime.parse(data['expiryDate'] as String)
            : DateTime.now(),
        status: _parseSubscriptionStatus(data['status'] as String?),
        paymentId:
            data['payment_id'] as String? ?? data['paymentId'] as String? ?? '',
        createdAt: data['created_at'] != null
            ? DateTime.parse(data['created_at'] as String)
            : data['createdAt'] != null
            ? DateTime.parse(data['createdAt'] as String)
            : DateTime.now(),
        lastRenewalDate: data['last_renewal_date'] != null
            ? DateTime.parse(data['last_renewal_date'] as String)
            : data['lastRenewalDate'] != null
            ? DateTime.parse(data['lastRenewalDate'] as String)
            : null,
      );
    });
  }

  SubscriptionStatus _parseSubscriptionStatus(String? status) {
    return SubscriptionStatusExtension.parse(status);
  }

  // ============================================
  // WITHDRAWAL
  // ============================================

  @override
  Future<RepositoryResult<WithdrawResult>> requestWithdraw(
    WithdrawRequest request,
  ) async {
    try {
      if (!request.isValid) {
        String error;
        if (request.isBelowMin) {
          error =
              'Minimum withdrawal amount is Rp ${WithdrawRequest.minAmount.toStringAsFixed(0)}';
        } else if (request.exceedsMax) {
          error =
              'Maximum withdrawal amount is Rp ${WithdrawRequest.maxAmount.toStringAsFixed(0)}';
        } else {
          error = 'Invalid withdrawal amount';
        }
        return RepositoryResult.error(error);
      }

      // Convert request to DTO
      final requestDto = WithdrawalMapper.requestToDto(request);

      // Call remote datasource
      final responseJson = await _remoteDatasource.requestWithdraw(
        requestDto.amount,
      );

      // Parse response
      final responseDto = WithdrawResponseDto.fromJson(responseJson);

      // Convert to result
      final result = WithdrawalMapper.responseToResult(responseDto);

      return RepositoryResult.success(result);
    } catch (e) {
      // Parse error for common cases
      final errorStr = e.toString().toLowerCase();
      String errorMessage = 'Failed to request withdrawal: $e';

      if (errorStr.contains('insufficient') || errorStr.contains('balance')) {
        errorMessage = 'Insufficient available balance';
      } else if (errorStr.contains('verified')) {
        errorMessage = 'You must be verified to withdraw funds';
      } else if (errorStr.contains('ditinjau') ||
          errorStr.contains('not reviewed') ||
          errorStr.contains('bank_account_not_reviewed')) {
        errorMessage =
            'Rekening bank Anda belum ditinjau oleh admin untuk pencairan dana. Silakan hubungi admin atau gunakan rekening yang sudah terdaftar sebelum perubahan terakhir.';
      } else if (errorStr.contains('bank')) {
        errorMessage = 'Please add a default bank account first';
      } else if (errorStr.contains('minimum')) {
        errorMessage =
            'Minimum withdrawal amount is Rp ${WithdrawRequest.minAmount.toStringAsFixed(0)}';
      } else if (errorStr.contains('maximum')) {
        errorMessage =
            'Maximum withdrawal amount is Rp ${WithdrawRequest.maxAmount.toStringAsFixed(0)}';
      }

      return RepositoryResult.error(errorMessage);
    }
  }

  @override
  Future<RepositoryResult<List<Withdrawal>>> getWithdrawHistory({
    int limit = 100,
    int offset = 0,
  }) async {
    try {
      final responseJson = await _remoteDatasource.getWithdrawHistory(
        limit: limit,
        offset: offset,
      );

      final historyDto = WithdrawHistoryResponseDto.fromJson(responseJson);

      final withdrawals = WithdrawalMapper.fromDtoList(historyDto.withdrawals);

      return RepositoryResult.success(withdrawals);
    } catch (e) {
      return RepositoryResult.error('Failed to get withdrawal history: $e');
    }
  }
}
