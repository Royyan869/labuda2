import 'package:equatable/equatable.dart';

/// ============================================================================
/// REFERENCE TRUTH ALIGNMENT V1 - ATTACHMENT BOUNDARY CONTRACT
/// ============================================================================
///
/// **BATCH R1 REALIGNMENT COMPLETE:**
///
/// The Attachment system has been REALIGNED into TWO SEMANTIC CATEGORIES:
///
/// 1. TRUE ATTACHMENT (local payload owned by surface itself):
///    - LocationAttachment: location data (lat/lng, place name)
///    - Media: file uploads (images, videos, documents) - carried in mediaUrls
///    - These are truly "attachments" - data belonging to the message/content
///
/// **CANONICAL TRUTH:**
///    - ShareReference is the CANONICAL cross-domain reference pattern
///    - All object references (Content, Listing, Auction) now use ShareReference directly
///    - Deprecated Attachment wrappers (PostAttachment, ListingAttachment, etc.) removed
///
/// 2. WORKFLOW PAYLOAD (domain-specific business state):
///    - NegotiationOfferAttachment: active negotiation state (Negotiation domain)
///    - NegotiationResultAttachment: negotiation outcome (Negotiation domain)
///    - ShippingQuoteAttachment: shipping offer data (Shipping domain)
///    - BidAttachment: auction bid in comment (Auction domain)
///    - These are NOT "attachments" - they are workflow/business payloads
///    - Kept here for BACKWARD COMPATIBILITY - should move to respective domain modules in V2
///
/// **SEMANTIC RULES:**
/// - TRUE ATTACHMENT: Use for local payload only (location, media)
/// - OBJECT REFERENCE: Use ShareReference directly for all cross-domain object references
/// - WORKFLOW PAYLOAD: Use canonical ID for all actions, never use cached data for business logic
/// ============================================================================

/// Base class for all attachment-like entities in the system.
///
/// **BATCH R1:** This base class unifies three semantic categories for compatibility.
/// New code should be aware of the semantic differences and handle appropriately.
abstract class Attachment extends Equatable {
  const Attachment();

  /// Get the attachment category for routing/handling
  AttachmentCategory get category;

  /// Check if this attachment has a canonical ID for backend queries
  String? get canonicalId;

  /// Check if this attachment supports live status resolution
  bool get supportsLiveStatus;
}

/// Attachment Category for type-safe routing
enum AttachmentCategory {
  /// True local payload (location, media)
  trueAttachment,

  /// Cross-object reference (uses ShareReference pattern)
  objectReference,

  /// Domain-specific workflow payload
  workflowPayload,
}

/// Attachment Type enum for type discrimination and serialization
///
/// **REFERENCE TRUTH ALIGNMENT V1:**
/// This enum contains TWO SEMANTICALLY DIFFERENT categories.
/// Use AttachmentCategory for semantic grouping.
enum AttachmentType {
  // True Attachments (local payload)
  location,

  // Workflow Payloads (domain-specific, kept for compatibility)
  negotiationOffer,
  negotiationProposal,
  negotiationResult,
  shippingQuote,
  bid,
}

/// Location attachment - TRUE ATTACHMENT (local payload)
///
/// This is the only true attachment type - location data belongs to the message itself.
class LocationAttachment extends Attachment {
  final double latitude;
  final double longitude;
  final String? placeName;
  final String? address;

  const LocationAttachment({
    required this.latitude,
    required this.longitude,
    this.placeName,
    this.address,
  });

  @override
  List<Object?> get props => [latitude, longitude, placeName, address];

  @override
  AttachmentCategory get category => AttachmentCategory.trueAttachment;

  @override
  String? get canonicalId => null; // Local payload has no canonical ID

  @override
  bool get supportsLiveStatus => false; // Local payload has no live status
}

/// Negotiation Offer attachment - WORKFLOW PAYLOAD (Negotiation domain)
///
/// **SEMANTIC RULES (CRITICAL):**
/// - negotiationId adalah CANONICAL reference ke negosiasi (backend authoritative)
/// - forSaleId adalah CANONICAL reference ke for-sale terkait
/// - SEMUA field lain (listingName, status, round, price, dll) hanya PREVIEW/CACHE untuk UI
/// - SEMUA preview data bisa STALE - tidak ada live status provider
/// - Gunakan negotiationId untuk semua action (accept, counter, reject)
/// - Backend adalah source of truth untuk status negosiasi
///
/// **BATCH R1:** This is a WORKFLOW PAYLOAD, not a true attachment.
/// Kept in Attachment system for backward compatibility.
/// Should move to Negotiation domain module in V2.
class NegotiationOfferAttachment extends Attachment {
  /// CANONICAL REFERENCE - selalu gunakan ini untuk query ke backend
  final String negotiationId;
  final String forSaleId;

  /// PREVIEW DATA - bisa stale, gunakan hanya untuk UI display
  final String listingName;
  final String? listingImage;
  final double originalPrice;
  final double currentOfferPrice;
  final String lastOfferBy;
  final int round;

  /// PREVIEW STATUS - bisa stale, gunakan hanya untuk UI display
  /// Untuk business logic, selalu query ke backend dengan negotiationId
  final String status;
  final String buyerId;
  final String buyerName;
  final String sellerId;
  final String sellerName;
  final DateTime createdAt;
  final DateTime updatedAt;

  const NegotiationOfferAttachment({
    required this.negotiationId,
    required this.forSaleId,
    required this.listingName,
    this.listingImage,
    required this.originalPrice,
    required this.currentOfferPrice,
    required this.lastOfferBy,
    required this.round,
    required this.status,
    required this.buyerId,
    required this.buyerName,
    required this.sellerId,
    required this.sellerName,
    required this.createdAt,
    required this.updatedAt,
  });

  double get discountPercentage {
    if (originalPrice <= 0) return 0;
    return ((originalPrice - currentOfferPrice) / originalPrice) * 100;
  }

  /// Checks if negotiation appears active based on EMBEDDED status
  /// **WARNING:** This is based on stale embedded data - may not reflect current backend state
  /// For actual business decisions, query backend with negotiationId
  bool get isActive => status == 'pending' || status == 'countered';

  /// Checks if user can act based on EMBEDDED status
  /// **WARNING:** This is based on stale embedded data - may not reflect current backend state
  /// For actual business decisions, query backend with negotiationId
  bool canUserAct(String userId) {
    if (!isActive) return false;
    return lastOfferBy == 'buyer' ? userId == sellerId : userId == buyerId;
  }

  @override
  List<Object?> get props => [
    negotiationId,
    forSaleId,
    listingName,
    listingImage,
    originalPrice,
    currentOfferPrice,
    lastOfferBy,
    round,
    status,
    buyerId,
    buyerName,
    sellerId,
    sellerName,
    createdAt,
    updatedAt,
  ];

  @override
  AttachmentCategory get category => AttachmentCategory.workflowPayload;

  @override
  String? get canonicalId => negotiationId;

  @override
  bool get supportsLiveStatus => false; // **R1.1 HONEST:** No provider implemented - embedded status may be stale
}

/// Negotiation Proposal attachment - WORKFLOW PAYLOAD (Negotiation domain)
///
/// Mirrors the backend `negotiation_proposal` attachment emitted by
/// `negotiation_event_handler.go` for `negotiation.started` and
/// `negotiation.message_sent` events. Carries only the fields the backend
/// actually sends — no display caches, no fabricated buyer/seller names.
///
/// Initial proposal carries: sessionId, proposalSequence (=1), price,
/// resourceType, resourceId, note.
/// Counter offer carries: sessionId, proposalSequence (>1), price.
///
/// **CANONICAL REFERENCE:** sessionId is the negotiation session identifier;
/// any business action must resolve via backend.
class NegotiationProposalAttachment extends Attachment {
  /// CANONICAL REFERENCE - backend negotiation session id
  final String sessionId;

  /// 1-based proposal sequence; 1 = initial proposal, >1 = counter offer
  final int proposalSequence;

  /// Proposal price in minor units as the backend emits it
  final int price;

  /// Resource type (e.g. "listing"); only present on initial proposal
  final String? resourceType;

  /// Resource id (e.g. fixed-price sale or auction id); only present on initial proposal
  final String? resourceId;

  /// Optional note from sender; only present on initial proposal
  final String? note;

  const NegotiationProposalAttachment({
    required this.sessionId,
    required this.proposalSequence,
    required this.price,
    this.resourceType,
    this.resourceId,
    this.note,
  });

  /// True if this is the initial proposal (sequence 1)
  bool get isInitialProposal => proposalSequence <= 1;

  @override
  List<Object?> get props => [
    sessionId,
    proposalSequence,
    price,
    resourceType,
    resourceId,
    note,
  ];

  @override
  AttachmentCategory get category => AttachmentCategory.workflowPayload;

  @override
  String? get canonicalId => sessionId;

  @override
  bool get supportsLiveStatus => false;
}

/// Negotiation Result attachment - WORKFLOW PAYLOAD (Negotiation domain)
///
/// **SEMANTIC RULES (CRITICAL):**
/// - negotiationId adalah CANONICAL reference ke negosiasi (backend authoritative)
/// - forSaleId adalah CANONICAL reference ke for-sale terkait
/// - Field lain (listingName, listingImage, agreedPrice) hanya PREVIEW/CACHE untuk UI
/// - Untuk checkout, selalu resolve lewat backend canonical flow dengan negotiationId
///
/// **BATCH R1:** This is a WORKFLOW PAYLOAD, not a true attachment.
/// Kept in Attachment system for backward compatibility.
/// Should move to Negotiation domain module in V2.
class NegotiationResultAttachment extends Attachment {
  /// CANONICAL REFERENCE - selalu gunakan ini untuk query ke backend
  final String negotiationId;
  final String forSaleId;

  /// PREVIEW DATA - bisa stale, gunakan hanya untuk UI display
  final String listingName;
  final String? listingImage;
  final double originalPrice;
  final double? agreedPrice;
  final String status;
  final int totalRounds;
  final DateTime createdAt;
  final DateTime? completedAt;
  final bool canPurchase;

  const NegotiationResultAttachment({
    required this.negotiationId,
    required this.forSaleId,
    required this.listingName,
    this.listingImage,
    required this.originalPrice,
    this.agreedPrice,
    required this.status,
    required this.totalRounds,
    required this.createdAt,
    this.completedAt,
    this.canPurchase = false,
  });

  @override
  List<Object?> get props => [
    negotiationId,
    forSaleId,
    listingName,
    listingImage,
    originalPrice,
    agreedPrice,
    status,
    totalRounds,
    createdAt,
    completedAt,
    canPurchase,
  ];

  @override
  AttachmentCategory get category => AttachmentCategory.workflowPayload;

  @override
  String? get canonicalId => negotiationId;

  @override
  bool get supportsLiveStatus => false; // **R1.1 HONEST:** No provider implemented - embedded status may be stale
}

/// Shipping Quote attachment - WORKFLOW PAYLOAD (Shipping domain)
///
/// **SEMANTIC RULES (CRITICAL):**
/// - offerId adalah CANONICAL reference ke shipping quote (backend authoritative)
/// - linkedItemId + linkedItemType adalah CANONICAL reference ke item terkait
/// - Field lain (linkedItemName, rate, validUntil) hanya PREVIEW/CACHE untuk UI
/// - JANGAN gunakan attachment rate/status sebagai business input
/// - Untuk checkout/shipping logic, selalu resolve lewat backend canonical flow
///
/// **WHY?** Attachment data bisa stale (rate expired, status changed). Backend adalah source of truth.
///
/// **BATCH R1:** This is a WORKFLOW PAYLOAD, not a true attachment.
/// Kept in Attachment system for backward compatibility.
/// Should move to Shipping domain module in V2.
class ShippingQuoteAttachment extends Attachment {
  /// CANONICAL REFERENCE - selalu gunakan ini untuk query ke backend
  final String offerId;

  /// CANONICAL REFERENCE ke item terkait - gunakan untuk query ke backend
  final String linkedItemId;
  final String linkedItemType;

  /// SHIPPING PREVIEW DATA - bisa stale, gunakan hanya untuk UI display
  final String linkedItemName;
  final String? linkedItemImage;
  final double linkedItemPrice;
  final double? linkedItemBuyNowPrice; // For auction with buy now option
  final String shippingType;
  final String shippingTypeName;
  final String shippingTypeEmoji;
  final double rate;
  final String? notes;
  final DateTime validUntil;
  final String status;
  final String sellerId;

  const ShippingQuoteAttachment({
    required this.offerId,
    required this.linkedItemId,
    required this.linkedItemType,
    required this.linkedItemName,
    String?
    linkedImage, // NOTE: Backwards compatibility alias - parameter name for API compatibility
    required this.linkedItemPrice,
    this.linkedItemBuyNowPrice,
    required this.shippingType,
    required this.shippingTypeName,
    required this.shippingTypeEmoji,
    required this.rate,
    this.notes,
    required this.validUntil,
    required this.status,
    required this.sellerId,
  }) : linkedItemImage = linkedImage;

  // Backwards compatibility: alias for linkedItemImage
  String? get linkedImage => linkedItemImage;

  String get displayName => '$shippingTypeEmoji $shippingTypeName';

  @override
  List<Object?> get props => [
    offerId,
    linkedItemId,
    linkedItemType,
    linkedItemName,
    linkedItemImage,
    linkedItemPrice,
    linkedItemBuyNowPrice,
    shippingType,
    shippingTypeName,
    shippingTypeEmoji,
    rate,
    notes,
    validUntil,
    status,
    sellerId,
  ];

  @override
  AttachmentCategory get category => AttachmentCategory.workflowPayload;

  @override
  String? get canonicalId => offerId;

  @override
  bool get supportsLiveStatus => false; // **R1.1 HONEST:** No provider implemented - embedded status may be stale
}

/// Bid attachment - WORKFLOW PAYLOAD (Auction domain)
///
/// **BATCH R1:** This is a WORKFLOW PAYLOAD, not a true attachment.
/// Represents a bid placed in an auction comment.
/// Kept in Attachment system for backward compatibility.
/// Should move to Auction domain module in V2.
class BidAttachment extends Attachment {
  final String auctionId;
  final double bidAmount;
  final String currency;

  const BidAttachment({
    required this.auctionId,
    required this.bidAmount,
    this.currency = 'IDR',
  });

  @override
  List<Object?> get props => [auctionId, bidAmount, currency];

  @override
  AttachmentCategory get category => AttachmentCategory.workflowPayload;

  @override
  String? get canonicalId => auctionId; // References the auction, not a specific bid entity

  @override
  bool get supportsLiveStatus => false; // **R1.1 HONEST:** No provider implemented - embedded bid may be stale
}
