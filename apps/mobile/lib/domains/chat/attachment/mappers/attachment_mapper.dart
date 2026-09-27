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
      // Note: 'post', 'forSale', 'auction', 'request' removed - now use ShareReference
      case 'location':
        return _mapToLocationAttachment(data);
      case 'negotiation_proposal':
        return _mapToNegotiationProposalAttachment(data);
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
    } else if (attachment is NegotiationProposalAttachment) {
      return _negotiationProposalAttachmentToMap(attachment);
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

  static ShippingQuoteAttachment _mapToShippingQuoteAttachment(
    Map<String, dynamic> data,
  ) {
    final linkedItemType = data['linkedItemType'] as String? ?? 'forSale';
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
