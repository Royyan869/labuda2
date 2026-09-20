/// Order Parameter Types
///
/// Parameter types for the canonical Order repository operations
/// exercised by the buyer/seller order flow.
library;

import 'order_pricing.dart';
import 'order_status.dart';
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

  /// Source type for pricing preview: 'for_sale' | 'auction' | 'negotiation'
  /// Required by backend GeneratePreviewRequest (binding:"required")
  final String? sourceType;

  /// Source ID: FixedPriceSale UUID | Auction UUID | Negotiation UUID
  /// Required by backend GeneratePreviewRequest (binding:"required")
  final String? sourceId;

  /// ID of the seller's manual shipping quote
  /// When provided, the preview will use the seller's quoted shipping price
  final String? shippingQuoteId;

  /// Standard shipping option ID selected by buyer from forSale options.
  /// Mutually exclusive with shippingQuoteId — backend requires exactly one.
  final String? shippingSetupId;

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
    this.shippingSetupId,
  });
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
///   - "standard": Standard forSale shipping options (user selects shipping)
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
  // - "standard": Standard forSale shipping options (user selects shipping)
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

  // Convenience getters for pricing fields.
  // The legacy `total` alias was purged: the canonical buyer payable is
  // `totalPayableAmount` (PD + S + F) as emitted by the backend preview.
  double get subtotal => pricing.subtotal;
  double get shippingCost => pricing.shippingCost;
  double get commissionAmount => pricing.commissionAmount;
  double? get serviceFeeAmount => pricing.serviceFeeAmount;
  double? get totalPayableAmount => pricing.totalPayableAmount;
  double? get totalBeforeCoinsAmount => pricing.totalBeforeCoinsAmount;

  /// Returns true if this preview uses a shipping quote (fixed price)
  /// instead of standard shipping options
  bool get isUsingShippingQuote => shippingMode == 'quote';
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
    if (endDate != null) {
      params['end_date'] = endDate!.toIso8601String();
    }
    if (page != null) params['page'] = page;
    if (pageSize != null) params['page_size'] = pageSize;
    if (limit != null) params['limit'] = limit;
    if (searchQuery != null) params['search'] = searchQuery;
    return params;
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

/// Parameters for forSale refunds
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

// ==================== PRICING CALCULATION ====================
// INVALID TYPE REMOVED: CalculatePricingParams
// INVALID TYPE REMOVED: PricingItem
// These types were proven invalid in the truth audit.
