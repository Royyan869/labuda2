/// Discount DTOs for Go API communication
library;

import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';

enum DiscountTypeDto {
  percentage,
  flatAmount,
  freeShipping;

  String toJson() {
    switch (this) {
      case DiscountTypeDto.percentage:
        return 'percentage';
      case DiscountTypeDto.flatAmount:
        return 'flat_amount';
      case DiscountTypeDto.freeShipping:
        return 'free_shipping';
    }
  }

  static DiscountTypeDto fromJson(String value) {
    switch (value) {
      case 'percentage':
        return DiscountTypeDto.percentage;
      case 'flat_amount':
        return DiscountTypeDto.flatAmount;
      case 'free_shipping':
        return DiscountTypeDto.freeShipping;
      default:
        return DiscountTypeDto.percentage;
    }
  }

  DiscountType toEntity() {
    switch (this) {
      case DiscountTypeDto.percentage:
        return DiscountType.percentage;
      case DiscountTypeDto.flatAmount:
        return DiscountType.flatAmount;
      case DiscountTypeDto.freeShipping:
        return DiscountType.freeShipping;
    }
  }

  static DiscountTypeDto fromEntity(DiscountType type) {
    switch (type) {
      case DiscountType.percentage:
        return DiscountTypeDto.percentage;
      case DiscountType.flatAmount:
        return DiscountTypeDto.flatAmount;
      case DiscountType.freeShipping:
        return DiscountTypeDto.freeShipping;
    }
  }
}

enum DiscountAppliesToDto {
  listing,
  auction,
  both;

  String toJson() {
    switch (this) {
      case DiscountAppliesToDto.listing:
        return 'listing';
      case DiscountAppliesToDto.auction:
        return 'auction';
      case DiscountAppliesToDto.both:
        return 'both';
    }
  }

  static DiscountAppliesToDto fromJson(String value) {
    switch (value) {
      case 'listing':
        return DiscountAppliesToDto.listing;
      case 'auction':
        return DiscountAppliesToDto.auction;
      case 'both':
        return DiscountAppliesToDto.both;
      default:
        return DiscountAppliesToDto.both;
    }
  }

  DiscountAppliesTo toEntity() {
    switch (this) {
      case DiscountAppliesToDto.listing:
        return DiscountAppliesTo.listing;
      case DiscountAppliesToDto.auction:
        return DiscountAppliesTo.auction;
      case DiscountAppliesToDto.both:
        return DiscountAppliesTo.both;
    }
  }

  static DiscountAppliesToDto fromEntity(DiscountAppliesTo value) {
    switch (value) {
      case DiscountAppliesTo.listing:
        return DiscountAppliesToDto.listing;
      case DiscountAppliesTo.auction:
        return DiscountAppliesToDto.auction;
      case DiscountAppliesTo.both:
        return DiscountAppliesToDto.both;
    }
  }
}

enum DiscountTargetModeDto {
  sellerWide,
  selectedItems;

  String toJson() {
    switch (this) {
      case DiscountTargetModeDto.sellerWide:
        return 'seller_wide';
      case DiscountTargetModeDto.selectedItems:
        return 'selected_items';
    }
  }

  static DiscountTargetModeDto fromJson(String value) {
    switch (value) {
      case 'seller_wide':
        return DiscountTargetModeDto.sellerWide;
      case 'selected_items':
        return DiscountTargetModeDto.selectedItems;
      default:
        return DiscountTargetModeDto.sellerWide;
    }
  }

  DiscountTargetMode toEntity() {
    switch (this) {
      case DiscountTargetModeDto.sellerWide:
        return DiscountTargetMode.sellerWide;
      case DiscountTargetModeDto.selectedItems:
        return DiscountTargetMode.selectedItems;
    }
  }

  static DiscountTargetModeDto fromEntity(DiscountTargetMode value) {
    switch (value) {
      case DiscountTargetMode.sellerWide:
        return DiscountTargetModeDto.sellerWide;
      case DiscountTargetMode.selectedItems:
        return DiscountTargetModeDto.selectedItems;
    }
  }
}

class DiscountResponseDto {
  final String id;
  final String code;
  final String description;
  final DiscountTypeDto type;
  final double value;
  final double? minPurchase;
  final double? maxDiscount;
  final int? maxUsagePerUser;
  final int? totalUsageLimit;
  final DiscountAppliesToDto appliesTo;
  final DiscountTargetModeDto targetMode;
  final String? sellerId;
  final List<String>? applicableListingIds;
  final List<String>? applicableAuctionIds;
  final DateTime validFrom;
  final DateTime validUntil;
  final bool isActive;
  final int currentUsageCount;
  final DateTime createdAt;
  final String createdBy;

  const DiscountResponseDto({
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
    required this.currentUsageCount,
    required this.createdAt,
    required this.createdBy,
  });

  factory DiscountResponseDto.fromJson(Map<String, dynamic> json) {
    return DiscountResponseDto(
      id: json['id'] as String,
      code: json['code'] as String,
      description: (json['description'] as String?) ?? json['code'] as String,
      type: DiscountTypeDto.fromJson(json['type'] as String),
      value: (json['value'] as num).toDouble(),
      minPurchase: json['min_purchase'] != null
          ? (json['min_purchase'] as num).toDouble()
          : null,
      maxDiscount: json['max_discount'] != null
          ? (json['max_discount'] as num).toDouble()
          : null,
      maxUsagePerUser: json['max_usage_per_user'] as int?,
      totalUsageLimit: json['total_usage_limit'] as int?,
      appliesTo: DiscountAppliesToDto.fromJson(
        json['applies_to'] as String? ?? json['scope'] as String? ?? 'both',
      ),
      targetMode: DiscountTargetModeDto.fromJson(
        json['target_mode'] as String? ?? 'seller_wide',
      ),
      sellerId: json['seller_id'] as String?,
      applicableListingIds: json['applicable_listing_ids'] != null
          ? List<String>.from(json['applicable_listing_ids'] as List)
          : null,
      applicableAuctionIds: json['applicable_auction_ids'] != null
          ? List<String>.from(json['applicable_auction_ids'] as List)
          : null,
      validFrom: DateTime.parse(json['valid_from'] as String),
      validUntil: DateTime.parse(json['valid_until'] as String),
      isActive: json['is_active'] as bool,
      currentUsageCount: json['current_usage_count'] as int? ?? 0,
      createdAt: DateTime.parse(json['created_at'] as String),
      createdBy: (json['created_by'] as String?) ?? '',
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'code': code,
      'description': description,
      'type': type.toJson(),
      'value': value,
      'min_purchase': minPurchase,
      'max_discount': maxDiscount,
      'max_usage_per_user': maxUsagePerUser,
      'total_usage_limit': totalUsageLimit,
      'applies_to': appliesTo.toJson(),
      'target_mode': targetMode.toJson(),
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

  Discount toEntity() {
    return Discount(
      id: id,
      code: code,
      description: description,
      type: type.toEntity(),
      value: value,
      minPurchase: minPurchase,
      maxDiscount: maxDiscount,
      maxUsagePerUser: maxUsagePerUser,
      totalUsageLimit: totalUsageLimit,
      appliesTo: appliesTo.toEntity(),
      targetMode: targetMode.toEntity(),
      sellerId: sellerId,
      applicableListingIds: applicableListingIds,
      applicableAuctionIds: applicableAuctionIds,
      validFrom: validFrom,
      validUntil: validUntil,
      isActive: isActive,
      currentUsageCount: currentUsageCount,
      createdAt: createdAt,
      createdBy: createdBy,
    );
  }
}

class DiscountListResponseDto {
  final List<DiscountResponseDto> discounts;
  final int totalCount;

  const DiscountListResponseDto({
    required this.discounts,
    required this.totalCount,
  });

  factory DiscountListResponseDto.fromJson(Map<String, dynamic> json) {
    return DiscountListResponseDto(
      discounts: (json['discounts'] as List? ?? [])
          .map((e) => DiscountResponseDto.fromJson(e as Map<String, dynamic>))
          .toList(),
      totalCount: json['count'] as int? ?? json['discounts']?.length ?? 0,
    );
  }
}

class CreateDiscountRequestDto {
  final String code;
  final String description;
  final DiscountTypeDto type;
  final double value;
  final double? minPurchase;
  final double? maxDiscount;
  final int? maxUsagePerUser;
  final int? totalUsageLimit;
  final DiscountAppliesToDto appliesTo;
  final DiscountTargetModeDto targetMode;
  final String? sellerId;
  final List<String>? applicableListingIds;
  final List<String>? applicableAuctionIds;
  final DateTime validFrom;
  final DateTime validUntil;

  const CreateDiscountRequestDto({
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
  });

  static CreateDiscountRequestDto fromEntity(Discount entity) {
    return CreateDiscountRequestDto(
      code: entity.code,
      description: entity.description,
      type: DiscountTypeDto.fromEntity(entity.type),
      value: entity.value,
      minPurchase: entity.minPurchase,
      maxDiscount: entity.maxDiscount,
      maxUsagePerUser: entity.maxUsagePerUser,
      totalUsageLimit: entity.totalUsageLimit,
      appliesTo: DiscountAppliesToDto.fromEntity(entity.appliesTo),
      targetMode: DiscountTargetModeDto.fromEntity(entity.targetMode),
      sellerId: entity.sellerId,
      applicableListingIds: entity.applicableListingIds,
      applicableAuctionIds: entity.applicableAuctionIds,
      validFrom: entity.validFrom,
      validUntil: entity.validUntil,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'code': code,
      'description': description,
      'type': type.toJson(),
      'value': value,
      'min_purchase': minPurchase,
      'max_discount': maxDiscount,
      'max_usage_per_user': maxUsagePerUser,
      'total_usage_limit': totalUsageLimit,
      'applies_to': appliesTo.toJson(),
      'target_mode': targetMode.toJson(),
      'seller_id': sellerId,
      'applicable_listing_ids': applicableListingIds,
      'applicable_auction_ids': applicableAuctionIds,
      'valid_from': validFrom.toIso8601String(),
      'valid_until': validUntil.toIso8601String(),
    };
  }
}

class UpdateDiscountRequestDto {
  final String? description;
  final DiscountTypeDto? type;
  final double? value;
  final double? minPurchase;
  final double? maxDiscount;
  final int? maxUsagePerUser;
  final int? totalUsageLimit;
  final DiscountAppliesToDto? appliesTo;
  final DiscountTargetModeDto? targetMode;
  final String? sellerId;
  final List<String>? applicableListingIds;
  final List<String>? applicableAuctionIds;
  final DateTime? validFrom;
  final DateTime? validUntil;
  final bool? isActive;

  const UpdateDiscountRequestDto({
    this.description,
    this.type,
    this.value,
    this.minPurchase,
    this.maxDiscount,
    this.maxUsagePerUser,
    this.totalUsageLimit,
    this.appliesTo,
    this.targetMode,
    this.sellerId,
    this.applicableListingIds,
    this.applicableAuctionIds,
    this.validFrom,
    this.validUntil,
    this.isActive,
  });

  static UpdateDiscountRequestDto fromEntity(Discount entity) {
    return UpdateDiscountRequestDto(
      description: entity.description,
      type: DiscountTypeDto.fromEntity(entity.type),
      value: entity.value,
      minPurchase: entity.minPurchase,
      maxDiscount: entity.maxDiscount,
      maxUsagePerUser: entity.maxUsagePerUser,
      totalUsageLimit: entity.totalUsageLimit,
      appliesTo: DiscountAppliesToDto.fromEntity(entity.appliesTo),
      targetMode: DiscountTargetModeDto.fromEntity(entity.targetMode),
      sellerId: entity.sellerId,
      applicableListingIds: entity.applicableListingIds,
      applicableAuctionIds: entity.applicableAuctionIds,
      validFrom: entity.validFrom,
      validUntil: entity.validUntil,
      isActive: entity.isActive,
    );
  }

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};

    if (description != null) json['description'] = description;
    if (type != null) json['type'] = type!.toJson();
    if (value != null) json['value'] = value;
    if (minPurchase != null) json['min_purchase'] = minPurchase;
    if (maxDiscount != null) json['max_discount'] = maxDiscount;
    if (maxUsagePerUser != null) json['max_usage_per_user'] = maxUsagePerUser;
    if (totalUsageLimit != null) json['total_usage_limit'] = totalUsageLimit;
    if (appliesTo != null) json['applies_to'] = appliesTo!.toJson();
    if (targetMode != null) json['target_mode'] = targetMode!.toJson();
    if (sellerId != null) json['seller_id'] = sellerId;
    if (applicableListingIds != null) {
      json['applicable_listing_ids'] = applicableListingIds;
    }
    if (applicableAuctionIds != null) {
      json['applicable_auction_ids'] = applicableAuctionIds;
    }
    if (validFrom != null) json['valid_from'] = validFrom!.toIso8601String();
    if (validUntil != null) json['valid_until'] = validUntil!.toIso8601String();
    if (isActive != null) json['is_active'] = isActive;

    return json;
  }
}

class ApiErrorResponseDto {
  final String message;
  final int? statusCode;
  final String? errorCode;
  final Map<String, dynamic>? details;

  const ApiErrorResponseDto({
    required this.message,
    this.statusCode,
    this.errorCode,
    this.details,
  });

  factory ApiErrorResponseDto.fromJson(Map<String, dynamic> json) {
    return ApiErrorResponseDto(
      message: json['message'] as String? ?? 'Unknown error',
      statusCode: json['status_code'] as int?,
      errorCode: json['error_code'] as String?,
      details: json['details'] as Map<String, dynamic>?,
    );
  }
}
