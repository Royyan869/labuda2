/// Pricing Preview DTO
///
/// Data transfer objects for pricing preview API
/// Backend endpoint: POST /api/v1/pricing/preview
library;

import 'package:labuda/domains/commerce/pricing/pricing_preview/domain/entities/pricing_snapshot.dart';

/// Pricing Preview Response DTO
///
/// Response from POST /api/v1/pricing/preview
class PricingPreviewResponseDto {
  final String token;
  final String expiresAt;
  final Map<String, dynamic> pricingSnapshot;

  PricingPreviewResponseDto({
    required this.token,
    required this.expiresAt,
    required this.pricingSnapshot,
  });

  factory PricingPreviewResponseDto.fromJson(Map<String, dynamic> json) {
    return PricingPreviewResponseDto(
      token: json['token'] as String,
      expiresAt: json['expires_at'] as String,
      pricingSnapshot: json['pricing_snapshot'] as Map<String, dynamic>,
    );
  }

  /// Convert to PricingSnapshot entity
  PricingSnapshot toEntity() {
    final snapshot = pricingSnapshot;

    // Parse shipping option if available
    ShippingOptionInfo? shippingOption;
    if (snapshot['shipping_option'] != null) {
      final shipping = snapshot['shipping_option'] as Map<String, dynamic>;
      shippingOption = ShippingOptionInfo(
        id: shipping['id']?.toString() ?? '',
        name: shipping['name']?.toString() ?? '',
        transportType: shipping['transport_type']?.toString() ?? '',
        expeditionName: shipping['expedition_name']?.toString() ?? '',
        estimatedDays: shipping['estimated_days'] as int? ?? 0,
      );
    }

    return PricingSnapshot(
      token: token,
      expiresAt: DateTime.parse(expiresAt),
      unitPrice: snapshot['unit_price'] as int? ?? 0,
      quantity: snapshot['quantity'] as int? ?? 1,
      subtotal: snapshot['subtotal'] as int? ?? 0,
      shippingTotal: snapshot['shipping_total'] as int? ?? 0,
      commissionPercent:
          (snapshot['commission_percent'] as num?)?.toDouble() ?? 0.0,
      commissionAmount: snapshot['commission_amount'] as int? ?? 0,
      discountAmount: snapshot['discount_amount'] as int? ?? 0,
      discountCode: snapshot['discount_code']?.toString(),
      discountType: snapshot['discount_type']?.toString(),
      discountValue: snapshot['discount_value']?.toString(),
      escrowAmount: snapshot['escrow_amount'] as int? ?? 0,
      shippingOption: shippingOption,
      addressId: snapshot['address_id']?.toString() ?? '',
      coinsAmount: snapshot['coins_amount'] as int? ?? 0,
      originalPrice: snapshot['original_price'] as int?,
      totalSavings: snapshot['total_savings'] as int?,
    );
  }
}

/// Pricing Preview Request DTO
///
/// Request body for POST /api/v1/pricing/preview
///
/// Backend requires: product_id, source_type, source_id (all binding:"required").
/// source_type = 'fixed_price_sale' | 'auction' | 'negotiation'
/// source_id   = FixedPriceSale ID | Auction ID | Negotiation ID
class PricingPreviewRequestDto {
  final String productId;
  final String sourceType;
  final String sourceId;
  final int quantity;
  final String? shippingOptionId;
  final String? shippingQuoteId;
  final String addressId;
  final String? discountCode;

  PricingPreviewRequestDto({
    required this.productId,
    required this.sourceType,
    required this.sourceId,
    required this.quantity,
    this.shippingOptionId,
    this.shippingQuoteId,
    required this.addressId,
    this.discountCode,
  });

  Map<String, dynamic> toJson() {
    return {
      'product_id': productId,
      'source_type': sourceType,
      'source_id': sourceId,
      'quantity': quantity,
      if (shippingOptionId != null) 'shipping_option_id': shippingOptionId,
      if (shippingQuoteId != null) 'shipping_quote_id': shippingQuoteId,
      'address_id': addressId,
      if (discountCode != null) 'discount_code': discountCode,
    };
  }
}

/// Negotiation Pricing Preview Request DTO
///
/// Specialized request for negotiation checkout.
/// Backend service (GenerateForNegotiation) derives product/price from the
/// accepted negotiation record — no product_id or source_type needed.
class NegotiationPricingPreviewRequestDto {
  final String negotiationId;
  final String? shippingOptionId;
  final String? shippingQuoteId;
  final String addressId;

  NegotiationPricingPreviewRequestDto({
    required this.negotiationId,
    this.shippingOptionId,
    this.shippingQuoteId,
    required this.addressId,
  });

  Map<String, dynamic> toJson() {
    return {
      'negotiation_id': negotiationId,
      if (shippingOptionId != null) 'shipping_option_id': shippingOptionId,
      if (shippingQuoteId != null) 'shipping_quote_id': shippingQuoteId,
      'address_id': addressId,
    };
  }
}
