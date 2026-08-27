/// Coin Notifier for Coins feature
///
/// Riverpod Notifier for coin state management.
///
/// IMPORTANT: Coins are LOYALTY POINTS, NOT money.
///
/// This is the SINGLE SOURCE OF TRUTH for coin state management.
/// Other features (checkout, payment) must consume this, NOT create their own.
library;

import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:labuda/domains/finance/wallet/coins/presentation/providers/coin_state.dart';
import 'package:labuda/domains/finance/wallet/coins/coins_di.dart';
// R4.2: Import authenticatedUserProvider directly instead of mega-barrel
import 'package:labuda/shared/providers/authenticated_account_provider.dart'
    show authenticatedUserProvider;

part 'coin_notifier.g.dart';

// ============================================================================
// Coin Notifier (LOYALTY POINTS)
// ============================================================================

/// Coin notifier for state management
///
/// Provides a unified interface for coin operations.
/// This is the SINGLE SOURCE OF TRUTH for coin state management.
@riverpod
class CoinNotifier extends _$CoinNotifier {
  @override
  CoinState build() {
    return const CoinState.initial();
  }

  String _getUserId() {
    final user = ref.read(authenticatedUserProvider);
    return user?.id ?? '';
  }

  /// Get coin balance for current user
  Future<void> getBalance() async {
    state = const CoinState.loading();
    final repo = ref.read(coinRepositoryProvider);

    final userId = _getUserId();
    if (userId.isEmpty) {
      state = const CoinState.error('User not authenticated');
      return;
    }

    final result = await repo.getCoinBalance(userId);

    result.fold(
      (error) => state = CoinState.error(error),
      (balance) => state = CoinState.balanceLoaded(balance.balance, balance),
    );
  }

  /// Get coin transactions
  Future<void> getTransactions({int page = 1}) async {
    state = const CoinState.loading();
    final repo = ref.read(coinRepositoryProvider);

    final userId = _getUserId();
    if (userId.isEmpty) {
      state = const CoinState.error('User not authenticated');
      return;
    }

    final result = await repo.getTransactions(
      userId: userId,
      limit: 20,
      offset: (page - 1) * 20,
    );

    result.fold(
      (error) => state = CoinState.error(error),
      (transactions) => state = CoinState.transactionsLoaded(transactions),
    );
  }

  /// Check if user has enough coins
  bool hasEnoughCoins(int amount) {
    return state.maybeWhen(
      balanceLoaded: (balance, _) => balance >= amount,
      orElse: () => false,
    );
  }

  /// Clear error
  void clearError() {
    state = const CoinState.initial();
  }

  /// Reset state
  void reset() {
    state = const CoinState.initial();
  }
}
