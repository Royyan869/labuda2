/// Refund DTO - Data Transfer Object for Refund API
///
/// BACKEND AUTHORITY:
/// Backend Go entity is the SINGLE SOURCE OF TRUTH for all refund data.
/// This DTO parses backend response exactly - no client-side calculation.
///
/// Backend entity: backend/internal/domain/refund/entity/refund.go
/// API endpoint: GET /api/v1/orders/{id}/refunds, GET /api/v1/refunds/{id}
library;

import 'package:labuda/domains/commerce/transaction/order/domain/entities/refund_request.dart';

/// RefundDto - Complete refund data from backend
///
/// Parses ALL fields from backend Refund entity including:
/// - Requested amount (buyer's claim)
/// - Seller decision fields (percent, amount, notes)
/// - Admin decision fields (percent, amount, notes, reviewer)
/// - Final refund amount (calculated by backend)
class RefundDto {
  // Core identifiers
  final String id;
  final String orderId;
  final String buyerId;
  final String sellerId;

  // Refund details
  final String reason;
  final String? description;
  final List<String>? evidenceUrls;

  // Status
  final String status;

  // Amount fields (from backend)
  // Backend sends amounts in int64 (Rupiah, full unit - no cents for IDR)
  // Frontend uses double for display formatting
  final int? requestedAmount; // Buyer's initial claim (in Rupiah)
  final int? sellerApprovedAmount; // Seller's approved amount (in Rupiah)
  final int? adminApprovedAmount; // Admin's approved amount (in Rupiah)
  final int? finalRefundAmount; // Final refund amount (in Rupiah)

  // Percentage fields
  final int? sellerApprovedPercent; // 0-100
  final int? adminApprovedPercent; // 0-100

  // Decision metadata
  final String? sellerNotes;
  final DateTime? sellerReviewedAt;
  final String? adminNotes;
  final String? reviewedBy; // Admin user ID
  final DateTime? adminReviewedAt;

  // Timestamps
  final DateTime openedAt;
  final DateTime? approvedAt;
  final DateTime? rejectedAt;
  final DateTime? refundedAt;
  final DateTime createdAt;
  final DateTime updatedAt;

  const RefundDto({
    required this.id,
    required this.orderId,
    required this.buyerId,
    required this.sellerId,
    required this.reason,
    this.description,
    this.evidenceUrls,
    required this.status,
    this.requestedAmount,
    this.sellerApprovedAmount,
    this.adminApprovedAmount,
    this.finalRefundAmount,
    this.sellerApprovedPercent,
    this.adminApprovedPercent,
    this.sellerNotes,
    this.sellerReviewedAt,
    this.adminNotes,
    this.reviewedBy,
    this.adminReviewedAt,
    required this.openedAt,
    this.approvedAt,
    this.rejectedAt,
    this.refundedAt,
    required this.createdAt,
    required this.updatedAt,
  });

  /// Parse from backend JSON response
  ///
  /// Backend field naming follows Go conventions (snake_case).
  /// This mapper handles all nullable fields and timestamp conversions.
  factory RefundDto.fromJson(Map<String, dynamic> json) {
    // Parse timestamps
    DateTime parseTimestamp(dynamic value) {
      if (value == null) return DateTime.now();
      if (value is DateTime) return value;
      if (value is String) {
        final parsed = DateTime.tryParse(value);
        if (parsed != null) return parsed;
      }
      if (value is num) {
        // Handle Unix timestamp (seconds or milliseconds)
        final timestamp = value.toInt();
        if (timestamp > 1000000000000) {
          // Milliseconds
          return DateTime.fromMillisecondsSinceEpoch(timestamp);
        } else {
          // Seconds
          return DateTime.fromMillisecondsSinceEpoch(timestamp * 1000);
        }
      }
      return DateTime.now();
    }

    DateTime? parseNullableTimestamp(dynamic value) {
      if (value == null) return null;
      return parseTimestamp(value);
    }

    return RefundDto(
      id: json['id'] as String? ?? '',
      orderId: json['order_id'] as String? ?? '',
      buyerId: json['buyer_id'] as String? ?? '',
      sellerId: json['seller_id'] as String? ?? '',
      reason: json['reason'] as String? ?? '',
      description: json['description'] as String?,
      evidenceUrls: (json['evidence_urls'] as List<dynamic>?)
          ?.map((e) => e.toString())
          .toList(),
      status: json['status'] as String? ?? '',
      requestedAmount: json['requested_amount'] as int?,
      sellerApprovedAmount: json['seller_approved_amount'] as int?,
      adminApprovedAmount: json['admin_approved_amount'] as int?,
      finalRefundAmount: json['final_refund_amount'] as int?,
      sellerApprovedPercent: json['seller_approved_percent'] as int?,
      adminApprovedPercent: json['admin_approved_percent'] as int?,
      sellerNotes: json['seller_notes'] as String?,
      sellerReviewedAt: parseNullableTimestamp(json['seller_reviewed_at']),
      adminNotes: json['admin_notes'] as String?,
      reviewedBy: json['reviewed_by'] as String?,
      adminReviewedAt: parseNullableTimestamp(json['admin_reviewed_at']),
      openedAt: parseTimestamp(json['opened_at']),
      approvedAt: parseNullableTimestamp(json['approved_at']),
      rejectedAt: parseNullableTimestamp(json['rejected_at']),
      refundedAt: parseNullableTimestamp(json['refunded_at']),
      createdAt: parseTimestamp(json['created_at']),
      updatedAt: parseTimestamp(json['updated_at']),
    );
  }

  /// Convert to JSON (for debugging/testing)
  Map<String, dynamic> toJson() => {
    'id': id,
    'order_id': orderId,
    'buyer_id': buyerId,
    'seller_id': sellerId,
    'reason': reason,
    if (description != null) 'description': description,
    if (evidenceUrls != null) 'evidence_urls': evidenceUrls,
    'status': status,
    if (requestedAmount != null) 'requested_amount': requestedAmount,
    if (sellerApprovedAmount != null)
      'seller_approved_amount': sellerApprovedAmount,
    if (adminApprovedAmount != null)
      'admin_approved_amount': adminApprovedAmount,
    if (finalRefundAmount != null) 'final_refund_amount': finalRefundAmount,
    if (sellerApprovedPercent != null)
      'seller_approved_percent': sellerApprovedPercent,
    if (adminApprovedPercent != null)
      'admin_approved_percent': adminApprovedPercent,
    if (sellerNotes != null) 'seller_notes': sellerNotes,
    if (sellerReviewedAt != null)
      'seller_reviewed_at': sellerReviewedAt!.toIso8601String(),
    if (adminNotes != null) 'admin_notes': adminNotes,
    if (reviewedBy != null) 'reviewed_by': reviewedBy,
    if (adminReviewedAt != null)
      'admin_reviewed_at': adminReviewedAt!.toIso8601String(),
    'opened_at': openedAt.toIso8601String(),
    if (approvedAt != null) 'approved_at': approvedAt!.toIso8601String(),
    if (rejectedAt != null) 'rejected_at': rejectedAt!.toIso8601String(),
    if (refundedAt != null) 'refunded_at': refundedAt!.toIso8601String(),
    'created_at': createdAt.toIso8601String(),
    'updated_at': updatedAt.toIso8601String(),
  };

  /// Convert to RefundRequest domain entity
  ///
  /// Backend sends amounts in int64 (Rupiah, full unit - no cents for IDR).
  /// Frontend uses double for display formatting.
  /// For Rupiah: 100000 in backend = 100000.0 in frontend = Rp 100.000
  RefundRequest toEntity() {
    // Helper: Convert backend amount (int64 Rupiah) to frontend amount (double)
    // NO conversion needed - Rupiah uses full units, no cents
    double? convertAmount(int? amount) {
      if (amount == null) return null;
      return amount.toDouble();
    }

    return RefundRequest(
      id: id,
      orderId: orderId,
      buyerId: buyerId,
      sellerId: sellerId,
      reason: _mapRefundReason(reason),
      description: description,
      evidenceUrls: evidenceUrls,
      status: _mapRefundStatus(status),
      // Use requested_amount from backend, fallback to 0
      refundAmount: convertAmount(requestedAmount) ?? 0.0,
      // Seller decision fields - now populated from backend!
      sellerApprovedPercent: sellerApprovedPercent,
      sellerApprovedAmount: convertAmount(sellerApprovedAmount),
      sellerNotes: sellerNotes,
      sellerReviewedAt: sellerReviewedAt,
      // Admin decision fields - now populated from backend!
      adminApprovedPercent: adminApprovedPercent,
      adminApprovedAmount: convertAmount(adminApprovedAmount),
      adminNotes: adminNotes,
      reviewedBy: reviewedBy,
      adminReviewedAt: adminReviewedAt,
      // Final outcome - now populated from backend!
      finalRefundAmount: convertAmount(finalRefundAmount),
      createdAt: createdAt,
      approvedAt: approvedAt,
      rejectedAt: rejectedAt,
      refundedAt: refundedAt,
    );
  }

  static RefundReason _mapRefundReason(String reason) {
    switch (reason.toLowerCase()) {
      case 'item_not_received':
        return RefundReason.itemNotReceived;
      case 'item_not_as_described':
        return RefundReason.itemNotAsDescribed;
      case 'item_damaged':
        return RefundReason.itemDamaged;
      case 'defective_item':
        return RefundReason.defectiveItem;
      case 'wrong_item':
        return RefundReason.wrongItem;
      case 'change_of_mind':
        return RefundReason.changeOfMind;
      case 'delivery_delay':
        return RefundReason.deliveryDelay;
      default:
        return RefundReason.other;
    }
  }

  static RefundStatus _mapRefundStatus(String status) {
    switch (status.toLowerCase()) {
      case 'pending_seller_review':
      case 'pending':
        return RefundStatus.pendingSellerReview;
      case 'seller_approved':
      case 'approved':
        return RefundStatus.sellerApproved;
      case 'escalated_to_admin':
      case 'escalated':
        return RefundStatus.escalatedToAdmin;
      case 'seller_rejected':
        return RefundStatus.sellerRejected;
      case 'admin_refunded':
      case 'admin_approved':
        return RefundStatus.adminApproved;
      case 'admin_released':
        // Admin released to seller = no refund for buyer
        return RefundStatus.rejected;
      case 'rejected':
        return RefundStatus.rejected;
      case 'refunded':
        return RefundStatus.refunded;
      default:
        return RefundStatus.pendingSellerReview;
    }
  }
}

/// RefundListDto - Paginated list of refunds
class RefundListDto {
  final List<RefundDto> data;
  final int? total;
  final int? page;
  final int? pageSize;

  const RefundListDto({
    required this.data,
    this.total,
    this.page,
    this.pageSize,
  });

  factory RefundListDto.fromJson(Map<String, dynamic> json) {
    final refundsData = json['data'] as List<dynamic>? ?? [];
    final refunds = refundsData
        .map((e) => RefundDto.fromJson(e as Map<String, dynamic>))
        .toList();

    return RefundListDto(
      data: refunds,
      total: json['total'] as int?,
      page: json['page'] as int?,
      pageSize: json['page_size'] as int?,
    );
  }

  Map<String, dynamic> toJson() => {
    'data': data.map((e) => e.toJson()).toList(),
    if (total != null) 'total': total,
    if (page != null) 'page': page,
    if (pageSize != null) 'page_size': pageSize,
  };
}
