// Domain entities (path: data/dto -> domain/entities = ../../)
import '../../domain/entities/order.dart'
    show
        Order,
        DecisionContract,
        DisplayHints,
        Action,
        ActionBlockedReason,
        InputSchema,
        InputFieldDefinition,
        InputFieldValidation;
import '../../domain/entities/order_item.dart';
import '../../domain/entities/order_pricing.dart';
import '../../domain/entities/order_status.dart'
    show OrderStatus, OrderStatusExtension, EscrowStatusExtension;
import '../../domain/entities/order_source.dart'
    show OrderSource, OrderSourceExtension;
import '../../domain/entities/shipping_info.dart' show ShippingInfo;
import 'package:labuda/core/common/types/payment_types.dart'
    show PaymentMethodType, PaymentStatus, PaymentChannel;
import 'package:labuda/core/common/types/preparation_time.dart'
    show PreparationTime;
import '../../domain/entities/shipping_types.dart' show ShippingMethod;

// Request DTOs

/// Create Order Request DTO
///
/// **PRICING TOKEN REQUIRED:**
/// All order creation MUST go through preview endpoint first to obtain a pricing token.
/// The pricing_token is mandatory and ensures the order uses the exact same pricing
/// snapshot that was shown to the user during preview.
///
/// Direct order creation without pricing_token is NOT supported.
class CreateOrderDto {
  final String productId;
  final int quantity;
  final String? discountCode;
  final bool? useCoins;
  final String? notes;
  final ShippingAddressRequestDto shippingAddress;

  /// PRICING TOKEN: Required snapshot ID from preview response
  /// Order creation will fail without a valid pricing token from POST /orders/preview
  final String pricingToken;

  /// Canonical sale surface type.
  final String sourceType;

  /// Canonical sale surface ID.
  final String sourceId;

  /// AUCTION ID: Optional auction context for auction checkout (winning bid or buy now)
  final String? auctionId;

  /// CHAT COMMERCE CONTEXT: Optional context for negotiation checkout
  final String? negotiationId;

  const CreateOrderDto({
    required this.productId,
    required this.quantity,
    this.discountCode,
    this.useCoins,
    this.notes,
    required this.shippingAddress,
    required this.pricingToken,
    required this.sourceType,
    required this.sourceId,
    this.auctionId,
    this.negotiationId,
  });

  Map<String, dynamic> toJson() => {
    'product_id': productId,
    'source_type': sourceType,
    'source_id': sourceId,
    'quantity': quantity,
    if (discountCode != null) 'discount_code': discountCode,
    if (useCoins != null) 'use_coins': useCoins,
    if (notes != null) 'notes': notes,
    'shipping_address': shippingAddress.toJson(),
    'pricing_token': pricingToken,
    if (auctionId != null) 'auction_id': auctionId,
    if (negotiationId != null) 'negotiation_id': negotiationId,
  };
}

class ShippingAddressRequestDto {
  final String recipientName;
  final String phoneNumber;
  final String addressLine1;
  final String? addressLine2;
  final String? city;
  final String? province;
  final String? postalCode;

  const ShippingAddressRequestDto({
    required this.recipientName,
    required this.phoneNumber,
    required this.addressLine1,
    this.addressLine2,
    this.city,
    this.province,
    this.postalCode,
  });

  Map<String, dynamic> toJson() => {
    'recipient_name': recipientName,
    'phone_number': phoneNumber,
    'address_line1': addressLine1,
    if (addressLine2 != null) 'address_line2': addressLine2,
    if (city != null) 'city': city,
    if (province != null) 'province': province,
    if (postalCode != null) 'postal_code': postalCode,
  };
}

/// Preview Order Request DTO
///
/// Request for POST /orders/preview
/// Used to get pricing breakdown before creating an order
///
/// **CHAT COMMERCE SUPPORT:**
/// - For direct fixed-price sale purchase: provide productId
/// - For negotiation: provide negotiationId (negotiation must be accepted)
/// - For auction: provide auctionId (winning bid or buy now)
///
/// **IMPORTANT:** Only ONE of productId/negotiationId/auctionId
/// should be provided. The backend will validate the commerce context and return pricing token.
class PreviewOrderRequestDto {
  final String productId;
  final int quantity;
  final ShippingAddressRequestDto shippingAddress;
  final bool useCoins;
  final String? discountCode;
  final String? notes;

  /// Chat commerce context - mutually exclusive with productId for private pricing
  final String? negotiationId;

  /// Auction checkout context - for winning bid or buy now
  final String? auctionId;

  const PreviewOrderRequestDto({
    required this.productId,
    required this.quantity,
    required this.shippingAddress,
    this.useCoins = false,
    this.discountCode,
    this.notes,
    this.negotiationId,
    this.auctionId,
  });

  Map<String, dynamic> toJson() => {
    'product_id': productId,
    'quantity': quantity,
    'shipping_address': shippingAddress.toJson(),
    'use_coins': useCoins,
    if (discountCode != null) 'discount_code': discountCode,
    if (notes != null) 'notes': notes,
    if (negotiationId != null) 'negotiation_id': negotiationId,
    if (auctionId != null) 'auction_id': auctionId,
  };

  /// Create request for direct fixed-price sale purchase
  factory PreviewOrderRequestDto.forProduct({
    required String productId,
    required int quantity,
    required ShippingAddressRequestDto shippingAddress,
    bool useCoins = false,
    String? discountCode,
    String? notes,
  }) {
    return PreviewOrderRequestDto(
      productId: productId,
      quantity: quantity,
      shippingAddress: shippingAddress,
      useCoins: useCoins,
      discountCode: discountCode,
      notes: notes,
    );
  }

  /// Create request for negotiation purchase
  factory PreviewOrderRequestDto.forNegotiation({
    required String negotiationId,
    required String productId,
    required int quantity,
    required ShippingAddressRequestDto shippingAddress,
    bool useCoins = false,
    String? discountCode,
    String? notes,
  }) {
    return PreviewOrderRequestDto(
      productId: productId,
      quantity: quantity,
      shippingAddress: shippingAddress,
      useCoins: useCoins,
      discountCode: discountCode,
      notes: notes,
      negotiationId: negotiationId,
    );
  }

  /// Create request for auction purchase (winning bid or buy now)
  ///
  /// **AUCTION PRICING:** Final auction price (winning bid or buy now price)
  /// is determined by backend. Frontend only provides auctionId for validation.
  ///
  /// discountCode is optional for auction purchases and is validated by the
  /// backend against the auction checkout context.
  factory PreviewOrderRequestDto.forAuction({
    required String auctionId,
    required String productId,
    required int quantity,
    required ShippingAddressRequestDto shippingAddress,
    bool useCoins = false,
    String? discountCode,
    String? notes,
  }) {
    return PreviewOrderRequestDto(
      productId: productId,
      quantity: quantity,
      shippingAddress: shippingAddress,
      useCoins: useCoins,
      discountCode: discountCode,
      notes: notes,
      auctionId: auctionId,
    );
  }
}

/// Preview Order Response DTO
///
/// Response from POST /orders/preview
/// Contains pricing breakdown from backend
///
/// PRICING TOKEN: Backend returns a unique pricing_token that must be passed
/// when creating the order. This ensures the final order uses the exact same
/// pricing snapshot that was shown to the user during preview.
class PreviewOrderResponseDto {
  final double subtotal;
  final double shippingCost;
  final double? serviceFeeAmount;
  final double? adminFee;
  final double? coinDiscount;
  final double discount;
  final double total;
  final double? totalPayableAmount;
  final String? currency;
  final String? discountCode;
  final String? discountDescription;
  final List<String>? appliedPromos;
  final Map<String, dynamic>? extraData;

  /// PRICING TOKEN: Unique snapshot ID from backend
  /// Must be passed to order creation to lock in the previewed pricing
  final String pricingToken;

  const PreviewOrderResponseDto({
    required this.subtotal,
    required this.shippingCost,
    this.serviceFeeAmount,
    this.adminFee,
    this.coinDiscount,
    required this.discount,
    required this.total,
    this.totalPayableAmount,
    this.currency,
    this.discountCode,
    this.discountDescription,
    this.appliedPromos,
    this.extraData,
    required this.pricingToken,
  });

  factory PreviewOrderResponseDto.fromJson(Map<String, dynamic> json) {
    return PreviewOrderResponseDto(
      subtotal: _parseAmount(json['subtotal']),
      shippingCost: _parseAmount(json['shipping_cost']),
      serviceFeeAmount:
          _parseNullableAmount(json['service_fee_amount']) ??
          _parseNullableAmount(json['admin_fee']),
      adminFee: _parseNullableAmount(json['admin_fee']),
      coinDiscount: _parseNullableAmount(json['coin_discount']),
      discount: _parseAmount(json['discount']),
      total: _parseAmount(json['total']),
      totalPayableAmount: _parseNullableAmount(json['total_payable_amount']),
      currency: json['currency'] as String?,
      discountCode: json['discount_code'] as String?,
      discountDescription: json['discount_description'] as String?,
      appliedPromos: (json['applied_promos'] as List<dynamic>?)
          ?.map((e) => e.toString())
          .toList(),
      extraData: json['extra_data'] as Map<String, dynamic>?,
      pricingToken: json['pricing_token'] as String? ?? '',
    );
  }

  /// Helper to parse amount from backend (float64/full Rupiah)
  static double _parseAmount(dynamic value) {
    if (value == null) return 0.0;
    if (value is num) return value.toDouble();
    if (value is String) return double.tryParse(value) ?? 0.0;
    return 0.0;
  }

  /// Helper to parse nullable amount from backend
  static double? _parseNullableAmount(dynamic value) {
    if (value == null) return null;
    if (value is num) return value.toDouble();
    if (value is String) return double.tryParse(value);
    return null;
  }

  Map<String, dynamic> toJson() => {
    'subtotal': subtotal,
    'shipping_cost': shippingCost,
    'service_fee_amount': serviceFeeAmount,
    'admin_fee': adminFee,
    'coin_discount': coinDiscount,
    'discount': discount,
    'total': total,
    'total_payable_amount': totalPayableAmount,
    if (currency != null) 'currency': currency,
    if (discountCode != null) 'discount_code': discountCode,
    if (discountDescription != null)
      'discount_description': discountDescription,
    if (appliedPromos != null) 'applied_promos': appliedPromos,
    if (extraData != null) 'extra_data': extraData,
    'pricing_token': pricingToken,
  };

  /// Convert to OrderPricing domain entity
  OrderPricing toOrderPricing() {
    return OrderPricing(
      subtotal: subtotal,
      shippingCost: shippingCost,
      serviceFeeAmount: serviceFeeAmount,
      adminFee: adminFee,
      paymentFee: null,
      discount: discount,
      total: total,
      totalPayableAmount: totalPayableAmount,
      discountCode: discountCode,
      discountDescription: discountDescription,
    );
  }
}

class UpdateOrderStatusDto {
  final String status;
  final String? notes;

  const UpdateOrderStatusDto({required this.status, this.notes});

  Map<String, dynamic> toJson() => {
    'status': status,
    if (notes != null) 'notes': notes,
  };
}

class CreateRefundDto {
  final String orderId;
  final String reason;
  final String description;
  final List<String>? evidence;

  const CreateRefundDto({
    required this.orderId,
    required this.reason,
    required this.description,
    this.evidence,
  });

  Map<String, dynamic> toJson() => {
    'order_id': orderId,
    'reason': reason,
    'description': description,
    if (evidence != null) 'evidence': evidence,
  };
}

class RefundActionDto {
  final String response;

  const RefundActionDto({required this.response});

  Map<String, dynamic> toJson() => {'response': response};
}

// =============================================================================
// Response DTOs
// =============================================================================

/// OrderResponseDto - Complete parsing of backend Order response
///
/// Backend Go is the SINGLE SOURCE OF TRUTH.
/// ALL fields from backend response are parsed - even if not yet used by UI.
///
/// TRACK 9: OrderResponse DTO Creation
/// - Parses 40+ fields from backend
/// - Includes nested objects: shipping_address, product, decision
/// - Provides toEntity() conversion to Order domain entity
class OrderResponseDto {
  // Core identifiers
  final String id;
  final String? orderNumber;
  final String buyerId;
  final String sellerId;
  final String? productId;

  // Status
  final String status;

  // Pricing fields (backend authority - values in full Rupiah (float64 from backend))
  final num totalAmount;
  final num? shippingFee;
  final num? discountAmount;
  final num? coinDiscount;
  final num? platformFee;
  final num? serviceFeeAmount;
  final num? totalPayableAmount;
  final num? finalAmount;

  // References
  final String? paymentId;
  final String? discountId;

  // Order details
  final int? quantity;
  final String? notes;

  // Refund
  final String? refundId;

  // Deadlines (ISO 8601 strings)
  final String? sellerAcceptDeadline;
  final String? buyerConfirmDeadline;

  // Time remaining (in seconds)
  final int? sellerAcceptTimeRemainingSeconds;
  final int? buyerConfirmTimeRemainingSeconds;

  // Auto actions
  final bool? shouldAutoCancel;
  final bool? shouldAutoComplete;

  // Timestamps (ISO 8601 strings)
  final String createdAt;
  final String? updatedAt;
  final String? confirmedAt;
  final String? shippedAt;
  final String? completedAt;
  final String? cancelledAt;
  final String? deletedAt;

  // TRACK 4: Additional backend fields
  final String? idempotencyKey;
  final String? priceSnapshotId;

  // Source metadata (aligned with backend OrderSourceType)
  final String? sourceType;
  final String? sourceId;

  // Shipping Readiness Snapshot (for overdue calculation)
  final String? preparationTimeSnapshot;
  final String? preparationNoteSnapshot;
  final String? readyToShipBy;

  // Overdue Display Layer (computed by backend, not persisted)
  final String?
  overdueTier; // none, overdue, severely_overdue, critical_overdue
  final int? overdueDays; // Days past ready_to_ship_by (null if not overdue)
  final bool? isOverdue; // Convenience boolean

  // SHIPPING CONFIRMATION TRUTH: Shipping reference fields
  final String?
  shippingReference; // Renamed from tracking_number - resi, phone/WA, or other
  final String? referenceType; // "tracking" | "phone" | "other"
  final String? shippingNote; // Optional shipping note from seller

  // Shipping source + origin (I1-C1: where shipping cost originated + seller origin)
  final String? shippingSource; // "fixed_price_sale" or "shipping_quote"
  final Map<String, dynamic>?
  shippingOriginSnapshot; // Seller farm/warehouse address snapshot

  // Nested objects
  final ShippingAddressResponseDto? shippingAddress;
  final ProductSummaryDto? product;
  final DecisionContractResponseDto? decision;

  // 🔒 ESCROW STATUS - Financial state of the order
  // Backend authority: backend/internal/commerce/order/entity/escrow_status.go
  // Values: none, holding, frozen, released, refunded, partially_refunded, partially_released
  final String? escrowStatus;

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

  // Additional fields not yet in entity (stored in DTO only)
  final Map<String, dynamic>? extraData;

  const OrderResponseDto({
    required this.id,
    this.orderNumber,
    required this.buyerId,
    required this.sellerId,
    this.productId,
    required this.status,
    required this.totalAmount,
    this.shippingFee,
    this.discountAmount,
    this.coinDiscount,
    this.platformFee,
    this.serviceFeeAmount,
    this.totalPayableAmount,
    this.finalAmount,
    this.paymentId,
    this.discountId,
    this.quantity,
    this.notes,
    this.refundId,
    this.sellerAcceptDeadline,
    this.buyerConfirmDeadline,
    this.sellerAcceptTimeRemainingSeconds,
    this.buyerConfirmTimeRemainingSeconds,
    this.shouldAutoCancel,
    this.shouldAutoComplete,
    required this.createdAt,
    this.updatedAt,
    this.confirmedAt,
    this.shippedAt,
    this.completedAt,
    this.cancelledAt,
    this.deletedAt,
    this.idempotencyKey,
    this.priceSnapshotId,
    this.sourceType,
    this.sourceId,
    this.preparationTimeSnapshot,
    this.preparationNoteSnapshot,
    this.readyToShipBy,
    this.overdueTier,
    this.overdueDays,
    this.isOverdue,
    this.shippingReference,
    this.referenceType,
    this.shippingNote,
    this.shippingSource,
    this.shippingOriginSnapshot,
    this.shippingAddress,
    this.product,
    this.decision,
    this.escrowStatus,
    // Stage 2 identity parse-only fields
    this.sellerUsername,
    this.sellerFarmName,
    this.sellerAvatarUrl,
    this.buyerUsername,
    this.extraData,
  });

  /// Parse from backend JSON response
  ///
  /// Handles snake_case backend field names and converts to camelCase.
  /// All backend fields are preserved - none are skipped.
  ///
  /// TIMESTAMP HANDLING: Backend sends timestamps as int64 Unix seconds.
  /// This parser converts Unix timestamps to ISO 8601 strings for internal use.
  factory OrderResponseDto.fromJson(Map<String, dynamic> json) {
    // Parse decision object if present
    DecisionContractResponseDto? decision;
    if (json['decision'] != null) {
      decision = DecisionContractResponseDto.fromJson(
        json['decision'] as Map<String, dynamic>,
      );
    }

    // Parse shipping address if present
    ShippingAddressResponseDto? shippingAddress;
    if (json['shipping_address'] != null) {
      shippingAddress = ShippingAddressResponseDto.fromJson(
        json['shipping_address'] as Map<String, dynamic>,
      );
    }

    // Parse product summary if present
    ProductSummaryDto? product;
    if (json['product'] != null) {
      product = ProductSummaryDto.fromJson(
        json['product'] as Map<String, dynamic>,
      );
    }

    return OrderResponseDto(
      id: json['id'] as String,
      orderNumber: json['order_number'] as String?,
      buyerId: json['buyer_id'] as String,
      sellerId: json['seller_id'] as String,
      productId: json['product_id'] as String?,
      status: json['status'] as String,
      totalAmount: _parseAmount(json['total_amount']),
      shippingFee: _parseNullableAmount(json['shipping_fee']),
      discountAmount: _parseNullableAmount(json['discount_amount']),
      coinDiscount: _parseNullableAmount(json['coin_discount']),
      platformFee: _parseNullableAmount(json['platform_fee']),
      serviceFeeAmount: _parseNullableAmount(json['service_fee_amount']),
      totalPayableAmount: _parseNullableAmount(json['total_payable_amount']),
      finalAmount: _parseNullableAmount(json['final_amount']),
      paymentId: json['payment_id'] as String?,
      discountId: json['discount_id'] as String?,
      quantity: json['quantity'] as int?,
      notes: json['notes'] as String?,
      refundId: json['refund_id'] as String?,
      sellerAcceptDeadline: _parseTimestamp(json['seller_accept_deadline']),
      buyerConfirmDeadline: _parseTimestamp(json['buyer_confirm_deadline']),
      sellerAcceptTimeRemainingSeconds:
          json['seller_accept_time_remaining_seconds'] as int?,
      buyerConfirmTimeRemainingSeconds:
          json['buyer_confirm_time_remaining_seconds'] as int?,
      shouldAutoCancel: json['should_auto_cancel'] as bool?,
      shouldAutoComplete: json['should_auto_complete'] as bool?,
      createdAt: _parseTimestamp(json['created_at']) ?? '',
      updatedAt: _parseTimestamp(json['updated_at']),
      confirmedAt: _parseTimestamp(json['confirmed_at']),
      shippedAt: _parseTimestamp(json['shipped_at']),
      completedAt: _parseTimestamp(json['completed_at']),
      cancelledAt: _parseTimestamp(json['cancelled_at']),
      deletedAt: _parseTimestamp(json['deleted_at']),
      idempotencyKey: json['idempotency_key'] as String?,
      priceSnapshotId: json['price_snapshot_id'] as String?,
      // Source metadata from backend
      sourceType: json['source_type'] as String?,
      sourceId: json['source_id'] as String?,
      // Shipping Readiness Snapshot
      preparationTimeSnapshot: json['preparation_time_snapshot'] as String?,
      preparationNoteSnapshot: json['preparation_note_snapshot'] as String?,
      readyToShipBy: _parseTimestamp(json['ready_to_ship_by']),
      // Overdue Display Layer
      overdueTier: json['overdue_tier'] as String?,
      overdueDays: json['overdue_days'] as int?,
      isOverdue: json['is_overdue'] as bool?,
      // Shipping Confirmation (TRUTH)
      shippingReference:
          json['shipping_reference'] as String? ??
          json['tracking_number']
              as String?, // Fallback for backward compatibility
      referenceType: json['reference_type'] as String?,
      shippingNote: json['shipping_note'] as String?,
      shippingSource: json['shipping_source'] as String?,
      shippingOriginSnapshot: json['shipping_origin'] as Map<String, dynamic>?,
      shippingAddress: shippingAddress,
      product: product,
      decision: decision,
      // 🔒 ESCROW STATUS - Parse from backend
      escrowStatus: json['escrow_status'] as String?,
      // Stage 2 identity parse-only fields. Tolerate old payload (null) and
      // new payload. No fullName fallback — owner truth is username/farm.
      sellerUsername: json['seller_username'] as String?,
      sellerFarmName: json['seller_farm_name'] as String?,
      sellerAvatarUrl: json['seller_avatar_url'] as String?,
      buyerUsername: json['buyer_username'] as String?,
    );
  }

  /// Parse timestamp from backend.
  ///
  /// Backend sends int64 Unix timestamps (seconds since epoch).
  /// For backward compatibility, also handles ISO 8601 strings.
  /// Returns ISO 8601 string or null.
  static String? _parseTimestamp(dynamic value) {
    if (value == null) return null;

    // If already a string (ISO 8601), return as-is
    if (value is String) {
      return value.isEmpty ? null : value;
    }

    // If int/num (Unix timestamp), convert to ISO 8601 string
    if (value is num) {
      final timestamp = value.toInt();
      // Handle both seconds (< 1000000000000) and milliseconds
      final dateTime = timestamp > 1000000000000
          ? DateTime.fromMillisecondsSinceEpoch(timestamp)
          : DateTime.fromMillisecondsSinceEpoch(timestamp * 1000);
      return dateTime.toIso8601String();
    }

    return null;
  }

  /// Convert to JSON (for debugging/testing)
  Map<String, dynamic> toJson() => {
    'id': id,
    'order_number': orderNumber,
    'buyer_id': buyerId,
    'seller_id': sellerId,
    'product_id': productId,
    'status': status,
    'total_amount': totalAmount,
    'shipping_fee': shippingFee,
    'discount_amount': discountAmount,
    'coin_discount': coinDiscount,
    'platform_fee': platformFee,
    'service_fee_amount': serviceFeeAmount,
    'total_payable_amount': totalPayableAmount,
    'final_amount': finalAmount,
    'payment_id': paymentId,
    'discount_id': discountId,
    'quantity': quantity,
    'notes': notes,
    'refund_id': refundId,
    'seller_accept_deadline': sellerAcceptDeadline,
    'buyer_confirm_deadline': buyerConfirmDeadline,
    'seller_accept_time_remaining_seconds': sellerAcceptTimeRemainingSeconds,
    'buyer_confirm_time_remaining_seconds': buyerConfirmTimeRemainingSeconds,
    'should_auto_cancel': shouldAutoCancel,
    'should_auto_complete': shouldAutoComplete,
    'created_at': createdAt,
    'updated_at': updatedAt,
    'confirmed_at': confirmedAt,
    'shipped_at': shippedAt,
    'completed_at': completedAt,
    'cancelled_at': cancelledAt,
    'deleted_at': deletedAt,
    'idempotency_key': idempotencyKey,
    'price_snapshot_id': priceSnapshotId,
    // Shipping Confirmation
    'shipping_reference': shippingReference,
    'reference_type': referenceType,
    'shipping_note': shippingNote,
    'shipping_address': shippingAddress?.toJson(),
    'product': product?.toJson(),
    'decision': decision?.toJson(),
    // 🔒 ESCROW STATUS - Serialize to JSON
    if (escrowStatus != null) 'escrow_status': escrowStatus,
  };

  /// Convert DTO to Order domain entity
  ///
  /// Maps parsed backend fields to Order entity.
  /// Fields that don't exist in Order entity are preserved in DTO only.
  ///
  /// TRACK 9: This is a PURE mapping function - no business logic.
  /// Backend is SINGLE SOURCE OF TRUTH for all values.
  Order toEntity({
    List<OrderItem>? items,
    PaymentMethodType? paymentMethod,
    PaymentStatus? paymentStatus,
    PaymentChannel? paymentChannel,
    ShippingInfo? shippingInfo,
    OrderPricing? pricing,
    OrderSource? source,
  }) {
    // Parse status string to OrderStatus enum
    final orderStatus =
        OrderStatusExtension.parse(status) ?? OrderStatus.pending;

    // Parse source from backend source_type field
    final parsedSource = sourceType != null
        ? OrderSourceExtension.fromJson(sourceType!)
        : OrderSource.forSale;

    // Use provided source override or parsed source from backend
    final orderSource = source ?? parsedSource;

    // Parse timestamps
    final created = _parseDateTime(createdAt) ?? DateTime.now();
    final confirmed = _parseDateTime(confirmedAt);
    final shipped = _parseDateTime(shippedAt);
    final completed = _parseDateTime(completedAt);
    final cancelled = _parseDateTime(cancelledAt);
    final buyerConfirmDeadline = _parseDateTime(this.buyerConfirmDeadline);
    final deletedAt = _parseDateTime(this.deletedAt);
    final readyToShipBy = _parseDateTime(this.readyToShipBy);

    // Convert DecisionContractResponseDto to DecisionContract using V2 structure
    final domainDecision = decision != null
        ? _convertDecisionDtoToDomain(decision!)
        : null;

    // Parse preparation time snapshot
    final prepTime = PreparationTime.fromJson(preparationTimeSnapshot);

    // 🔒 ESCROW STATUS - Parse from backend string to enum
    // CRITICAL: If backend sends unknown value, log error and use null
    // UI must handle null gracefully (display: "Escrow status unknown")
    final parsedEscrowStatus = EscrowStatusExtension.parse(escrowStatus);

    if (escrowStatus != null && parsedEscrowStatus == null) {
      // Unknown escrow status from backend - this should NEVER happen.
      // UI handles null gracefully. TODO: Send to error tracking (Sentry/Firebase Crashlytics)
    }

    return Order(
      id: id,
      buyerId: buyerId,
      sellerId: sellerId,
      items: items ?? const [], // Caller must provide or use empty list
      status: orderStatus,
      // NOTE: primaryStatus removed - redundant (always duplicated status)
      // NOTE: activeIssues removed - never populated, use decision.state for status
      paymentMethod: paymentMethod ?? PaymentMethodType.bankTransfer,
      paymentStatus: paymentStatus ?? PaymentStatus.pending,
      paymentChannel: paymentChannel,
      tokenRegenerationCount: 0,
      shippingInfo: shippingInfo ?? _createDefaultShippingInfo(),
      pricing: pricing ?? _createDefaultPricing(),
      notes: notes,
      preparationTimeSnapshot: prepTime,
      preparationNoteSnapshot: preparationNoteSnapshot,
      readyToShipBy: readyToShipBy,
      overdueTier: overdueTier,
      overdueDays: overdueDays,
      isOverdue: isOverdue,
      createdAt: created,
      paidAt: confirmed, // Assuming confirmed_at = paid_at
      shippedAt: shipped,
      deliveredAt: null, // Backend sends delivered_at separately if needed
      cancelledAt: cancelled,
      completedAt: completed,
      // TRACK 4: Backend fields - pure mapping from DTO
      orderNumber: orderNumber,
      idempotencyKey: idempotencyKey,
      priceSnapshotId: priceSnapshotId,
      discountId: discountId,
      buyerConfirmDeadline: buyerConfirmDeadline,
      confirmedAt: confirmed,
      deletedAt: deletedAt,
      source: orderSource,
      sourceId: sourceId ?? productId,
      decision: domainDecision,
      // Payment deadline from sellerAcceptDeadline
      paymentDeadline: _parseDateTime(sellerAcceptDeadline),
      acceptanceDeadline: _parseDateTime(sellerAcceptDeadline),
      // Refund tracking
      activeRefundId: refundId,
      refundStatus: null,
      // 🔒 ESCROW STATUS - Pass parsed enum to Order entity
      escrowStatus: parsedEscrowStatus,
      // STAGE 3 — IDENTITY FIELDS (Phase 5)
      // Owner Truth identity scalars from Stage 1 backend payload.
      // Nullable; old payloads land null. No fake fallback. No fullName fallback.
      sellerUsername: sellerUsername,
      sellerFarmName: sellerFarmName,
      sellerAvatarUrl: sellerAvatarUrl,
      buyerUsername: buyerUsername,
    );
  }

  /// Helper to parse DateTime from ISO 8601 string
  DateTime? _parseDateTime(String? isoString) {
    if (isoString == null) return null;
    return DateTime.tryParse(isoString);
  }

  /// Static helper to parse amount from backend (float64/full Rupiah)
  /// Returns num to preserve precision from backend float64 values
  static num _parseAmount(dynamic value) {
    if (value == null) return 0;
    if (value is num) return value;
    if (value is String) return num.tryParse(value) ?? 0;
    return 0;
  }

  /// Static helper to parse nullable amount from backend (float64/full Rupiah)
  /// Returns num? to preserve precision from backend float64 values
  static num? _parseNullableAmount(dynamic value) {
    if (value == null) return null;
    if (value is num) return value;
    if (value is String) return num.tryParse(value);
    return null;
  }

  /// Parse PayoutStatus from string
  // Note: PayoutStatus enum is in seller_earnings.dart
  // We'll return the string value for now

  /// Create default ShippingInfo for fallback
  ShippingInfo _createDefaultShippingInfo() {
    return const ShippingInfo(
      recipientName: '',
      phone: '',
      address: '',
      method: ShippingMethod.courier,
      shippingCost: 0,
    );
  }

  /// Create OrderPricing from backend response
  ///
  /// NOTE: Uses backend-provided values. Null financial fields indicate
  /// backend hasn't provided them yet - NOT defaults to 0.
  /// Financial values (adminFee, paymentFee) are nullable.
  /// sellerCommission and sellerEarnings REMOVED (Wave 3.1B) - seller financial data
  /// belongs in finance-derived sources, not Order domain.
  OrderPricing _createDefaultPricing() {
    return OrderPricing(
      subtotal: totalAmount.toDouble(),
      shippingCost: (shippingFee ?? 0).toDouble(),
      serviceFeeAmount: (serviceFeeAmount ?? platformFee ?? 0).toDouble(),
      adminFee: platformFee?.toDouble(),
      paymentFee: null, // Backend must provide
      discount: (discountAmount ?? 0).toDouble(),
      total: totalAmount.toDouble(),
      totalPayableAmount: (totalPayableAmount ?? totalAmount).toDouble(),
    );
  }

  /// Convert DecisionContractResponseDto to DecisionContract domain entity
  /// This handles V2 structure with primary_action and secondary_actions
  DecisionContract _convertDecisionDtoToDomain(
    DecisionContractResponseDto dto,
  ) {
    return DecisionContract(
      state: dto.state,
      version: dto.version,
      decisionVersion: dto.decisionVersion,
      primaryAction: dto.primaryAction?.toDomain(),
      secondaryActions: dto.secondaryActions.map((a) => a.toDomain()).toList(),
      display: dto.display != null
          ? DisplayHints.fromJson(dto.display!.toJson())
          : null,
    );
  }
}

/// Decision Contract V2 DTO from backend
///
/// Backend is the SINGLE SOURCE OF TRUTH for all business decisions.
/// Frontend MUST NOT derive state or allowed actions from other fields.
///
/// NOTE: Named DecisionContractResponseDto to avoid conflict with domain DecisionContract.
class DecisionContractResponseDto {
  final String state; // Authoritative business state (order status)
  final String version; // Decision contract version
  final int decisionVersion; // Optimistic concurrency counter
  final ActionResponseDto? primaryAction; // Main call-to-action
  final List<ActionResponseDto> secondaryActions; // Alternative actions
  final DisplayHintsDto? display; // UI rendering hints

  const DecisionContractResponseDto({
    required this.state,
    this.version = '3.0.0',
    this.decisionVersion = 0,
    this.primaryAction,
    this.secondaryActions = const [],
    this.display,
  });

  factory DecisionContractResponseDto.fromJson(Map<String, dynamic> json) {
    // Parse primary_action
    final primaryActionJson = json['primary_action'];
    ActionResponseDto? primaryAction;
    if (primaryActionJson != null &&
        primaryActionJson is Map<String, dynamic>) {
      primaryAction = ActionResponseDto.fromJson(primaryActionJson);
    }

    // Parse secondary_actions
    final secondaryActionsJson = json['secondary_actions'];
    List<ActionResponseDto> secondaryActions = const [];
    if (secondaryActionsJson != null && secondaryActionsJson is List) {
      secondaryActions = secondaryActionsJson
          .map(
            (e) => e is Map<String, dynamic>
                ? ActionResponseDto.fromJson(e)
                : null,
          )
          .whereType<ActionResponseDto>()
          .toList();
    }

    // Parse display hints
    final displayJson = json['display'];
    DisplayHintsDto? display;
    if (displayJson != null && displayJson is Map<String, dynamic>) {
      display = DisplayHintsDto.fromJson(displayJson);
    }

    return DecisionContractResponseDto(
      state: json['state'] as String? ?? '',
      version: json['version'] as String? ?? '3.0.0',
      decisionVersion: json['decision_version'] as int? ?? 0,
      primaryAction: primaryAction,
      secondaryActions: secondaryActions,
      display: display,
    );
  }

  Map<String, dynamic> toJson() => {
    'state': state,
    'version': version,
    'decision_version': decisionVersion,
    if (primaryAction != null) 'primary_action': primaryAction!.toJson(),
    if (secondaryActions.isNotEmpty)
      'secondary_actions': secondaryActions.map((a) => a.toJson()).toList(),
    if (display != null) 'display': display!.toJson(),
  };
}

/// Action Response DTO - represents a single executable action on an order
/// Frontend renders buttons directly from this structure - no business logic in UI
class ActionResponseDto {
  final String type; // Action type enum (mark_shipped, complete, refund, etc.)
  final String labelKey; // Localization key for button label
  final bool enabled; // Whether the action is currently enabled
  final ActionBlockedReasonDto? blocked; // Why blocked (if disabled)
  final String endpoint; // API endpoint to call
  final String method; // HTTP method (POST, PATCH, etc.)
  final bool requiresIdempotency; // Whether action requires idempotency key
  final bool financial; // Whether action affects money (ledger validation)
  final InputSchemaDto? inputSchema; // Structured input definition

  // Deprecated fields - kept for backward compatibility during migration
  final bool? requiresInput;
  final String? inputHint;
  final String? inputType;

  const ActionResponseDto({
    required this.type,
    required this.labelKey,
    required this.enabled,
    this.blocked,
    required this.endpoint,
    required this.method,
    required this.requiresIdempotency,
    required this.financial,
    this.inputSchema,
    this.requiresInput,
    this.inputHint,
    this.inputType,
  });

  factory ActionResponseDto.fromJson(Map<String, dynamic> json) {
    return ActionResponseDto(
      type: json['type'] as String? ?? '',
      labelKey: json['label_key'] as String? ?? '',
      enabled: json['enabled'] as bool? ?? true,
      blocked: json['blocked'] != null
          ? ActionBlockedReasonDto.fromJson(
              json['blocked'] as Map<String, dynamic>,
            )
          : null,
      endpoint: json['endpoint'] as String? ?? '',
      method: json['method'] as String? ?? 'POST',
      requiresIdempotency: json['requires_idempotency'] as bool? ?? false,
      financial: json['financial'] as bool? ?? false,
      inputSchema: json['input_schema'] != null
          ? InputSchemaDto.fromJson(
              json['input_schema'] as Map<String, dynamic>,
            )
          : null,
      // Deprecated fields
      requiresInput: json['requires_input'] as bool?,
      inputHint: json['input_hint'] as String?,
      inputType: json['input_type'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
    'type': type,
    'label_key': labelKey,
    'enabled': enabled,
    if (blocked != null) 'blocked': blocked!.toJson(),
    'endpoint': endpoint,
    'method': method,
    'requires_idempotency': requiresIdempotency,
    'financial': financial,
    if (inputSchema != null) 'input_schema': inputSchema!.toJson(),
    if (requiresInput != null) 'requires_input': requiresInput,
    if (inputHint != null) 'input_hint': inputHint,
    if (inputType != null) 'input_type': inputType,
  };

  /// Convert to domain Action entity
  Action toDomain() {
    return Action(
      type: type,
      labelKey: labelKey,
      enabled: enabled,
      blocked: blocked?.toDomain(),
      endpoint: endpoint,
      method: method,
      requiresIdempotency: requiresIdempotency,
      financial: financial,
      inputSchema: inputSchema?.toDomain(),
      requiresInput: requiresInput,
      inputHint: inputHint,
      inputType: inputType,
    );
  }
}

/// Action Blocked Reason DTO - explains why an action is not available
class ActionBlockedReasonDto {
  final String action;
  final String messageKey;
  final String? reason;
  final String code;
  final String? resolutionAction;
  final String? resolutionLabel;

  const ActionBlockedReasonDto({
    required this.action,
    required this.messageKey,
    this.reason,
    required this.code,
    this.resolutionAction,
    this.resolutionLabel,
  });

  factory ActionBlockedReasonDto.fromJson(Map<String, dynamic> json) {
    return ActionBlockedReasonDto(
      action: json['action'] as String? ?? '',
      messageKey: json['message_key'] as String? ?? '',
      reason: json['reason'] as String?,
      code: json['code'] as String? ?? '',
      resolutionAction: json['resolution_action'] as String?,
      resolutionLabel: json['resolution_label'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
    'action': action,
    'message_key': messageKey,
    if (reason != null) 'reason': reason,
    'code': code,
    if (resolutionAction != null) 'resolution_action': resolutionAction,
    if (resolutionLabel != null) 'resolution_label': resolutionLabel,
  };

  /// Convert to domain ActionBlockedReason entity
  ActionBlockedReason toDomain() {
    return ActionBlockedReason(
      action: action,
      messageKey: messageKey,
      reason: reason,
      code: code,
      resolutionAction: resolutionAction,
      resolutionLabel: resolutionLabel,
    );
  }
}

/// Input Field Validation DTO - validation rules for an input field
class InputFieldValidationDto {
  final bool? required;
  final int? minLength;
  final int? maxLength;
  final int? min;
  final int? max;
  final String? pattern;
  final List<String>? options;

  const InputFieldValidationDto({
    this.required,
    this.minLength,
    this.maxLength,
    this.min,
    this.max,
    this.pattern,
    this.options,
  });

  factory InputFieldValidationDto.fromJson(Map<String, dynamic> json) {
    return InputFieldValidationDto(
      required: json['required'] as bool?,
      minLength: json['min_length'] as int?,
      maxLength: json['max_length'] as int?,
      min: json['min'] as int?,
      max: json['max'] as int?,
      pattern: json['pattern'] as String?,
      options: (json['options'] as List<dynamic>?)
          ?.map((e) => e.toString())
          .toList(),
    );
  }

  Map<String, dynamic> toJson() => {
    if (required != null) 'required': required,
    if (minLength != null) 'min_length': minLength,
    if (maxLength != null) 'max_length': maxLength,
    if (min != null) 'min': min,
    if (max != null) 'max': max,
    if (pattern != null) 'pattern': pattern,
    if (options != null) 'options': options,
  };

  /// Convert to domain InputFieldValidation entity
  InputFieldValidation toDomain() {
    return InputFieldValidation(
      required: required,
      minLength: minLength,
      maxLength: maxLength,
      min: min,
      max: max,
      pattern: pattern,
      options: options,
    );
  }
}

/// Input Field Definition DTO - defines a single input field in the schema
class InputFieldDefinitionDto {
  final String key;
  final String labelKey;
  final String type;
  final String? placeholder;
  final InputFieldValidationDto? validation;
  final dynamic defaultValue;

  const InputFieldDefinitionDto({
    required this.key,
    required this.labelKey,
    required this.type,
    this.placeholder,
    this.validation,
    this.defaultValue,
  });

  factory InputFieldDefinitionDto.fromJson(Map<String, dynamic> json) {
    return InputFieldDefinitionDto(
      key: json['key'] as String? ?? '',
      labelKey: json['label_key'] as String? ?? '',
      type: json['type'] as String? ?? 'text',
      placeholder: json['placeholder'] as String?,
      validation: json['validation'] != null
          ? InputFieldValidationDto.fromJson(
              json['validation'] as Map<String, dynamic>,
            )
          : null,
      defaultValue: json['default'],
    );
  }

  Map<String, dynamic> toJson() => {
    'key': key,
    'label_key': labelKey,
    'type': type,
    if (placeholder != null) 'placeholder': placeholder,
    if (validation != null) 'validation': validation!.toJson(),
    if (defaultValue != null) 'default': defaultValue,
  };

  /// Convert to domain InputFieldDefinition entity
  InputFieldDefinition toDomain() {
    return InputFieldDefinition(
      key: key,
      labelKey: labelKey,
      type: type,
      placeholder: placeholder,
      validation: validation?.toDomain(),
      defaultValue: defaultValue,
    );
  }
}

/// Input Schema DTO - defines the structured input for an action
class InputSchemaDto {
  final List<InputFieldDefinitionDto> fields;

  const InputSchemaDto({this.fields = const []});

  factory InputSchemaDto.fromJson(Map<String, dynamic> json) {
    return InputSchemaDto(
      fields:
          (json['fields'] as List<dynamic>?)
              ?.map(
                (e) =>
                    InputFieldDefinitionDto.fromJson(e as Map<String, dynamic>),
              )
              .toList() ??
          [],
    );
  }

  Map<String, dynamic> toJson() => {
    'fields': fields.map((f) => f.toJson()).toList(),
  };

  /// Convert to domain InputSchema entity
  InputSchema toDomain() {
    return InputSchema(fields: fields.map((f) => f.toDomain()).toList());
  }
}

/// Display Hints DTO from backend (NON-AUTHORITATIVE)
///
/// These are UI hints ONLY. Frontend MUST NOT derive state or
/// allowed_actions from these hints.
class DisplayHintsDto {
  final String? badge;
  final String? badgeVariant;
  final String? primaryAction;
  final String? warning;
  final String? info;
  final int? timeRemainingSeconds;

  const DisplayHintsDto({
    this.badge,
    this.badgeVariant,
    this.primaryAction,
    this.warning,
    this.info,
    this.timeRemainingSeconds,
  });

  factory DisplayHintsDto.fromJson(Map<String, dynamic> json) {
    return DisplayHintsDto(
      badge: json['badge'] as String?,
      badgeVariant: json['badge_variant'] as String?,
      primaryAction: json['primary_action'] as String?,
      warning: json['warning'] as String?,
      info: json['info'] as String?,
      timeRemainingSeconds: json['time_remaining_seconds'] as int?,
    );
  }

  Map<String, dynamic> toJson() => {
    'badge': badge,
    'badge_variant': badgeVariant,
    'primary_action': primaryAction,
    'warning': warning,
    'info': info,
    'time_remaining_seconds': timeRemainingSeconds,
  };
}

/// Shipping Address Response DTO
class ShippingAddressResponseDto {
  final String recipientName;
  final String phoneNumber;
  final String addressLine1;
  final String? addressLine2;
  final String? city;
  final String? province;
  final String? postalCode;

  const ShippingAddressResponseDto({
    required this.recipientName,
    required this.phoneNumber,
    required this.addressLine1,
    this.addressLine2,
    this.city,
    this.province,
    this.postalCode,
  });

  factory ShippingAddressResponseDto.fromJson(Map<String, dynamic> json) {
    return ShippingAddressResponseDto(
      recipientName: json['recipient_name'] as String? ?? '',
      phoneNumber:
          json['phone_number'] as String? ?? json['phone'] as String? ?? '',
      addressLine1:
          json['address_line1'] as String? ?? json['address'] as String? ?? '',
      addressLine2: json['address_line2'] as String?,
      city: json['city'] as String?,
      province: json['province'] as String?,
      postalCode: json['postal_code'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
    'recipient_name': recipientName,
    'phone_number': phoneNumber,
    'address_line1': addressLine1,
    if (addressLine2 != null) 'address_line2': addressLine2,
    if (city != null) 'city': city,
    if (province != null) 'province': province,
    if (postalCode != null) 'postal_code': postalCode,
  };
}

/// Product Summary DTO (nested in Order response)
///
/// Lightweight version of Product - only includes essential fields
/// that backend sends in Order response.
class ProductSummaryDto {
  final String id;
  final String title;
  final String? image;
  final List<String>? mediaUrls;
  final double? price;
  final String? variety;

  const ProductSummaryDto({
    required this.id,
    required this.title,
    this.image,
    this.mediaUrls,
    this.price,
    this.variety,
  });

  factory ProductSummaryDto.fromJson(Map<String, dynamic> json) {
    // Handle both single image and mediaUrls array
    String? singleImage = json['image'] as String?;
    List<String>? mediaUrls = (json['media_urls'] as List<dynamic>?)
        ?.map((e) => e.toString())
        .toList();

    // If image exists but mediaUrls doesn't, create single-element array
    if (singleImage != null && mediaUrls == null) {
      mediaUrls = [singleImage];
    }

    return ProductSummaryDto(
      id: json['id'] as String,
      title: json['title'] as String? ?? '',
      image: singleImage,
      mediaUrls: mediaUrls,
      price: (json['price'] as num?)?.toDouble(),
      variety: json['variety'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'title': title,
    if (image != null) 'image': image,
    if (mediaUrls != null) 'media_urls': mediaUrls,
    if (price != null) 'price': price,
    if (variety != null) 'variety': variety,
  };
}
