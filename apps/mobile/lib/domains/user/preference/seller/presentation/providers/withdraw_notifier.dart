/// Withdraw Notifier
///
/// Riverpod Notifier for withdraw operations.
library;

import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../data/seller_providers.dart';
import '../../domain/domain.dart';
import 'withdraw_state.dart';

/// Withdraw Notifier
///
/// Manages withdraw request and history state.
class WithdrawNotifier extends Notifier<WithdrawState> {
  SellerRepository get _repository => ref.read(sellerRepositoryProvider);

  @override
  WithdrawState build() {
    return const WithdrawInitial();
  }

  /// Request a withdrawal
  ///
  /// Returns true if successful, false otherwise
  Future<bool> requestWithdraw(double amount) async {
    // Validate amount first
    final request = WithdrawRequest(amount: amount);
    if (!request.isValid) {
      if (request.isBelowMin) {
        state = WithdrawError.belowMinimum(WithdrawRequest.minAmount);
      } else if (request.exceedsMax) {
        state = WithdrawError.aboveMaximum(WithdrawRequest.maxAmount);
      } else {
        state = const WithdrawError('Invalid withdrawal amount');
      }
      return false;
    }

    state = WithdrawProcessing(amount);

    final result = await _repository.requestWithdraw(request);

    if (result.isSuccess && result.data != null) {
      final withdrawResult = result.data!;
      state = WithdrawSuccess(
        withdrawalId: withdrawResult.withdrawalId,
        status: withdrawResult.status,
        amount: amount,
        feeAmount: withdrawResult.feeAmount,
        totalDebitAmount: withdrawResult.totalDebitAmount,
      );

      // Return to initial state after a delay
      Future.delayed(const Duration(seconds: 3), () {
        if (state is WithdrawSuccess) {
          state = const WithdrawInitial();
        }
      });

      return true;
    } else {
      state = WithdrawError(result.error ?? 'Failed to request withdrawal');
      return false;
    }
  }

  /// Load withdrawal history
  Future<void> loadHistory() async {
    state = const WithdrawLoading();

    final result = await _repository.getWithdrawHistory();

    if (result.isSuccess && result.data != null) {
      state = WithdrawHistoryLoaded(result.data!);
    } else {
      state = WithdrawError(
        result.error ?? 'Failed to load withdrawal history',
      );
    }
  }

  /// Reset state to initial
  void reset() {
    state = const WithdrawInitial();
  }

  /// Clear error state
  void clearError() {
    if (state is WithdrawError) {
      state = const WithdrawInitial();
    }
  }
}

/// Provider for withdraw notifier
final withdrawNotifierProvider =
    NotifierProvider<WithdrawNotifier, WithdrawState>(WithdrawNotifier.new);

/// Provider for withdrawal history
///
/// Fetches and exposes the seller's withdrawal history.
/// Call ref.invalidate(withdrawalHistoryProvider) to refresh.
final withdrawalHistoryProvider = FutureProvider.autoDispose<List<Withdrawal>>((
  ref,
) async {
  final repository = ref.watch(sellerRepositoryProvider);
  final result = await repository.getWithdrawHistory();

  return result.fold((data) => data, (_) => const []);
});
