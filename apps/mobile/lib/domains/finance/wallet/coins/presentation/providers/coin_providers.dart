// Riverpod providers for Coins feature.
//
// All DI is handled via Riverpod - no get_it usage.
//
// IMPORTANT: Coins are LOYALTY POINTS, NOT money.

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/finance/wallet/coins/coins_di.dart';

// ⚠️ ATURAN: Presentation layer TIDAK BOLEH import data layer langsung
// Gunakan provider dari DI file untuk repository

// ============================================================
// Balance Providers
// ============================================================

/// Provides user's Coin balance (one-time fetch)
final coinBalanceProvider = FutureProvider.family<CoinBalance?, String>((
  ref,
  userId,
) async {
  final repository = ref.watch(coinRepositoryProvider);
  final result = await repository.getCoinBalance(userId);

  return result.fold((error) => throw Exception(error), (balance) => balance);
});

/// Watches user's Coin balance for real-time updates
final coinBalanceStreamProvider = StreamProvider.family<CoinBalance?, String>((
  ref,
  userId,
) {
  final repository = ref.watch(coinRepositoryProvider);
  return repository
      .watchCoinBalance(userId)
      .map((result) => result.fold((error) => null, (balance) => balance));
});

// ============================================================
// Transaction Providers
// ============================================================

/// Provides transaction history (one-time fetch)
final coinTransactionsProvider =
    FutureProvider.family<
      List<CoinTransaction>,
      ({String userId, int limit, int offset})
    >((ref, params) async {
      final repository = ref.watch(coinRepositoryProvider);
      final result = await repository.getTransactions(
        userId: params.userId,
        limit: params.limit,
        offset: params.offset,
      );

      return result.fold(
        (error) => throw Exception(error),
        (transactions) => transactions,
      );
    });

/// Watches transaction history for real-time updates
final coinTransactionsStreamProvider =
    StreamProvider.family<List<CoinTransaction>, ({String userId, int limit})>((
      ref,
      params,
    ) {
      final repository = ref.watch(coinRepositoryProvider);
      return repository
          .watchTransactions(userId: params.userId, limit: params.limit)
          .map(
            (result) => result.fold(
              (error) => <CoinTransaction>[],
              (transactions) => transactions,
            ),
          );
    });

// ============================================================
// Helper Providers
// ============================================================

/// Checks if user has enough coins for a given discount amount
final hasEnoughCoinsProvider =
    FutureProvider.family<bool, ({String userId, int requiredAmount})>((
      ref,
      params,
    ) async {
      final balance = await ref.watch(
        coinBalanceProvider(params.userId).future,
      );

      if (balance == null) return false;
      return balance.hasEnoughCoins(params.requiredAmount);
    });

/// Gets the current total coins for a user
final totalCoinsProvider = FutureProvider.family<int, String>((
  ref,
  userId,
) async {
  final balance = await ref.watch(coinBalanceProvider(userId).future);
  return balance?.balance ?? 0;
});

/// Gets the estimated discount value from coins
///
/// Calculates based on exchange rate: 1 coin = Rp1 (ESTIMATED only)
/// Backend determines actual discount amount during checkout.
final estimatedCoinsValueProvider = FutureProvider.family<int, String>((
  ref,
  userId,
) async {
  final balance = await ref.watch(coinBalanceProvider(userId).future);
  if (balance == null) return 0;
  // Exchange rate: 1 coin = Rp1 (for ESTIMATION only)
  return balance.balance * 1;
});
