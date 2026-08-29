import 'package:equatable/equatable.dart';

/// Enum for discount type — canonical model: percentage, flat_amount only.
enum DiscountType { percentage, flatAmount }

/// Checkout contexts supported by discounts.
enum DiscountAppliesTo { forSale, auction, both }

/// Entity for Discount
///
/// Represents a seller-owned discount/voucher code that can be applied to orders.
///
/// CANONICAL MODEL (DISCOUNT-003):
/// - Discount applicability is by SELLING SURFACE ONLY (for_sale / auction / both)
/// - No specific item/surface targeting — discount applies to ALL surfaces of the seller's chosen type
/// - Discount types: percentage, flat_amount only
/// - Validity: expiry-only (validUntil). Discount is active from creation.
/// - Usage: optional totalUsageLimit (0 = unlimited). No per-user limit.
/// - Minimum purchase: optional minPurchase against the final transaction price P
/// - Anyone who knows the code may attempt to use it
class Discount extends Equatable {
  final String id;
  final String code;
  final String description;
  final DiscountType type;
  final double value;
  final double minPurchase;
  final int? totalUsageLimit;
  final DiscountAppliesTo appliesTo;
  final String? sellerId;
  final DateTime validUntil;
  final bool isActive;
  final int currentUsageCount;
  final DateTime createdAt;
  final String createdBy;

  const Discount({
    required this.id,
    required this.code,
    required this.description,
    required this.type,
    required this.value,
    this.minPurchase = 0.0,
    this.totalUsageLimit,
    required this.appliesTo,
    this.sellerId,
    required this.validUntil,
    required this.isActive,
    this.currentUsageCount = 0,
    required this.createdAt,
    required this.createdBy,
  });

  bool get isExpired => DateTime.now().isAfter(validUntil);

  bool get isUsable => isActive && !isExpired;

  bool get hasReachedTotalLimit {
    if (totalUsageLimit == null) return false;
    return currentUsageCount >= totalUsageLimit!;
  }

  @override
  List<Object?> get props => [
    id,
    code,
    description,
    type,
    value,
    minPurchase,
    totalUsageLimit,
    appliesTo,
    sellerId,
    validUntil,
    isActive,
    currentUsageCount,
    createdAt,
    createdBy,
  ];
}
