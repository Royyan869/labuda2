import 'package:equatable/equatable.dart';

/// Negotiation Entity - buyer-initiated private pricing negotiation
///
/// **Domain Layer** - bebas dari Firebase, Flutter, Riverpod
///
/// **OWNERSHIP TRUTH:**
/// - Backend NegotiationService is the SINGLE SOURCE OF TRUTH
/// - This entity is a READ-ONLY projection of backend state
/// - All status transitions must go through backend API
/// - Chat is only a transport layer for proposals, not the owner
///
/// **DISPLAY FIELDS (UI-ONLY CACHE):**
/// - listingName, listingImage, buyerName, buyerAvatar, sellerAvatar
/// - These are NOT provided by backend API
/// - Mobile app populates these from chat context for UI display only
/// - For any commerce logic, always resolve via backend canonical flow
///
/// **BACKEND STATES (authoritative):**
/// - active: negotiation is in progress
/// - accepted: seller accepted, ready for checkout
/// - cancelled: buyer cancelled
/// - expired: negotiation timed out
///
/// **IMPORTANT:** Do NOT use this entity's display fields for business logic.
/// Use only fixedPriceSaleId, buyerId, sellerId, and status for any decisions.
class Negotiation extends Equatable {
  final String id;
  final String chatId;
  final String fixedPriceSaleId;
  final String listingName;
  final String? listingImage;
  final double originalPrice;

  /// Buyer info
  final String buyerId;
  final String buyerName;
  final String? buyerAvatar;

  /// Seller info
  final String sellerId;
  final String? sellerAvatar;

  /// Current negotiation state
  final NegotiationStatus status;
  final double currentOfferPrice;
  final String lastOfferBy;
  final int round;

  /// History of all offers
  final List<NegotiationOffer> offers;

  /// Final agreed price (jika status == accepted)
  ///
  /// **CONVERSION TO CHECKOUT:**
  /// When status is accepted, buyer can proceed to checkout using:
  /// - PreviewOrderParams.forNegotiation(negotiationId, fixedPriceSaleId, ...)
  /// - Backend validates negotiation state and returns pricing token
  /// - Order creation requires valid pricing token (10 min expiry)
  /// - Backend stores OrderID in NegotiationSession to prevent duplicate settlement
  final double? agreedPrice;

  /// Timestamps
  final DateTime createdAt;
  final DateTime updatedAt;
  final DateTime? completedAt;

  /// Optional notes
  final String? rejectionReason;

  const Negotiation({
    required this.id,
    required this.chatId,
    required this.fixedPriceSaleId,
    required this.listingName,
    this.listingImage,
    required this.originalPrice,
    required this.buyerId,
    required this.buyerName,
    this.buyerAvatar,
    required this.sellerId,
    this.sellerAvatar,
    required this.status,
    required this.currentOfferPrice,
    required this.lastOfferBy,
    this.round = 1,
    this.offers = const [],
    this.agreedPrice,
    required this.createdAt,
    required this.updatedAt,
    this.completedAt,
    this.rejectionReason,
  });

  /// Check if current user can take action (send counter offer or accept)
  ///
  /// **RULES:**
  /// - Only active negotiations can receive actions
  /// - If buyer made last offer, only seller can counter or accept
  /// - If seller made last offer (counter), only buyer can counter or accept
  /// - Only seller can accept (finalize the negotiation)
  bool canUserAct(String userId) {
    // Only active negotiations allow actions
    if (!status.isActive) {
      return false;
    }
    // If buyer made last offer, seller can respond
    if (lastOfferBy == 'buyer') {
      return userId == sellerId;
    }
    // If seller made last offer, buyer can respond
    return userId == buyerId;
  }

  /// Check if seller can accept (finalize) the negotiation
  ///
  /// **RULE:** Only seller can accept, and only when negotiation is active
  bool canSellerAccept(String userId) {
    return status.isActive && userId == sellerId;
  }

  /// Check if user is buyer
  bool isBuyer(String userId) => userId == buyerId;

  /// Check if user is seller
  bool isSeller(String userId) => userId == sellerId;

  /// Get discount percentage
  ///
  /// **PRECISION NOTE:** This is a DISPLAY-ONLY calculation for UI preview.
  /// Authoritative pricing is ALWAYS provided by the backend via pricing token.
  /// Any discrepancy between this calculation and backend pricing should be
  /// resolved by using the backend-provided value.
  double get discountPercentage {
    if (originalPrice <= 0) return 0;
    return ((originalPrice - currentOfferPrice) / originalPrice) * 100;
  }

  /// Check if negotiation is still active
  ///
  /// **NOTE:** Uses the status extension's isActive property
  /// which checks if status == NegotiationStatus.active
  bool get isActive => status.isActive;

  @override
  List<Object?> get props => [
    id,
    chatId,
    fixedPriceSaleId,
    listingName,
    listingImage,
    originalPrice,
    buyerId,
    buyerName,
    buyerAvatar,
    sellerId,
    sellerAvatar,
    status,
    currentOfferPrice,
    lastOfferBy,
    round,
    offers,
    agreedPrice,
    createdAt,
    updatedAt,
    completedAt,
    rejectionReason,
  ];

  Negotiation copyWith({
    String? id,
    String? chatId,
    String? fixedPriceSaleId,
    String? listingName,
    String? listingImage,
    double? originalPrice,
    String? buyerId,
    String? buyerName,
    String? buyerAvatar,
    String? sellerId,
    String? sellerAvatar,
    NegotiationStatus? status,
    double? currentOfferPrice,
    String? lastOfferBy,
    int? round,
    List<NegotiationOffer>? offers,
    double? agreedPrice,
    DateTime? createdAt,
    DateTime? updatedAt,
    DateTime? completedAt,
    String? rejectionReason,
  }) {
    return Negotiation(
      id: id ?? this.id,
      chatId: chatId ?? this.chatId,
      fixedPriceSaleId: fixedPriceSaleId ?? this.fixedPriceSaleId,
      listingName: listingName ?? this.listingName,
      listingImage: listingImage ?? this.listingImage,
      originalPrice: originalPrice ?? this.originalPrice,
      buyerId: buyerId ?? this.buyerId,
      buyerName: buyerName ?? this.buyerName,
      buyerAvatar: buyerAvatar ?? this.buyerAvatar,
      sellerId: sellerId ?? this.sellerId,
      sellerAvatar: sellerAvatar ?? this.sellerAvatar,
      status: status ?? this.status,
      currentOfferPrice: currentOfferPrice ?? this.currentOfferPrice,
      lastOfferBy: lastOfferBy ?? this.lastOfferBy,
      round: round ?? this.round,
      offers: offers ?? this.offers,
      agreedPrice: agreedPrice ?? this.agreedPrice,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
      completedAt: completedAt ?? this.completedAt,
      rejectionReason: rejectionReason ?? this.rejectionReason,
    );
  }
}

/// Single offer dalam negotiation history
class NegotiationOffer extends Equatable {
  final int round;
  final String offeredBy;
  final String offeredByUserId;
  final String offeredByName;
  final double price;
  final DateTime timestamp;
  final String? note;

  const NegotiationOffer({
    required this.round,
    required this.offeredBy,
    required this.offeredByUserId,
    required this.offeredByName,
    required this.price,
    required this.timestamp,
    this.note,
  });

  @override
  List<Object?> get props => [
    round,
    offeredBy,
    offeredByUserId,
    offeredByName,
    price,
    timestamp,
    note,
  ];
}

/// Negotiation Status
///
/// **BACKEND STATES (authoritative):**
/// The backend only recognizes 4 states:
/// - `active`: negotiation is in progress, messages can be exchanged
/// - `accepted`: seller has accepted the price, ready for checkout
/// - `cancelled`: buyer cancelled the negotiation
/// - `expired`: negotiation timed out
///
/// **MOBILE STATES (must align with backend):**
/// - `active`: maps to backend `active` (negotiation in progress)
/// - `accepted`: maps to backend `accepted` (seller accepted)
/// - `cancelled`: maps to backend `cancelled` (buyer cancelled)
/// - `expired`: maps to backend `expired` (timed out)
///
/// **DEPRECATED STATES (removed for honesty):**
/// - `pending`: was mapping to backend `active`, caused confusion
/// - `countered`: was FAKE state, doesn't exist in backend
/// - `rejected`: no backend equivalent, never properly implemented
///
/// **UI CONSIDERATIONS:**
/// To show "who made last offer" in UI, use `lastOfferBy` field and `currentOfferPrice`.
/// Do NOT create separate states for this.
enum NegotiationStatus {
  /// Negotiation is in progress (backend: active)
  /// Initial state after creation, persists through counter offers
  active,

  /// Seller accepted the price (backend: accepted)
  /// Ready for checkout with pricing token
  accepted,

  /// Buyer cancelled (backend: cancelled)
  cancelled,

  /// Negotiation timed out (backend: expired)
  expired,
}

/// Status string mapper (untuk API/UI)
///
/// **IMPORTANT:** This maps backend status strings to mobile enum.
/// Backend is the source of truth for status values.
extension NegotiationStatusExtension on NegotiationStatus {
  String get name {
    switch (this) {
      case NegotiationStatus.active:
        return 'active';
      case NegotiationStatus.accepted:
        return 'accepted';
      case NegotiationStatus.cancelled:
        return 'cancelled';
      case NegotiationStatus.expired:
        return 'expired';
    }
  }

  static NegotiationStatus fromString(String value) {
    switch (value) {
      case 'active':
        return NegotiationStatus.active;
      case 'accepted':
      case 'completed': // Legacy API response, treat as accepted
        return NegotiationStatus.accepted;
      case 'cancelled':
        return NegotiationStatus.cancelled;
      case 'expired':
        return NegotiationStatus.expired;
      // Legacy mappings for data migration
      case 'pending':
        // Old 'pending' state maps to 'active'
        return NegotiationStatus.active;
      case 'countered':
        // Old 'countered' state was fake, map to 'active'
        return NegotiationStatus.active;
      case 'rejected':
        // Old 'rejected' state, map to 'cancelled' for backward compatibility
        return NegotiationStatus.cancelled;
      default:
        return NegotiationStatus.active;
    }
  }

  /// Check if negotiation is still active (can accept counter offers)
  bool get isActive => this == NegotiationStatus.active;

  /// Check if negotiation is in terminal state (no more actions possible)
  bool get isTerminal {
    return this == NegotiationStatus.accepted ||
        this == NegotiationStatus.cancelled ||
        this == NegotiationStatus.expired;
  }
}
