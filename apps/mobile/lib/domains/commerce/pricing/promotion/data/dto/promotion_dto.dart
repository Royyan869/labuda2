/// Promotion DTOs
///
/// Data Transfer Objects for promotion entities.
/// Used for serialization/deserialization from API responses.
library;

import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/instance_status.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/ownership_status.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/promotion_instance.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/promotion_ownership.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/promotion_package.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/target_type.dart';

/// DTO for PromotionPackage - handles API serialization
class PromotionPackageDto {
  final String id;
  final String name;
  final int totalDurationHours;
  final int validityWindowHours;
  final int priceAmount;
  final List<String> allowedTargetTypes;
  final bool isActive;
  final DateTime createdAt;

  const PromotionPackageDto({
    required this.id,
    required this.name,
    required this.totalDurationHours,
    required this.validityWindowHours,
    required this.priceAmount,
    required this.allowedTargetTypes,
    required this.isActive,
    required this.createdAt,
  });

  /// Creates DTO from JSON (API response)
  factory PromotionPackageDto.fromJson(Map<String, dynamic> json) {
    return PromotionPackageDto(
      id: json['id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      totalDurationHours: json['total_duration_hours'] as int? ?? 0,
      validityWindowHours: json['validity_window_hours'] as int? ?? 0,
      priceAmount: json['price_amount'] as int? ?? 0,
      allowedTargetTypes:
          (json['allowed_target_types'] as List<dynamic>?)
              ?.map((e) => e as String)
              .toList() ??
          [],
      isActive: json['is_active'] as bool? ?? true,
      createdAt: _parseDateTime(json['created_at']),
    );
  }

  /// Converts to entity
  PromotionPackage toEntity() {
    return PromotionPackage(
      id: id,
      name: name,
      totalDurationHours: totalDurationHours,
      validityWindowHours: validityWindowHours,
      priceAmount: priceAmount,
      allowedTargetTypes: allowedTargetTypes
          .map((type) => TargetType.fromString(type))
          .toList(),
      isActive: isActive,
      createdAt: createdAt,
    );
  }

  static DateTime _parseDateTime(dynamic value) {
    if (value is DateTime) return value;
    if (value is String) return DateTime.parse(value);
    return DateTime.now();
  }
}

/// DTO for PromotionOwnership - handles API serialization
class PromotionOwnershipDto {
  final String id;
  final String userId;
  final String packageId;
  final String status;
  final DateTime purchasedAt;
  final DateTime expiresAt;
  final int totalDurationHours;
  final int consumedDurationHours;
  final DateTime createdAt;
  final DateTime updatedAt;

  const PromotionOwnershipDto({
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

  /// Creates DTO from JSON (API response)
  factory PromotionOwnershipDto.fromJson(Map<String, dynamic> json) {
    return PromotionOwnershipDto(
      id: json['id'] as String? ?? '',
      userId: json['user_id'] as String? ?? '',
      packageId: json['package_id'] as String? ?? '',
      status: json['status'] as String? ?? 'available',
      purchasedAt: _parseDateTime(json['purchased_at']),
      expiresAt: _parseDateTime(json['expires_at']),
      totalDurationHours: json['total_duration_hours'] as int? ?? 0,
      consumedDurationHours: json['consumed_duration_hours'] as int? ?? 0,
      createdAt: _parseDateTime(json['created_at']),
      updatedAt: _parseDateTime(json['updated_at']),
    );
  }

  /// Converts to entity
  PromotionOwnership toEntity() {
    return PromotionOwnership(
      id: id,
      userId: userId,
      packageId: packageId,
      status: OwnershipStatus.fromString(status),
      purchasedAt: purchasedAt,
      expiresAt: expiresAt,
      totalDurationHours: totalDurationHours,
      consumedDurationHours: consumedDurationHours,
      createdAt: createdAt,
      updatedAt: updatedAt,
    );
  }

  static DateTime _parseDateTime(dynamic value) {
    if (value is DateTime) return value;
    if (value is String) return DateTime.parse(value);
    return DateTime.now();
  }
}

/// DTO for PromotionInstance - handles API serialization
class PromotionInstanceDto {
  final String id;
  final String ownershipId;
  final String userId;
  final String targetType;
  final String? targetId;
  final String status;
  final DateTime? activatedAt;
  final DateTime? stoppedAt;
  final String? stopReason;
  final DateTime createdAt;
  final DateTime updatedAt;

  const PromotionInstanceDto({
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

  /// Creates DTO from JSON (API response)
  factory PromotionInstanceDto.fromJson(Map<String, dynamic> json) {
    return PromotionInstanceDto(
      id: json['id'] as String? ?? '',
      ownershipId: json['ownership_id'] as String? ?? '',
      userId: json['user_id'] as String? ?? '',
      targetType: json['target_type'] as String? ?? 'for_sale',
      targetId: json['target_id'] as String?,
      status: json['status'] as String? ?? 'inactive',
      activatedAt: json['activated_at'] != null
          ? _parseDateTime(json['activated_at'])
          : null,
      stoppedAt: json['stopped_at'] != null
          ? _parseDateTime(json['stopped_at'])
          : null,
      stopReason: json['stop_reason'] as String?,
      createdAt: _parseDateTime(json['created_at']),
      updatedAt: _parseDateTime(json['updated_at']),
    );
  }

  /// Converts to entity
  PromotionInstance toEntity() {
    return PromotionInstance(
      id: id,
      ownershipId: ownershipId,
      userId: userId,
      targetType: TargetType.fromString(targetType),
      targetId: targetId,
      status: InstanceStatus.fromString(status),
      activatedAt: activatedAt,
      stoppedAt: stoppedAt,
      stopReason: stopReason,
      createdAt: createdAt,
      updatedAt: updatedAt,
    );
  }

  static DateTime _parseDateTime(dynamic value) {
    if (value is DateTime) return value;
    if (value is String) return DateTime.parse(value);
    return DateTime.now();
  }
}

/// Request DTO for activating a promotion
class ActivatePromotionRequestDto {
  final String ownershipId;
  final String targetType;
  final String? targetId;

  const ActivatePromotionRequestDto({
    required this.ownershipId,
    required this.targetType,
    this.targetId,
  });

  /// Converts to JSON for API request
  Map<String, dynamic> toJson() {
    return {
      'ownership_id': ownershipId,
      'target_type': targetType,
      if (targetId != null) 'target_id': targetId,
    };
  }
}

/// Request DTO for deactivating a promotion
class DeactivatePromotionRequestDto {
  final String reason; // 'user_paused' or 'user_cancelled'

  const DeactivatePromotionRequestDto({required this.reason});

  /// Converts to JSON for API request
  Map<String, dynamic> toJson() {
    return {'reason': reason};
  }
}

/// Request DTO for reassigning a promotion
class ReassignPromotionRequestDto {
  final String newTargetType;
  final String? newTargetId;

  const ReassignPromotionRequestDto({
    required this.newTargetType,
    this.newTargetId,
  });

  /// Converts to JSON for API request
  Map<String, dynamic> toJson() {
    return {
      'new_target_type': newTargetType,
      if (newTargetId != null) 'new_target_id': newTargetId,
    };
  }
}

// =============================================================================
// PROMOTION DISCOVERY DTOs (Phase 4)
// =============================================================================

/// Response DTO for promoted items discovery endpoint
/// Used by search, home, and other discovery surfaces
class PromotedItemDto {
  final String instanceId;
  final String targetType;
  final String? targetId;
  final String? externalUrl;
  final String? externalTitle;
  final String? externalMediaUrl;

  const PromotedItemDto({
    required this.instanceId,
    required this.targetType,
    this.targetId,
    this.externalUrl,
    this.externalTitle,
    this.externalMediaUrl,
  });

  /// Creates DTO from JSON (API response)
  factory PromotedItemDto.fromJson(Map<String, dynamic> json) {
    return PromotedItemDto(
      instanceId: json['instance_id'] as String? ?? '',
      targetType: json['target_type'] as String? ?? 'for_sale',
      targetId: json['target_id'] as String?,
      externalUrl: json['external_url'] as String?,
      // Legacy inline path sends 'external_title'; public external_product
      // entity path sends 'title'. Prefer the source key when both are present.
      externalTitle:
          json['external_title'] as String? ?? json['title'] as String?,
      externalMediaUrl: json['external_media_url'] as String?,
    );
  }

  /// Whether this is an external product promotion
  bool get isExternalProduct => targetType == 'external_product';

  /// Whether this is a for-sale promotion (canonical backend wire: 'for_sale')
  bool get isForSale => targetType == 'for_sale';

  /// Whether this is an auction promotion
  bool get isAuction => targetType == 'auction';
}

/// Response wrapper for promoted items discovery
class PromotedItemsResponse {
  final List<PromotedItemDto> promotedItems;
  final int count;

  const PromotedItemsResponse({
    required this.promotedItems,
    required this.count,
  });

  /// Creates response from JSON (API response)
  factory PromotedItemsResponse.fromJson(Map<String, dynamic> json) {
    final itemsList = json['promoted_items'] as List<dynamic>? ?? [];
    final items = itemsList
        .map((item) => PromotedItemDto.fromJson(item as Map<String, dynamic>))
        .toList();

    return PromotedItemsResponse(
      promotedItems: items,
      count: json['count'] as int? ?? items.length,
    );
  }

  /// Empty response
  static const empty = PromotedItemsResponse(promotedItems: [], count: 0);

  /// Whether there are any promoted items
  bool get hasItems => promotedItems.isNotEmpty;
}

// =============================================================================
// PROMOTION PURCHASE DTOs (Phase 5)
// =============================================================================

/// Request DTO for purchasing a promotion package
///
/// PURCHASE TRUTH ENFORCEMENT:
/// - Client provides ONLY package_id
/// - Price is ALWAYS derived from server package data
/// - No amount or price is sent from client (security)
class PurchasePromotionPackageRequestDto {
  final String packageId;

  const PurchasePromotionPackageRequestDto({required this.packageId});

  /// Converts to JSON for API request
  Map<String, dynamic> toJson() {
    return {'package_id': packageId};
  }
}

/// Response DTO for promotion package purchase initiation
class PurchasePromotionPackageResponseDto {
  final String billingId;
  final String? message;
  final int amount;

  const PurchasePromotionPackageResponseDto({
    required this.billingId,
    this.message,
    required this.amount,
  });

  /// Creates response from JSON (API response)
  factory PurchasePromotionPackageResponseDto.fromJson(
    Map<String, dynamic> json,
  ) {
    return PurchasePromotionPackageResponseDto(
      billingId: json['billing_id'] as String? ?? '',
      message: json['message'] as String?,
      amount: json['amount'] as int? ?? 0,
    );
  }
}

/// Response DTO for billing payment initiation.
class InitiateBillingPaymentResponseDto {
  final String paymentId;
  final String paymentUrl;
  final int grossAmount;
  final DateTime? expiredAt;

  const InitiateBillingPaymentResponseDto({
    required this.paymentId,
    required this.paymentUrl,
    required this.grossAmount,
    required this.expiredAt,
  });

  factory InitiateBillingPaymentResponseDto.fromJson(
    Map<String, dynamic> json,
  ) {
    return InitiateBillingPaymentResponseDto(
      paymentId: json['payment_id'] as String? ?? '',
      paymentUrl: json['payment_url'] as String? ?? '',
      grossAmount: json['gross_amount'] as int? ?? 0,
      expiredAt: json['expired_at'] != null
          ? DateTime.tryParse(json['expired_at'] as String)
          : null,
    );
  }
}
