/// Checkout Response Domain Entity
///
/// Represents the Order returned by POST /orders.
/// Backend returns a raw Order entity — NOT a payment URL.
/// Payment initiation is a SEPARATE step (POST /payments).
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

  /// Total escrow amount (backend-authoritative).
  final int escrowAmount;

  /// Coins applied at order creation (null / 0 = none). Display-only; server computes total.
  final int? coinsUsed;

  final DateTime createdAt;

  const CheckoutResponse({
    required this.orderId,
    this.orderNumber,
    required this.status,
    required this.subtotal,
    required this.shippingTotal,
    required this.commissionAmount,
    required this.escrowAmount,
    this.coinsUsed,
    required this.createdAt,
  });

  CheckoutResponse copyWith({
    String? orderId,
    String? orderNumber,
    String? status,
    int? subtotal,
    int? shippingTotal,
    int? commissionAmount,
    int? escrowAmount,
    int? coinsUsed,
    DateTime? createdAt,
  }) {
    return CheckoutResponse(
      orderId: orderId ?? this.orderId,
      orderNumber: orderNumber ?? this.orderNumber,
      status: status ?? this.status,
      subtotal: subtotal ?? this.subtotal,
      shippingTotal: shippingTotal ?? this.shippingTotal,
      commissionAmount: commissionAmount ?? this.commissionAmount,
      escrowAmount: escrowAmount ?? this.escrowAmount,
      coinsUsed: coinsUsed ?? this.coinsUsed,
      createdAt: createdAt ?? this.createdAt,
    );
  }

  @override
  List<Object?> get props => [
    orderId,
    orderNumber,
    status,
    subtotal,
    shippingTotal,
    commissionAmount,
    escrowAmount,
    coinsUsed,
    createdAt,
  ];
}
