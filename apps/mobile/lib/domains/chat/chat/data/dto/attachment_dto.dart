import 'package:equatable/equatable.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/shared/object/object_preview.dart' as obj;

Map<String, dynamic>? _singleEntryMap(String key, Object? value) {
  if (value == null) return null;
  return <String, dynamic>{key: value};
}

/// ============================================================================
/// ATTACHMENT DTO V2 - SOCIAL FIX 1: SINGLE REFERENCE MODEL
/// ============================================================================
///
/// **UNIFIED REFERENCE SYSTEM:**
/// - Object references (listing, auction, post, request, profile) use ShareReference
/// - Workflow payloads (negotiation, shipping, bid) keep separate structure
/// - True attachments (location) keep separate structure
///
/// All attachments follow: { "type": "...", "data": { ... } }
///
/// This is the API contract for chat message attachments.
/// Use AttachmentMapper in shared/attachment for domain conversion.
/// ============================================================================

/// Base attachment DTO - strict type wrapper
class AttachmentDto extends Equatable {
  final String type;
  final Map<String, dynamic> data;

  const AttachmentDto({required this.type, required this.data});

  /// Create from JSON
  factory AttachmentDto.fromJson(Map<String, dynamic> json) {
    return AttachmentDto(
      type: json['type'] as String,
      data: json['data'] as Map<String, dynamic>? ?? {},
    );
  }

  /// Convert to JSON
  Map<String, dynamic> toJson() => {'type': type, 'data': data};

  @override
  List<Object?> get props => [type, data];
}

/// ============================================================================
/// OBJECT REFERENCES - Using ShareReference format
/// ============================================================================

/// ShareReference Attachment DTO - UNIFIED object reference
///
/// **SOCIAL FIX 1:** All object references use the canonical ShareReference format.
/// - target_type: for_sale, auction, content, profile
/// - target_id: canonical ID for backend queries
/// - preview: cached preview data for UI display
class ShareReferenceAttachmentDto extends AttachmentDto {
  final ShareTargetType targetType;
  final String targetId;
  final SharePreviewDto preview;
  final String wireTargetType;

  ShareReferenceAttachmentDto({
    required this.targetType,
    required this.targetId,
    required this.preview,
    String? wireTargetType,
  }) : wireTargetType = wireTargetType ?? targetType.wireValue,
       super(
         type: 'reference',
         data: {
           'target_type': wireTargetType ?? targetType.wireValue,
           'target_id': targetId,
           'preview': preview.toJson(),
         },
       );

  factory ShareReferenceAttachmentDto.fromJson(Map<String, dynamic> json) {
    final data = json['data'] as Map<String, dynamic>? ?? const {};
    final targetTypeRaw = data['target_type'] as String?;
    final targetIdRaw = data['target_id'] as String?;
    if (targetTypeRaw == null || targetIdRaw == null) {
      throw const FormatException(
        'reference attachment requires data.target_type and data.target_id',
      );
    }
    final parsedType = _parseLogicalReferenceType(targetTypeRaw);
    return ShareReferenceAttachmentDto(
      targetType: parsedType,
      targetId: targetIdRaw,
      wireTargetType: targetTypeRaw,
      preview: SharePreviewDto.fromJson(
        data['preview'] as Map<String, dynamic>? ?? {},
      ),
    );
  }

  /// Convert to domain ShareReference
  ShareReference toShareReference() {
    return ShareReference(
      targetType: targetType,
      targetId: targetId,
      wireTargetType: wireTargetType,
      preview: obj.ObjectPreview(
        id: targetId,
        type: targetType.objectType,
        title: preview.title,
        imageUrl: preview.imageUrl,
        status: _getStatusFromPreview(preview),
        price: null,
        isAvailable: preview.isAvailable,
        isSold: preview.isSold,
        isClosed: preview.isClosed,
        isDeleted: preview.isDeleted,
      ),
    );
  }

  String _getStatusFromPreview(SharePreviewDto preview) {
    if (preview.isDeleted) return 'deleted';
    if (preview.isSold) return 'sold';
    if (preview.isClosed) return 'ended';
    if (preview.isAvailable) return 'available';
    return 'unknown';
  }

  /// Create from domain ShareReference
  factory ShareReferenceAttachmentDto.fromShareReference(ShareReference ref) {
    return ShareReferenceAttachmentDto(
      targetType: ref.targetType,
      targetId: ref.targetId,
      preview: SharePreviewDto.fromObjectPreview(ref.preview),
      wireTargetType: ref.wireTargetType,
    );
  }

  @override
  List<Object?> get props => [targetType, targetId, wireTargetType, preview];
}

ShareTargetType _parseLogicalReferenceType(String targetType) {
  final parsed = ShareTargetType.fromString(targetType);
  if (parsed != null) {
    return parsed;
  }
  throw FormatException('invalid reference target_type: $targetType');
}

/// SharePreview DTO - cached preview data
class SharePreviewDto extends Equatable {
  final String title;
  final String? imageUrl;
  final bool isAvailable;
  final bool isSold;
  final bool isClosed;
  final bool isDeleted;

  const SharePreviewDto({
    required this.title,
    this.imageUrl,
    this.isAvailable = true,
    this.isSold = false,
    this.isClosed = false,
    this.isDeleted = false,
  });

  factory SharePreviewDto.fromJson(Map<String, dynamic> json) {
    return SharePreviewDto(
      title: json['title'] as String? ?? '',
      imageUrl: json['imageUrl'] as String?,
      isAvailable: json['isAvailable'] as bool? ?? true,
      isSold: json['isSold'] as bool? ?? false,
      isClosed: json['isClosed'] as bool? ?? false,
      isDeleted: json['isDeleted'] as bool? ?? false,
    );
  }

  /// Create from domain ObjectPreview
  factory SharePreviewDto.fromObjectPreview(obj.ObjectPreview preview) {
    return SharePreviewDto(
      title: preview.title,
      imageUrl: preview.imageUrl,
      isAvailable: preview.isAvailable,
      isSold: preview.isSold,
      isClosed: preview.isClosed,
      isDeleted: preview.isDeleted,
    );
  }

  Map<String, dynamic> toJson() => {
    'title': title,
    if (imageUrl != null) 'imageUrl': imageUrl,
    'isAvailable': isAvailable,
    'isSold': isSold,
    'isClosed': isClosed,
    'isDeleted': isDeleted,
  };

  @override
  List<Object?> get props => [
    title,
    imageUrl,
    isAvailable,
    isSold,
    isClosed,
    isDeleted,
  ];
}

/// ============================================================================
/// PHASE 1 COMPLETE - LEGACY WRAPPERS REMOVED
/// ============================================================================
/// The following legacy DTOs have been removed:
/// - ListingAttachmentDto, AuctionAttachmentDto, PostAttachmentDto, RequestAttachmentDto
/// - All object references now use the canonical ShareReferenceAttachmentDto format
/// - Use appropriate targetType: listing, auction, content
/// Legacy attachment types (listing, auction, post, request) are still supported
/// for backward compatibility in parseAttachmentDto() but map to ShareReference.
/// ============================================================================

/// ============================================================================
/// WORKFLOW PAYLOADS (Domain-specific business state)
/// ============================================================================

/// Negotiation Offer Attachment DTO
class NegotiationOfferAttachmentDto extends AttachmentDto {
  NegotiationOfferAttachmentDto({
    required String negotiationId,
    required String forSaleId,
    required String status,
    required SharePreviewDto preview,
  }) : super(
         type: 'negotiation_offer',
         data: {
           'negotiation_id': negotiationId,
           'for_sale_id': forSaleId,
           'status': status,
           'preview': preview.toJson(),
         },
       );

  factory NegotiationOfferAttachmentDto.fromJson(Map<String, dynamic> json) {
    return NegotiationOfferAttachmentDto(
      negotiationId: json['data']['negotiation_id'] as String,
      forSaleId: json['data']['for_sale_id'] as String,
      status: json['data']['status'] as String,
      preview: SharePreviewDto.fromJson(
        json['data']['preview'] as Map<String, dynamic>? ?? {},
      ),
    );
  }

  String get negotiationId => data['negotiation_id'] as String;
  String get forSaleId => data['for_sale_id'] as String;
  String get status => data['status'] as String;
  SharePreviewDto get preview =>
      SharePreviewDto.fromJson(data['preview'] as Map<String, dynamic>? ?? {});
}

/// Negotiation Proposal Attachment DTO
///
/// Wire format emitted by backend `negotiation_event_handler.go`.
/// Canonical shape only: `{type, data}`.
class NegotiationProposalAttachmentDto extends AttachmentDto {
  NegotiationProposalAttachmentDto({
    required String sessionId,
    required int proposalSequence,
    required int price,
    String? resourceType,
    String? resourceId,
    String? note,
  }) : super(
         type: 'negotiation_proposal',
         data: {
           'session_id': sessionId,
           'proposal_sequence': proposalSequence,
           'price': price,
           ...?_singleEntryMap('resource_type', resourceType),
           ...?_singleEntryMap('resource_id', resourceId),
           ...?_singleEntryMap('note', note),
         },
       );

  factory NegotiationProposalAttachmentDto.fromJson(Map<String, dynamic> json) {
    final nested = json['data'];
    if (nested is! Map<String, dynamic>) {
      throw const FormatException(
        'negotiation_proposal attachment requires nested data object',
      );
    }
    final src = nested;
    return NegotiationProposalAttachmentDto(
      sessionId: src['session_id'] as String? ?? '',
      proposalSequence: (src['proposal_sequence'] as num?)?.toInt() ?? 0,
      price: (src['price'] as num?)?.toInt() ?? 0,
      resourceType: src['resource_type'] as String?,
      resourceId: src['resource_id'] as String?,
      note: src['note'] as String?,
    );
  }

  String get sessionId => data['session_id'] as String? ?? '';
  int get proposalSequence => (data['proposal_sequence'] as num?)?.toInt() ?? 0;
  int get price => (data['price'] as num?)?.toInt() ?? 0;
  String? get resourceType => data['resource_type'] as String?;
  String? get resourceId => data['resource_id'] as String?;
  String? get note => data['note'] as String?;
}

/// Negotiation Result Attachment DTO
class NegotiationResultAttachmentDto extends AttachmentDto {
  NegotiationResultAttachmentDto({
    required String negotiationId,
    required String forSaleId,
    required String status,
    required SharePreviewDto preview,
  }) : super(
         type: 'negotiation_result',
         data: {
           'negotiation_id': negotiationId,
           'for_sale_id': forSaleId,
           'status': status,
           'preview': preview.toJson(),
         },
       );

  factory NegotiationResultAttachmentDto.fromJson(Map<String, dynamic> json) {
    return NegotiationResultAttachmentDto(
      negotiationId: json['data']['negotiation_id'] as String,
      forSaleId: json['data']['for_sale_id'] as String,
      status: json['data']['status'] as String,
      preview: SharePreviewDto.fromJson(
        json['data']['preview'] as Map<String, dynamic>? ?? {},
      ),
    );
  }

  String get negotiationId => data['negotiation_id'] as String;
  String get forSaleId => data['for_sale_id'] as String;
  String get status => data['status'] as String;
  SharePreviewDto get preview =>
      SharePreviewDto.fromJson(data['preview'] as Map<String, dynamic>? ?? {});
}

/// Shipping Quote Attachment DTO
class ShippingQuoteAttachmentDto extends AttachmentDto {
  ShippingQuoteAttachmentDto({
    required String offerId,
    required String linkedItemId,
    required String linkedItemType,
    String? linkedItemName,
    String? linkedItemImage,
    double? linkedItemPrice,
    required String shippingType,
    required String shippingTypeName,
    required String shippingTypeEmoji,
    required double rate,
    String? notes,
    String? validUntil,
    required String status,
    required String sellerId,
  }) : super(
         type: 'shipping_quote',
         data: {
           'offer_id': offerId,
           'linked_item_id': linkedItemId,
           'linked_item_type': linkedItemType,
           ...?_singleEntryMap('linked_item_name', linkedItemName),
           ...?_singleEntryMap('linked_item_image', linkedItemImage),
           ...?_singleEntryMap('linked_item_price', linkedItemPrice),
           'shipping_type': shippingType,
           'shipping_type_name': shippingTypeName,
           'shipping_type_emoji': shippingTypeEmoji,
           'rate': rate,
           ...?_singleEntryMap('notes', notes),
           ...?_singleEntryMap('valid_until', validUntil),
           'status': status,
           'seller_id': sellerId,
         },
       );

  factory ShippingQuoteAttachmentDto.fromJson(Map<String, dynamic> json) {
    final data = json['data'] as Map<String, dynamic>;
    return ShippingQuoteAttachmentDto(
      offerId: data['offer_id'] as String,
      linkedItemId: data['linked_item_id'] as String,
      linkedItemType: data['linked_item_type'] as String,
      linkedItemName: data['linked_item_name'] as String?,
      linkedItemImage: data['linked_item_image'] as String?,
      linkedItemPrice: (data['linked_item_price'] as num?)?.toDouble(),
      shippingType: data['shipping_type'] as String? ?? 'manual',
      shippingTypeName:
          data['shipping_type_name'] as String? ?? 'Ongkir Manual',
      shippingTypeEmoji: data['shipping_type_emoji'] as String? ?? '🚚',
      rate: (data['rate'] as num).toDouble(),
      notes: data['notes'] as String?,
      validUntil: data['valid_until'] as String?,
      status: data['status'] as String? ?? 'ACTIVE',
      sellerId: data['seller_id'] as String? ?? '',
    );
  }

  String get offerId => data['offer_id'] as String;
  String get linkedItemId => data['linked_item_id'] as String;
  String get linkedItemType => data['linked_item_type'] as String;
  String? get linkedItemName => data['linked_item_name'] as String?;
  String? get linkedItemImage => data['linked_item_image'] as String?;
  double? get linkedItemPrice => data['linked_item_price'] as double?;
  String get shippingType => data['shipping_type'] as String;
  String get shippingTypeName => data['shipping_type_name'] as String;
  String get shippingTypeEmoji => data['shipping_type_emoji'] as String;
  double get rate => data['rate'] as double;
  String? get notes => data['notes'] as String?;
  String? get validUntil => data['valid_until'] as String?;
  String get status => data['status'] as String? ?? 'ACTIVE';
  String get sellerId => data['seller_id'] as String? ?? '';
}

/// ============================================================================
/// TRUE ATTACHMENTS (Local payload)
/// ============================================================================

/// Location Attachment DTO
class LocationAttachmentDto extends AttachmentDto {
  LocationAttachmentDto({
    required double latitude,
    required double longitude,
    String? placeName,
    String? address,
  }) : super(
         type: 'location',
         data: {
           'latitude': latitude,
           'longitude': longitude,
           ...?_singleEntryMap('placeName', placeName),
           ...?_singleEntryMap('address', address),
         },
       );

  factory LocationAttachmentDto.fromJson(Map<String, dynamic> json) {
    return LocationAttachmentDto(
      latitude: (json['data']['latitude'] as num).toDouble(),
      longitude: (json['data']['longitude'] as num).toDouble(),
      placeName: json['data']['placeName'] as String?,
      address: json['data']['address'] as String?,
    );
  }

  double get latitude => data['latitude'] as double;
  double get longitude => data['longitude'] as double;
  String? get placeName => data['placeName'] as String?;
  String? get address => data['address'] as String?;
}

/// ============================================================================
/// ATTACHMENT PARSER
/// ============================================================================

/// Attachment type parser - creates typed DTO from base AttachmentDto
///
/// Throws FormatException for unknown types
AttachmentDto parseAttachmentDto(Map<String, dynamic> json) {
  final base = AttachmentDto.fromJson(json);
  switch (base.type) {
    case 'reference':
      return ShareReferenceAttachmentDto.fromJson(json);
    // Workflow payloads
    case 'negotiation_offer':
      return NegotiationOfferAttachmentDto.fromJson(json);
    case 'negotiation_proposal':
      return NegotiationProposalAttachmentDto.fromJson(json);
    case 'negotiation_result':
      return NegotiationResultAttachmentDto.fromJson(json);
    case 'shipping_quote':
      return ShippingQuoteAttachmentDto.fromJson(json);
    // True attachments
    case 'location':
      return LocationAttachmentDto.fromJson(json);
    default:
      throw FormatException('Invalid attachment type: ${base.type}');
  }
}
