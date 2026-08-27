/// Coin API Repository Implementation
///
/// READ-ONLY implementation of CoinRepository.  Coins are loyalty points —
/// earn, spend, and refund are all backend-authoritative operations.
/// Mobile has no mutation authority.
library;

import 'package:labuda/core/common/result.dart';
import 'package:labuda/domains/finance/wallet/coins/data/mappers/coin_mapper.dart';
import 'package:labuda/domains/finance/wallet/coins/data/remote/coin_api_datasource.dart';
import 'package:labuda/domains/finance/wallet/coins/domain/entities/coin_balance.dart';
import 'package:labuda/domains/finance/wallet/coins/domain/entities/coin_transaction.dart';
import 'package:labuda/domains/finance/wallet/coins/domain/repositories/coin_repository.dart';

class CoinApiRepositoryImpl implements CoinRepository {
  final CoinApiDatasource _datasource;

  CoinApiRepositoryImpl({required CoinApiDatasource datasource})
    : _datasource = datasource;

  // ============================================================
  // Balance
  // ============================================================

  @override
  Future<Result<CoinBalance>> getCoinBalance(String userId) async {
    final result = await _datasource.getBalance(userId);
    return result.fold(
      (error) => Result.error(error),
      (dto) => Result.success(CoinMapper.balanceToEntity(dto)),
    );
  }

  @override
  Stream<Result<CoinBalance>> watchCoinBalance(String userId) {
    return _datasource
        .watchBalance(userId)
        .map((dto) => Result.success(CoinMapper.balanceToEntity(dto)));
  }

  // ============================================================
  // Transactions
  // ============================================================

  @override
  Future<Result<List<CoinTransaction>>> getTransactions({
    required String userId,
    int limit = 50,
    int offset = 0,
  }) async {
    final result = await _datasource.getTransactions(
      userId: userId,
      limit: limit,
      offset: offset,
    );
    return result.fold(
      (error) => Result.error(error),
      (dtos) => Result.success(CoinMapper.transactionsToEntities(dtos)),
    );
  }

  @override
  Stream<Result<List<CoinTransaction>>> watchTransactions({
    required String userId,
    int limit = 50,
  }) {
    return _datasource
        .watchTransactions(userId: userId, limit: limit)
        .map((dtos) => Result.success(CoinMapper.transactionsToEntities(dtos)));
  }

  // ============================================================
  // Utility
  // ============================================================

  @override
  Future<Result<bool>> hasEnoughCoins({
    required String userId,
    required int requiredAmount,
  }) async {
    final balanceResult = await getCoinBalance(userId);
    return balanceResult.fold(
      (error) => Result.error(error),
      (balance) => Result.success(balance.hasEnoughCoins(requiredAmount)),
    );
  }

  @override
  Future<Result<int>> getTotalEarnedFromSource({
    required String userId,
    required CoinSourceType sourceType,
  }) async {
    final transactionsResult = await getTransactions(
      userId: userId,
      limit: 1000,
    );
    return transactionsResult.fold((error) => Result.error(error), (
      transactions,
    ) {
      final total = transactions
          .where(
            (t) =>
                t.sourceType == sourceType &&
                t.type == CoinTransactionType.earn,
          )
          .fold<int>(0, (sum, t) => sum + t.amount);
      return Result.success(total);
    });
  }

  @override
  Future<Result<int>> getTotalSpentOnSource({
    required String userId,
    required CoinSourceType sourceType,
  }) async {
    final transactionsResult = await getTransactions(
      userId: userId,
      limit: 1000,
    );
    return transactionsResult.fold((error) => Result.error(error), (
      transactions,
    ) {
      final total = transactions
          .where(
            (t) =>
                t.sourceType == sourceType &&
                t.type == CoinTransactionType.spend,
          )
          .fold<int>(0, (sum, t) => sum + t.absoluteAmount);
      return Result.success(total);
    });
  }
}
