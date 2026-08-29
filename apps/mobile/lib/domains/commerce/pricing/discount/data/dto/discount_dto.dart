/// Discount DTOs for Go API communication
library;

import 'package:labuda/domains/commerce/pricing/discount/domain/entities/discount_entity.dart';

enum DiscountTypeDto {
  percentage,
  flatAmount;

  String toJson() {
    switch (this) {
      case DiscountTypeDto.percentage:
        return 'percentage';
      case DiscountTypeDto.flatAmount:
        return 'flat_amount';
    }
  }

  static DiscountTypeDto fromJson(String value) {
    switch (value) {
      case 'percentage':
        return DiscountTypeDto.percentage;
      case 'flat_amount':
        return DiscountTypeDto.flatAmount;
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
    }
  }

  static DiscountTypeDto fromEntity(DiscountType type) {
    switch (type) {
      case DiscountType.percentage:
        return DiscountTypeDto.percentage;
      case DiscountType.flatAmount:
        return DiscountTypeDto.flatAmount;
    }
  }
}

enum DiscountAppliesToDto {
  forSale,
  auction,
  both;

  String toJson() {
    switch (this) {
      case DiscountAppliesToDto.forSale:
        return 'for_sale';
      case DiscountAppliesToDto.auction:
        return 'auction';
      case DiscountAppliesToDto.both:
        return 'both';
    }
  }

  static DiscountAppliesToDto fromJson(String value) {
    switch (value) {
      case 'for_sale':
        return DiscountAppliesToDto.forSale;
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
      case DiscountAppliesToDto.forSale:
        return DiscountAppliesTo.forSale;
      case DiscountAppliesToDto.auction:
        return DiscountAppliesTo.auction;
      case DiscountAppliesToDto.both:
        return DiscountAppliesTo.both;
    }
  }

  static DiscountAppliesToDto fromEntity(DiscountAppliesTo value) {
    switch (value) {
      case DiscountAppliesTo.forSale:
        return DiscountAppliesToDto.forSale;
      case DiscountAppliesTo.auction:
        return DiscountAppliesToDto.auction;
      case DiscountAppliesTo.both:
        return DiscountAppliesToDto.both;
    }
  }
}

class DiscountResponseDto {
  final String id;
  final String code;
  final String description;
  final DiscountTypeDto type;
  final double value;
  final double minPurchase;
  final int? totalUsageLimit;
  final DiscountAppliesToDto appliesTo;
  final String? sellerId;
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
    this.minPurchase = 0.0,
    this.totalUsageLimit,
    required this.appliesTo,
    this.sellerId,
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
      minPurchase: (json['min_purchase'] as num?)?.toDouble() ?? 0.0,
      totalUsageLimit: json['total_usage_limit'] as int?,
      appliesTo: DiscountAppliesToDto.fromJson(
        json['applies_to'] as String? ?? 'both',
      ),
      sellerId: json['seller_id'] as String?,
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
      'total_usage_limit': totalUsageLimit,
      'applies_to': appliesTo.toJson(),
      'seller_id': sellerId,
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
      totalUsageLimit: totalUsageLimit,
      appliesTo: appliesTo.toEntity(),
      sellerId: sellerId,
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
  final double minPurchase;
  final int? totalUsageLimit;
  final DiscountAppliesToDto appliesTo;
  final String? sellerId;
  final DateTime validUntil;

  const CreateDiscountRequestDto({
    required this.code,
    required this.description,
    required this.type,
    required this.value,
    this.minPurchase = 0.0,
    this.totalUsageLimit,
    required this.appliesTo,
    this.sellerId,
    required this.validUntil,
  });

  static CreateDiscountRequestDto fromEntity(Discount entity) {
    return CreateDiscountRequestDto(
      code: entity.code,
      description: entity.description,
      type: DiscountTypeDto.fromEntity(entity.type),
      value: entity.value,
      minPurchase: entity.minPurchase,
      totalUsageLimit: entity.totalUsageLimit,
      appliesTo: DiscountAppliesToDto.fromEntity(entity.appliesTo),
      sellerId: entity.sellerId,
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
      'total_usage_limit': totalUsageLimit,
      'applies_to': appliesTo.toJson(),
      'seller_id': sellerId,
      'valid_until': validUntil.toIso8601String(),
    };
  }
}

class UpdateDiscountRequestDto {
  final String? description;
  final DiscountTypeDto? type;
  final double? value;
  final double? minPurchase;
  final int? totalUsageLimit;
  final DiscountAppliesToDto? appliesTo;
  final String? sellerId;
  final DateTime? validUntil;
  final bool? isActive;

  const UpdateDiscountRequestDto({
    this.description,
    this.type,
    this.value,
    this.minPurchase,
    this.totalUsageLimit,
    this.appliesTo,
    this.sellerId,
    this.validUntil,
    this.isActive,
  });

  static UpdateDiscountRequestDto fromEntity(Discount entity) {
    return UpdateDiscountRequestDto(
      description: entity.description,
      type: DiscountTypeDto.fromEntity(entity.type),
      value: entity.value,
      minPurchase: entity.minPurchase,
      totalUsageLimit: entity.totalUsageLimit,
      appliesTo: DiscountAppliesToDto.fromEntity(entity.appliesTo),
      sellerId: entity.sellerId,
      validUntil: entity.validUntil,
    );
  }

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{};

    if (description != null) json['description'] = description;
    if (type != null) json['type'] = type!.toJson();
    if (value != null) json['value'] = value;
    if (minPurchase != null) json['min_purchase'] = minPurchase;
    if (totalUsageLimit != null) json['total_usage_limit'] = totalUsageLimit;
    if (appliesTo != null) json['applies_to'] = appliesTo!.toJson();
    if (sellerId != null) json['seller_id'] = sellerId;
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
