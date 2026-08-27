/// Seller Earnings Entities
///
/// Pure Dart entities for seller earnings - no Firebase/Flutter dependencies.
library;

import 'package:equatable/equatable.dart';

/// Withdrawal record
class WithdrawalRecord extends Equatable {
  final String id;
  final double amount;
  final double feeAmount;
  final double totalDebitAmount;
  final DateTime withdrawalDate;
  final WithdrawalRecordStatus status;
  final String? bankAccount;

  const WithdrawalRecord({
    required this.id,
    required this.amount,
    this.feeAmount = 0,
    this.totalDebitAmount = 0,
    required this.withdrawalDate,
    required this.status,
    this.bankAccount,
  });

  @override
  List<Object?> get props => [
    id,
    amount,
    feeAmount,
    totalDebitAmount,
    withdrawalDate,
    status,
    bankAccount,
  ];
}

/// Withdrawal record status
enum WithdrawalRecordStatus { success, pending, failed }

/// Seller Earnings Entity
///
/// **EARNINGS TRUTH (CANONICAL):**
/// This entity represents seller earnings from the backend API.
/// The backend returns 4 core metrics - all other fields are either
/// calculated or placeholder values.
///
/// **BACKEND SOURCE:** GET /api/v1/seller/earnings
/// Returns: available_balance, pending_balance, total_withdrawn, total_earned
///
/// **METRIC DEFINITIONS:**
/// - totalRevenue (from total_earned): Total credits ever to SELLER_PAYABLE account
/// - pendingRevenue (from pending_balance): Escrow for shipped/delivered orders
/// - availableBalance (from available_balance): Matured, withdrawable funds
/// - totalWithdrawn (from total_withdrawn): Sum of COMPLETED withdrawals
///
/// **PLACEHOLDER FIELDS** (not in simplified API, set to 0/null):
/// - totalPlatformFees, totalWithdrawals, lastWithdrawalDate, nextWithdrawalDate
/// - totalCompletedOrders, platformFeePercentage
///
/// **IMPORTANT:**
/// - pending_balance is GROSS escrow (includes platform commission)
/// - Total earned is BEFORE any refunds/adjustments
/// - Use available_balance for actual withdrawable amount
class SellerEarnings extends Equatable {
  final String sellerId;

  /// Total revenue (from backend total_earned)
  /// SOURCE: Sum of all debit entries to SELLER_PAYABLE account
  /// TRUTH: Lifetime credits to seller account (before adjustments)
  final double totalRevenue;

  /// Pending revenue (from backend pending_balance)
  /// SOURCE: Sum(escrow_amount) WHERE status IN ('shipped', 'delivered') AND escrow_status = 'holding'
  /// TRUTH: Gross escrow amount (includes platform commission)
  /// NOTE: This is NOT the seller's net - commission will be deducted upon release
  final double pendingRevenue;

  /// Total platform fees (PLACEHOLDER: not in simplified API)
  /// TODO: Calculate from backend or separate endpoint
  final double totalPlatformFees;

  /// Available balance (from backend available_balance)
  /// SOURCE: SELLER_PAYABLE ledger balance minus dispute freezes
  /// TRUTH: Freeze-aware withdrawable amount
  /// MIN: Rp 10,000 required to withdraw
  final double availableBalance;
  final double withdrawalFeeAmount;

  // Balance breakdown (J1-C): explains why availableBalance may be < grossPayable.
  // Nullable for backward compatibility — old API responses won't have these.

  /// Raw SELLER_PAYABLE ledger balance before freeze deductions.
  final double? grossPayable;

  /// Funds frozen by active disputes.
  final double? activeDisputeFreeze;

  /// Total withdrawn (from backend total_withdrawn)
  /// SOURCE: Sum(withdrawal.amount) WHERE status IN ('SETTLED', 'COMPLETED')
  /// TRUTH: Actual amount successfully withdrawn to seller's bank account
  final double totalWithdrawn;

  /// Total withdrawals count (PLACEHOLDER: not in simplified API)
  final int totalWithdrawals;
  final DateTime? lastWithdrawalDate;
  final DateTime? nextWithdrawalDate;

  /// Total completed orders (PLACEHOLDER: not in simplified API)
  final int totalCompletedOrders;

  final double platformFeePercentage;
  final DateTime calculatedAt;

  const SellerEarnings({
    required this.sellerId,
    required this.totalRevenue,
    required this.pendingRevenue,
    required this.totalPlatformFees,
    required this.availableBalance,
    required this.withdrawalFeeAmount,
    required this.totalWithdrawn,
    required this.totalWithdrawals,
    this.lastWithdrawalDate,
    this.nextWithdrawalDate,
    required this.totalCompletedOrders,
    this.platformFeePercentage = 4.0,
    required this.calculatedAt,
    this.grossPayable,
    this.activeDisputeFreeze,
  });

  /// Net earnings after fees
  /// NOTE: This is only accurate if totalPlatformFees is properly set
  double get netEarnings => totalRevenue - totalPlatformFees;

  /// Average order value
  /// NOTE: This is only accurate if totalCompletedOrders is properly set
  double get averageOrderValue =>
      totalCompletedOrders > 0 ? totalRevenue / totalCompletedOrders : 0;

  /// Whether the balance breakdown fields are available from the backend.
  bool get hasBalanceBreakdown => grossPayable != null;

  /// Create empty earnings
  factory SellerEarnings.empty(String sellerId) {
    return SellerEarnings(
      sellerId: sellerId,
      totalRevenue: 0,
      pendingRevenue: 0,
      totalPlatformFees: 0,
      availableBalance: 0,
      withdrawalFeeAmount: 0,
      totalWithdrawn: 0,
      totalWithdrawals: 0,
      totalCompletedOrders: 0,
      calculatedAt: DateTime.now(),
    );
  }

  @override
  List<Object?> get props => [
    sellerId,
    totalRevenue,
    pendingRevenue,
    totalPlatformFees,
    availableBalance,
    withdrawalFeeAmount,
    totalWithdrawn,
    totalWithdrawals,
    lastWithdrawalDate,
    nextWithdrawalDate,
    totalCompletedOrders,
    platformFeePercentage,
    calculatedAt,
    grossPayable,
    activeDisputeFreeze,
  ];
}
