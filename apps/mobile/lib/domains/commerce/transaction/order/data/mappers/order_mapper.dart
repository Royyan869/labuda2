import 'package:flutter/foundation.dart' show debugPrint;

import '../../domain/domain.dart';
import 'package:labuda/core/common/types/preparation_time.dart';
import '../models/api/order_api_response_dtos.dart'
    show OrderApiResponse, ActiveRefundApiResponse;

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
  static Order toOrder(OrderApiResponse dto) {
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
      notes: dto.buyerNotes,
      preparationTimeSnapshot: _mapPreparationTime(dto.preparationTimeSnapshot),
      preparationNoteSnapshot: dto.preparationNoteSnapshot,
      readyToShipBy: dto.readyToShipBy,
      createdAt: dto.createdAt,
      paidAt: null, // confirmed_at removed from canonical contract
      shippedAt: null, // no shipped timestamp exists on the canonical contract
      completedAt: dto.completedAt,
      cancelledAt: null, // cancelled_at removed from canonical contract
      acceptanceDeadline: null, // seller_accept_deadline removed from canonical contract
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

  static List<Order> toOrderList(List<OrderApiResponse> dtos) {
    return dtos.map(toOrder).toList();
  }

  /// CANONICAL line items — backend key `items` (OrderItemDTO[]).
  ///
  /// No placeholder item is invented and no `product` summary fallback is
  /// honored: that key does not exist on the order contract, so the legacy
  /// parse (and the `productId` field it read) were purged. The backend order
  /// payload carries no product image, so `forSaleImage` stays empty rather
  /// than being filled with an invented URL.
  static List<OrderItem> _buildOrderItems(OrderApiResponse dto) {
    return dto.items
        .map(
          (item) => OrderItem(
            id: item.id,
            productId: item.productId,
            forSaleName: item.name,
            forSaleImage: '',
            price: item.unitPrice,
            quantity: item.quantity,
          ),
        )
        .toList();
  }

  /// CANONICAL address mapping — backend key `shipping_address`, which is the
  /// immutable `orders.address_snapshot` (AddressSnapshot) frozen at creation.
  /// Every component is mapped field-for-field; the client never re-derives or
  /// invents an address part.
  static ShippingInfo _buildShippingInfo(OrderApiResponse dto) {
    final addr = dto.shippingAddress;
    return ShippingInfo(
      recipientName: addr?.recipientName ?? '',
      phone: addr?.phone ?? '',
      address: addr?.streetAddress ?? '',
      provinceId: addr?.provinceId,
      provinceName: addr?.provinceName,
      cityId: addr?.cityId,
      cityName: addr?.cityName,
      districtId: addr?.districtId,
      districtName: addr?.districtName,
      villageId: addr?.villageId,
      villageName: addr?.villageName,
      postalCode: addr?.postalCode,
      latitude: addr?.latitude,
      longitude: addr?.longitude,
      method: ShippingMethod.courier,
      shippingCost: dto.shippingTotal,
      // SHIPPING CONFIRMATION TRUTH: canonical tracking reference fields
      trackingNumber: dto.trackingNumber,
      referenceType: _mapProofType(dto.proofType),
      shippingNote: dto.shippingNote,
      courierName: null,
    );
  }

  /// Translate the backend shipping-proof vocabulary
  /// ("tracking" | "phone" | "manual") into the UI labeling vocabulary
  /// ("tracking" | "phone" | "other"). A "manual" reference has no courier
  /// resi, so it is surfaced as "other" instead of being mislabeled as a
  /// tracking number.
  static String? _mapProofType(String? proofType) {
    switch (proofType) {
      case 'tracking':
        return 'tracking';
      case 'phone':
        return 'phone';
      case 'manual':
        return 'other';
      default:
        return null;
    }
  }

  static OrderPricing _buildOrderPricing(OrderApiResponse dto) {
    // CANONICAL: Backend emits subtotal (P), shipping_total (S),
    // commission_amount (C), service_fee_amount (F),
    // total_payable_amount (PD+S+F), total_before_coins_amount (PD+S).
    return OrderPricing(
      subtotal: dto.subtotal,
      shippingCost: dto.shippingTotal,
      commissionAmount: dto.commissionAmount,
      serviceFeeAmount: dto.serviceFeeAmount,
      totalPayableAmount: dto.totalPayableAmount,
      totalBeforeCoinsAmount: dto.totalBeforeCoinsAmount,
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
      // Canonical backend value for a platform-initiated refund.
      case 'system_refunded':
        return RefundStatus.refunded;
      default:
        return RefundStatus.pendingSellerReview;
    }
  }

  // Public version of _mapPaymentStatus for external use
  static PaymentStatus mapPaymentStatus(String status) {
    return _mapPaymentStatus(status);
  }
}
