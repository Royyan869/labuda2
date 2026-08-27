import 'package:equatable/equatable.dart';

/// Represents a user's Coin balance in the Labuda loyalty system.
///
/// IMPORTANT: Coins are LOYALTY POINTS ONLY, NOT money.
/// Coins CANNOT be:
/// - Withdrawn as cash
/// - Transferred to other users
/// - Used as payment instrument
/// - Stored as monetary value
/// - Exchanged for fiat currency
///
/// Coins MAY ONLY be used as:
/// - Discount input during checkout (backend determines amount)
/// - Promotional rewards
///
/// COINS DO NOT EXPIRE - Per business decision, coins are permanent loyalty points.
/// No distinction between regular and promo coins - single balance only.
///
/// Exchange rate reference: 1 coin = Rp 10 (for ESTIMATED discount calculation only)
/// Max balance: 1,000,000 coins
class CoinBalance extends Equatable {
  /// User ID who owns this balance
  final String userId;

  /// Total coins balance (single balance, no regular/promo split)
  final int balance;

  /// Lifetime total coins earned (all time)
  final int lifetimeEarned;

  /// Lifetime total coins spent (all time)
  final int lifetimeSpent;

  /// When this balance was created (first transaction)
  /// NULL if user has no transactions yet - HONEST null, not fake fallback
  final DateTime? createdAt;

  /// When this balance was last updated
  final DateTime updatedAt;

  /// When the last transaction occurred
  final DateTime? lastTransactionAt;

  const CoinBalance({
    required this.userId,
    required this.balance,
    required this.lifetimeEarned,
    required this.lifetimeSpent,
    this.createdAt,
    required this.updatedAt,
    this.lastTransactionAt,
  });

  /// Creates an empty balance for a new user
  factory CoinBalance.empty(String userId) {
    final now = DateTime.now();
    return CoinBalance(
      userId: userId,
      balance: 0,
      lifetimeEarned: 0,
      lifetimeSpent: 0,
      createdAt: null, // HONEST: new user has no first transaction
      updatedAt: now,
      lastTransactionAt: null,
    );
  }

  /// Checks if balance has reached maximum limit
  ///
  /// Max balance: 1,000,000 points
  bool get isAtMaxBalance => balance >= 1000000;

  /// Checks if balance is near maximum limit (>= 500k points)
  bool get isNearMaxBalance => balance >= 500000;

  /// Checks if user has any points
  bool get hasCoins => balance > 0;

  /// Checks if user has enough points for a given discount amount
  bool hasEnoughCoins(int requiredPoints) => balance >= requiredPoints;

  /// Returns remaining capacity before hitting max balance
  int get remainingCapacity {
    const maxBalance = 1000000;
    final remaining = maxBalance - balance;
    return remaining > 0 ? remaining : 0;
  }

  /// Creates a copy with updated fields
  CoinBalance copyWith({
    String? userId,
    int? balance,
    int? lifetimeEarned,
    int? lifetimeSpent,
    DateTime? createdAt,
    DateTime? updatedAt,
    DateTime? lastTransactionAt,
  }) {
    return CoinBalance(
      userId: userId ?? this.userId,
      balance: balance ?? this.balance,
      lifetimeEarned: lifetimeEarned ?? this.lifetimeEarned,
      lifetimeSpent: lifetimeSpent ?? this.lifetimeSpent,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
      lastTransactionAt: lastTransactionAt ?? this.lastTransactionAt,
    );
  }

  @override
  List<Object?> get props => [
    userId,
    balance,
    lifetimeEarned,
    lifetimeSpent,
    createdAt,
    updatedAt,
    lastTransactionAt,
  ];

  @override
  String toString() {
    return 'CoinBalance('
        'userId: $userId, '
        'balance: $balance, '
        'lifetimeEarned: $lifetimeEarned, '
        'lifetimeSpent: $lifetimeSpent'
        ')';
  }
}
