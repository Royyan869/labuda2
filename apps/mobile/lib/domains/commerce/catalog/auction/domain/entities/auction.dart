/// Auction domain entity
/// Pure Dart entity - no Firebase, Flutter, or HTTP dependencies
/// This is the core business entity for auction functionality
///
/// SEMANTIC TRUTH:
/// - Auction has INDEPENDENT business data (title, description, media, pricing)
/// - Auction state machine is BACKEND-AUTHORITATIVE
/// - productId is OPTIONAL metadata for checkout integration only
/// - Product is NOT a source of truth for auction data
/// - Winning an auction creates an order via the linked product (if any)
///
/// NOTE: While auction data is independent, checkout requires a productId
/// to create the order. If productId is null, winner checkout is NOT available.
library;

// Auction enums (backend-aligned)
import 'auction_status.dart';

// Import MediaEntity
import 'package:labuda/domains/social/content/domain/entities/content.dart';

// Shipping readiness vocabulary (canonical shared core type).
import 'package:labuda/core/common/types/preparation_time.dart';

// Canonical governance lifecycle vocabulary (E8.2 seller user-axis).
import 'package:labuda/shared/governance/content_lifecycle.dart';

// Canonical commerce detail action authority (per-viewer capabilities).
import 'package:labuda/domains/commerce/catalog/shared/domain/entities/commerce_viewer_capabilities.dart';

// ============================================================
// ACTION AUTHORITY (Backend is Authority)
// ============================================================
// All business decisions come from the backend.
// The canonical per-viewer action authority for the auction detail surface
// is `viewer_capabilities` (CommerceViewerCapabilities), emitted by
// EvaluateAuctionViewerCapabilities on GET /api/v1/auctions/:id.
// The legacy P11 DecisionContract (decision/allowed_actions) was a phantom
// contract — the auction backend never emitted it — and is PURGED.
// Buyer bid-position authority lives in GET /api/v1/bidding.

/// Media type enum for auction media
enum AuctionMediaType { photo, video }

/// Koi details for auction
class KoiDetails {
  final String variety;
  final double sizeInCm;
  final int ageInMonths;
  final String gender;
  final List<String> certificates;
  final String? breeder;
  final String? bloodline;

  const KoiDetails({
    required this.variety,
    required this.sizeInCm,
    required this.ageInMonths,
    required this.gender,
    this.certificates = const [],
    this.breeder,
    this.bloodline,
  });

  // Computed properties
  String get sizeDisplay => '${sizeInCm.toStringAsFixed(0)} cm';
  String get ageDisplay => '$ageInMonths months';
  bool get hasCertificates => certificates.isNotEmpty;

  KoiDetails copyWith({
    String? variety,
    double? sizeInCm,
    int? ageInMonths,
    String? gender,
    List<String>? certificates,
    String? breeder,
    String? bloodline,
  }) {
    return KoiDetails(
      variety: variety ?? this.variety,
      sizeInCm: sizeInCm ?? this.sizeInCm,
      ageInMonths: ageInMonths ?? this.ageInMonths,
      gender: gender ?? this.gender,
      certificates: certificates ?? this.certificates,
      breeder: breeder ?? this.breeder,
      bloodline: bloodline ?? this.bloodline,
    );
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is KoiDetails &&
        other.variety == variety &&
        other.sizeInCm == sizeInCm &&
        other.ageInMonths == ageInMonths &&
        other.gender == gender;
  }

  @override
  int get hashCode =>
      variety.hashCode ^
      sizeInCm.hashCode ^
      ageInMonths.hashCode ^
      gender.hashCode;
}

/// Auction entity - core business entity
class Auction {
  final String id;
  final String sellerId;
  final String? sellerUsername;
  final String? sellerFarmName;
  final String? sellerAvatar;

  /// E8.2 — Canonical seller user-identity lifecycle ({active, unavailable,
  /// removed}). Sourced from the wire's nested
  /// `auction.seller.user.lifecycle` slot populated by E8.1.
  ///
  /// AXIS BOUNDARY: USER axis only. Seller-trust/capability axis is
  /// carried on [sellerTrustLifecycle].
  ///
  /// Defaults to active when wire is null/missing/unknown so legacy
  /// payloads keep rendering today's behavior.
  final ContentLifecycle sellerUserLifecycle;

  /// Expired-seller visibility — seller-trust lifecycle (subscription
  /// expired/lapsed) from the wire's top-level `auction.seller.lifecycle`
  /// slot. INDEPENDENT axis from [sellerUserLifecycle]. User-axis degraded =
  /// block/redact; seller-trust degraded = badge + transaction CTA disable
  /// (bid + buy-now). Defaults to active so legacy payloads keep current
  /// render.
  final ContentLifecycle sellerTrustLifecycle;

  /// Seller reputation tier — raw wire value from `auction.seller.tier`.
  ///
  /// Values: "pro", "elite". Null when backend emits no tier (flag
  /// disabled, user-identity/trust-axis degraded, or tier is Basic).
  ///
  /// RENDER RULE: SellerTierBadge hides for null/basic/unknown. Additional
  /// mobile gate: MUST NOT render when [sellerTrustLifecycle] is not active
  /// (expired subscription) — see AuctionSellerCard for enforcement.
  final String? sellerTier;

  /// Canonical per-viewer action authority for the auction detail surface.
  ///
  /// Populated from the wire's `viewer_capabilities` slot emitted by
  /// `auctionToDetailResponseWithSeller` →
  /// `commerceshared.EvaluateAuctionViewerCapabilities` (viewer identity,
  /// status, seller-trust, buy-now price evaluated server-side). The detail
  /// UI reads [canBid]/[canBuyNow]/[canChat]/[role] from here instead of
  /// re-inferring transaction permission locally.
  ///
  /// Null on list/discovery payloads (no viewer-scoped capability is emitted
  /// there). The detail screen always receives it via the detail endpoint;
  /// when null, callers keep the pre-convergence presentation gating so the
  /// shared read model stays consistent across surfaces.
  final CommerceViewerCapabilities? viewerCapabilities;

  final String title;
  final String description;

  // Media
  final List<MediaEntity> media;

  // Koi Details
  final KoiDetails koiDetails;

  // Pricing — canonical int read representation (backend int64/bigint wire;
  // nullable fields follow factual backend nullability).
  final int openingBid; // OB - Opening Bid
  final int currentBid;
  final int bidIncrement; // KB - Kelipatan Bid
  final int? buyNowPrice; // BIN - Buy It Now (optional)

  // Item details (backend-aligned)
  //
  // PURGED: `condition` (AuctionCondition) — the auction wire never emitted a
  // condition value (DTO parse was always null); the entity slot is dead.

  // Shipping Readiness — preparation time before the item can ship.
  // Read-only Product content carried on the auction detail wire
  // (preparation_time / preparation_note). Mirrors the ForSale surface read
  // model. Null when the backend Product carries no preparation value.
  final PreparationTime? preparationTime;
  final String? preparationNote;

  // Timing (backend authority)
  final DateTime startTime;
  final DateTime endTime;
  //
  // PURGED timing fields: startedAt/endedAt (backend never emits these on the
  // auction wire), settlementDeadline as a wire field (now DERIVED below from
  // end_at + 24h per Auction.SettlementDeadline()), and isScheduled (state
  // derivable from status — status is the single lifecycle authority).

  // Auction State — canonical winner authority is winnerId (current_winner_id).
  final AuctionStatus status;
  final String? winnerId;
  //
  // PURGED counters: totalBidders/totalViews — the auction wire never emitted
  // total_bids/views_count, so these were always-zero fake truths. Bid count
  // on the detail screen derives from the live bid stream (bids.length).

  final DateTime createdAt;
  final DateTime? updatedAt;
  //
  // PURGED: version (optimistic-locking hint the backend never emitted) and
  // location (AuctionLocation — never hydrated from any wire payload).

  // Shipping options
  final String? farmAddressId;

  // Checkout integration - optional reference to product for checkout flow
  final String? productId;

  const Auction({
    required this.id,
    required this.sellerId,
    this.sellerUsername,
    this.sellerFarmName,
    this.sellerAvatar,
    this.sellerUserLifecycle = ContentLifecycle.active,
    this.sellerTrustLifecycle = ContentLifecycle.active,
    this.sellerTier,
    this.viewerCapabilities,
    required this.title,
    required this.description,
    this.media = const [],
    required this.koiDetails,
    required this.openingBid,
    required this.currentBid,
    required this.bidIncrement,
    this.buyNowPrice,
    this.preparationTime,
    this.preparationNote,
    required this.startTime,
    required this.endTime,
    required this.status,
    this.winnerId,
    required this.createdAt,
    this.updatedAt,
    this.farmAddressId,
    this.productId,
  });

  // ============================================================
  // BACKEND AUTHORITY: These helpers provide DERIVED presentation states
  // NOT canonical business states - those come from backend
  // ============================================================

  // BOUNDARY NORMALIZATION (PHASE 1D):
  // - Status is the authoritative source of truth from backend
  // - Time boundary (endTime) is a display hint, NOT a decision factor
  // - Flutter MUST NOT compute business state from time alone
  //
  // For bid operability, use backend's decision contract:
  // - decision.allowedActions.contains('bid')
  // - decision.state == 'active'
  //
  // These computed properties are for PRESENTATION ONLY - they help UI
  // decide what to show, but NOT what actions are allowed.

  /// Check if auction is currently active (status-based only)
  /// PRESENTATION ONLY - use decision.state for business logic
  bool get isActive => status == AuctionStatus.active;

  /// Check if auction has ended (status-based only)
  /// BOUNDARY NORMALIZATION: Time check removed - backend status is authoritative
  /// PRESENTATION ONLY - use decision.state for business logic
  bool get hasEnded => status == AuctionStatus.ended;

  /// Check if auction is in a terminal state (ended or cancelled)
  /// PRESENTATION ONLY - use decision.state for business logic
  bool get isTerminal =>
      status == AuctionStatus.ended || status == AuctionStatus.cancelled;

  /// Calculate minimum next bid amount (display only) — factual derivation
  /// from currentBid + bidIncrement. No wire phantom.
  int get minimumNextBid => currentBid + bidIncrement;

  /// DERIVED PRESENTATION STATE (NOT a backend canonical state)
  /// True if auction ended with a winner (sold)
  /// Backend determines this via winnerId field, not status
  bool get isSold => status == AuctionStatus.ended && winnerId != null;

  /// DERIVED PRESENTATION STATE (NOT a backend canonical state)
  /// True if auction ended without a winner (expired with no bids)
  /// Backend determines this via winnerId field, not status
  bool get isExpired => status == AuctionStatus.ended && winnerId == null;

  /// Canonical settlement deadline derivation (backend rule:
  /// Auction.SettlementDeadline() = end_at + 24h). There is NO stored deadline
  /// authority — backend derives it and so does the client, from the same
  /// factual end_at field.
  DateTime get settlementDeadline =>
      endTime.add(const Duration(hours: 24));

  /// Check if current user is the winner (requires winnerId comparison)
  bool isUserWinner(String userId) => winnerId != null && winnerId == userId;

  // Safe computed properties (data display only, no business logic)
  bool get hasBuyNow => buyNowPrice != null;
  int get startingBid => openingBid;

  Auction copyWith({
    String? id,
    String? sellerId,
    String? sellerUsername,
    String? sellerFarmName,
    String? sellerAvatar,
    ContentLifecycle? sellerUserLifecycle,
    ContentLifecycle? sellerTrustLifecycle,
    String? sellerTier,
    CommerceViewerCapabilities? viewerCapabilities,
    String? title,
    String? description,
    List<MediaEntity>? media,
    KoiDetails? koiDetails,
    int? openingBid,
    int? currentBid,
    int? bidIncrement,
    int? buyNowPrice,
    PreparationTime? preparationTime,
    String? preparationNote,
    DateTime? startTime,
    DateTime? endTime,
    AuctionStatus? status,
    String? winnerId,
    DateTime? createdAt,
    DateTime? updatedAt,
    String? farmAddressId,
    String? productId,
  }) {
    return Auction(
      id: id ?? this.id,
      sellerId: sellerId ?? this.sellerId,
      sellerUsername: sellerUsername ?? this.sellerUsername,
      sellerFarmName: sellerFarmName ?? this.sellerFarmName,
      sellerAvatar: sellerAvatar ?? this.sellerAvatar,
      sellerUserLifecycle: sellerUserLifecycle ?? this.sellerUserLifecycle,
      sellerTrustLifecycle: sellerTrustLifecycle ?? this.sellerTrustLifecycle,
      sellerTier: sellerTier ?? this.sellerTier,
      viewerCapabilities: viewerCapabilities ?? this.viewerCapabilities,
      title: title ?? this.title,
      description: description ?? this.description,
      media: media ?? this.media,
      koiDetails: koiDetails ?? this.koiDetails,
      openingBid: openingBid ?? this.openingBid,
      currentBid: currentBid ?? this.currentBid,
      bidIncrement: bidIncrement ?? this.bidIncrement,
      buyNowPrice: buyNowPrice ?? this.buyNowPrice,
      preparationTime: preparationTime ?? this.preparationTime,
      preparationNote: preparationNote ?? this.preparationNote,
      startTime: startTime ?? this.startTime,
      endTime: endTime ?? this.endTime,
      status: status ?? this.status,
      winnerId: winnerId ?? this.winnerId,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
      farmAddressId: farmAddressId ?? this.farmAddressId,
      productId: productId ?? this.productId,
    );
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is Auction && other.id == id;
  }

  @override
  int get hashCode => id.hashCode;
}
