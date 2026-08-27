/// Order Parameter Types
///
/// Parameter types for Order repository operations.
/// These types were originally intended to be in admin_stubs.dart
/// but have been moved to the Order domain for proper DDD layering.
///
/// RECOVERY: Created to fix compile errors from missing types.
library;

import 'order_pricing.dart';
import 'order_status.dart';
import 'shipping_info.dart';
import 'refund_request.dart' show RefundReason, RefundStatus;

// ==================== PREVIEW ORDER ====================

/// Parameters for previewing an order before creation
///
/// Backend resolves the full address from addressId — no inline address object.
class PreviewOrderParams {
  final String? productId;
  final int quantity;
  final String? addressId;
  final bool useCoins;
  final String? notes;
  final String? negotiationId;
  final String? auctionId;
  final String? discountCode;

  /// Source type for pricing preview: 'fixed_price_sale' | 'auction' | 'negotiation'
  /// Required by backend GeneratePreviewRequest (binding:"required")
  final String? sourceType;

  /// Source ID: FixedPriceSale UUID | Auction UUID | Negotiation UUID
  /// Required by backend GeneratePreviewRequest (binding:"required")
  final String? sourceId;

  /// ID of the seller's manual shipping quote
  /// When provided, the preview will use the seller's quoted shipping price
  final String? shippingQuoteId;

  /// Standard shipping option ID selected by buyer from listing options.
  /// Mutually exclusive with shippingQuoteId — backend requires exactly one.
  final String? shippingOptionId;

  const PreviewOrderParams({
    this.productId,
    this.quantity = 1,
    this.addressId,
    this.useCoins = false,
    this.notes,
    this.negotiationId,
    this.auctionId,
    this.discountCode,
    this.sourceType,
    this.sourceId,
    this.shippingQuoteId,
    this.shippingOptionId,
  });

  Map<String, dynamic> toJson() {
    return {
      'product_id': productId,
      'quantity': quantity,
      if (addressId != null) 'address_id': addressId,
      'use_coins': useCoins,
      'notes': notes,
      'negotiation_id': negotiationId,
      'auction_id': auctionId,
      'discount_code': discountCode,
      if (shippingQuoteId != null) 'shipping_quote_id': shippingQuoteId,
      if (shippingOptionId != null) 'shipping_option_id': shippingOptionId,
    };
  }
}

/// Result of order preview operation
///
/// **NEGOTIATION VALIDITY TRUTH:**
/// - The `pricingToken` encapsulates the validated negotiation state at preview time
/// - Backend validates negotiation status (must be 'accepted') when generating token
/// - Token expires after 10 minutes - prevents stale pricing usage
/// - Order creation with expired/invalid token fails with explicit error
/// - UI MUST NOT imply "ready to buy" if pricing token is missing or expired
///
/// **SHIPPING MODE INDICATOR (UI CONTRACT FIX):**
/// - `shippingMode` indicates the shipping source for proper UI display:
///   - "quote": Manual shipping quote from seller (no shipping option selection)
///   - "standard": Standard listing shipping options (user selects shipping)
/// - This allows UI to hide shipping dropdown when using quote and prevent dual source confusion
class PreviewOrderResult {
  final OrderPricing pricing;
  final bool isValid;
  final String? errorMessage;
  final List<String>? validationErrors;
  final bool isAvailable;
  final DateTime? expiresAt;

  // Additional metadata from preview
  final String? pricingToken;
  final String? sellerId;
  final String? buyerId;

  // ============================================================================
  // SHIPPING MODE INDICATOR (UI CONTRACT FIX)
  // ============================================================================
  // Indicates the shipping source for UI to properly display:
  // - "quote": Manual shipping quote from seller (no shipping option selection)
  // - "standard": Standard listing shipping options (user selects shipping)
  final String shippingMode;

  const PreviewOrderResult({
    required this.pricing,
    this.isValid = true,
    this.errorMessage,
    this.validationErrors,
    this.isAvailable = true,
    this.expiresAt,
    this.pricingToken,
    this.sellerId,
    this.buyerId,
    this.shippingMode = 'standard',
  });

  factory PreviewOrderResult.fromJson(Map<String, dynamic> json) {
    return PreviewOrderResult(
      pricing: OrderPricing(
        subtotal: (json['subtotal'] as num?)?.toDouble() ?? 0.0,
        shippingCost: (json['shipping_cost'] as num?)?.toDouble() ?? 0.0,
        serviceFeeAmount:
            (json['service_fee_amount'] as num?)?.toDouble() ??
            (json['admin_fee'] as num?)?.toDouble(),
        adminFee: (json['admin_fee'] as num?)?.toDouble(),
        paymentFee: (json['payment_fee'] as num?)?.toDouble(),
        discount: (json['discount'] as num?)?.toDouble() ?? 0.0,
        total: (json['total'] as num?)?.toDouble() ?? 0.0,
        totalPayableAmount: (json['total_payable_amount'] as num?)?.toDouble(),
        discountCode: json['discount_code'] as String?,
        discountDescription: json['discount_description'] as String?,
      ),
      isValid: json['is_valid'] as bool? ?? true,
      errorMessage: json['error_message'] as String?,
      validationErrors: (json['validation_errors'] as List<dynamic>?)
          ?.map((e) => e.toString())
          .toList(),
      isAvailable: json['is_available'] as bool? ?? true,
      expiresAt: json['expires_at'] != null
          ? DateTime.parse(json['expires_at'] as String)
          : null,
      pricingToken: json['pricing_token'] as String?,
      sellerId: json['seller_id'] as String?,
      buyerId: json['buyer_id'] as String?,
      shippingMode: json['shipping_mode'] as String? ?? 'standard',
    );
  }

  // Convenience getters for pricing fields
  double get subtotal => pricing.subtotal;
  double get shippingCost => pricing.shippingCost;
  double? get serviceFeeAmount => pricing.serviceFeeAmount;
  double? get adminFee => pricing.adminFee;
  double? get paymentFee => pricing.paymentFee;
  double get discount => pricing.discount;
  double get total => pricing.total;
  double? get totalPayableAmount => pricing.totalPayableAmount;

  // Note: coinDiscount is not part of OrderPricing, keeping for compatibility
  double get coinDiscount => 0.0;

  /// Returns true if this preview uses a shipping quote (fixed price)
  /// instead of standard shipping options
  bool get isUsingShippingQuote => shippingMode == 'quote';

  Map<String, dynamic> toJson() {
    return {
      'subtotal': pricing.subtotal,
      'shipping_cost': pricing.shippingCost,
      'service_fee_amount': pricing.serviceFeeAmount,
      'admin_fee': pricing.adminFee,
      'payment_fee': pricing.paymentFee,
      'discount': pricing.discount,
      'total': pricing.total,
      'total_payable_amount': pricing.totalPayableAmount,
      'discount_code': pricing.discountCode,
      'discount_description': pricing.discountDescription,
      'is_valid': isValid,
      'error_message': errorMessage,
      'validation_errors': validationErrors,
      'is_available': isAvailable,
      'expires_at': expiresAt?.toIso8601String(),
      'pricing_token': pricingToken,
      'seller_id': sellerId,
      'buyer_id': buyerId,
      'shipping_mode': shippingMode,
    };
  }
}

// ==================== CREATE ORDER ====================

/// Order item parameters for order creation
class OrderItemParams {
  final String productId;
  final int quantity;

  const OrderItemParams({required this.productId, required this.quantity});

  Map<String, dynamic> toJson() {
    return {'product_id': productId, 'quantity': quantity};
  }
}

/// Parameters for creating an order
/// TRUTHFUL: This type is required by active order creation flow
/// Repository implementation uses: items, shippingInfo, discountCode, useCoins, notes, pricingToken
class CreateOrderParams {
  final List<OrderItemParams> items;
  final ShippingInfo shippingInfo;
  final String pricingToken;
  final String? discountCode;
  final bool? useCoins;
  final String? notes;
  final String? auctionId;
  final String? negotiationId;

  const CreateOrderParams({
    required this.items,
    required this.shippingInfo,
    required this.pricingToken,
    this.discountCode,
    this.useCoins,
    this.notes,
    this.auctionId,
    this.negotiationId,
  });

  Map<String, dynamic> toJson() {
    return {
      'items': items.map((e) => e.toJson()).toList(),
      'shipping_info': {
        'recipient_name': shippingInfo.recipientName,
        'phone': shippingInfo.phone,
        'address': shippingInfo.address,
        'province_id': shippingInfo.provinceId,
        'city_id': shippingInfo.cityId,
        'district_id': shippingInfo.districtId,
        'village_id': shippingInfo.villageId,
        'postal_code': shippingInfo.postalCode,
        'latitude': shippingInfo.latitude,
        'longitude': shippingInfo.longitude,
        'method': shippingInfo.method.name,
        'courier_name': shippingInfo.courierName,
        'shipping_option_id': shippingInfo.shippingOptionId,
      },
      'pricing_token': pricingToken,
      if (discountCode != null) 'discount_code': discountCode,
      if (useCoins != null) 'use_coins': useCoins,
      if (notes != null) 'notes': notes,
      if (auctionId != null) 'auction_id': auctionId,
      if (negotiationId != null) 'negotiation_id': negotiationId,
    };
  }
}

// ==================== GET ORDERS ====================

/// Parameters for getting orders
class GetOrdersParams {
  final String? userId;
  final OrderStatus? status;
  final DateTime? startDate;
  final DateTime? endDate;
  final int? page;
  final int? pageSize;
  final int? limit;
  final String? searchQuery;

  const GetOrdersParams({
    this.userId,
    this.status,
    this.startDate,
    this.endDate,
    this.page,
    this.pageSize,
    this.limit,
    this.searchQuery,
  });

  Map<String, dynamic> toQueryParams() {
    final params = <String, dynamic>{};
    if (userId != null) params['user_id'] = userId;
    if (status != null) params['status'] = status!.name;
    if (startDate != null) {
      params['start_date'] = startDate!.toIso8601String();
    }
    if (endDate != null) params['end_date'] = endDate!.toIso8601String();
    if (page != null) params['page'] = page;
    if (pageSize != null) params['page_size'] = pageSize;
    if (limit != null) params['limit'] = limit;
    if (searchQuery != null) params['search'] = searchQuery;
    return params;
  }
}

// ==================== GET ORDER STATS ====================

/// Parameters for getting order statistics
class GetOrderStatsParams {
  final String sellerId;
  final DateTime? startDate;
  final DateTime? endDate;

  const GetOrderStatsParams({
    required this.sellerId,
    this.startDate,
    this.endDate,
  });

  /// Alias for sellerId - matches API datasource parameter
  bool get asSeller => true;

  Map<String, dynamic> toQueryParams() {
    final params = <String, dynamic>{'seller_id': sellerId};
    if (startDate != null) {
      params['start_date'] = startDate!.toIso8601String();
    }
    if (endDate != null) params['end_date'] = endDate!.toIso8601String();
    return params;
  }
}

// ==================== UPDATE ORDER STATUS ====================

/// Parameters for updating order status
class UpdateOrderStatusParams {
  final String orderId;
  final OrderStatus status;
  final String? reason;

  const UpdateOrderStatusParams({
    required this.orderId,
    required this.status,
    this.reason,
  });

  Map<String, dynamic> toJson() {
    return {
      'order_id': orderId,
      'status': status.name,
      if (reason != null) 'reason': reason,
    };
  }
}

// ==================== CANCEL ORDER ====================

/// Parameters for canceling an order
class CancelOrderParams {
  final String? reason;
  // INVALID TYPE REMOVED: detailedReason (CancelReason enum)

  const CancelOrderParams({this.reason});

  Map<String, dynamic> toJson() {
    return {if (reason != null) 'reason': reason};
  }
}

// INVALID TYPE REMOVED: CancelReason enum
// This type was proven invalid in the truth audit.

// ==================== MARK AS SHIPPED ====================

/// Parameters for marking order as shipped
///
/// SHIPPING CONFIRMATION TRUTH:
/// - shippingReference: REQUIRED - resi number, phone/WA number
/// - referenceType: "tracking" | "phone" | "other" (UI value)
/// - note: Optional shipping note
///
/// BACKEND CONTRACT (POST /orders/:id/ship):
/// - proof_type: "tracking" | "phone" (UI "other" maps to "tracking")
/// - tracking_number: the reference value
/// - note: optional
class MarkAsShippedParams {
  final String orderId;
  final String shippingReference;
  final String? referenceType;
  final String? note;

  const MarkAsShippedParams({
    required this.orderId,
    required this.shippingReference,
    this.referenceType,
    this.note,
  });

  Map<String, dynamic> toJson() {
    // Map UI referenceType to backend proof_type.
    // "other" has no photo-upload support yet, falls back to "tracking".
    final proofType = (referenceType == 'phone') ? 'phone' : 'tracking';
    return {
      'proof_type': proofType,
      'tracking_number': shippingReference,
      if (note != null) 'note': note,
    };
  }
}

// ==================== MARK AS DELIVERED ====================

/// Parameters for marking order as delivered
class MarkAsDeliveredParams {
  final String orderId;
  final String? deliveryNote;

  const MarkAsDeliveredParams({required this.orderId, this.deliveryNote});

  Map<String, dynamic> toJson() {
    return {
      'order_id': orderId,
      if (deliveryNote != null) 'delivery_note': deliveryNote,
    };
  }
}

// ==================== PROCESS PAYMENT ====================

/// Parameters for processing payment
class ProcessPaymentParams {
  final String paymentMethod;
  final String? paymentToken;
  final double? amount;

  const ProcessPaymentParams({
    required this.paymentMethod,
    this.paymentToken,
    this.amount,
  });

  Map<String, dynamic> toJson() {
    return {
      'payment_method': paymentMethod,
      if (paymentToken != null) 'payment_token': paymentToken,
      if (amount != null) 'amount': amount,
    };
  }
}

// ==================== UPDATE PAYMENT TOKEN ====================

/// Parameters for updating payment token
/// TRUTHFUL: Required by repository interface for payment token refresh
class UpdatePaymentTokenParams {
  final String orderId;
  final String paymentToken;

  const UpdatePaymentTokenParams({
    required this.orderId,
    required this.paymentToken,
  });

  Map<String, dynamic> toJson() {
    return {'order_id': orderId, 'payment_token': paymentToken};
  }
}

// ==================== UPDATE SHIPPING INFO ====================

/// Parameters for updating shipping information
/// TRUTHFUL: Required by repository interface for shipping info updates
class UpdateShippingInfoParams {
  final String recipientName;
  final String phone;
  final String address;
  final String? city;
  final String? province;
  final String? postalCode;

  const UpdateShippingInfoParams({
    required this.recipientName,
    required this.phone,
    required this.address,
    this.city,
    this.province,
    this.postalCode,
  });

  Map<String, dynamic> toJson() {
    return {
      'recipient_name': recipientName,
      'phone': phone,
      'address': address,
      if (city != null) 'city': city,
      if (province != null) 'province': province,
      if (postalCode != null) 'postal_code': postalCode,
    };
  }
}

// ==================== WATCH ORDERS ====================

/// Parameters for watching orders via stream
class WatchOrdersParams {
  final String userId;
  final OrderStatus? status;
  final int? limit;

  const WatchOrdersParams({required this.userId, this.status, this.limit});
}

// ==================== REFUND PARAMS ====================

/// Parameters for creating a refund request
class CreateRefundParams {
  final String orderId;
  final RefundReason reason;
  final String description;
  final double? requestedAmount;
  final List<String>? evidence;

  /// Alias for evidence - matches repository implementation expectation
  List<String>? get evidenceUrls => evidence;

  const CreateRefundParams({
    required this.orderId,
    required this.reason,
    required this.description,
    this.requestedAmount,
    this.evidence,
  });

  Map<String, dynamic> toJson() {
    return {
      'order_id': orderId,
      'reason': reason.apiValue,
      'description': description,
      if (requestedAmount != null) 'requested_amount': requestedAmount,
      if (evidence != null) 'evidence_urls': evidence,
    };
  }
}

/// Parameters for listing refunds
class ListRefundsParams {
  final String? orderId;
  final RefundStatus? status;
  final int? page;
  final int? pageSize;

  /// Alias for pageSize - matches repository implementation expectation
  int? get limit => pageSize;

  const ListRefundsParams({
    this.orderId,
    this.status,
    this.page,
    this.pageSize,
  });

  Map<String, dynamic> toQueryParams() {
    final params = <String, dynamic>{};
    if (orderId != null) params['order_id'] = orderId;
    if (status != null) params['status'] = status!.name;
    if (page != null) params['page'] = page;
    if (pageSize != null) params['page_size'] = pageSize;
    return params;
  }
}

// ==================== ORDER STATS ====================

/// Order statistics entity
class OrderStats {
  final int totalOrders;
  final int pendingOrders;
  final int completedOrders;
  final int cancelledOrders;
  final int shippedOrders;
  final double totalRevenue;

  const OrderStats({
    required this.totalOrders,
    required this.pendingOrders,
    required this.completedOrders,
    required this.cancelledOrders,
    this.shippedOrders = 0,
    required this.totalRevenue,
  });

  factory OrderStats.fromJson(Map<String, dynamic> json) {
    return OrderStats(
      totalOrders: json['total_orders'] as int? ?? 0,
      pendingOrders: json['pending_orders'] as int? ?? 0,
      completedOrders: json['completed_orders'] as int? ?? 0,
      cancelledOrders: json['cancelled_orders'] as int? ?? 0,
      shippedOrders: json['shipped_orders'] as int? ?? 0,
      totalRevenue: (json['total_revenue'] as num?)?.toDouble() ?? 0.0,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'total_orders': totalOrders,
      'pending_orders': pendingOrders,
      'completed_orders': completedOrders,
      'cancelled_orders': cancelledOrders,
      'shipped_orders': shippedOrders,
      'total_revenue': totalRevenue,
    };
  }
}

// ==================== PRICING CALCULATION ====================
// INVALID TYPE REMOVED: CalculatePricingParams
// INVALID TYPE REMOVED: PricingItem
// These types were proven invalid in the truth audit.
