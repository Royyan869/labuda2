/// Pricing Snapshot Entity — STRICT MODE READ ONLY
/// Backend source: POST /api/v1/pricing/preview
library;

import 'package:equatable/equatable.dart';

/// Pricing Snapshot — complete pricing breakdown from backend.
///
/// Phase 2E-C3 (2026-06-21): removed stale listingId field — backend never returns it.
class PricingSnapshot extends Equatable {
  final String token;
  final DateTime expiresAt;
  final int unitPrice;
  final int quantity;
  final int subtotal;
  final int shippingTotal;
  final double commissionPercent;
  final int commissionAmount;
  final int discountAmount;
  final String? discountCode;
  final String? discountType;
  final String? discountValue;
  final int escrowAmount;
  final ShippingOptionInfo? shippingOption;
  final String addressId;
  final int coinsAmount;
  final int? originalPrice;
  final int? totalSavings;

  int get totalAmount => escrowAmount;
  bool get hasDiscount => discountAmount > 0;
  bool get hasCoins => coinsAmount > 0;
  bool get isNegotiated =>
      originalPrice != null && originalPrice! > escrowAmount;
  bool get isExpired => DateTime.now().isAfter(expiresAt);
  Duration get timeUntilExpiry =>
      isExpired ? Duration.zero : expiresAt.difference(DateTime.now());

  const PricingSnapshot({
    required this.token,
    required this.expiresAt,
    required this.unitPrice,
    required this.quantity,
    required this.subtotal,
    required this.shippingTotal,
    required this.commissionPercent,
    required this.commissionAmount,
    required this.discountAmount,
    this.discountCode,
    this.discountType,
    this.discountValue,
    required this.escrowAmount,
    this.shippingOption,
    required this.addressId,
    this.coinsAmount = 0,
    this.originalPrice,
    this.totalSavings,
  });

  @override
  List<Object?> get props => [
    token,
    expiresAt,
    unitPrice,
    quantity,
    subtotal,
    shippingTotal,
    commissionPercent,
    commissionAmount,
    discountAmount,
    discountCode,
    discountType,
    discountValue,
    escrowAmount,
    shippingOption,
    addressId,
    coinsAmount,
    originalPrice,
    totalSavings,
  ];

  PricingSnapshot copyWith({
    String? token,
    DateTime? expiresAt,
    int? unitPrice,
    int? quantity,
    int? subtotal,
    int? shippingTotal,
    double? commissionPercent,
    int? commissionAmount,
    int? discountAmount,
    String? discountCode,
    String? discountType,
    String? discountValue,
    int? escrowAmount,
    ShippingOptionInfo? shippingOption,
    String? addressId,
    int? coinsAmount,
    int? originalPrice,
    int? totalSavings,
  }) {
    return PricingSnapshot(
      token: token ?? this.token,
      expiresAt: expiresAt ?? this.expiresAt,
      unitPrice: unitPrice ?? this.unitPrice,
      quantity: quantity ?? this.quantity,
      subtotal: subtotal ?? this.subtotal,
      shippingTotal: shippingTotal ?? this.shippingTotal,
      commissionPercent: commissionPercent ?? this.commissionPercent,
      commissionAmount: commissionAmount ?? this.commissionAmount,
      discountAmount: discountAmount ?? this.discountAmount,
      discountCode: discountCode ?? this.discountCode,
      discountType: discountType ?? this.discountType,
      discountValue: discountValue ?? this.discountValue,
      escrowAmount: escrowAmount ?? this.escrowAmount,
      shippingOption: shippingOption ?? this.shippingOption,
      addressId: addressId ?? this.addressId,
      coinsAmount: coinsAmount ?? this.coinsAmount,
      originalPrice: originalPrice ?? this.originalPrice,
      totalSavings: totalSavings ?? this.totalSavings,
    );
  }
}

class ShippingOptionInfo extends Equatable {
  final String id;
  final String name;
  final String transportType;
  final String expeditionName;
  final int estimatedDays;

  const ShippingOptionInfo({
    required this.id,
    required this.name,
    required this.transportType,
    required this.expeditionName,
    required this.estimatedDays,
  });

  @override
  List<Object?> get props => [
    id,
    name,
    transportType,
    expeditionName,
    estimatedDays,
  ];
}
