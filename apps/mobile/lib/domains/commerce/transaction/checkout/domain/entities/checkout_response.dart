/// Checkout Response Domain Entity
///
/// Represents the created order returned by POST /orders.
/// Backend returns the lightweight OrderCreateResponse DTO (id, order_number,
/// status, canonical pricing snapshot, created_at) — NOT a raw Order entity and
/// NOT a payment URL. Payment initiation is a SEPARATE step (POST /payments).
library;

import 'package:equatable/equatable.dart';

/// Response from POST /orders — represents the created Order.
///
/// The 2-step checkout contract:
///   1. POST /orders  → returns this (Order with id, status, pricing snapshot)
///   2. POST /payments → returns PaymentIntent with payment_url
class CheckoutResponse extends Equatable {
  /// Order UUID from backend
  final String orderId;

  /// Human-readable order number (e.g., ORD-20260519-ABCD1234)
  final String? orderNumber;

  /// Order status — should be "pending_payment" after creation
  final String status;

  /// Pricing snapshot frozen at order creation
  final int subtotal;
  final int shippingTotal;
  final int commissionAmount;

  /// Canonical buyer-funded base PD+S (backend-authoritative total_before_coins_amount).
  final int totalBeforeCoinsAmount;

  // NOTE: the legacy `coinsUsed` field was purged. Coins are NOT an Order
  // snapshot authority (the canonical coin authority lives in the coins
  // domain) and POST /orders does not emit `coins_used`, so the field could
  // only ever hold a hardcoded null.

  final DateTime createdAt;

  const CheckoutResponse({
    required this.orderId,
    this.orderNumber,
    required this.status,
    required this.subtotal,
    required this.shippingTotal,
    required this.commissionAmount,
    required this.totalBeforeCoinsAmount,
    required this.createdAt,
  });

  // NOTE: the unused `copyWith` was purged. This entity is the immutable
  // projection of the backend's `POST /orders` response; nothing may rewrite a
  // created order's snapshot client side.

  @override
  List<Object?> get props => [
    orderId,
    orderNumber,
    status,
    subtotal,
    shippingTotal,
    commissionAmount,
    totalBeforeCoinsAmount,
    createdAt,
  ];
}
