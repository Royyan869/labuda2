/// Shipping Quote DTOs
///
/// Data Transfer Objects for Shipping Quote API integration.
/// Backend API: /api/v1/chat/:chat_id/shipping-quote
library;

import 'package:equatable/equatable.dart';

// =============================================================================
// Response DTOs
// =============================================================================

/// Shipping quote response from Go backend.
///
/// Backend API: GET /api/v1/shipping-quote/:id
class ShippingQuoteResponseDto extends Equatable {
  final String id;
  final String chatId;
  final String productId;
  final String sourceType;
  final String sourceId;
  final String? auctionId;
  final String sellerId;
  final String buyerId;
  final int cost;
  final String? note;
  final String status;
  final String? destinationCityId;
  final String? destinationProvinceId;
  final String? expiresAt;
  final String? usedAt;
  final String createdAt;

  const ShippingQuoteResponseDto({
    required this.id,
    required this.chatId,
    required this.productId,
    required this.sourceType,
    required this.sourceId,
    this.auctionId,
    required this.sellerId,
    required this.buyerId,
    required this.cost,
    this.note,
    required this.status,
    this.destinationCityId,
    this.destinationProvinceId,
    this.expiresAt,
    this.usedAt,
    required this.createdAt,
  });

  factory ShippingQuoteResponseDto.fromJson(Map<String, dynamic> json) {
    return ShippingQuoteResponseDto(
      id: json['id'] as String,
      chatId: json['chat_id'] as String,
      productId: json['product_id'] as String,
      sourceType: json['source_type'] as String,
      sourceId: json['source_id'] as String,
      auctionId: json['auction_id'] as String?,
      sellerId: json['seller_id'] as String,
      buyerId: json['buyer_id'] as String,
      cost: json['cost'] as int,
      note: json['note'] as String?,
      status: json['status'] as String? ?? 'ACTIVE',
      destinationCityId: json['destination_city_id'] as String?,
      destinationProvinceId: json['destination_province_id'] as String?,
      expiresAt: json['expires_at'] as String?,
      usedAt: json['used_at'] as String?,
      createdAt: json['created_at'] as String,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'id': id,
      'chat_id': chatId,
      'product_id': productId,
      'source_type': sourceType,
      'source_id': sourceId,
      if (auctionId != null) 'auction_id': auctionId,
      'seller_id': sellerId,
      'buyer_id': buyerId,
      'cost': cost,
      'note': note,
      'status': status,
      if (destinationCityId != null) 'destination_city_id': destinationCityId,
      if (destinationProvinceId != null)
        'destination_province_id': destinationProvinceId,
      if (expiresAt != null) 'expires_at': expiresAt,
      if (usedAt != null) 'used_at': usedAt,
      'created_at': createdAt,
    };
  }

  bool get isActive => status == 'ACTIVE';

  DateTime? get createdAtDateTime {
    try {
      return DateTime.parse(createdAt);
    } catch (_) {
      return null;
    }
  }

  DateTime? get expiresAtDateTime {
    if (expiresAt == null) return null;
    try {
      return DateTime.parse(expiresAt!);
    } catch (_) {
      return null;
    }
  }

  @override
  List<Object?> get props => [
    id,
    chatId,
    productId,
    sourceType,
    sourceId,
    auctionId,
    sellerId,
    buyerId,
    cost,
    note,
    status,
    destinationCityId,
    destinationProvinceId,
    expiresAt,
    usedAt,
    createdAt,
  ];
}

// =============================================================================
// Request DTOs
// =============================================================================

/// Request body for creating a shipping quote.
///
/// Backend API: POST /api/v1/chat/:chat_id/shipping-quote
class CreateShippingQuoteRequestDto {
  final String productId;
  final String sourceType;
  final String sourceId;
  final String? auctionId;
  final int cost;
  final String? note;

  const CreateShippingQuoteRequestDto({
    required this.productId,
    required this.sourceType,
    required this.sourceId,
    required this.cost,
    this.auctionId,
    this.note,
  });

  Map<String, dynamic> toJson() {
    final json = <String, dynamic>{
      'product_id': productId,
      'source_type': sourceType,
      'source_id': sourceId,
      'cost': cost,
    };
    if (auctionId != null) {
      json['auction_id'] = auctionId;
    }
    if (note != null) {
      json['note'] = note;
    }
    return json;
  }
}
