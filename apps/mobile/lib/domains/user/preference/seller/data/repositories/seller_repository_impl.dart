/// Seller Repository Implementation
///
/// Implements repository interface using remote datasource and mappers.
/// NO FALLBACK LOGIC - all data comes from backend API.
library;

import 'package:labuda/core/common/result.dart';
import 'package:labuda/shared/domain/entities/resource_projection.dart';

import '../../domain/entities/seller_analytics_read.dart';
import '../../domain/entities/seller_performance.dart';
import '../../domain/entities/seller_earnings.dart';
import '../../domain/entities/seller_subscription.dart';
import '../../domain/entities/withdrawal.dart';
import '../../domain/repositories/seller_repository.dart';
import '../dto/seller_analytics_dto.dart';
import '../dto/withdraw_dto.dart';
import '../mappers/seller_mapper.dart';
import '../mappers/withdraw_mapper.dart';
import '../remote/seller_remote_datasource.dart';

/// Seller Repository Implementation
///
class SellerRepositoryImpl implements SellerRepository {
  final SellerRemoteDatasource _remoteDatasource;

  SellerRepositoryImpl({required SellerRemoteDatasource remoteDatasource})
    : _remoteDatasource = remoteDatasource;

  // ============================================
  // ANALYTICS
  // ============================================

  @override
  Future<Result<SellerAnalytics>> getAnalytics(String sellerId) async {
    try {
      final json = await _remoteDatasource.getAnalytics();
      final dto = SellerAnalyticsDto.fromJson(json);
      return Result.success(
        SellerAnalytics(summary: dto.summary, products: dto.products),
      );
    } catch (e) {
      return Result.error('Failed to get analytics: $e');
    }
  }

  // ============================================
  // PERFORMANCE
  // ============================================

  @override
  Future<Result<SellerPerformance>> getPerformance(String sellerId) async {
    try {
      final perfJson = await _remoteDatasource.getPerformance(sellerId);
      return Result.success(
        SellerPerformanceMapper.toEntity(sellerId: sellerId, json: perfJson),
      );
    } catch (e) {
      return Result.error('Failed to get performance: $e');
    }
  }

  // ============================================
  // EARNINGS
  // ============================================

  @override
  Future<Result<SellerEarnings>> getEarnings(String sellerId) async {
    try {
      final apiModel = await _remoteDatasource.getEarnings(sellerId);
      final earnings = SellerEarningsMapper.fromApiModel(
        sellerId: sellerId,
        apiModel: apiModel,
      );
      return Result.success(earnings);
    } catch (e) {
      return Result.error('Failed to get earnings: $e');
    }
  }

  @override
  Future<Result<SellerEarnings>> getEarningsBreakdown({
    required String sellerId,
    required DateTime startDate,
    required DateTime endDate,
  }) async {
    // For now, delegate to getEarnings
    // API can be enhanced to support period breakdown
    return getEarnings(sellerId);
  }

  @override
  Future<Result<List<WithdrawalRecord>>> getWithdrawalHistory({
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

      return Result.success(withdrawals);
    } catch (e) {
      return Result.error('Failed to get withdrawal history: $e');
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
  // SUBSCRIPTION
  // ============================================

  @override
  Future<Result<SellerSubscription>> getSubscription(String sellerId) async {
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

      return Result.success(
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
      return Result.error('Failed to get subscription: $e');
    }
  }

  SubscriptionStatus _parseSubscriptionStatus(String? status) {
    return SubscriptionStatusExtension.parse(status);
  }

  // ============================================
  // WITHDRAWAL
  // ============================================

  @override
  Future<Result<WithdrawResult>> requestWithdraw(
    WithdrawRequest request,
  ) async {
    try {
      if (!request.isValid) {
        String error;
        if (request.isBelowMin) {
          error =
              'Minimum withdrawal amount is Rp ${formatGroupedAmount(WithdrawRequest.minAmount.round())}';
        } else if (request.exceedsMax) {
          error =
              'Maximum withdrawal amount is Rp ${formatGroupedAmount(WithdrawRequest.maxAmount.round())}';
        } else {
          error = 'Invalid withdrawal amount';
        }
        return Result.error(error);
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

      return Result.success(result);
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
            'Minimum withdrawal amount is Rp ${formatGroupedAmount(WithdrawRequest.minAmount.round())}';
      } else if (errorStr.contains('maximum')) {
        errorMessage =
            'Maximum withdrawal amount is Rp ${formatGroupedAmount(WithdrawRequest.maxAmount.round())}';
      }

      return Result.error(errorMessage);
    }
  }

  @override
  Future<Result<List<Withdrawal>>> getWithdrawHistory({
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

      return Result.success(withdrawals);
    } catch (e) {
      return Result.error('Failed to get withdrawal history: $e');
    }
  }
}
