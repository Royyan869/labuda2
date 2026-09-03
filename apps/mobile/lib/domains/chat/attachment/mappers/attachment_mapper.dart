import 'package:labuda/shared/attachment/entities/attachment.dart';

/// Unified Attachment Mapper - converts between Attachment entities and JSON maps
/// Used by both Chat and Comment modules for consistent serialization
///
/// ZERO LEGACY MODE: Only canonical attachment types accepted
/// FIRESTORE SUNSET (2025-02-20): Now uses JSON for Backend API communication.
class AttachmentMapper {
  /// Convert map data to Attachment domain entity
  ///
  /// **PHASE 1 CLEANUP:** Object reference attachments removed - use ShareReference instead
  /// Only workflow payload and true attachments are handled here
  static Attachment? fromMap(Map<String, dynamic>? data) {
    if (data == null) return null;

    final type = data['type'] as String?;
    switch (type) {
      // Note: 'post', 'listing', 'auction', 'request' removed - now use ShareReference
      case 'location':
        return _mapToLocationAttachment(data);
      case 'negotiation_offer':
        return _mapToNegotiationOfferAttachment(data);
      case 'negotiation_proposal':
        return _mapToNegotiationProposalAttachment(data);
      case 'negotiation_result':
        return _mapToNegotiationResultAttachment(data);
      case 'shipping_quote':
        return _mapToShippingQuoteAttachment(data);
      case 'bid':
        return _mapToBidAttachment(data);
      default:
        throw FormatException('Invalid attachment type: $type');
    }
  }

  /// Convert Attachment domain entity to map for API
  ///
  /// **PHASE 1 CLEANUP:** Object reference attachments removed - use ShareReference instead
  /// Only workflow payload and true attachments are handled here
  static Map<String, dynamic>? toMap(Attachment? attachment) {
    if (attachment == null) return null;

    if (attachment is LocationAttachment) {
      return _locationAttachmentToMap(attachment);
    } else if (attachment is NegotiationOfferAttachment) {
      return _negotiationOfferAttachmentToMap(attachment);
    } else if (attachment is NegotiationProposalAttachment) {
      return _negotiationProposalAttachmentToMap(attachment);
    } else if (attachment is NegotiationResultAttachment) {
      return _negotiationResultAttachmentToMap(attachment);
    } else if (attachment is ShippingQuoteAttachment) {
      return _shippingQuoteAttachmentToMap(attachment);
    } else if (attachment is BidAttachment) {
      return _bidAttachmentToMap(attachment);
    }
    return null;
  }

  // ===== HELPER: Parse DateTime from ISO 8601 String =====
  static DateTime? _parseDateTime(dynamic value) {
    if (value == null) return null;
    if (value is String) return DateTime.parse(value);
    if (value is DateTime) return value;
    return null;
  }

  static DateTime _parseDateTimeRequired(dynamic value, {DateTime? fallback}) {
    return _parseDateTime(value) ?? fallback ?? DateTime.now();
  }

  // ===== MAP TO ATTACHMENT HELPERS =====

  static LocationAttachment _mapToLocationAttachment(
    Map<String, dynamic> data,
  ) {
    return LocationAttachment(
      latitude: (data['latitude'] as num).toDouble(),
      longitude: (data['longitude'] as num).toDouble(),
      placeName: data['placeName'] as String?,
      address: data['address'] as String?,
    );
  }

  static NegotiationOfferAttachment _mapToNegotiationOfferAttachment(
    Map<String, dynamic> data,
  ) {
    return NegotiationOfferAttachment(
      negotiationId: data['negotiationId'] as String,
      forSaleId: data['forSaleId'] as String,
      listingName: data['listingName'] as String,
      listingImage: data['listingImage'] as String?,
      originalPrice: (data['originalPrice'] as num).toDouble(),
      currentOfferPrice: (data['currentOfferPrice'] as num).toDouble(),
      lastOfferBy: data['lastOfferBy'] as String,
      round: data['round'] as int,
      status: data['status'] as String,
      buyerId: data['buyerId'] as String,
      buyerName: data['buyerName'] as String,
      sellerId: data['sellerId'] as String,
      sellerName: data['sellerName'] as String,
      createdAt: _parseDateTimeRequired(data['createdAt']),
      updatedAt: _parseDateTimeRequired(data['updatedAt']),
    );
  }

  static NegotiationProposalAttachment _mapToNegotiationProposalAttachment(
    Map<String, dynamic> data,
  ) {
    return NegotiationProposalAttachment(
      sessionId: data['sessionId'] as String? ?? '',
      proposalSequence: (data['proposalSequence'] as num?)?.toInt() ?? 0,
      price: (data['price'] as num?)?.toInt() ?? 0,
      resourceType: data['resourceType'] as String?,
      resourceId: data['resourceId'] as String?,
      note: data['note'] as String?,
    );
  }

  static NegotiationResultAttachment _mapToNegotiationResultAttachment(
    Map<String, dynamic> data,
  ) {
    return NegotiationResultAttachment(
      negotiationId: data['negotiationId'] as String,
      forSaleId: data['forSaleId'] as String,
      listingName: data['listingName'] as String,
      listingImage: data['listingImage'] as String?,
      originalPrice: (data['originalPrice'] as num).toDouble(),
      agreedPrice: (data['agreedPrice'] as num?)?.toDouble(),
      status: data['status'] as String,
      totalRounds: data['totalRounds'] as int,
      createdAt: _parseDateTimeRequired(data['createdAt']),
      completedAt: _parseDateTime(data['completedAt']),
      canPurchase: data['canPurchase'] as bool? ?? false,
    );
  }

  static ShippingQuoteAttachment _mapToShippingQuoteAttachment(
    Map<String, dynamic> data,
  ) {
    final linkedItemType = data['linkedItemType'] as String? ?? 'listing';
    final linkedItemName =
        data['linkedItemName'] as String? ??
        (linkedItemType == 'auction' ? 'Penawaran Lelang' : 'Penawaran Ongkir');
    final linkedItemPrice =
        (data['linkedItemPrice'] as num?)?.toDouble() ?? 0.0;

    return ShippingQuoteAttachment(
      offerId: data['offerId'] as String,
      linkedItemId: data['linkedItemId'] as String,
      linkedItemType: linkedItemType,
      linkedItemName: linkedItemName,
      linkedImage:
          data['linkedItemImage'] as String? ?? data['linkedImage'] as String?,
      linkedItemPrice: linkedItemPrice,
      linkedItemBuyNowPrice: data['linkedItemBuyNowPrice'] != null
          ? (data['linkedItemBuyNowPrice'] as num).toDouble()
          : null,
      shippingType: data['shippingType'] as String,
      shippingTypeName: data['shippingTypeName'] as String,
      shippingTypeEmoji: data['shippingTypeEmoji'] as String,
      rate: (data['rate'] as num).toDouble(),
      notes: data['notes'] as String?,
      validUntil: _parseDateTimeRequired(data['validUntil']),
      status: data['status'] as String? ?? 'active',
      sellerId: data['sellerId'] as String,
    );
  }

  static BidAttachment _mapToBidAttachment(Map<String, dynamic> data) {
    return BidAttachment(
      auctionId: data['auctionId'] as String,
      bidAmount: (data['bidAmount'] as num).toDouble(),
      currency: data['currency'] as String? ?? 'IDR',
    );
  }

  // ===== ATTACHMENT TO MAP HELPERS =====

  static Map<String, dynamic> _locationAttachmentToMap(
    LocationAttachment attachment,
  ) {
    return {
      'type': 'location',
      'latitude': attachment.latitude,
      'longitude': attachment.longitude,
      'placeName': attachment.placeName,
      'address': attachment.address,
    };
  }

  static Map<String, dynamic> _negotiationOfferAttachmentToMap(
    NegotiationOfferAttachment attachment,
  ) {
    return {
      'type': 'negotiation_offer',
      'negotiationId': attachment.negotiationId,
      'forSaleId': attachment.forSaleId,
      'listingName': attachment.listingName,
      'listingImage': attachment.listingImage,
      'originalPrice': attachment.originalPrice,
      'currentOfferPrice': attachment.currentOfferPrice,
      'lastOfferBy': attachment.lastOfferBy,
      'round': attachment.round,
      'status': attachment.status,
      'buyerId': attachment.buyerId,
      'buyerName': attachment.buyerName,
      'sellerId': attachment.sellerId,
      'sellerName': attachment.sellerName,
      'createdAt': attachment.createdAt.toIso8601String(),
      'updatedAt': attachment.updatedAt.toIso8601String(),
    };
  }

  static Map<String, dynamic> _negotiationProposalAttachmentToMap(
    NegotiationProposalAttachment attachment,
  ) {
    return {
      'type': 'negotiation_proposal',
      'session_id': attachment.sessionId,
      'proposal_sequence': attachment.proposalSequence,
      'price': attachment.price,
      if (attachment.resourceType != null)
        'resource_type': attachment.resourceType,
      if (attachment.resourceId != null) 'resource_id': attachment.resourceId,
      if (attachment.note != null) 'note': attachment.note,
    };
  }

  static Map<String, dynamic> _negotiationResultAttachmentToMap(
    NegotiationResultAttachment attachment,
  ) {
    return {
      'type': 'negotiation_result',
      'negotiationId': attachment.negotiationId,
      'forSaleId': attachment.forSaleId,
      'listingName': attachment.listingName,
      'listingImage': attachment.listingImage,
      'originalPrice': attachment.originalPrice,
      'agreedPrice': attachment.agreedPrice,
      'status': attachment.status,
      'totalRounds': attachment.totalRounds,
      'createdAt': attachment.createdAt.toIso8601String(),
      'completedAt': attachment.completedAt?.toIso8601String(),
      'canPurchase': attachment.canPurchase,
    };
  }

  static Map<String, dynamic> _shippingQuoteAttachmentToMap(
    ShippingQuoteAttachment attachment,
  ) {
    return {
      'type': 'shipping_quote',
      'offerId': attachment.offerId,
      'linkedItemId': attachment.linkedItemId,
      'linkedItemType': attachment.linkedItemType,
      'linkedItemName': attachment.linkedItemName,
      'linkedItemImage': attachment
          .linkedImage, // NOTE: Field renamed to linkedItemImage, kept for API compatibility
      'linkedItemPrice': attachment.linkedItemPrice,
      'linkedItemBuyNowPrice': attachment.linkedItemBuyNowPrice,
      'shippingType': attachment.shippingType,
      'shippingTypeName': attachment.shippingTypeName,
      'shippingTypeEmoji': attachment.shippingTypeEmoji,
      'rate': attachment.rate,
      'notes': attachment.notes,
      'validUntil': attachment.validUntil.toIso8601String(),
      'status': attachment.status,
      'sellerId': attachment.sellerId,
    };
  }

  static Map<String, dynamic> _bidAttachmentToMap(BidAttachment attachment) {
    return {
      'type': 'bid',
      'auctionId': attachment.auctionId,
      'bidAmount': attachment.bidAmount,
      'currency': attachment.currency,
    };
  }
}
