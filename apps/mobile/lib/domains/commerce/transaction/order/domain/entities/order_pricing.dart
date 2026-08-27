import 'package:equatable/equatable.dart';

/// ============================================
/// DISPLAY ONLY - READ-ONLY DATA CLASS
/// ============================================
/// Backend Authority - Do NOT calculate pricing on client
///
/// ALL pricing values MUST come from backend API response.
/// This class is purely a projection layer for displaying backend-calculated values.
///
/// TRACK 10 - Client-side calculation REMOVED
/// - No fee calculation on client
/// - No total calculation on client
/// - No payout calculation on client
///
/// FINANCIAL OWNERSHIP BOUNDARY (Wave 3.1B):
/// - sellerCommission and sellerEarnings REMOVED - these are seller financial data
/// - Seller financial UI must use finance-derived sources (SellerEarnings, SellerDashboardStats)
///
/// Source: OrderResponseDto.fromBackend() → Order entity
/// ============================================
///
/// Order Pricing Breakdown
class OrderPricing extends Equatable {
  final double subtotal;
  final double shippingCost;
  final double? serviceFeeAmount;
  final double? adminFee;
  final double? paymentFee;
  final String? paymentMethodKey;
  final double discount;
  final double total;
  final double? totalPayableAmount;
  final String? discountCode;
  final String? discountDescription;

  const OrderPricing({
    required this.subtotal,
    required this.shippingCost,
    this.serviceFeeAmount,
    this.adminFee,
    this.paymentFee,
    this.paymentMethodKey,
    required this.discount,
    required this.total,
    this.totalPayableAmount,
    this.discountCode,
    this.discountDescription,
  });

  factory OrderPricing.fromBreakdown({
    required double subtotal,
    required double shippingCost,
    double? serviceFeeAmount,
    double? adminFee,
    double? paymentFee,
    String? paymentMethodKey,
    required double total,
    double? totalPayableAmount,
    double discount = 0,
    String? discountCode,
    String? discountDescription,
  }) {
    return OrderPricing(
      subtotal: subtotal,
      shippingCost: shippingCost,
      serviceFeeAmount: serviceFeeAmount,
      adminFee: adminFee,
      paymentFee: paymentFee,
      paymentMethodKey: paymentMethodKey,
      discount: discount,
      total: total,
      totalPayableAmount: totalPayableAmount,
      discountCode: discountCode,
      discountDescription: discountDescription,
    );
  }

  /// Alias for `total` - safe to use (no calculation)
  double get buyerTotal => total;

  @override
  List<Object?> get props => [
    subtotal,
    shippingCost,
    serviceFeeAmount,
    adminFee,
    paymentFee,
    paymentMethodKey,
    discount,
    total,
    totalPayableAmount,
    discountCode,
    discountDescription,
  ];

  OrderPricing copyWith({
    double? subtotal,
    double? shippingCost,
    double? serviceFeeAmount,
    double? adminFee,
    double? paymentFee,
    String? paymentMethodKey,
    double? discount,
    double? total,
    double? totalPayableAmount,
    String? discountCode,
    String? discountDescription,
  }) {
    return OrderPricing(
      subtotal: subtotal ?? this.subtotal,
      shippingCost: shippingCost ?? this.shippingCost,
      serviceFeeAmount: serviceFeeAmount ?? this.serviceFeeAmount,
      adminFee: adminFee ?? this.adminFee,
      paymentFee: paymentFee ?? this.paymentFee,
      paymentMethodKey: paymentMethodKey ?? this.paymentMethodKey,
      discount: discount ?? this.discount,
      total: total ?? this.total,
      totalPayableAmount: totalPayableAmount ?? this.totalPayableAmount,
      discountCode: discountCode ?? this.discountCode,
      discountDescription: discountDescription ?? this.discountDescription,
    );
  }
}
