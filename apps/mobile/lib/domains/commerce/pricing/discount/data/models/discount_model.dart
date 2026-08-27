import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';

/// Model untuk Discount (Data Transfer Object)
class DiscountModel extends Discount {
  const DiscountModel({
    required super.id,
    required super.code,
    required super.description,
    required super.type,
    required super.value,
    super.minPurchase,
    super.maxDiscount,
    super.maxUsagePerUser,
    super.totalUsageLimit,
    required super.appliesTo,
    required super.targetMode,
    super.sellerId,
    super.applicableListingIds,
    super.applicableAuctionIds,
    required super.validFrom,
    required super.validUntil,
    required super.isActive,
    super.currentUsageCount = 0,
    required super.createdAt,
    required super.createdBy,
  });

  factory DiscountModel.fromMap(Map<String, dynamic> map, String id) {
    DateTime parseTimestamp(dynamic value) {
      if (value == null) return DateTime.now();
      if (value is DateTime) return value;
      if (value is String) return DateTime.parse(value);
      return DateTime.now();
    }

    return DiscountModel(
      id: id,
      code: map['code'] as String,
      description: (map['description'] as String?) ?? map['code'] as String,
      type: _parseDiscountType(map['type'] as String),
      value: (map['value'] as num).toDouble(),
      minPurchase: map['minPurchase'] != null
          ? (map['minPurchase'] as num).toDouble()
          : map['min_purchase'] != null
          ? (map['min_purchase'] as num).toDouble()
          : null,
      maxDiscount: map['maxDiscount'] != null
          ? (map['maxDiscount'] as num).toDouble()
          : map['max_discount'] != null
          ? (map['max_discount'] as num).toDouble()
          : null,
      maxUsagePerUser:
          map['maxUsagePerUser'] as int? ?? map['max_usage_per_user'] as int?,
      totalUsageLimit:
          map['totalUsageLimit'] as int? ?? map['total_usage_limit'] as int?,
      appliesTo: _parseDiscountAppliesTo(
        map['applies_to'] as String? ?? map['scope'] as String? ?? 'both',
      ),
      targetMode: _parseDiscountTargetMode(
        map['target_mode'] as String? ?? 'seller_wide',
      ),
      sellerId: map['sellerId'] as String? ?? map['seller_id'] as String?,
      applicableListingIds: _parseStringList(
        map['applicableListingIds'] ?? map['applicable_listing_ids'],
      ),
      applicableAuctionIds: _parseStringList(
        map['applicableAuctionIds'] ?? map['applicable_auction_ids'],
      ),
      validFrom: parseTimestamp(map['validFrom'] ?? map['valid_from']),
      validUntil: parseTimestamp(map['validUntil'] ?? map['valid_until']),
      isActive: map['isActive'] as bool? ?? map['is_active'] as bool? ?? true,
      currentUsageCount:
          map['currentUsageCount'] as int? ??
          map['current_usage_count'] as int? ??
          0,
      createdAt: parseTimestamp(map['createdAt'] ?? map['created_at']),
      createdBy:
          map['createdBy'] as String? ?? map['created_by'] as String? ?? '',
    );
  }

  Map<String, dynamic> toMap() {
    return {
      'code': code,
      'description': description,
      'type': _discountTypeToString(type),
      'value': value,
      'min_purchase': minPurchase,
      'max_discount': maxDiscount,
      'max_usage_per_user': maxUsagePerUser,
      'total_usage_limit': totalUsageLimit,
      'applies_to': _discountAppliesToToString(appliesTo),
      'target_mode': _discountTargetModeToString(targetMode),
      'seller_id': sellerId,
      'applicable_listing_ids': applicableListingIds,
      'applicable_auction_ids': applicableAuctionIds,
      'valid_from': validFrom.toIso8601String(),
      'valid_until': validUntil.toIso8601String(),
      'is_active': isActive,
      'current_usage_count': currentUsageCount,
      'created_at': createdAt.toIso8601String(),
      'created_by': createdBy,
    };
  }

  factory DiscountModel.fromEntity(Discount entity) {
    return DiscountModel(
      id: entity.id,
      code: entity.code,
      description: entity.description,
      type: entity.type,
      value: entity.value,
      minPurchase: entity.minPurchase,
      maxDiscount: entity.maxDiscount,
      maxUsagePerUser: entity.maxUsagePerUser,
      totalUsageLimit: entity.totalUsageLimit,
      appliesTo: entity.appliesTo,
      targetMode: entity.targetMode,
      sellerId: entity.sellerId,
      applicableListingIds: entity.applicableListingIds,
      applicableAuctionIds: entity.applicableAuctionIds,
      validFrom: entity.validFrom,
      validUntil: entity.validUntil,
      isActive: entity.isActive,
      currentUsageCount: entity.currentUsageCount,
      createdAt: entity.createdAt,
      createdBy: entity.createdBy,
    );
  }

  static DiscountType _parseDiscountType(String value) {
    switch (value) {
      case 'percentage':
        return DiscountType.percentage;
      case 'flat_amount':
      case 'flatAmount':
        return DiscountType.flatAmount;
      case 'free_shipping':
      case 'freeShipping':
        return DiscountType.freeShipping;
      default:
        return DiscountType.percentage;
    }
  }

  static DiscountAppliesTo _parseDiscountAppliesTo(String value) {
    switch (value) {
      case 'listing':
        return DiscountAppliesTo.listing;
      case 'auction':
        return DiscountAppliesTo.auction;
      case 'both':
      default:
        return DiscountAppliesTo.both;
    }
  }

  static DiscountTargetMode _parseDiscountTargetMode(String value) {
    switch (value) {
      case 'selected_items':
        return DiscountTargetMode.selectedItems;
      case 'seller_wide':
      default:
        return DiscountTargetMode.sellerWide;
    }
  }

  static List<String>? _parseStringList(dynamic value) {
    if (value == null) return null;
    if (value is List) {
      return value.map((e) => e.toString()).toList();
    }
    return null;
  }

  static String _discountTypeToString(DiscountType type) {
    switch (type) {
      case DiscountType.percentage:
        return 'percentage';
      case DiscountType.flatAmount:
        return 'flat_amount';
      case DiscountType.freeShipping:
        return 'free_shipping';
    }
  }

  static String _discountAppliesToToString(DiscountAppliesTo value) {
    switch (value) {
      case DiscountAppliesTo.listing:
        return 'listing';
      case DiscountAppliesTo.auction:
        return 'auction';
      case DiscountAppliesTo.both:
        return 'both';
    }
  }

  static String _discountTargetModeToString(DiscountTargetMode value) {
    switch (value) {
      case DiscountTargetMode.sellerWide:
        return 'seller_wide';
      case DiscountTargetMode.selectedItems:
        return 'selected_items';
    }
  }
}
