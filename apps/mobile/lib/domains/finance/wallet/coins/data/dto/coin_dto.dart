/// Coins DTOs
///
/// Data Transfer Objects for coin balance and transaction entities.
/// Used for serialization/deserialization from API responses.
///
/// IMPORTANT: Coins are LOYALTY POINTS, NOT money.
library;

import 'package:labuda/domains/finance/wallet/coins/domain/entities/coin_balance.dart';
import 'package:labuda/domains/finance/wallet/coins/domain/entities/coin_transaction.dart';

/// DTO for spending coins request
/// Used when spending coins for discounts or other purposes (NOT payment)
class SpendCoinsRequestDto {
  final int amount;
  final String? orderId;
  final String? description;

  const SpendCoinsRequestDto({
    required this.amount,
    this.orderId,
    this.description,
  });

  /// Converts to JSON for API request
  Map<String, dynamic> toJson() {
    return {
      'amount': amount,
      if (orderId != null) 'order_id': orderId,
      if (description != null) 'description': description,
    };
  }
}

/// DTO for CoinBalance - handles API serialization
class CoinBalanceDto {
  final String userId;
  final int balance;
  final int lifetimeEarned;
  final int lifetimeSpent;
  final DateTime? createdAt; // NULLABLE: null if user has no transactions
  final DateTime updatedAt;
  final DateTime? lastTransactionAt;

  const CoinBalanceDto({
    required this.userId,
    required this.balance,
    required this.lifetimeEarned,
    required this.lifetimeSpent,
    this.createdAt,
    required this.updatedAt,
    this.lastTransactionAt,
  });

  /// Creates DTO from JSON (API response)
  factory CoinBalanceDto.fromJson(Map<String, dynamic> json) {
    return CoinBalanceDto(
      userId: json['userId'] as String? ?? json['user_id'] as String? ?? '',
      balance: json['balance'] as int? ?? 0,
      lifetimeEarned:
          json['lifetimeEarned'] as int? ??
          json['lifetime_earned'] as int? ??
          0,
      lifetimeSpent:
          json['lifetimeSpent'] as int? ?? json['lifetime_spent'] as int? ?? 0,
      createdAt: _parseNullableDateTime(
        json['createdAt'] ?? json['created_at'],
      ),
      updatedAt: _parseDateTime(json['updatedAt'] ?? json['updated_at']),
      lastTransactionAt: _parseNullableDateTime(
        json['lastTransactionAt'] ?? json['last_transaction_at'],
      ),
    );
  }

  /// Creates DTO from entity
  factory CoinBalanceDto.fromEntity(CoinBalance entity) {
    return CoinBalanceDto(
      userId: entity.userId,
      balance: entity.balance,
      lifetimeEarned: entity.lifetimeEarned,
      lifetimeSpent: entity.lifetimeSpent,
      createdAt: entity.createdAt,
      updatedAt: entity.updatedAt,
      lastTransactionAt: entity.lastTransactionAt,
    );
  }

  /// Converts to entity
  CoinBalance toEntity() {
    return CoinBalance(
      userId: userId,
      balance: balance,
      lifetimeEarned: lifetimeEarned,
      lifetimeSpent: lifetimeSpent,
      createdAt: createdAt, // HONEST: pass through null if null
      updatedAt: updatedAt,
      lastTransactionAt: lastTransactionAt,
    );
  }

  /// Creates empty balance
  static CoinBalanceDto empty(String userId) {
    final now = DateTime.now();
    return CoinBalanceDto(
      userId: userId,
      balance: 0,
      lifetimeEarned: 0,
      lifetimeSpent: 0,
      createdAt: null, // HONEST: new user has no first transaction
      updatedAt: now,
      lastTransactionAt: null,
    );
  }

  static DateTime? _parseNullableDateTime(dynamic value) {
    if (value == null) return null;
    if (value is DateTime) return value;
    if (value is String) return DateTime.parse(value);
    if (value is int) return DateTime.fromMillisecondsSinceEpoch(value * 1000);
    return null; // HONEST: return null, not fake DateTime.now()
  }

  static DateTime _parseDateTime(dynamic value) {
    final parsed = _parseNullableDateTime(value);
    return parsed ?? DateTime.now(); // Fallback for required fields
  }
}

/// DTO for CoinTransaction - handles API serialization
class CoinTransactionDto {
  final String id;
  final String userId;
  final CoinTransactionType type;
  final CoinSourceType sourceType;
  final int amount;
  final int balanceAfter;
  final String? relatedId;
  final String? relatedType;
  final String description;
  final Map<String, dynamic>? metadata;
  final DateTime createdAt;

  const CoinTransactionDto({
    required this.id,
    required this.userId,
    required this.type,
    required this.sourceType,
    required this.amount,
    required this.balanceAfter,
    this.relatedId,
    this.relatedType,
    required this.description,
    this.metadata,
    required this.createdAt,
  });

  /// Creates DTO from JSON (API response)
  factory CoinTransactionDto.fromJson(Map<String, dynamic> json) {
    return CoinTransactionDto(
      id: json['id'] as String? ?? '',
      userId: json['userId'] as String? ?? json['user_id'] as String? ?? '',
      type: _parseTransactionType(json['type'] ?? json['transaction_type']),
      sourceType: _parseSourceType(json['sourceType'] ?? json['source_type']),
      amount: json['amount'] as int? ?? 0,
      balanceAfter:
          json['balanceAfter'] as int? ?? json['balance_after'] as int? ?? 0,
      relatedId: json['relatedId'] as String? ?? json['related_id'] as String?,
      relatedType:
          json['relatedType'] as String? ?? json['related_type'] as String?,
      description: json['description'] as String? ?? '',
      metadata: json['metadata'] as Map<String, dynamic>?,
      createdAt: _parseDateTime(json['createdAt'] ?? json['created_at']),
    );
  }

  /// Creates DTO from entity
  factory CoinTransactionDto.fromEntity(CoinTransaction entity) {
    return CoinTransactionDto(
      id: entity.id,
      userId: entity.userId,
      type: entity.type,
      sourceType: entity.sourceType,
      amount: entity.amount,
      balanceAfter: entity.balanceAfter,
      relatedId: entity.relatedId,
      relatedType: entity.relatedType,
      description: entity.description,
      metadata: entity.metadata,
      createdAt: entity.createdAt,
    );
  }

  /// Converts to entity
  CoinTransaction toEntity() {
    return CoinTransaction(
      id: id,
      userId: userId,
      type: type,
      sourceType: sourceType,
      amount: amount,
      balanceAfter: balanceAfter,
      relatedId: relatedId,
      relatedType: relatedType,
      description: description,
      metadata: metadata,
      createdAt: createdAt,
    );
  }

  static DateTime _parseDateTime(dynamic value) {
    if (value is DateTime) return value;
    if (value is String) return DateTime.parse(value);
    if (value is int) return DateTime.fromMillisecondsSinceEpoch(value * 1000);
    return DateTime.now();
  }

  static CoinTransactionType _parseTransactionType(dynamic value) {
    if (value is CoinTransactionType) return value;
    if (value is String) {
      return CoinTransactionType.values.firstWhere(
        (e) => e.name.toLowerCase() == value.toLowerCase(),
        orElse: () => CoinTransactionType.earn,
      );
    }
    return CoinTransactionType.earn;
  }

  static CoinSourceType _parseSourceType(dynamic value) {
    if (value is CoinSourceType) return value;
    if (value is String) {
      // Parse backend format (snake_case)
      final lower = value.toLowerCase();
      switch (lower) {
        case 'order_reward':
          return CoinSourceType.orderReward;
        case 'order_spend':
          return CoinSourceType.orderSpend;
        case 'refund_earn':
          return CoinSourceType.refundEarn;
        case 'refund_spend':
          return CoinSourceType.refundSpend;
        default:
          // Fallback for legacy values
          return CoinSourceType.orderReward;
      }
    }
    // Default fallback for invalid/missing values
    return CoinSourceType.orderReward;
  }
}
