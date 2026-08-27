/// Withdraw State
///
/// Sealed class states for withdraw operations.
library;

import '../../domain/entities/withdrawal.dart';

/// Base state class
abstract class WithdrawState {
  const WithdrawState();
}

/// Initial state
class WithdrawInitial extends WithdrawState {
  const WithdrawInitial();
}

/// Loading state
class WithdrawLoading extends WithdrawState {
  const WithdrawLoading();
}

/// Withdraw request processing state
class WithdrawProcessing extends WithdrawState {
  final double amount;

  const WithdrawProcessing(this.amount);
}

/// Withdraw success state
class WithdrawSuccess extends WithdrawState {
  final String withdrawalId;
  final WithdrawalStatus status;
  final double amount;
  final double feeAmount;
  final double totalDebitAmount;

  const WithdrawSuccess({
    required this.withdrawalId,
    required this.status,
    required this.amount,
    this.feeAmount = 0,
    this.totalDebitAmount = 0,
  });
}

/// Withdraw history loaded state
class WithdrawHistoryLoaded extends WithdrawState {
  final List<Withdrawal> withdrawals;

  const WithdrawHistoryLoaded(this.withdrawals);
}

/// Error state
class WithdrawError extends WithdrawState {
  final String message;

  const WithdrawError(this.message);

  /// Common error constructors
  factory WithdrawError.insufficientBalance() {
    return const WithdrawError('Insufficient available balance');
  }

  factory WithdrawError.notVerified() {
    return const WithdrawError('You must be verified to withdraw funds');
  }

  factory WithdrawError.noBankAccount() {
    return const WithdrawError('Please add a default bank account first');
  }

  factory WithdrawError.belowMinimum(double min) {
    return WithdrawError(
      'Minimum withdrawal amount is Rp ${min.toStringAsFixed(0)}',
    );
  }

  factory WithdrawError.aboveMaximum(double max) {
    return WithdrawError(
      'Maximum withdrawal amount is Rp ${max.toStringAsFixed(0)}',
    );
  }
}
