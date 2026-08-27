import 'package:equatable/equatable.dart';

import 'instance_status.dart';
import 'target_type.dart';

/// Promotion instance entity.
///
/// Represents an active promotion on a specific target.
///
/// Business truth: An instance is the active use of an ownership on a target.
/// Only one instance can be active per ownership at a time.
/// Duration stays at ownership level - instance is just a pointer to the target.
class PromotionInstance extends Equatable {
  /// Unique identifier
  final String id;

  /// Associated ownership
  final String ownershipId;

  /// User who owns this instance
  final String userId;

  /// Target type
  final TargetType targetType;

  /// Target ID (for listing/auction; for external_product points at external_products.id)
  final String? targetId;

  /// Current status
  final InstanceStatus status;

  /// When this instance was activated
  final DateTime? activatedAt;

  /// When this instance was stopped
  final DateTime? stoppedAt;

  /// Why this instance was stopped
  final String? stopReason;

  /// When this instance was created
  final DateTime createdAt;

  /// When this instance was last updated
  final DateTime updatedAt;

  const PromotionInstance({
    required this.id,
    required this.ownershipId,
    required this.userId,
    required this.targetType,
    this.targetId,
    required this.status,
    this.activatedAt,
    this.stoppedAt,
    this.stopReason,
    required this.createdAt,
    required this.updatedAt,
  });

  /// Checks if the instance is currently promoting
  bool get isActive => status.isActive;

  /// Checks if this instance is for an external product
  bool get isExternalProduct => targetType == TargetType.externalProduct;

  /// Gets the stop reason as a canonical constant
  /// Returns null if not stopped or reason is invalid
  String? get canonicalStopReason {
    if (stopReason == null) return null;
    // Validate against known reasons
    final knownReasons = [
      'user_paused',
      'user_cancelled',
      'admin_cancelled',
      'duration_exhausted',
      'validity_expired',
      'fixed_price_sale_sold',
      'fixed_price_sale_hidden',
      'fixed_price_sale_deleted',
      'fixed_price_sale_moderated',
      'fixed_price_sale_expired',
      'auction_ended',
      'auction_cancelled',
      'auction_deleted',
      'auction_moderated',
      'external_invalid',
    ];
    return knownReasons.contains(stopReason) ? stopReason : null;
  }

  /// Creates a copy with updated fields
  PromotionInstance copyWith({
    String? id,
    String? ownershipId,
    String? userId,
    TargetType? targetType,
    String? targetId,
    InstanceStatus? status,
    DateTime? activatedAt,
    DateTime? stoppedAt,
    String? stopReason,
    DateTime? createdAt,
    DateTime? updatedAt,
  }) {
    return PromotionInstance(
      id: id ?? this.id,
      ownershipId: ownershipId ?? this.ownershipId,
      userId: userId ?? this.userId,
      targetType: targetType ?? this.targetType,
      targetId: targetId ?? this.targetId,
      status: status ?? this.status,
      activatedAt: activatedAt ?? this.activatedAt,
      stoppedAt: stoppedAt ?? this.stoppedAt,
      stopReason: stopReason ?? this.stopReason,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
    );
  }

  @override
  List<Object?> get props => [
    id,
    ownershipId,
    userId,
    targetType,
    targetId,
    status,
    activatedAt,
    stoppedAt,
    stopReason,
    createdAt,
    updatedAt,
  ];

  @override
  String toString() {
    return 'PromotionInstance(id: $id, status: $status, targetType: $targetType)';
  }
}
