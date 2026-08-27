import 'package:labuda/core/common/result.dart';

import '../entities/coin_balance.dart';
import '../entities/coin_transaction.dart';

/// Repository interface for Labuda Coins (loyalty points) — READ-ONLY.
///
/// AUTHORITY MODEL:
///   - Earn coins:   backend-only (order completion worker).
///   - Spend coins:  backend-only (order creation, via useCoins flag).
///   - Refund coins: backend-only (coins.refund_required outbox worker).
///
/// Mobile only reads balance and transaction history.
abstract class CoinRepository {
  // ============================================================
  // Balance Operations
  // ============================================================

  /// Current coin balance for the authenticated user.
  ///
  /// [userId] is used by providers for Riverpod caching only;
  /// the backend identifies the caller from the auth token.
  Future<Result<CoinBalance>> getCoinBalance(String userId);

  /// Stream of balance updates (polling-based, 30-second interval).
  Stream<Result<CoinBalance>> watchCoinBalance(String userId);

  // ============================================================
  // Transaction Operations
  // ============================================================

  /// Paginated transaction history for the authenticated user.
  Future<Result<List<CoinTransaction>>> getTransactions({
    required String userId,
    int limit = 50,
    int offset = 0,
  });

  /// Stream of transaction history (polling-based, 30-second interval).
  Stream<Result<List<CoinTransaction>>> watchTransactions({
    required String userId,
    int limit = 50,
  });

  // ============================================================
  // Utility Operations
  // ============================================================

  /// Returns true if the authenticated user's balance >= [requiredAmount].
  Future<Result<bool>> hasEnoughCoins({
    required String userId,
    required int requiredAmount,
  });

  /// Total coins earned from a specific source type.
  Future<Result<int>> getTotalEarnedFromSource({
    required String userId,
    required CoinSourceType sourceType,
  });

  /// Total coins spent on a specific source type.
  Future<Result<int>> getTotalSpentOnSource({
    required String userId,
    required CoinSourceType sourceType,
  });
}
