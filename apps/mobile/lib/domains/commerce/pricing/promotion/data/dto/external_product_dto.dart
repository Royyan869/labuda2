/// DTOs for external product management APIs.
library;

import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product_media.dart';
import 'package:labuda/domains/commerce/pricing/promotion/domain/entities/external_product_review_status.dart';

// =============================================================================
// RESPONSE DTOs
// =============================================================================

/// DTO for ExternalProductMediaResponse from backend.
class ExternalProductMediaDto {
  final String id;
  final String externalProductId;
  final String mediaType;
  final String storageKey;
  final String url;
  final String? thumbnailUrl;
  final int sortOrder;
  final DateTime createdAt;

  const ExternalProductMediaDto({
    required this.id,
    required this.externalProductId,
    required this.mediaType,
    required this.storageKey,
    required this.url,
    this.thumbnailUrl,
    required this.sortOrder,
    required this.createdAt,
  });

  factory ExternalProductMediaDto.fromJson(Map<String, dynamic> json) {
    return ExternalProductMediaDto(
      id: json['id'] as String? ?? '',
      externalProductId: json['external_product_id'] as String? ?? '',
      mediaType: json['media_type'] as String? ?? 'image',
      storageKey: json['storage_key'] as String? ?? '',
      url: json['url'] as String? ?? '',
      thumbnailUrl: json['thumbnail_url'] as String?,
      sortOrder: json['sort_order'] as int? ?? 0,
      createdAt: _parseDateTime(json['created_at']),
    );
  }

  ExternalProductMedia toEntity() {
    return ExternalProductMedia(
      id: id,
      externalProductId: externalProductId,
      mediaType: mediaType,
      storageKey: storageKey,
      url: url,
      thumbnailUrl: thumbnailUrl,
      sortOrder: sortOrder,
      createdAt: createdAt,
    );
  }

  static DateTime _parseDateTime(dynamic value) {
    if (value is DateTime) return value;
    if (value is String) return DateTime.parse(value);
    return DateTime.now();
  }
}

/// DTO for ExternalProductResponse from backend.
class ExternalProductDto {
  final String id;
  final String ownerUserId;
  final String title;
  final String? description;
  final String externalUrl;
  final String normalizedExternalUrl;
  final String reviewStatus;
  final String? rejectionReason;
  final bool unsafeUrlFlag;
  final DateTime? submittedAt;
  final DateTime? approvedAt;
  final DateTime? rejectedAt;
  final DateTime? hiddenAt;
  final DateTime createdAt;
  final DateTime updatedAt;
  final List<ExternalProductMediaDto> media;
  final bool canEdit;
  final bool canSubmit;
  final bool canResubmit;
  final bool publicVisible;

  const ExternalProductDto({
    required this.id,
    required this.ownerUserId,
    required this.title,
    this.description,
    required this.externalUrl,
    required this.normalizedExternalUrl,
    required this.reviewStatus,
    this.rejectionReason,
    required this.unsafeUrlFlag,
    this.submittedAt,
    this.approvedAt,
    this.rejectedAt,
    this.hiddenAt,
    required this.createdAt,
    required this.updatedAt,
    this.media = const [],
    required this.canEdit,
    required this.canSubmit,
    required this.canResubmit,
    required this.publicVisible,
  });

  factory ExternalProductDto.fromJson(Map<String, dynamic> json) {
    final mediaJson = json['media'] as List<dynamic>? ?? [];
    return ExternalProductDto(
      id: json['id'] as String? ?? '',
      ownerUserId: json['owner_user_id'] as String? ?? '',
      title: json['title'] as String? ?? '',
      description: json['description'] as String?,
      externalUrl: json['external_url'] as String? ?? '',
      normalizedExternalUrl: json['normalized_external_url'] as String? ?? '',
      reviewStatus: json['review_status'] as String? ?? 'draft',
      rejectionReason: json['rejection_reason'] as String?,
      unsafeUrlFlag: json['unsafe_url_flag'] as bool? ?? false,
      submittedAt: _parseNullableDateTime(json['submitted_at']),
      approvedAt: _parseNullableDateTime(json['approved_at']),
      rejectedAt: _parseNullableDateTime(json['rejected_at']),
      hiddenAt: _parseNullableDateTime(json['hidden_at']),
      createdAt: _parseDateTime(json['created_at']),
      updatedAt: _parseDateTime(json['updated_at']),
      media: mediaJson
          .map(
            (m) => ExternalProductMediaDto.fromJson(m as Map<String, dynamic>),
          )
          .toList(),
      canEdit: json['can_edit'] as bool? ?? false,
      canSubmit: json['can_submit'] as bool? ?? false,
      canResubmit: json['can_resubmit'] as bool? ?? false,
      publicVisible: json['public_visible'] as bool? ?? false,
    );
  }

  ExternalProduct toEntity() {
    return ExternalProduct(
      id: id,
      ownerUserId: ownerUserId,
      title: title,
      description: description,
      externalUrl: externalUrl,
      normalizedExternalUrl: normalizedExternalUrl,
      reviewStatus: ExternalProductReviewStatus.fromString(reviewStatus),
      rejectionReason: rejectionReason,
      unsafeUrlFlag: unsafeUrlFlag,
      submittedAt: submittedAt,
      approvedAt: approvedAt,
      rejectedAt: rejectedAt,
      hiddenAt: hiddenAt,
      createdAt: createdAt,
      updatedAt: updatedAt,
      media: media.map((m) => m.toEntity()).toList(),
      canEdit: canEdit,
      canSubmit: canSubmit,
      canResubmit: canResubmit,
      publicVisible: publicVisible,
    );
  }

  static DateTime _parseDateTime(dynamic value) {
    if (value is DateTime) return value;
    if (value is String) return DateTime.parse(value);
    return DateTime.now();
  }

  static DateTime? _parseNullableDateTime(dynamic value) {
    if (value == null) return null;
    if (value is DateTime) return value;
    if (value is String) return DateTime.tryParse(value);
    return null;
  }
}

// =============================================================================
// REQUEST DTOs
// =============================================================================

/// Request DTO for creating an external product draft.
class CreateExternalProductRequestDto {
  final String title;
  final String externalUrl;
  final String? description;

  const CreateExternalProductRequestDto({
    required this.title,
    required this.externalUrl,
    this.description,
  });

  Map<String, dynamic> toJson() {
    return {
      'title': title,
      'external_url': externalUrl,
      if (description != null) 'description': description,
    };
  }
}

/// Request DTO for updating an external product.
class UpdateExternalProductRequestDto {
  final String? title;
  final String? description;
  final String? externalUrl;

  const UpdateExternalProductRequestDto({
    this.title,
    this.description,
    this.externalUrl,
  });

  Map<String, dynamic> toJson() {
    return {
      if (title != null) 'title': title,
      if (description != null) 'description': description,
      if (externalUrl != null) 'external_url': externalUrl,
    };
  }
}

/// Request DTO for submitting an external product for review.
class SubmitExternalProductRequestDto {
  final String? note;

  const SubmitExternalProductRequestDto({this.note});

  Map<String, dynamic> toJson() {
    return {if (note != null) 'note': note};
  }
}

/// Request DTO for attaching media to an external product.
class AttachExternalProductMediaRequestDto {
  final String mediaType; // 'image' or 'video'
  final String storageKey;
  final String url;
  final String? thumbnailUrl;
  final int? sortOrder;

  const AttachExternalProductMediaRequestDto({
    required this.mediaType,
    required this.storageKey,
    required this.url,
    this.thumbnailUrl,
    this.sortOrder,
  });

  Map<String, dynamic> toJson() {
    return {
      'media_type': mediaType,
      'storage_key': storageKey,
      'url': url,
      if (thumbnailUrl != null) 'thumbnail_url': thumbnailUrl,
      if (sortOrder != null) 'sort_order': sortOrder,
    };
  }
}
