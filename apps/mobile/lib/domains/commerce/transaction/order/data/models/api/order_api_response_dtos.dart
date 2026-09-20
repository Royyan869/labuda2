/// Order API Response DTOs
///
/// CANONICAL ORDER WIRE CONTRACT — the backend is the single source of truth.
///
///   POST /orders           → OrderCreateResponse (lightweight creation
///                            confirmation). Consumed by
///                            checkout_repository_impl.dart, NOT by this file.
///   GET  /orders           → { orders: [OrderListItem] }
///   GET  /orders/:id       → OrderDetailResponse
///   GET  /admin/orders     → AdminOrderSummary
///   GET  /admin/orders/:id → admin OrderDetailResponse
///
/// Canonical money (never re-derived on the client):
///   subtotal                  = P  (unit_price × quantity, before discount)
///   shipping_total            = S
///   commission_amount         = C  (seller-side)
///   service_fee_amount        = F  (buyer-side payment fee)
///   total_before_coins_amount = PD + S     (canonical buyer-funded base)
///   total_payable_amount      = PD + S + F (buyer's gross payable)
///
/// RULES ENFORCED IN THIS FILE:
///   - exactly ONE read per backend key (no camelCase fallback, no legacy
///     alias, no `??` dual read);
///   - no phantom key (a key the backend never emits is not parsed);
///   - no client-side money derivation.
library;

import 'package:labuda/domains/commerce/transaction/order/domain/entities/order_status.dart'
    show OrderStatus;

// ==================== ORDER API RESPONSE DTOS ====================

/// Parse timestamp from backend.
///
/// Backend sends int64 Unix timestamps (seconds since epoch) on the order wire
/// contract.
DateTime? _parseOrderTimestamp(dynamic value) {
  if (value == null) return null;

  if (value is String) {
    if (value.isEmpty) return null;
    return DateTime.tryParse(value);
  }

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

/// Order line item — GET /orders/:id `items[]` (backend OrderItemDTO).
///
/// Backend emits: id, order_id, product_id, name, unit_price_snapshot,
/// quantity, subtotal. `order_id` is redundant for a DTO that is already
/// scoped to one order, and `subtotal` is a backend-computed duplicate of
/// unit_price_snapshot × quantity (OrderItem.subtotal already derives it), so
/// neither is parsed — one canonical representation per value.
class OrderItemApiResponse {
  final String id;
  final String productId;
  final String name;
  final double unitPrice;
  final int quantity;

  const OrderItemApiResponse({
    required this.id,
    required this.productId,
    required this.name,
    required this.unitPrice,
    required this.quantity,
  });

  factory OrderItemApiResponse.fromJson(Map<String, dynamic> json) {
    return OrderItemApiResponse(
      id: json['id'] as String? ?? '',
      productId: json['product_id'] as String? ?? '',
      name: json['name'] as String? ?? '',
      unitPrice: (json['unit_price_snapshot'] as num?)?.toDouble() ?? 0.0,
      quantity: json['quantity'] as int? ?? 0,
    );
  }
}

class OrderApiResponse {
  final String id;
  final String orderNumber;
  final String buyerId;
  final String sellerId;
  final int quantity;

  // Canonical pricing fields matching backend OrderDetailResponse:
  // subtotal = P (unit_price × quantity, before discount)
  // shipping_total = S
  // commission_amount = C (seller-side)
  // service_fee_amount = F (buyer-side)
  // total_payable_amount = PD + S + F
  // total_before_coins_amount = PD + S (canonical buyer base)
  final double subtotal;
  final double shippingTotal;
  final double commissionAmount;
  final double? serviceFeeAmount;
  final double? totalPayableAmount;
  final double? totalBeforeCoinsAmount;

  final String status;
  final bool hasActiveRefund;
  final ActiveRefundApiResponse? activeRefund;
  final String paymentStatus;

  /// Buyer notes — backend key `buyer_notes` (OrderDetailResponse.BuyerNotes).
  final String? buyerNotes;

  final DateTime createdAt;

  /// Orders are completed at this time (backend `completed_at`).
  ///
  /// NOTE: no `shipped_at` is parsed because the backend Order entity persists
  /// no shipped timestamp; the legacy `shipped_at` read was a phantom key and
  /// was purged.
  final DateTime? completedAt;

  final String? sourceType;
  final String? sourceId;

  /// Canonical order line items — backend key `items` (OrderItemDTO[]).
  final List<OrderItemApiResponse> items;

  /// Immutable buyer address snapshot — backend key `shipping_address`
  /// (persisted as orders.address_snapshot). The legacy `shipping_destination`
  /// key was purged.
  final ShippingAddressApiResponse? shippingAddress;

  // Shipping Readiness Snapshot - frozen at order creation time
  final String? preparationTimeSnapshot;
  final String? preparationNoteSnapshot;
  final DateTime? readyToShipBy;

  // SHIPPING CONFIRMATION TRUTH: tracking reference fields.
  // Backend keys: tracking_number + proof_type ("tracking" | "phone" | "manual").
  // The legacy `shipping_reference` / `reference_type` reads were phantom keys
  // (only the dispute response uses `shipping_reference`) and were purged.
  final String? trackingNumber;
  final String? proofType;
  final String? shippingNote;

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
    required this.quantity,
    required this.subtotal,
    required this.shippingTotal,
    required this.commissionAmount,
    this.serviceFeeAmount,
    this.totalPayableAmount,
    this.totalBeforeCoinsAmount,
    required this.status,
    this.hasActiveRefund = false,
    this.activeRefund,
    required this.paymentStatus,
    this.buyerNotes,
    required this.createdAt,
    this.completedAt,
    this.sourceType,
    this.sourceId,
    this.items = const [],
    this.shippingAddress,
    this.preparationTimeSnapshot,
    this.preparationNoteSnapshot,
    this.readyToShipBy,
    this.trackingNumber,
    this.proofType,
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

  // fromJson factory for API response parsing.
  //
  // ONE read per canonical backend key. No camelCase fallback, no legacy key.
  factory OrderApiResponse.fromJson(Map<String, dynamic> json) {
    final rawItems = json['items'];

    return OrderApiResponse(
      id: json['id'] as String? ?? '',
      orderNumber: json['order_number'] as String? ?? '',
      buyerId: json['buyer_id'] as String? ?? '',
      sellerId: json['seller_id'] as String? ?? '',
      quantity: json['quantity'] as int? ?? 0,
      subtotal: (json['subtotal'] as num?)?.toDouble() ?? 0.0,
      shippingTotal: (json['shipping_total'] as num?)?.toDouble() ?? 0.0,
      commissionAmount: (json['commission_amount'] as num?)?.toDouble() ?? 0.0,
      serviceFeeAmount: (json['service_fee_amount'] as num?)?.toDouble(),
      totalPayableAmount: (json['total_payable_amount'] as num?)?.toDouble(),
      totalBeforeCoinsAmount: (json['total_before_coins_amount'] as num?)
          ?.toDouble(),
      status: json['status'] as String? ?? '',
      hasActiveRefund: json['has_active_refund'] as bool? ?? false,
      activeRefund: json['active_refund'] != null
          ? ActiveRefundApiResponse.fromJson(
              json['active_refund'] as Map<String, dynamic>,
            )
          : null,
      paymentStatus: json['payment_status'] as String? ?? '',
      buyerNotes: json['buyer_notes'] as String?,
      createdAt: _parseOrderTimestamp(json['created_at']) ?? DateTime.now(),
      completedAt: _parseOrderTimestamp(json['completed_at']),
      sourceType: json['source_type'] as String?,
      sourceId: json['source_id'] as String?,
      items: rawItems is List
          ? rawItems
                .whereType<Map<String, dynamic>>()
                .map(OrderItemApiResponse.fromJson)
                .toList()
          : const [],
      shippingAddress: json['shipping_address'] != null
          ? ShippingAddressApiResponse.fromJson(
              json['shipping_address'] as Map<String, dynamic>,
            )
          : null,
      preparationTimeSnapshot: json['preparation_time_snapshot'] as String?,
      preparationNoteSnapshot: json['preparation_note_snapshot'] as String?,
      readyToShipBy: _parseOrderTimestamp(json['ready_to_ship_by']),
      // SHIPPING CONFIRMATION TRUTH: canonical tracking reference fields
      trackingNumber: json['tracking_number'] as String?,
      proofType: json['proof_type'] as String?,
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
}

/// GET /orders envelope: `{ orders: [...], next_cursor?, limit }`.
///
/// Only `orders` is parsed — it is the single canonical key, and it is the only
/// field the repository consumes (pagination is cursor based, so the legacy
/// unread total/page/page_size siblings were purged).
class OrderListApiResponse {
  final List<OrderApiResponse> data;

  OrderListApiResponse({required this.data});

  factory OrderListApiResponse.fromJson(Map<String, dynamic> json) {
    final rawList = json['orders'];

    return OrderListApiResponse(
      data: rawList is List
          ? rawList
                .whereType<Map<String, dynamic>>()
                .map(OrderApiResponse.fromJson)
                .toList()
          : const [],
    );
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
      deliveryDate: json['delivery_date'] as String?,
      signature: json['signature'] as String?,
    );
  }
}

/// Immutable buyer address snapshot — backend `shipping_address`
/// (orders.address_snapshot, AddressSnapshot JSONB).
///
/// Canonical backend shape: recipient_name, phone, province_id, province_name,
/// city_id, city_name, district_id, district_name, village_id, village_name,
/// street_address, postal_code, latitude, longitude.
///
/// The legacy flattened shape (phone_number / address_line1 / city / province /
/// full_address + camelCase fallbacks) never matched this payload and was
/// purged.
class ShippingAddressApiResponse {
  final String recipientName;
  final String phone;
  final String streetAddress;
  final String? provinceId;
  final String? provinceName;
  final String? cityId;
  final String? cityName;
  final String? districtId;
  final String? districtName;
  final String? villageId;
  final String? villageName;
  final String? postalCode;
  final double? latitude;
  final double? longitude;

  ShippingAddressApiResponse({
    required this.recipientName,
    required this.phone,
    required this.streetAddress,
    this.provinceId,
    this.provinceName,
    this.cityId,
    this.cityName,
    this.districtId,
    this.districtName,
    this.villageId,
    this.villageName,
    this.postalCode,
    this.latitude,
    this.longitude,
  });

  factory ShippingAddressApiResponse.fromJson(Map<String, dynamic> json) {
    return ShippingAddressApiResponse(
      recipientName: json['recipient_name'] as String? ?? '',
      phone: json['phone'] as String? ?? '',
      streetAddress: json['street_address'] as String? ?? '',
      provinceId: json['province_id'] as String?,
      provinceName: json['province_name'] as String?,
      cityId: json['city_id'] as String?,
      cityName: json['city_name'] as String?,
      districtId: json['district_id'] as String?,
      districtName: json['district_name'] as String?,
      villageId: json['village_id'] as String?,
      villageName: json['village_name'] as String?,
      postalCode: json['postal_code'] as String?,
      latitude: (json['latitude'] as num?)?.toDouble(),
      longitude: (json['longitude'] as num?)?.toDouble(),
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
