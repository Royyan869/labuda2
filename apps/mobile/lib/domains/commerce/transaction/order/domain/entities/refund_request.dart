import 'package:equatable/equatable.dart';

/// Refund Request Entity
class RefundRequest extends Equatable {
  final String id;
  final String orderId;
  final String buyerId;
  final String sellerId;
  final RefundReason reason;
  final String? description;
  final List<String>? evidenceUrls;
  final RefundStatus status;
  final double refundAmount;

  // =========================================================================
  // BACKEND AUTHORITY: Refund Advanced Fields
  // =========================================================================
  // Backend NOW calculates and provides these values.
  // These fields are READ-ONLY and populated from backend API response.
  //
  // DO NOT:
  // - Calculate or derive these values in client
  // - Modify these values on client side
  //
  // Backend entity: backend/internal/domain/refund/entity/refund.go
  // API endpoint: GET /api/v1/orders/{id}/refunds, GET /api/v1/refunds/{id}
  // =========================================================================

  // Seller decision fields (populated from backend)
  final int? sellerApprovedPercent;
  final double? sellerApprovedAmount;
  final String? sellerNotes;
  final DateTime? sellerReviewedAt;

  // Admin decision fields (populated from backend)
  final int? adminApprovedPercent;
  final double? adminApprovedAmount;
  final String? adminNotes;
  final String? reviewedBy;
  final DateTime? adminReviewedAt;

  // Final outcome (populated from backend)
  final double? finalRefundAmount;

  // Timestamps
  final DateTime createdAt;
  final DateTime? approvedAt;
  final DateTime? rejectedAt;
  final DateTime? refundedAt;

  const RefundRequest({
    required this.id,
    required this.orderId,
    required this.buyerId,
    required this.sellerId,
    required this.reason,
    this.description,
    this.evidenceUrls,
    required this.status,
    required this.refundAmount,
    this.sellerApprovedPercent,
    this.sellerApprovedAmount,
    this.sellerNotes,
    this.sellerReviewedAt,
    this.adminApprovedPercent,
    this.adminApprovedAmount,
    this.adminNotes,
    this.reviewedBy,
    this.adminReviewedAt,
    this.finalRefundAmount,
    required this.createdAt,
    this.approvedAt,
    this.rejectedAt,
    this.refundedAt,
  });

  bool get isPendingSellerReview => status == RefundStatus.pendingSellerReview;
  bool get isSellerApproved => status == RefundStatus.sellerApproved;
  bool get isEscalatedToAdmin => status == RefundStatus.escalatedToAdmin;
  bool get isAdminApproved => status == RefundStatus.adminApproved;
  bool get isRejected =>
      status == RefundStatus.rejected || status == RefundStatus.sellerRejected;
  bool get isRefunded => status == RefundStatus.refunded;

  @override
  List<Object?> get props => [
    id,
    orderId,
    buyerId,
    sellerId,
    reason,
    description,
    evidenceUrls,
    status,
    refundAmount,
    sellerApprovedPercent,
    sellerApprovedAmount,
    sellerNotes,
    sellerReviewedAt,
    adminApprovedPercent,
    adminApprovedAmount,
    adminNotes,
    reviewedBy,
    adminReviewedAt,
    finalRefundAmount,
    createdAt,
    approvedAt,
    rejectedAt,
    refundedAt,
  ];
}

enum RefundReason {
  itemNotReceived,
  itemNotAsDescribed,
  itemDamaged,
  defectiveItem,
  wrongItem,
  changeOfMind,
  deliveryDelay,
  other;

  String get displayName {
    switch (this) {
      case RefundReason.itemNotReceived:
        return 'Item not received';
      case RefundReason.itemNotAsDescribed:
        return 'Item not as described';
      case RefundReason.itemDamaged:
        return 'Item damaged/defective';
      case RefundReason.defectiveItem:
        return 'Item dead/sick';
      case RefundReason.wrongItem:
        return 'Wrong item received';
      case RefundReason.changeOfMind:
        return 'Change of mind';
      case RefundReason.deliveryDelay:
        return 'Delivery delayed';
      case RefundReason.other:
        return 'Other';
    }
  }

  String get emoji {
    switch (this) {
      case RefundReason.itemNotReceived:
        return '📦';
      case RefundReason.itemNotAsDescribed:
        return '❌';
      case RefundReason.itemDamaged:
        return '⚠️';
      case RefundReason.defectiveItem:
        return '☠️';
      case RefundReason.wrongItem:
        return '🔄';
      case RefundReason.changeOfMind:
        return '🤔';
      case RefundReason.deliveryDelay:
        return '⏰';
      case RefundReason.other:
        return '❓';
    }
  }

  /// Convert to API string value (snake_case)
  String get apiValue {
    switch (this) {
      case RefundReason.itemNotReceived:
        return 'item_not_received';
      case RefundReason.itemNotAsDescribed:
        return 'item_not_as_described';
      case RefundReason.itemDamaged:
        return 'item_damaged';
      case RefundReason.defectiveItem:
        return 'defective_item';
      case RefundReason.wrongItem:
        return 'wrong_item';
      case RefundReason.changeOfMind:
        return 'change_of_mind';
      case RefundReason.deliveryDelay:
        return 'delivery_delay';
      case RefundReason.other:
        return 'other';
    }
  }
}

enum RefundStatus {
  pendingSellerReview,
  sellerApproved,
  sellerRejected,
  escalatedToAdmin,
  adminApproved,
  rejected,
  refunded;

  String get displayName {
    switch (this) {
      case RefundStatus.pendingSellerReview:
        return 'Awaiting Seller Review';
      case RefundStatus.sellerApproved:
        return 'Approved by Seller';
      case RefundStatus.sellerRejected:
        return 'Ditolak Penjual';
      case RefundStatus.escalatedToAdmin:
        return 'Awaiting Admin Decision';
      case RefundStatus.adminApproved:
        return 'Approved by Admin';
      case RefundStatus.rejected:
        return 'Rejected';
      case RefundStatus.refunded:
        return 'Funds Returned';
    }
  }

  String get emoji {
    switch (this) {
      case RefundStatus.pendingSellerReview:
        return '⏳';
      case RefundStatus.sellerApproved:
        return '✅';
      case RefundStatus.sellerRejected:
        return '❌';
      case RefundStatus.escalatedToAdmin:
        return '🔍';
      case RefundStatus.adminApproved:
        return '✅';
      case RefundStatus.rejected:
        return '❌';
      case RefundStatus.refunded:
        return '💰';
    }
  }

  /// Convert to API string value (snake_case)
  String get apiValue {
    switch (this) {
      case RefundStatus.pendingSellerReview:
        return 'pending_seller_review';
      case RefundStatus.sellerApproved:
        return 'seller_approved';
      case RefundStatus.sellerRejected:
        return 'seller_rejected';
      case RefundStatus.escalatedToAdmin:
        return 'escalated_to_admin';
      case RefundStatus.adminApproved:
        return 'admin_approved';
      case RefundStatus.rejected:
        return 'rejected';
      case RefundStatus.refunded:
        return 'refunded';
    }
  }

  /// Parse API string to RefundStatus
  static RefundStatus? parse(String? value) {
    if (value == null) return null;
    switch (value.toLowerCase()) {
      case 'pending_seller_review':
      case 'pending':
        return RefundStatus.pendingSellerReview;
      case 'seller_approved':
      case 'approved':
        return RefundStatus.sellerApproved;
      case 'seller_rejected':
        return RefundStatus.sellerRejected;
      case 'escalated_to_admin':
      case 'escalated':
        return RefundStatus.escalatedToAdmin;
      case 'admin_approved':
      case 'admin_refunded':
        return RefundStatus.adminApproved;
      case 'admin_released':
      case 'rejected':
        return RefundStatus.rejected;
      case 'refunded':
        return RefundStatus.refunded;
      default:
        return null;
    }
  }
}
