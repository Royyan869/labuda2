/// API Request/Response DTOs for Negotiation
///
/// **Data Layer** - mengubah API response ke domain entity
///
/// **BACKEND CONTRACT (Chat-Owned):**
/// All negotiation endpoints are scoped under /chat/rooms/:room_id/
/// - POST /chat/rooms/:room_id/negotiate  → Start negotiation
/// - POST /chat/rooms/:room_id/counter    → Counter offer
/// - POST /chat/rooms/:room_id/respond    → Accept or cancel
/// - GET  /chat/rooms/:room_id/negotiation → Get latest session
///
/// **IMPORTANT:** Backend response uses int64 for prices (IDR units, no decimals).
/// This DTO does NO state translation - see NegotiationMapper for entity conversion.
library;

/// Create Negotiation Request
/// Backend expects: { for_sale_id, price, note? }
class CreateNegotiationDto {
  final String forSaleId;
  final int price;
  final String? note;

  CreateNegotiationDto({
    required this.forSaleId,
    required this.price,
    this.note,
  });

  Map<String, dynamic> toJson() {
    return {
      'for_sale_id': forSaleId,
      'price': price,
      if (note != null) 'note': note,
    };
  }
}

/// Counter Offer Request
/// Backend expects: { session_id, price, note? }
class CounterOfferDto {
  final String sessionId;
  final int price;
  final String? note;

  CounterOfferDto({required this.sessionId, required this.price, this.note});

  Map<String, dynamic> toJson() {
    return {
      'session_id': sessionId,
      'price': price,
      if (note != null) 'note': note,
    };
  }
}

/// Respond Negotiation Request
/// Backend expects: { session_id, action: "accept"|"cancel" }
class RespondNegotiationDto {
  final String sessionId;
  final String action;

  RespondNegotiationDto({required this.sessionId, required this.action});

  Map<String, dynamic> toJson() {
    return {'session_id': sessionId, 'action': action};
  }
}

/// Negotiation API Response
///
/// Matches backend sessionToResponse() output.
/// Nullable fields are only present when non-nil on backend.
class NegotiationResponseDto {
  final String id;
  final String resourceType;
  final String resourceId;
  final String buyerId;
  final String sellerId;
  final String status;
  final int proposalSequence;
  final DateTime createdAt;
  final DateTime updatedAt;
  // Nullable fields
  final String? forSaleId;
  final String? chatRoomId;
  final int? currentPrice;
  final int? acceptedPrice;
  final DateTime? expiresAt;
  final DateTime? acceptedAt;
  final String? orderId;
  final bool isExpired;

  NegotiationResponseDto({
    required this.id,
    required this.resourceType,
    required this.resourceId,
    required this.buyerId,
    required this.sellerId,
    required this.status,
    required this.proposalSequence,
    required this.createdAt,
    required this.updatedAt,
    this.forSaleId,
    this.chatRoomId,
    this.currentPrice,
    this.acceptedPrice,
    this.expiresAt,
    this.acceptedAt,
    this.orderId,
    this.isExpired = false,
  });

  factory NegotiationResponseDto.fromJson(Map<String, dynamic> json) {
    // Backend sends for_sale_id (not resource_id) for for_sale resources.
    // resource_id is not present in the session response.
    final forSaleId = json['for_sale_id'] as String?;
    return NegotiationResponseDto(
      id: json['id'] as String,
      resourceType: json['resource_type'] as String,
      // For for_sale resources, resourceId is the for_sale_id.
      // Backend does not emit a separate resource_id field.
      resourceId: forSaleId ?? json['resource_id'] as String? ?? '',
      buyerId: json['buyer_id'] as String,
      sellerId: json['seller_id'] as String,
      status: json['status'] as String,
      proposalSequence: json['proposal_sequence'] as int,
      createdAt: DateTime.parse(json['created_at'] as String),
      updatedAt: DateTime.parse(json['updated_at'] as String),
      forSaleId: forSaleId,
      chatRoomId: json['chat_room_id'] as String?,
      currentPrice: json['current_price'] as int?,
      acceptedPrice: json['accepted_price'] as int?,
      expiresAt: json['expires_at'] != null
          ? DateTime.parse(json['expires_at'] as String)
          : null,
      acceptedAt: json['accepted_at'] != null
          ? DateTime.parse(json['accepted_at'] as String)
          : null,
      orderId: json['order_id'] as String?,
      isExpired: json['is_expired'] as bool? ?? false,
    );
  }
}
