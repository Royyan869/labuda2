import 'package:equatable/equatable.dart';

import 'target_type.dart';

/// Promotion package entity.
///
/// Represents a purchasable promotion package.
/// This is the product catalog - read-only after creation.
class PromotionPackage extends Equatable {
  /// Unique identifier
  final String id;

  /// Package name (e.g., "Promote Basic (3 Days)")
  final String name;

  /// Total duration in hours (e.g., 72, 168, 336)
  final int totalDurationHours;

  /// Validity window in hours (e.g., 336 = 14 days to use)
  final int validityWindowHours;

  /// Price, Rupiah integer
  final int priceAmount;

  /// Allowed target types for this package
  final List<TargetType> allowedTargetTypes;

  /// Whether this package is currently active
  final bool isActive;

  /// When this package was created
  final DateTime createdAt;

  const PromotionPackage({
    required this.id,
    required this.name,
    required this.totalDurationHours,
    required this.validityWindowHours,
    required this.priceAmount,
    required this.allowedTargetTypes,
    required this.isActive,
    required this.createdAt,
  });

  /// Checks if this package allows promoting the given target type
  bool allowsTargetType(TargetType targetType) {
    return allowedTargetTypes.contains(targetType);
  }

  /// Calculates expiry timestamp for an ownership purchased at given time
  DateTime calculateExpiry(DateTime purchasedAt) {
    return purchasedAt.add(Duration(hours: validityWindowHours));
  }

  /// Creates a copy with updated fields
  PromotionPackage copyWith({
    String? id,
    String? name,
    int? totalDurationHours,
    int? validityWindowHours,
    int? priceAmount,
    List<TargetType>? allowedTargetTypes,
    bool? isActive,
    DateTime? createdAt,
  }) {
    return PromotionPackage(
      id: id ?? this.id,
      name: name ?? this.name,
      totalDurationHours: totalDurationHours ?? this.totalDurationHours,
      validityWindowHours: validityWindowHours ?? this.validityWindowHours,
      priceAmount: priceAmount ?? this.priceAmount,
      allowedTargetTypes: allowedTargetTypes ?? this.allowedTargetTypes,
      isActive: isActive ?? this.isActive,
      createdAt: createdAt ?? this.createdAt,
    );
  }

  @override
  List<Object?> get props => [
    id,
    name,
    totalDurationHours,
    validityWindowHours,
    priceAmount,
    allowedTargetTypes,
    isActive,
    createdAt,
  ];

  @override
  String toString() {
    return 'PromotionPackage(id: $id, name: $name, priceAmount: $priceAmount)';
  }
}
