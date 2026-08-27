/// Withdrawal Entities
///
/// Pure Dart entities for seller withdrawals - no Firebase/Flutter dependencies.
library;

import 'package:equatable/equatable.dart';

/// Withdrawal Status Enum
enum WithdrawalStatus {
  requested('REQUESTED'),
  processing('PROCESSING'),
  submitted('SUBMITTED'),
  settling('SETTLING'),
  settled('SETTLED'),
  completed('COMPLETED'),
  failed('FAILED'),
  failedRetryable('FAILED_RETRYABLE'),
  failedFinal('FAILED_FINAL'),
  // Seller is not on the payout pilot whitelist; withdrawal is administratively
  // blocked. Not final (admin can re-process), not a failure.
  pilotBlocked('PILOT_BLOCKED'),
  unknown('UNKNOWN');

  const WithdrawalStatus(this.value);
  final String value;

  /// Parse from backend wire string.
  ///
  /// Unknown values fall back to [unknown] so the UI does not mislabel them
  /// as a real withdrawal lifecycle state.
  static WithdrawalStatus fromString(String value) {
    return WithdrawalStatus.values.firstWhere(
      (status) => status.value == value,
      orElse: () => WithdrawalStatus.unknown,
    );
  }

  /// True only for terminal states that will not be retried by the system.
  /// Mirrors backend WithdrawalStatus.IsFinal().
  /// pilotBlocked is NOT final — admin can promote it.
  bool get isFinal =>
      this == WithdrawalStatus.settled ||
      this == WithdrawalStatus.completed ||
      this == WithdrawalStatus.failed ||
      this == WithdrawalStatus.failedFinal;

  bool get isPending => !isFinal && this != WithdrawalStatus.unknown;

  bool get isSuccessful =>
      this == WithdrawalStatus.settled || this == WithdrawalStatus.completed;

  /// pilotBlocked is NOT a failure — it is an administrative hold.
  bool get isFailed =>
      this == WithdrawalStatus.failed ||
      this == WithdrawalStatus.failedFinal ||
      this == WithdrawalStatus.failedRetryable;
}

/// Withdrawal Entity
class Withdrawal extends Equatable {
  final String id;
  final String sellerId;
  final double amount;
  final double feeAmount;
  final WithdrawalStatus status;
  final String? bankNameSnapshot;
  final String? bankCodeSnapshot; // For payout rail integration
  final String? accountNumberSnapshot;
  final String? accountHolderSnapshot;
  final DateTime createdAt;
  final DateTime updatedAt;

  const Withdrawal({
    required this.id,
    required this.sellerId,
    required this.amount,
    required this.feeAmount,
    required this.status,
    this.bankNameSnapshot,
    this.bankCodeSnapshot,
    this.accountNumberSnapshot,
    this.accountHolderSnapshot,
    required this.createdAt,
    required this.updatedAt,
  });

  @override
  List<Object?> get props => [
    id,
    sellerId,
    amount,
    feeAmount,
    status,
    bankNameSnapshot,
    bankCodeSnapshot,
    accountNumberSnapshot,
    accountHolderSnapshot,
    createdAt,
    updatedAt,
  ];
}

/// Withdraw Request Entity
class WithdrawRequest extends Equatable {
  final double amount;

  const WithdrawRequest({required this.amount});

  /// Minimum withdrawal amount in Rupiah
  static const double minAmount = 10000;

  /// Maximum withdrawal amount in Rupiah
  static const double maxAmount = 50000000;

  /// Validate if amount is within limits
  bool get isValid => amount >= minAmount && amount <= maxAmount;

  /// Check if amount is below minimum
  bool get isBelowMin => amount < minAmount;

  /// Check if amount exceeds maximum
  bool get exceedsMax => amount > maxAmount;

  @override
  List<Object?> get props => [amount];
}

/// Withdraw Result Entity
class WithdrawResult extends Equatable {
  final String withdrawalId;
  final WithdrawalStatus status;
  final double feeAmount;
  final double totalDebitAmount;
  final String? error;

  const WithdrawResult({
    required this.withdrawalId,
    required this.status,
    this.feeAmount = 0,
    this.totalDebitAmount = 0,
    this.error,
  });

  factory WithdrawResult.success(
    String withdrawalId,
    WithdrawalStatus status, {
    double feeAmount = 0,
    double totalDebitAmount = 0,
  }) {
    return WithdrawResult(
      withdrawalId: withdrawalId,
      status: status,
      feeAmount: feeAmount,
      totalDebitAmount: totalDebitAmount,
    );
  }

  factory WithdrawResult.failure(String error) {
    return WithdrawResult(
      withdrawalId: '',
      status: WithdrawalStatus.failed,
      error: error,
    );
  }

  bool get isSuccess => error == null;

  @override
  List<Object?> get props => [
    withdrawalId,
    status,
    feeAmount,
    totalDebitAmount,
    error,
  ];
}
