/// Order API Response DTOs
///
/// These DTOs were originally defined in admin_stubs.dart but have been
/// extracted to the Order module as they are Order domain entities.
///
/// This file contains the EXTRACTED DTOs from admin_stubs.dart (lines 580-674, 1910-2462).
library;

import 'package:labuda/domains/commerce/transaction/order/domain/entities/order_status.dart'
    show OrderStatus;

// ==================== ORDER API RESPONSE DTOS ====================

/// Parse timestamp from backend.
///
/// Backend sends int64 Unix timestamps (seconds since epoch).
/// For backward compatibility, also handles ISO 8601 strings.
/// Returns DateTime or null.
DateTime? _parseOrderTimestamp(dynamic value) {
  if (value == null) return null;

  // If already a string (ISO 8601), parse it
  if (value is String) {
    if (value.isEmpty) return null;
    return DateTime.tryParse(value);
  }

  // If int/num (Unix timestamp), convert to DateTime
  if (value is num) {
    final timestamp = value.toInt();
    // Handle both seconds (< 1000000000000) and milliseconds
    return timestamp > 1000000000000
        ? DateTime.fromMillisecondsSinceEpoch(timestamp)
        : DateTime.fromMillisecondsSinceEpoch(timestamp * 1000);
  }

  return null;
}

// Order Filter Params
class OrderFilterParams {
  final OrderStatus? status;
  final DateTime? startDate;
  final DateTime? endDate;
  final String? searchQuery;
  final int? page;
  final int? pageSize;

  OrderFilterParams({
    this.status,
    this.startDate,
    this.endDate,
    this.searchQuery,
    this.page,
    this.pageSize,
  });

  Map<String, dynamic> toQueryParams() {
    final params = <String, dynamic>{};
    if (status != null) {
      // O1: Removed 'processing' - not a real backend status
      // O1: Added 'expired' status mapping
      params['status'] = status == OrderStatus.pending
          ? 'pending'
          : status == OrderStatus.paid
          ? 'paid'
          : status == OrderStatus.shipped
          ? 'shipped'
          : status == OrderStatus.delivered
          ? 'delivered'
          : status == OrderStatus.completed
          ? 'completed'
          : status == OrderStatus.cancelled
          ? 'cancelled'
          : status == OrderStatus.refunded
          ? 'refunded'
          : status == OrderStatus.disputeOpen
          ? 'dispute_open'
          : status == OrderStatus.partiallyRefunded
          ? 'partially_refunded'
          : status == OrderStatus.expired
          ? 'expired'
          : 'pending';
    }
    if (startDate != null) {
      params['start_date'] = startDate!.toIso8601String();
    }
    if (endDate != null) {
      params['end_date'] = endDate!.toIso8601String();
    }
    if (searchQuery != null) {
      params['search'] = searchQuery;
    }
    if (page != null) {
      params['page'] = page;
    }
    if (pageSize != null) {
      params['page_size'] = pageSize;
    }
    return params;
  }
}

// Refund Filter Params
class RefundFilterParams {
  final String? status;
  final DateTime? startDate;
  final DateTime? endDate;
  final int? page;
  final int? pageSize;

  RefundFilterParams({
    this.status,
    this.startDate,
    this.endDate,
    this.page,
    this.pageSize,
  });

  Map<String, dynamic> toQueryParams() {
    final params = <String, dynamic>{};
    if (status != null) {
      params['status'] = status;
    }
    if (startDate != null) {
      params['start_date'] = startDate!.toIso8601String();
    }
    if (endDate != null) {
      params['end_date'] = endDate!.toIso8601String();
    }
    if (page != null) {
      params['page'] = page;
    }
    if (pageSize != null) {
      params['page_size'] = pageSize;
    }
    return params;
  }
}

class OrderApiResponse {
  final String id;
  final String orderNumber;
  final String buyerId;
  final String sellerId;
  final String productId;
  final int quantity;
  final double totalAmount;
  final double finalAmount;
  final double shippingFee;
  final double discountAmount;
  final double coinDiscount;
  final double? serviceFeeAmount;
  final double? totalPayableAmount;
  final String status;
  final bool hasActiveRefund;
  final ActiveRefundApiResponse? activeRefund;
  final String paymentStatus;
  final String? notes;
  final DateTime createdAt;
  final DateTime? confirmedAt;
  final DateTime? shippedAt;
  final DateTime? completedAt;
  final DateTime? cancelledAt;
  final DateTime? sellerAcceptDeadline;
  final String? sourceType;
  final String? sourceId;
  final ShippingAddressApiResponse? shippingAddress;
  final ProductSummaryApiResponse? product;

  // Shipping Readiness Snapshot - frozen at order creation time
  final String? preparationTimeSnapshot;
  final String? preparationNoteSnapshot;
  final DateTime? readyToShipBy;

  // SHIPPING CONFIRMATION TRUTH: Shipping reference fields
  // These fields provide honest labeling for shipping references
  final String? shippingReference; // Resi, phone/WA, or other
  final String? referenceType; // "tracking" | "phone" | "other"
  final String? shippingNote; // Seller's shipping note

  // Overdue Display Layer (computed by backend, not persisted)
  final String?
  overdueTier; // none, overdue, severely_overdue, critical_overdue
  final int? overdueDays; // Days past ready_to_ship_by (null if not overdue)
  final bool? isOverdue; // Convenience boolean

  // ===========================================================================
  // STAGE 2 — IDENTITY PARSE-ONLY FIELDS (Phase 5)
  // ===========================================================================
  // Owner-truth identity scalars added by backend Stage 1 at order top-level.
  // Receive-only plumbing: not yet wired into the entity / UI mapping.
  // Order entity has NO seller/buyer name fields today, so these stay
  // DTO-only until Stage 3 mapper switch.
  // - seller_username   = account/user identity
  // - seller_farm_name  = seller/store identity (Owner Truth: farm name)
  // - seller_avatar_url = display avatar
  // - buyer_username    = buyer account/user identity
  final String? sellerUsername;
  final String? sellerFarmName;
  final String? sellerAvatarUrl;
  final String? buyerUsername;

  // Payment identity — set when a payment row exists for this order.
  final String? paymentId;

  OrderApiResponse({
    required this.id,
    required this.orderNumber,
    required this.buyerId,
    required this.sellerId,
    required this.productId,
    required this.quantity,
    required this.totalAmount,
    required this.finalAmount,
    required this.shippingFee,
    required this.discountAmount,
    required this.coinDiscount,
    this.serviceFeeAmount,
    this.totalPayableAmount,
    required this.status,
    this.hasActiveRefund = false,
    this.activeRefund,
    required this.paymentStatus,
    this.notes,
    required this.createdAt,
    this.confirmedAt,
    this.shippedAt,
    this.completedAt,
    this.cancelledAt,
    this.sellerAcceptDeadline,
    this.sourceType,
    this.sourceId,
    this.shippingAddress,
    this.product,
    this.preparationTimeSnapshot,
    this.preparationNoteSnapshot,
    this.readyToShipBy,
    this.shippingReference,
    this.referenceType,
    this.shippingNote,
    this.overdueTier,
    this.overdueDays,
    this.isOverdue,
    // Stage 2 identity parse-only fields
    this.sellerUsername,
    this.sellerFarmName,
    this.sellerAvatarUrl,
    this.buyerUsername,
    this.paymentId,
  });

  // fromJson factory for API response parsing
  factory OrderApiResponse.fromJson(Map<String, dynamic> json) {
    return OrderApiResponse(
      id: json['id'] as String? ?? '',
      orderNumber:
          json['order_number'] as String? ??
          json['orderNumber'] as String? ??
          '',
      buyerId: json['buyer_id'] as String? ?? json['buyerId'] as String? ?? '',
      sellerId:
          json['seller_id'] as String? ?? json['sellerId'] as String? ?? '',
      productId: json['product_id'] as String? ?? '',
      quantity: json['quantity'] as int? ?? 0,
      totalAmount:
          (json['total_amount'] as num?)?.toDouble() ??
          json['totalAmount'] as double? ??
          0.0,
      finalAmount:
          (json['final_amount'] as num?)?.toDouble() ??
          json['finalAmount'] as double? ??
          0.0,
      shippingFee:
          (json['shipping_fee'] as num?)?.toDouble() ??
          json['shippingFee'] as double? ??
          0.0,
      discountAmount:
          (json['discount_amount'] as num?)?.toDouble() ??
          json['discountAmount'] as double? ??
          0.0,
      coinDiscount:
          (json['coin_discount'] as num?)?.toDouble() ??
          json['coinDiscount'] as double? ??
          0.0,
      serviceFeeAmount:
          (json['service_fee_amount'] as num?)?.toDouble() ??
          (json['serviceFeeAmount'] as num?)?.toDouble(),
      totalPayableAmount:
          (json['total_payable_amount'] as num?)?.toDouble() ??
          (json['totalPayableAmount'] as num?)?.toDouble() ??
          (json['final_amount'] as num?)?.toDouble() ??
          json['finalAmount'] as double? ??
          0.0,
      status: json['status'] as String? ?? '',
      hasActiveRefund: json['has_active_refund'] as bool? ?? false,
      activeRefund: json['active_refund'] != null
          ? ActiveRefundApiResponse.fromJson(
              json['active_refund'] as Map<String, dynamic>,
            )
          : null,
      paymentStatus:
          json['payment_status'] as String? ??
          json['paymentStatus'] as String? ??
          '',
      notes: json['notes'] as String?,
      createdAt: _parseOrderTimestamp(json['created_at']) ?? DateTime.now(),
      confirmedAt: _parseOrderTimestamp(json['confirmed_at']),
      shippedAt: _parseOrderTimestamp(json['shipped_at']),
      completedAt: _parseOrderTimestamp(json['completed_at']),
      cancelledAt: _parseOrderTimestamp(json['cancelled_at']),
      sellerAcceptDeadline: _parseOrderTimestamp(
        json['seller_accept_deadline'],
      ),
      sourceType: json['source_type'] as String?,
      sourceId: json['source_id'] as String?,
      shippingAddress: json['shipping_address'] != null
          ? ShippingAddressApiResponse.fromJson(
              json['shipping_address'] as Map<String, dynamic>,
            )
          : null,
      product: json['product'] != null
          ? ProductSummaryApiResponse.fromJson(
              json['product'] as Map<String, dynamic>,
            )
          : null,
      preparationTimeSnapshot: json['preparation_time_snapshot'] as String?,
      preparationNoteSnapshot: json['preparation_note_snapshot'] as String?,
      readyToShipBy: _parseOrderTimestamp(json['ready_to_ship_by']),
      // SHIPPING CONFIRMATION TRUTH: Parse shipping reference fields
      shippingReference:
          json['shipping_reference'] as String? ??
          json['tracking_number']
              as String?, // Fallback for backward compatibility
      referenceType: json['reference_type'] as String?,
      shippingNote: json['shipping_note'] as String?,
      // Overdue Display Layer
      overdueTier: json['overdue_tier'] as String?,
      overdueDays: json['overdue_days'] as int?,
      isOverdue: json['is_overdue'] as bool?,
      // Stage 2 identity parse-only fields. Tolerate old payload (null) and
      // new payload. No fullName fallback — owner truth is username/farm.
      sellerUsername: json['seller_username'] as String?,
      sellerFarmName: json['seller_farm_name'] as String?,
      sellerAvatarUrl: json['seller_avatar_url'] as String?,
      buyerUsername: json['buyer_username'] as String?,
      paymentId: json['payment_id'] as String?,
    );
  }

  // Map method for OrderMapper compatibility
  Map<String, dynamic> toMap() => {
    'id': id,
    'orderNumber': orderNumber,
    'buyerId': buyerId,
    'sellerId': sellerId,
    'productId': productId,
    'quantity': quantity,
    'totalAmount': totalAmount,
    'finalAmount': finalAmount,
    'shippingFee': shippingFee,
    'discountAmount': discountAmount,
    'coinDiscount': coinDiscount,
    'serviceFeeAmount': serviceFeeAmount,
    'totalPayableAmount': totalPayableAmount,
    'status': status,
    'hasActiveRefund': hasActiveRefund,
    'activeRefund': activeRefund?.toMap(),
    'paymentStatus': paymentStatus,
    'notes': notes,
    'createdAt': createdAt.toIso8601String(),
    'confirmedAt': confirmedAt?.toIso8601String(),
    'shippedAt': shippedAt?.toIso8601String(),
    'completedAt': completedAt?.toIso8601String(),
    'cancelledAt': cancelledAt?.toIso8601String(),
    'sellerAcceptDeadline': sellerAcceptDeadline?.toIso8601String(),
    'sourceType': sourceType,
    'sourceId': sourceId,
    'shippingReference': shippingReference,
    'referenceType': referenceType,
    'shippingNote': shippingNote,
    'overdueTier': overdueTier,
    'overdueDays': overdueDays,
    'isOverdue': isOverdue,
    'paymentId': paymentId,
  };

  // Map method for RepositoryResult compatibility
  R map<R>(R Function(OrderApiResponse data) transform) {
    return transform(this);
  }
}

class ActiveRefundApiResponse {
  final String id;
  final String orderId;
  final String buyerId;
  final String sellerId;
  final String status;
  final String reason;
  final String? description;
  final double requestedAmount;
  final String? sellerNotes;
  final List<String>? evidenceUrls;
  final DateTime createdAt;
  final DateTime updatedAt;
  final String? adminNotes;
  final DateTime? resolvedAt;
  final String? gatewayStatus;

  const ActiveRefundApiResponse({
    required this.id,
    required this.orderId,
    required this.buyerId,
    required this.sellerId,
    required this.status,
    required this.reason,
    this.description,
    required this.requestedAmount,
    this.sellerNotes,
    this.evidenceUrls,
    required this.createdAt,
    required this.updatedAt,
    this.adminNotes,
    this.resolvedAt,
    this.gatewayStatus,
  });

  factory ActiveRefundApiResponse.fromJson(Map<String, dynamic> json) {
    return ActiveRefundApiResponse(
      id: json['id'] as String? ?? '',
      orderId: json['order_id'] as String? ?? '',
      buyerId: json['buyer_id'] as String? ?? '',
      sellerId: json['seller_id'] as String? ?? '',
      status: json['status'] as String? ?? '',
      reason: json['reason'] as String? ?? '',
      description: json['description'] as String?,
      requestedAmount: (json['requested_amount'] as num?)?.toDouble() ?? 0.0,
      sellerNotes: json['seller_notes'] as String?,
      evidenceUrls: (json['evidence_urls'] as List<dynamic>?)
          ?.map((e) => e.toString())
          .toList(),
      createdAt: _parseOrderTimestamp(json['created_at']) ?? DateTime.now(),
      updatedAt: _parseOrderTimestamp(json['updated_at']) ?? DateTime.now(),
      adminNotes: json['admin_notes'] as String?,
      resolvedAt: _parseOrderTimestamp(json['resolved_at']),
      gatewayStatus: json['gateway_status'] as String?,
    );
  }

  Map<String, dynamic> toMap() => {
    'id': id,
    'order_id': orderId,
    'buyer_id': buyerId,
    'seller_id': sellerId,
    'status': status,
    'reason': reason,
    'description': description,
    'requested_amount': requestedAmount,
    'seller_notes': sellerNotes,
    'evidence_urls': evidenceUrls,
    'created_at': createdAt.toIso8601String(),
    'updated_at': updatedAt.toIso8601String(),
    'admin_notes': adminNotes,
    'resolved_at': resolvedAt?.toIso8601String(),
    'gateway_status': gatewayStatus,
  };
}

class OrderListApiResponse {
  final List<OrderApiResponse> data;
  final int? total;
  final int? page;
  final int? pageSize;

  OrderListApiResponse({
    required this.data,
    this.total,
    this.page,
    this.pageSize,
  });

  factory OrderListApiResponse.fromJson(Map<String, dynamic> json) {
    final rawList =
        json['orders'] ?? json['data'] ?? json['items'] ?? const <dynamic>[];

    final orderList = rawList is List
        ? rawList
              .whereType<Map<String, dynamic>>()
              .map(OrderApiResponse.fromJson)
              .toList()
        : <OrderApiResponse>[];

    return OrderListApiResponse(
      data: orderList,
      total: json['total'] as int?,
      page: json['page'] as int?,
      pageSize: json['page_size'] as int? ?? json['pageSize'] as int?,
    );
  }

  R map<R>(R Function(OrderListApiResponse data) transform) {
    return transform(this);
  }
}

class OrderStatsApiResponse {
  final int totalOrders;
  final int pendingOrders;
  final int completedOrders;
  final int cancelledOrders;
  final double totalRevenue;

  OrderStatsApiResponse({
    required this.totalOrders,
    required this.pendingOrders,
    required this.completedOrders,
    required this.cancelledOrders,
    required this.totalRevenue,
  });

  factory OrderStatsApiResponse.fromJson(Map<String, dynamic> json) {
    return OrderStatsApiResponse(
      totalOrders:
          json['total_orders'] as int? ?? json['totalOrders'] as int? ?? 0,
      pendingOrders:
          json['pending_orders'] as int? ?? json['pendingOrders'] as int? ?? 0,
      completedOrders:
          json['completed_orders'] as int? ??
          json['completedOrders'] as int? ??
          0,
      cancelledOrders:
          json['cancelled_orders'] as int? ??
          json['cancelledOrders'] as int? ??
          0,
      totalRevenue:
          (json['total_revenue'] as num?)?.toDouble() ??
          json['totalRevenue'] as double? ??
          0.0,
    );
  }

  R map<R>(R Function(OrderStatsApiResponse data) transform) {
    return transform(this);
  }
}

class RefundApiResponse {
  final String id;
  final String orderId;
  final String userId;
  final String? buyerId;
  final String? sellerId;
  final String reason;
  final String description;
  final String status;
  final double? requestedAmount;
  final double? approvedAmount;
  final double? refundAmount;
  final List<String>? evidence;
  final String? sellerResponse;
  final String? adminResponse;
  final String? adminId;
  final DateTime? sellerRespondAt;
  final DateTime? adminRespondAt;
  final DateTime? completedAt;
  final bool? isResolved;
  final DateTime createdAt;

  RefundApiResponse({
    required this.id,
    required this.orderId,
    required this.userId,
    this.buyerId,
    this.sellerId,
    required this.reason,
    required this.description,
    required this.status,
    this.requestedAmount,
    this.approvedAmount,
    this.refundAmount,
    this.evidence,
    this.sellerResponse,
    this.adminResponse,
    this.adminId,
    this.sellerRespondAt,
    this.adminRespondAt,
    this.completedAt,
    this.isResolved,
    required this.createdAt,
  });

  // fromJson factory for API response parsing
  factory RefundApiResponse.fromJson(Map<String, dynamic> json) {
    return RefundApiResponse(
      id: json['id'] as String? ?? '',
      orderId: json['order_id'] as String? ?? json['orderId'] as String? ?? '',
      userId: json['user_id'] as String? ?? json['userId'] as String? ?? '',
      buyerId: json['buyer_id'] as String? ?? json['buyerId'] as String?,
      sellerId: json['seller_id'] as String? ?? json['sellerId'] as String?,
      reason: json['reason'] as String? ?? '',
      description: json['description'] as String? ?? '',
      status: json['status'] as String? ?? '',
      requestedAmount:
          (json['requested_amount'] as num?)?.toDouble() ??
          json['requestedAmount'] as double?,
      approvedAmount:
          (json['approved_amount'] as num?)?.toDouble() ??
          json['approvedAmount'] as double?,
      refundAmount:
          (json['refund_amount'] as num?)?.toDouble() ??
          json['refundAmount'] as double?,
      evidence: (json['evidence'] as List<dynamic>?)
          ?.map((e) => e.toString())
          .toList(),
      sellerResponse:
          json['seller_response'] as String? ??
          json['sellerResponse'] as String?,
      adminResponse:
          json['admin_response'] as String? ?? json['adminResponse'] as String?,
      adminId: json['admin_id'] as String? ?? json['adminId'] as String?,
      sellerRespondAt: json['seller_respond_at'] != null
          ? DateTime.parse(json['seller_respond_at'] as String)
          : (json['sellerRespondAt'] as DateTime?),
      adminRespondAt: json['admin_respond_at'] != null
          ? DateTime.parse(json['admin_respond_at'] as String)
          : (json['adminRespondAt'] as DateTime?),
      completedAt: json['completed_at'] != null
          ? DateTime.parse(json['completed_at'] as String)
          : (json['completedAt'] as DateTime?),
      isResolved: json['is_resolved'] as bool? ?? json['isResolved'] as bool?,
      createdAt: DateTime.parse(
        json['created_at'] as String? ?? DateTime.now().toIso8601String(),
      ),
    );
  }

  // Map method for RepositoryResult compatibility
  Map<String, dynamic> toMap() => {
    'id': id,
    'orderId': orderId,
    'userId': userId,
    'buyerId': buyerId,
    'sellerId': sellerId,
    'reason': reason,
    'description': description,
    'status': status,
    'requestedAmount': requestedAmount,
    'approvedAmount': approvedAmount,
    'refundAmount': refundAmount,
    'evidence': evidence,
    'sellerResponse': sellerResponse,
    'adminResponse': adminResponse,
    'adminId': adminId,
    'sellerRespondAt': sellerRespondAt?.toIso8601String(),
    'adminRespondAt': adminRespondAt?.toIso8601String(),
    'completedAt': completedAt?.toIso8601String(),
    'isResolved': isResolved,
    'createdAt': createdAt.toIso8601String(),
  };
}

class RefundListApiResponse {
  final List<RefundApiResponse> data;
  final int? total;

  RefundListApiResponse({required this.data, this.total});

  factory RefundListApiResponse.fromJson(Map<String, dynamic> json) {
    return RefundListApiResponse(
      data:
          (json['data'] as List<dynamic>?)
              ?.map(
                (e) => RefundApiResponse.fromJson(e as Map<String, dynamic>),
              )
              .toList() ??
          [],
      total: json['total'] as int?,
    );
  }

  R map<R>(R Function(RefundListApiResponse data) transform) {
    return transform(this);
  }
}

class CheckDeliveryApiResponse {
  final bool delivered;
  final String? deliveryDate;
  final String? signature;

  CheckDeliveryApiResponse({
    required this.delivered,
    this.deliveryDate,
    this.signature,
  });

  factory CheckDeliveryApiResponse.fromJson(Map<String, dynamic> json) {
    return CheckDeliveryApiResponse(
      delivered: json['delivered'] as bool? ?? false,
      deliveryDate:
          json['delivery_date'] as String? ?? json['deliveryDate'] as String?,
      signature: json['signature'] as String?,
    );
  }

  R map<R>(R Function(CheckDeliveryApiResponse data) transform) {
    return transform(this);
  }
}

class ShippingProofApiResponse {
  final String id;
  final String orderId;
  final String trackingNumber;
  final String? proofImageUrl;
  final DateTime createdAt;

  ShippingProofApiResponse({
    required this.id,
    required this.orderId,
    required this.trackingNumber,
    this.proofImageUrl,
    required this.createdAt,
  });

  factory ShippingProofApiResponse.fromJson(Map<String, dynamic> json) {
    return ShippingProofApiResponse(
      id: json['id'] as String? ?? '',
      orderId: json['order_id'] as String? ?? json['orderId'] as String? ?? '',
      trackingNumber:
          json['tracking_number'] as String? ??
          json['trackingNumber'] as String? ??
          '',
      proofImageUrl:
          json['proof_image_url'] as String? ??
          json['proofImageUrl'] as String?,
      createdAt: json['created_at'] != null
          ? DateTime.parse(json['created_at'] as String)
          : DateTime.now(),
    );
  }

  R map<R>(R Function(ShippingProofApiResponse data) transform) {
    return transform(this);
  }
}

class ShippingAddressApiResponse {
  final String recipientName;
  final String phoneNumber;
  final String addressLine1;
  final String? addressLine2;
  final String? city;
  final String? province;
  final String? postalCode;
  final String fullAddress;

  ShippingAddressApiResponse({
    required this.recipientName,
    required this.phoneNumber,
    required this.addressLine1,
    this.addressLine2,
    this.city,
    this.province,
    this.postalCode,
    this.fullAddress = '',
  });

  factory ShippingAddressApiResponse.fromJson(Map<String, dynamic> json) {
    return ShippingAddressApiResponse(
      recipientName:
          json['recipient_name'] as String? ??
          json['recipientName'] as String? ??
          '',
      phoneNumber:
          json['phone_number'] as String? ??
          json['phoneNumber'] as String? ??
          '',
      addressLine1:
          json['address_line1'] as String? ??
          json['addressLine1'] as String? ??
          '',
      addressLine2:
          json['address_line2'] as String? ?? json['addressLine2'] as String?,
      city: json['city'] as String?,
      province: json['province'] as String?,
      postalCode:
          json['postal_code'] as String? ?? json['postalCode'] as String?,
      fullAddress:
          json['full_address'] as String? ??
          json['fullAddress'] as String? ??
          '',
    );
  }
}

class ProductSummaryApiResponse {
  final String id;
  final String title;
  final String? imageUrl;
  final double price;

  ProductSummaryApiResponse({
    required this.id,
    required this.title,
    this.imageUrl,
    required this.price,
  });

  factory ProductSummaryApiResponse.fromJson(Map<String, dynamic> json) {
    return ProductSummaryApiResponse(
      id: json['id'] as String? ?? '',
      title: json['title'] as String? ?? '',
      imageUrl: json['image_url'] as String? ?? json['imageUrl'] as String?,
      price: (json['price'] as num?)?.toDouble() ?? 0.0,
    );
  }
}

class CheckDeliveryApiRequest {
  final String orderId;
  final String? courier;
  final String? trackingNumber;

  CheckDeliveryApiRequest({
    required this.orderId,
    this.courier,
    this.trackingNumber,
  });

  Map<String, dynamic> toJson() => {
    'order_id': orderId,
    if (courier != null) 'courier': courier,
    if (trackingNumber != null) 'tracking_number': trackingNumber,
  };
}

class CreateShippingProofApiRequest {
  final String trackingNumber;
  final String? proofImageUrl;

  CreateShippingProofApiRequest({
    required this.trackingNumber,
    this.proofImageUrl,
  });

  Map<String, dynamic> toJson() => {
    'tracking_number': trackingNumber,
    if (proofImageUrl != null) 'proof_image_url': proofImageUrl,
  };
}
