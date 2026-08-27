import 'package:flutter/foundation.dart' show debugPrint;

import '../../domain/domain.dart';
import 'package:labuda/core/common/types/preparation_time.dart';
import '../dto/dto_barrel.dart';
import '../models/api/order_api_response_dtos.dart'
    show ActiveRefundApiResponse;

/// Order Mapper - converts between DTOs and Domain Entities
class OrderMapper {
  /// STAGE 3 — IDENTITY MAPPER SWITCH (Phase 5)
  /// Owner Truth identity scalars from Stage 1 backend payload are now
  /// populated on the Order entity:
  ///   sellerUsername  ← dto.sellerUsername
  ///   sellerFarmName  ← dto.sellerFarmName
  ///   sellerAvatarUrl ← dto.sellerAvatarUrl
  ///   buyerUsername   ← dto.buyerUsername
  /// All nullable; old payloads simply land null. No fake fallback
  /// (no Unknown / User / Buyer / Seller / Anonymous). No fullName
  /// fallback. UI consumption is deferred to Stage 4.
  static Order toOrder(OrderDto dto) {
    return Order(
      id: dto.id,
      buyerId: dto.buyerId,
      sellerId: dto.sellerId,
      items: _buildOrderItems(dto),
      status: _mapOrderStatus(dto.status),
      // NOTE: primaryStatus removed - redundant (always duplicated status)
      paymentMethod: PaymentMethodType.bankTransfer,
      paymentStatus: _mapPaymentStatus(dto.paymentStatus),
      shippingInfo: _buildShippingInfo(dto),
      pricing: _buildOrderPricing(dto),
      notes: dto.notes,
      preparationTimeSnapshot: _mapPreparationTime(dto.preparationTimeSnapshot),
      preparationNoteSnapshot: dto.preparationNoteSnapshot,
      readyToShipBy: dto.readyToShipBy,
      createdAt: dto.createdAt,
      paidAt: dto.confirmedAt,
      shippedAt: dto.shippedAt,
      completedAt: dto.completedAt,
      cancelledAt: dto.cancelledAt,
      acceptanceDeadline: dto.sellerAcceptDeadline,
      source: OrderSource.forSale,
      sourceId: dto.sourceId,
      hasActiveRefund: dto.hasActiveRefund,
      activeRefund: _mapActiveRefund(dto.activeRefund),
      // Stage 3 identity fields (Owner Truth: username/farmName/avatar).
      sellerUsername: dto.sellerUsername,
      sellerFarmName: dto.sellerFarmName,
      sellerAvatarUrl: dto.sellerAvatarUrl,
      buyerUsername: dto.buyerUsername,
      paymentId: dto.paymentId,
    );
  }

  static List<Order> toOrderList(List<OrderDto> dtos) {
    return dtos.map(toOrder).toList();
  }

  static CreateOrderDto toCreateOrderDto({
    required String productId,
    required int quantity,
    required ShippingInfo shippingInfo,
    String? discountCode,
    bool? useCoins,
    String? notes,
    required String pricingToken,
    String? auctionId,
    String? negotiationId,
  }) {
    return CreateOrderDto(
      productId: productId,
      quantity: quantity,
      discountCode: discountCode,
      useCoins: useCoins,
      notes: notes,
      shippingAddress: ShippingAddressRequestDto(
        recipientName: shippingInfo.recipientName,
        phoneNumber: shippingInfo.phone,
        addressLine1: shippingInfo.address,
        city: shippingInfo.cityName,
        province: shippingInfo.provinceName,
        postalCode: shippingInfo.postalCode,
      ),
      pricingToken: pricingToken,
      sourceType: auctionId != null ? 'auction' : 'fixed_price_sale',
      sourceId: auctionId ?? productId,
      auctionId: auctionId,
      negotiationId: negotiationId,
    );
  }

  static List<OrderItem> _buildOrderItems(OrderDto dto) {
    final product = dto.product;
    if (product == null) {
      return [
        OrderItem(
          id: dto.productId,
          productId: dto.productId,
          listingName: 'Item',
          listingImage: '',
          price: dto.totalAmount,
          quantity: dto.quantity,
        ),
      ];
    }

    return [
      OrderItem(
        id: product.id,
        productId: product.id,
        listingName: product.title,
        listingImage: product.imageUrl ?? '',
        price: product.price,
        quantity: dto.quantity,
      ),
    ];
  }

  static ShippingInfo _buildShippingInfo(OrderDto dto) {
    final addr = dto.shippingAddress;
    if (addr == null) {
      return ShippingInfo(
        recipientName: '',
        phone: '',
        address: '',
        method: ShippingMethod.courier,
        shippingCost: dto.shippingFee,
        // SHIPPING CONFIRMATION TRUTH: Map shipping reference fields
        trackingNumber: dto.shippingReference,
        referenceType: dto.referenceType,
        shippingNote: dto.shippingNote,
        courierName: null,
      );
    }

    return ShippingInfo(
      recipientName: addr.recipientName,
      phone: addr.phoneNumber,
      address: addr.fullAddress.isNotEmpty
          ? addr.fullAddress
          : addr.addressLine1,
      cityName: addr.city,
      provinceName: addr.province,
      postalCode: addr.postalCode,
      method: ShippingMethod.courier,
      shippingCost: dto.shippingFee,
      // SHIPPING CONFIRMATION TRUTH: Map shipping reference fields
      trackingNumber: dto.shippingReference,
      referenceType: dto.referenceType,
      shippingNote: dto.shippingNote,
      courierName: null,
    );
  }

  static OrderPricing _buildOrderPricing(OrderDto dto) {
    final subtotal = dto.totalAmount;
    final shippingCost = dto.shippingFee;

    // BACKEND OWNERSHIP: Use backend-provided discountAmount directly.
    // Mobile does NOT derive or combine discount values.
    // NOTE: dto.coinDiscount exists but is not added to discount field here.
    // The backend sends separate discount components (discountAmount, coinDiscount)
    // but OrderPricing entity has a single discount field.
    // This maps discountAmount conservatively - backend is source of truth.
    final discount = dto.discountAmount;

    final total = dto.finalAmount;

    // Financial values (adminFee, paymentFee) MUST come from backend
    // sellerCommission and sellerEarnings REMOVED (Wave 3.1B) - seller financial data
    // belongs in finance-derived sources, not Order domain
    return OrderPricing(
      subtotal: subtotal,
      shippingCost: shippingCost,
      serviceFeeAmount: dto.serviceFeeAmount,
      adminFee: null, // Backend must provide
      paymentFee: null, // Backend must provide
      discount: discount,
      total: total,
      totalPayableAmount: dto.totalPayableAmount,
    );
  }

  /// Map backend order status string to OrderStatus enum.
  ///
  /// FAILS LOUDLY: Throws FormatException for unknown statuses.
  /// Unknown order statuses indicate a contract mismatch between
  /// frontend and backend - this should be surfaced immediately.
  static OrderStatus _mapOrderStatus(String status) {
    switch (status.toLowerCase()) {
      case 'pending':
      case 'pending_payment': // Backend canonical wire value (StatusPending = "pending_payment")
        return OrderStatus.pending;
      case 'waiting_payment':
      case 'waitingpayment':
        return OrderStatus.pending;
      case 'paid':
        return OrderStatus
            .paid; // Backend 'paid' → frontend OrderStatus.paid (P11 aligned)
      case 'confirmed':
        // Legacy: old frontend 'confirmed' → now mapped to paid (P11 migration)
        return OrderStatus.paid;
      // O1: processing was removed - not a real backend status, map to paid for safety
      case 'processing':
        return OrderStatus.paid;
      case 'shipped':
        return OrderStatus.shipped;
      case 'delivered':
        return OrderStatus.delivered;
      case 'completed':
        return OrderStatus.completed;
      case 'cancelled':
        return OrderStatus.cancelled;
      case 'cancelled_timeout':
      case 'cancelledtimeout':
        return OrderStatus.cancelledTimeout;
      case 'refunded':
        return OrderStatus.refunded;
      case 'disputed':
        return OrderStatus.pending;
      // O1: Added expired status from backend
      case 'expired':
        return OrderStatus.expired;
      case 'dispute_open':
      case 'disputeopen':
        return OrderStatus.disputeOpen;
      case 'partially_refunded':
      case 'partiallyrefunded':
        return OrderStatus.partiallyRefunded;
      default:
        throw FormatException(
          'Unknown order status: "$status". '
          'Frontend enum does not contain this value. '
          'Backend may have added a new status.',
        );
    }
  }

  static String mapOrderStatusToString(OrderStatus status) {
    switch (status) {
      case OrderStatus.pending:
        return 'pending';
      case OrderStatus.paid:
        return 'paid'; // Frontend OrderStatus.paid → backend 'paid' (P11 aligned)
      case OrderStatus.shipped:
        return 'shipped';
      case OrderStatus.delivered:
        return 'delivered';
      case OrderStatus.completed:
        return 'completed';
      case OrderStatus.cancelled:
        return 'cancelled';
      case OrderStatus.cancelledTimeout:
        return 'cancelled_timeout';
      case OrderStatus.refunded:
        return 'refunded';
      case OrderStatus.disputeOpen:
        return 'dispute_open';
      case OrderStatus.partiallyRefunded:
        return 'partially_refunded';
      case OrderStatus.expired:
        return 'expired';
    }
  }

  /// Map backend payment status string to PaymentStatus enum.
  ///
  /// TOLERANT: Returns PaymentStatus.pending for absent/empty/unknown values
  /// instead of throwing. This prevents order screens from crashing when the
  /// backend does not yet populate payment_status (e.g. no payment record exists).
  ///
  /// Known gateway statuses:
  ///   settlement / capture  → paid
  ///   pending               → pending
  ///   failed                → failed
  ///   cancelled / expired   → expired
  ///   refunded              → refunded
  ///   challenge             → processing (gateway hold)
  ///   absent / empty / unknown → pending (safe fallback, logged in debug builds)
  static PaymentStatus _mapPaymentStatus(String status) {
    switch (status.toLowerCase()) {
      case 'pending':
      case '':
        return PaymentStatus.pending;
      case 'paid':
      case 'success':
      case 'settlement':
      case 'capture':
        return PaymentStatus.paid;
      case 'failed':
        return PaymentStatus.failed;
      case 'cancelled':
      case 'expired':
        return PaymentStatus.expired;
      case 'refunded':
        return PaymentStatus.refunded;
      case 'challenge':
        return PaymentStatus.processing;
      default:
        // Unknown status from backend — degrade gracefully rather than crashing.
        // This preserves order screen usability when gateway adds new status values.
        debugPrint(
          'OrderMapper._mapPaymentStatus: unknown payment status "$status" — '
          'add a case when backend introduces new payment statuses.',
        );
        return PaymentStatus.pending;
    }
  }

  static String mapPaymentStatusToString(PaymentStatus status) {
    switch (status) {
      case PaymentStatus.pending:
        return 'pending';
      case PaymentStatus.paid:
        return 'paid';
      case PaymentStatus.failed:
        return 'failed';
      case PaymentStatus.expired:
        return 'expired';
      case PaymentStatus.refunded:
        return 'refunded';
      case PaymentStatus.processing:
        return 'processing';
    }
  }

  /// Map preparation_time_snapshot string to PreparationTime enum
  /// Defaults to 'immediate' for null/unknown values (safe default)
  static PreparationTime _mapPreparationTime(String? preparationTime) {
    return PreparationTime.fromJson(preparationTime);
  }

  static RefundRequest? _mapActiveRefund(ActiveRefundApiResponse? refund) {
    if (refund == null) return null;
    return RefundRequest(
      id: refund.id,
      orderId: refund.orderId,
      buyerId: refund.buyerId,
      sellerId: refund.sellerId,
      reason: _mapRefundReason(refund.reason),
      description: refund.description,
      evidenceUrls: refund.evidenceUrls,
      status: _mapRefundStatus(refund.status),
      refundAmount: refund.requestedAmount,
      sellerNotes: refund.sellerNotes,
      adminNotes: refund.adminNotes,
      createdAt: refund.createdAt,
      approvedAt: null,
      rejectedAt: null,
      refundedAt: null,
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
      case 'seller_rejected':
        return RefundStatus.sellerRejected;
      case 'escalated_to_admin':
      case 'escalated':
        return RefundStatus.escalatedToAdmin;
      case 'admin_refunded':
      case 'admin_approved':
        return RefundStatus.adminApproved;
      case 'admin_released':
      case 'rejected':
        return RefundStatus.rejected;
      case 'refunded':
        return RefundStatus.refunded;
      default:
        return RefundStatus.pendingSellerReview;
    }
  }

  // Convert OrderStatsApiResponse to OrderStats (domain entity)
  static OrderStats toOrderStats(OrderStatsDto dto) {
    return OrderStats(
      totalOrders: dto.totalOrders,
      pendingOrders: dto.pendingOrders,
      completedOrders: dto.completedOrders,
      cancelledOrders: dto.cancelledOrders,
      totalRevenue: dto.totalRevenue,
    );
  }

  // Public version of _mapPaymentStatus for external use
  static PaymentStatus mapPaymentStatus(String status) {
    return _mapPaymentStatus(status);
  }
}
