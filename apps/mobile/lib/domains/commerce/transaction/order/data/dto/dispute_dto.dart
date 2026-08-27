/// Dispute DTOs for dispute/escalation flow
library;

import 'package:labuda/domains/commerce/transaction/order/domain/entities/refund_request.dart';

/// Dispute DTO - represents a dispute created by buyer after seller rejection
class DisputeDto {
  final String id;
  final String orderId;
  final String buyerId;
  final String sellerId;
  final DisputeStatus status;
  final String reason;
  final String? description;
  final List<String>? evidenceUrls;
  // Admin resolution fields - populated when dispute is resolved
  final String? adminNotes;
  final String? reviewedBy;
  final DateTime? reviewedAt;
  final DateTime createdAt;
  final DateTime? resolvedAt;

  DisputeDto({
    required this.id,
    required this.orderId,
    required this.buyerId,
    required this.sellerId,
    required this.status,
    required this.reason,
    this.description,
    this.evidenceUrls,
    this.adminNotes,
    this.reviewedBy,
    this.reviewedAt,
    required this.createdAt,
    this.resolvedAt,
  });

  factory DisputeDto.fromJson(Map<String, dynamic> json) {
    return DisputeDto(
      id: json['id'] as String? ?? '',
      orderId: json['order_id'] as String? ?? json['orderId'] as String? ?? '',
      buyerId: json['buyer_id'] as String? ?? json['buyerId'] as String? ?? '',
      sellerId:
          json['seller_id'] as String? ?? json['sellerId'] as String? ?? '',
      status: _parseDisputeStatus(json['status'] as String? ?? ''),
      reason: json['reason'] as String? ?? '',
      description: json['description'] as String?,
      evidenceUrls:
          (json['evidence_urls'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          (json['evidenceUrls'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList(),
      // Resolution notes from admin - maps to backend's resolution_notes
      adminNotes:
          json['resolution_notes'] as String? ??
          json['admin_notes'] as String? ??
          json['adminNotes'] as String?,
      // Admin who resolved - maps to backend's resolved_by
      reviewedBy:
          json['resolved_by'] as String? ??
          json['reviewed_by'] as String? ??
          json['reviewedBy'] as String?,
      // When resolved - maps to backend's resolved_at
      reviewedAt: json['resolved_at'] != null
          ? DateTime.parse(json['resolved_at'] as String)
          : (json['reviewed_at'] != null
                ? DateTime.parse(json['reviewed_at'] as String)
                : (json['reviewedAt'] != null
                      ? DateTime.parse(json['reviewedAt'] as String)
                      : null)),
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
      resolvedAt: json['resolved_at'] != null
          ? DateTime.parse(json['resolved_at'] as String)
          : (json['resolvedAt'] != null
                ? DateTime.parse(json['resolvedAt'] as String)
                : null),
    );
  }

  static DisputeStatus _parseDisputeStatus(String status) {
    switch (status.toLowerCase()) {
      case 'under_review':
      case 'underreview':
        return DisputeStatus.underReview;
      case 'resolved_refund':
      case 'resolvedrefund':
        return DisputeStatus.resolvedRefund;
      case 'resolved_release':
      case 'resolvedrelease':
        return DisputeStatus.resolvedRelease;
      case 'resolved_partial':
      case 'resolvedpartial':
        return DisputeStatus.resolvedPartial;
      default:
        return DisputeStatus.underReview;
    }
  }

  /// Convert to RefundStatus for compatibility with existing UI
  RefundStatus toRefundStatus() {
    switch (status) {
      case DisputeStatus.underReview:
        return RefundStatus.escalatedToAdmin;
      case DisputeStatus.resolvedRefund:
        return RefundStatus.adminApproved;
      case DisputeStatus.resolvedRelease:
        return RefundStatus.rejected;
      case DisputeStatus.resolvedPartial:
        return RefundStatus.adminApproved;
    }
  }
}

/// Dispute status enum
enum DisputeStatus {
  underReview,
  resolvedRefund,
  resolvedRelease,
  resolvedPartial;

  String get displayName {
    switch (this) {
      case DisputeStatus.underReview:
        return 'Under Admin Review';
      case DisputeStatus.resolvedRefund:
        return 'Refund Approved';
      case DisputeStatus.resolvedRelease:
        return 'Released to Seller';
      case DisputeStatus.resolvedPartial:
        return 'Partially Resolved';
    }
  }

  String get emoji {
    switch (this) {
      case DisputeStatus.underReview:
        return '🔍';
      case DisputeStatus.resolvedRefund:
        return '✅';
      case DisputeStatus.resolvedRelease:
        return '💰';
      case DisputeStatus.resolvedPartial:
        return '🤝';
    }
  }

  String toApiString() {
    switch (this) {
      case DisputeStatus.underReview:
        return 'under_review';
      case DisputeStatus.resolvedRefund:
        return 'resolved_refund';
      case DisputeStatus.resolvedRelease:
        return 'resolved_release';
      case DisputeStatus.resolvedPartial:
        return 'resolved_partial';
    }
  }
}

/// Create dispute request DTO
class CreateDisputeDto {
  final String reason;
  final String? reasonCode;
  final String? description;
  final List<String>? evidenceUrls;
  final String? videoUrl;

  CreateDisputeDto({
    required this.reason,
    this.reasonCode,
    this.description,
    this.evidenceUrls,
    this.videoUrl,
  });

  Map<String, dynamic> toJson() {
    return {
      'reason': reason,
      if (reasonCode != null) 'reason_code': reasonCode,
      if (description != null) 'description': description,
      if (evidenceUrls != null && evidenceUrls!.isNotEmpty)
        'evidence_urls': evidenceUrls,
      if (videoUrl != null) 'video_url': videoUrl,
    };
  }
}

/// Dispute list response DTO
class DisputeListDto {
  final List<DisputeDto> data;
  final int? total;
  final int? page;
  final int? pageSize;

  DisputeListDto({required this.data, this.total, this.page, this.pageSize});

  factory DisputeListDto.fromJson(Map<String, dynamic> json) {
    return DisputeListDto(
      data:
          (json['data'] as List<dynamic>?)
              ?.map((e) => DisputeDto.fromJson(e as Map<String, dynamic>))
              .toList() ??
          [],
      total: json['total'] as int?,
      page: json['page'] as int?,
      pageSize: json['page_size'] as int? ?? json['pageSize'] as int?,
    );
  }
}

/// Dispute filter params for listing
class DisputeFilterParams {
  final String? status;
  final int? page;
  final int? pageSize;

  DisputeFilterParams({this.status, this.page, this.pageSize});

  Map<String, dynamic> toQueryParams() {
    final params = <String, dynamic>{};
    if (status != null) params['status'] = status;
    if (page != null) params['page'] = page;
    if (pageSize != null) params['page_size'] = pageSize;
    return params;
  }
}

/// Admin dispute resolution request DTO
class AdminDisputeResolutionDto {
  final String? notes;

  AdminDisputeResolutionDto({this.notes});

  Map<String, dynamic> toJson() {
    final map = <String, dynamic>{};
    if (notes != null) map['notes'] = notes;
    return map;
  }
}
