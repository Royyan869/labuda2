import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';

/// Model untuk Discount (Data Transfer Object)
///
/// CANONICAL MODEL (DISCOUNT-003):
/// - Discount applicability is by SELLING SURFACE ONLY (for_sale / auction / both)
/// - No specific item/surface targeting
class DiscountModel extends Discount {
  const DiscountModel({
    required super.id,
    required super.code,
    required super.description,
    required super.type,
    required super.value,
    super.minPurchase = 0.0,
    super.totalUsageLimit,
    required super.appliesTo,
    super.sellerId,
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
      minPurchase: (map['min_purchase'] as num?)?.toDouble() ?? 0.0,
      totalUsageLimit:
          map['totalUsageLimit'] as int? ?? map['total_usage_limit'] as int?,
      appliesTo: _parseDiscountAppliesTo(
        map['applies_to'] as String? ?? 'both',
      ),
      sellerId: map['sellerId'] as String? ?? map['seller_id'] as String?,
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
      'total_usage_limit': totalUsageLimit,
      'applies_to': _discountAppliesToToString(appliesTo),
      'seller_id': sellerId,
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
      totalUsageLimit: entity.totalUsageLimit,
      appliesTo: entity.appliesTo,
      sellerId: entity.sellerId,
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
        return DiscountType.flatAmount;
      default:
        return DiscountType.percentage;
    }
  }

  static DiscountAppliesTo _parseDiscountAppliesTo(String value) {
    switch (value) {
      case 'for_sale':
        return DiscountAppliesTo.forSale;
      case 'auction':
        return DiscountAppliesTo.auction;
      case 'both':
      default:
        return DiscountAppliesTo.both;
    }
  }

  static String _discountTypeToString(DiscountType type) {
    switch (type) {
      case DiscountType.percentage:
        return 'percentage';
      case DiscountType.flatAmount:
        return 'flat_amount';
    }
  }

  static String _discountAppliesToToString(DiscountAppliesTo value) {
    switch (value) {
      case DiscountAppliesTo.forSale:
        return 'for_sale';
      case DiscountAppliesTo.auction:
        return 'auction';
      case DiscountAppliesTo.both:
        return 'both';
    }
  }
}
