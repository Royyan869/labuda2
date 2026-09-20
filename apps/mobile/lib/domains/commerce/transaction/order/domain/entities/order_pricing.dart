import 'package:equatable/equatable.dart';

/// ============================================
/// CANONICAL ORDER PRICING — DISPLAY ONLY
/// ============================================
/// Backend Authority - Do NOT calculate pricing on client
///
/// CANONICAL MONEY MODEL:
///   P  = subtotal (unit_price × quantity, before seller discount)
///   PD = discounted product amount (derived: totalBeforeCoinsAmount - shippingCost)
///   S  = shippingCost (shipping_total)
///   C  = commissionAmount (seller-side, platform commission)
///   F  = serviceFeeAmount (buyer-side, payment service fee)
///
/// CANONICAL BASES:
///   totalBeforeCoinsAmount = PD + S  (canonical buyer-funded base)
///   totalPayableAmount     = PD + S + F  (buyer's gross payable after payment method selection)
///
/// Source: OrderMapper._buildOrderPricing() → Order entity
/// ============================================

class OrderPricing extends Equatable {
  final double subtotal;
  final double shippingCost;
  final double commissionAmount;
  final double? serviceFeeAmount;
  final double? totalPayableAmount;
  final double? totalBeforeCoinsAmount;

  const OrderPricing({
    required this.subtotal,
    required this.shippingCost,
    this.commissionAmount = 0,
    this.serviceFeeAmount,
    this.totalPayableAmount,
    this.totalBeforeCoinsAmount,
  });

  factory OrderPricing.fromBreakdown({
    required double subtotal,
    required double shippingCost,
    double commissionAmount = 0,
    double? serviceFeeAmount,
    double? totalPayableAmount,
    double? totalBeforeCoinsAmount,
  }) {
    return OrderPricing(
      subtotal: subtotal,
      shippingCost: shippingCost,
      commissionAmount: commissionAmount,
      serviceFeeAmount: serviceFeeAmount,
      totalPayableAmount: totalPayableAmount,
      totalBeforeCoinsAmount: totalBeforeCoinsAmount,
    );
  }

  // NO DERIVED MONEY GETTERS.
  // Money is backend authority: the canonical sources are the persisted fields
  // above (`totalBeforeCoinsAmount` = PD + S, `totalPayableAmount` = PD + S + F).
  // Consumers must read those fields directly — re-deriving a buyer base
  // (`subtotal + shippingCost`) or a payable total on the client is forbidden —
  // and must fail closed when the backend has not emitted them.

  @override
  List<Object?> get props => [
    subtotal,
    shippingCost,
    commissionAmount,
    serviceFeeAmount,
    totalPayableAmount,
    totalBeforeCoinsAmount,
  ];

  OrderPricing copyWith({
    double? subtotal,
    double? shippingCost,
    double? commissionAmount,
    double? serviceFeeAmount,
    double? totalPayableAmount,
    double? totalBeforeCoinsAmount,
  }) {
    return OrderPricing(
      subtotal: subtotal ?? this.subtotal,
      shippingCost: shippingCost ?? this.shippingCost,
      commissionAmount: commissionAmount ?? this.commissionAmount,
      serviceFeeAmount: serviceFeeAmount ?? this.serviceFeeAmount,
      totalPayableAmount: totalPayableAmount ?? this.totalPayableAmount,
      totalBeforeCoinsAmount: totalBeforeCoinsAmount ?? this.totalBeforeCoinsAmount,
    );
  }
}
