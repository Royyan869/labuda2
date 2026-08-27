import 'package:equatable/equatable.dart';

/// Type of loyalty point transaction
///
/// IMPORTANT: These are loyalty point transactions, NOT money transfers.
enum CoinTransactionType {
  /// User earned points as loyalty reward
  earn,

  /// User used points for discount (NOT payment)
  spend,
}

/// Extension methods for [CoinTransactionType]
extension CoinTransactionTypeX on CoinTransactionType {
  /// Returns display name for transaction type
  String get displayName {
    switch (this) {
      case CoinTransactionType.earn:
        return 'Earned';
      case CoinTransactionType.spend:
        return 'Used';
    }
  }

  /// Returns icon emoji for transaction type
  String get icon {
    switch (this) {
      case CoinTransactionType.earn:
        return '📥';
      case CoinTransactionType.spend:
        return '📤';
    }
  }

  /// Converts to string for storage.
  String toJson() {
    return name;
  }

  /// Parses from string.
  static CoinTransactionType fromJson(String value) {
    return CoinTransactionType.values.firstWhere(
      (e) => e.name == value,
      orElse: () => throw ArgumentError('Invalid CoinTransactionType: $value'),
    );
  }
}

/// Source of the loyalty point transaction
///
/// IMPORTANT: This aligns with backend's CoinReferenceType.
/// Coins can ONLY be earned from order completion or refund events.
/// Coins can ONLY be spent on orders or refunds.
///
/// SECURITY RESTRICTIONS (from backend):
/// - Promo rewards removed - use specific promo system instead
/// - Admin grants removed - security backdoor not allowed
/// - Only order-related and refund-related coin movements are valid
enum CoinSourceType {
  /// Coins earned from order completion
  orderReward,

  /// Coins spent on order (for discount)
  orderSpend,

  /// Coins earned from refund (cancelled order)
  refundEarn,

  /// Coins deducted during refund
  refundSpend,
}

/// Extension methods for [CoinSourceType]
extension CoinSourceTypeX on CoinSourceType {
  /// Returns display name for source type
  String get displayName {
    switch (this) {
      case CoinSourceType.orderReward:
        return 'Order Reward';
      case CoinSourceType.orderSpend:
        return 'Order Discount';
      case CoinSourceType.refundEarn:
        return 'Refund Credit';
      case CoinSourceType.refundSpend:
        return 'Refund Deduction';
    }
  }

  /// Converts to string for API (snake_case to match backend)
  String toJson() {
    switch (this) {
      case CoinSourceType.orderReward:
        return 'order_reward';
      case CoinSourceType.orderSpend:
        return 'order_spend';
      case CoinSourceType.refundEarn:
        return 'refund_earn';
      case CoinSourceType.refundSpend:
        return 'refund_spend';
    }
  }

  /// Parses from string (backend format)
  static CoinSourceType fromJson(String value) {
    return CoinSourceType.values.firstWhere(
      (e) => e.toJson() == value,
      orElse: () => throw ArgumentError('Invalid CoinSourceType: $value'),
    );
  }
}

/// Represents a single loyalty points transaction (earn or spend).
///
/// IMPORTANT: These are loyalty point transactions, NOT money transfers.
/// Points CANNOT be:
/// - Withdrawn as cash
/// - Transferred to other users
/// - Used as payment instrument
///
/// Points do NOT expire.
///
/// All point movements are tracked as transactions for audit trail
/// and transparency.
class CoinTransaction extends Equatable {
  /// Unique transaction ID
  final String id;

  /// User ID who performed this transaction
  final String userId;

  /// Type of transaction (earn, spend)
  final CoinTransactionType type;

  /// Source of the transaction
  final CoinSourceType sourceType;

  /// Amount of points (positive for earn, negative for spend)
  final int amount;

  /// Balance after this transaction
  final int balanceAfter;

  /// Related entity ID (order_id, refund_id, etc.)
  final String? relatedId;

  /// Related entity type (order, ads, etc.)
  final String? relatedType;

  /// Human-readable description
  final String description;

  /// Additional metadata as JSON
  final Map<String, dynamic>? metadata;

  /// When this transaction was created
  final DateTime createdAt;

  const CoinTransaction({
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

  /// Returns true if this transaction involves earning points
  bool get isEarning => type == CoinTransactionType.earn;

  /// Returns true if this transaction involves using points for discount
  bool get isSpending => type == CoinTransactionType.spend;

  /// Returns absolute amount (always positive)
  int get absoluteAmount => amount.abs();

  /// Creates a copy with updated fields
  CoinTransaction copyWith({
    String? id,
    String? userId,
    CoinTransactionType? type,
    CoinSourceType? sourceType,
    int? amount,
    int? balanceAfter,
    String? relatedId,
    String? relatedType,
    String? description,
    Map<String, dynamic>? metadata,
    DateTime? createdAt,
  }) {
    return CoinTransaction(
      id: id ?? this.id,
      userId: userId ?? this.userId,
      type: type ?? this.type,
      sourceType: sourceType ?? this.sourceType,
      amount: amount ?? this.amount,
      balanceAfter: balanceAfter ?? this.balanceAfter,
      relatedId: relatedId ?? this.relatedId,
      relatedType: relatedType ?? this.relatedType,
      description: description ?? this.description,
      metadata: metadata ?? this.metadata,
      createdAt: createdAt ?? this.createdAt,
    );
  }

  @override
  List<Object?> get props => [
    id,
    userId,
    type,
    sourceType,
    amount,
    balanceAfter,
    relatedId,
    relatedType,
    description,
    metadata,
    createdAt,
  ];

  @override
  String toString() {
    return 'CoinTransaction('
        'id: $id, '
        'type: $type, '
        'sourceType: $sourceType, '
        'amount: $amount, '
        'balanceAfter: $balanceAfter'
        ')';
  }
}
