/// Coin State for Coins feature
///
/// States for coin operations using Riverpod.
///
/// IMPORTANT: Coins are LOYALTY POINTS, NOT money.
library;

import 'package:freezed_annotation/freezed_annotation.dart';
import 'package:labuda/domains/finance/wallet/coins/domain/entities/coin_balance.dart';
import 'package:labuda/domains/finance/wallet/coins/domain/entities/coin_transaction.dart';

part 'coin_state.freezed.dart';

/// Base coin state
///
/// This is the SINGLE SOURCE OF TRUTH for coin state in the app.
/// Other features (checkout, payment) must consume this state, not create their own.
@freezed
class CoinState with _$CoinState {
  const factory CoinState.initial() = CoinInitial;

  const factory CoinState.loading() = CoinLoading;

  const factory CoinState.balanceLoaded(
    int balance,
    CoinBalance balanceEntity,
  ) = BalanceLoaded;

  const factory CoinState.transactionsLoaded(
    List<CoinTransaction> transactions,
  ) = TransactionsLoaded;

  const factory CoinState.error(String message) = CoinError;
}
