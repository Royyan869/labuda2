import 'package:equatable/equatable.dart';

import 'ownership_status.dart';

/// Promotion ownership entity.
///
/// Represents a user's entitlement to promote.
/// This is the SINGLE SOURCE OF TRUTH for duration accounting.
///
/// Business truth: Users purchase packages, which creates ownership.
/// The ownership tracks total duration, consumed duration, and validity window.
class PromotionOwnership extends Equatable {
  /// Unique identifier
  final String id;

  /// User who owns this entitlement
  final String userId;

  /// Package that was purchased
  final String packageId;

  /// Current status
  final OwnershipStatus status;

  /// When this ownership was purchased
  final DateTime purchasedAt;

  /// Hard expiry: purchased_at + validity_window
  final DateTime expiresAt;

  /// Total duration hours (fixed at purchase)
  final int totalDurationHours;

  /// Consumed duration hours (increments over time)
  final int consumedDurationHours;

  /// When this ownership was created
  final DateTime createdAt;

  /// When this ownership was last updated
  final DateTime updatedAt;

  const PromotionOwnership({
    required this.id,
    required this.userId,
    required this.packageId,
    required this.status,
    required this.purchasedAt,
    required this.expiresAt,
    required this.totalDurationHours,
    required this.consumedDurationHours,
    required this.createdAt,
    required this.updatedAt,
  });

  /// Calculates remaining duration in hours
  ///
  /// This is ALWAYS computed, never stored.
  int get remainingDurationHours {
    final remaining = totalDurationHours - consumedDurationHours;
    return remaining > 0 ? remaining : 0;
  }

  /// Checks if ownership has passed its validity window
  bool get isExpired => DateTime.now().isAfter(expiresAt);

  /// Checks if all duration has been consumed
  bool get isFullyConsumed => consumedDurationHours >= totalDurationHours;

  /// Checks if ownership can be used to activate a promotion
  bool get canActivate =>
      status == OwnershipStatus.available && !isExpired && !isFullyConsumed;

  /// Creates a copy with updated fields
  PromotionOwnership copyWith({
    String? id,
    String? userId,
    String? packageId,
    OwnershipStatus? status,
    DateTime? purchasedAt,
    DateTime? expiresAt,
    int? totalDurationHours,
    int? consumedDurationHours,
    DateTime? createdAt,
    DateTime? updatedAt,
  }) {
    return PromotionOwnership(
      id: id ?? this.id,
      userId: userId ?? this.userId,
      packageId: packageId ?? this.packageId,
      status: status ?? this.status,
      purchasedAt: purchasedAt ?? this.purchasedAt,
      expiresAt: expiresAt ?? this.expiresAt,
      totalDurationHours: totalDurationHours ?? this.totalDurationHours,
      consumedDurationHours:
          consumedDurationHours ?? this.consumedDurationHours,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }

  @override
  List<Object?> get props => [
    id,
    userId,
    packageId,
    status,
    purchasedAt,
    expiresAt,
    totalDurationHours,
    consumedDurationHours,
    createdAt,
    updatedAt,
  ];

  @override
  String toString() {
    return 'PromotionOwnership(id: $id, status: $status, remaining: $remainingDurationHours)';
  }
}
