import 'package:equatable/equatable.dart';

/// Enum for discount type
enum DiscountType { percentage, flatAmount, freeShipping }

/// Checkout contexts supported by discounts.
enum DiscountAppliesTo { listing, auction, both }

/// Target mode for a discount.
enum DiscountTargetMode { sellerWide, selectedItems }

/// Entity for Discount
///
/// Represents a seller-owned discount/voucher code that can be applied to orders.
class Discount extends Equatable {
  final String id;
  final String code;
  final String description;
  final DiscountType type;
  final double value;
  final double? minPurchase;
  final double? maxDiscount;
  final int? maxUsagePerUser;
  final int? totalUsageLimit;
  final DiscountAppliesTo appliesTo;
  final DiscountTargetMode targetMode;
  final String? sellerId;
  final List<String>? applicableListingIds;
  final List<String>? applicableAuctionIds;
  final DateTime validFrom;
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
    this.minPurchase,
    this.maxDiscount,
    this.maxUsagePerUser,
    this.totalUsageLimit,
    required this.appliesTo,
    required this.targetMode,
    this.sellerId,
    this.applicableListingIds,
    this.applicableAuctionIds,
    required this.validFrom,
    required this.validUntil,
    required this.isActive,
    this.currentUsageCount = 0,
    required this.createdAt,
    required this.createdBy,
  });

  bool get isExpired => DateTime.now().isAfter(validUntil);

  bool get isNotYetValid => DateTime.now().isBefore(validFrom);

  bool get isUsable => isActive && !isExpired && !isNotYetValid;

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
    maxDiscount,
    maxUsagePerUser,
    totalUsageLimit,
    appliesTo,
    targetMode,
    sellerId,
    applicableListingIds,
    applicableAuctionIds,
    validFrom,
    validUntil,
    isActive,
    currentUsageCount,
    createdAt,
    createdBy,
  ];
}
