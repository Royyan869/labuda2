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
import 'auction_condition.dart';

// Import MediaEntity
import 'package:labuda/domains/social/content/domain/entities/content.dart';

// Canonical governance lifecycle vocabulary (E8.2 seller user-axis).
import 'package:labuda/shared/governance/content_lifecycle.dart';

// ============================================================
// P11 PHASE 2: DECISION CONTRACT (Backend is Authority)
// ============================================================
// All business decisions come from backend via decision contract.
// Frontend MUST NOT compute state, allowed actions, or business rules.
//
// Use decision.allowed_actions for UI decisions (e.g., show bid button)
// Use decision.state for authoritative business state
// Use decision.display for UI rendering hints (badges, labels, warnings)

/// Decision Contract from Backend
class DecisionContract {
  final String state;
  final List<String> allowedActions;
  final DisplayHints? display;

  const DecisionContract({
    required this.state,
    this.allowedActions = const [],
    this.display,
  });

  factory DecisionContract.fromJson(Map<String, dynamic>? json) {
    if (json == null) {
      return const DecisionContract(state: '', allowedActions: []);
    }
    return DecisionContract(
      state: json['state'] as String? ?? '',
      allowedActions:
          (json['allowed_actions'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          [],
      display: json['display'] != null
          ? DisplayHints.fromJson(json['display'] as Map<String, dynamic>)
          : null,
    );
  }

  Map<String, dynamic> toJson() => {
    'state': state,
    'allowed_actions': allowedActions,
    if (display != null) 'display': display!.toJson(),
  };
}

/// Display Hints from Backend (NON-AUTHORITATIVE)
class DisplayHints {
  final String? badge;
  final String? badgeVariant;
  final String? primaryAction;
  final String? warning;
  final String? info;
  final int? timeRemainingSeconds;
  final double? minimumNextBid;

  const DisplayHints({
    this.badge,
    this.badgeVariant,
    this.primaryAction,
    this.warning,
    this.info,
    this.timeRemainingSeconds,
    this.minimumNextBid,
  });

  factory DisplayHints.fromJson(Map<String, dynamic> json) {
    return DisplayHints(
      badge: json['badge'] as String?,
      badgeVariant: json['badge_variant'] as String?,
      primaryAction: json['primary_action'] as String?,
      warning: json['warning'] as String?,
      info: json['info'] as String?,
      timeRemainingSeconds: json['time_remaining_seconds'] as int?,
      minimumNextBid: json['minimum_next_bid'] as double?,
    );
  }

  Map<String, dynamic> toJson() => {
    'badge': badge,
    'badge_variant': badgeVariant,
    'primary_action': primaryAction,
    'warning': warning,
    'info': info,
    'time_remaining_seconds': timeRemainingSeconds,
    'minimum_next_bid': minimumNextBid,
  };
}

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

/// Location info - simplified for domain
class AuctionLocation {
  final String cityId;
  final String cityName;
  final String provinceId;
  final String provinceName;

  const AuctionLocation({
    required this.cityId,
    required this.cityName,
    required this.provinceId,
    required this.provinceName,
  });

  AuctionLocation copyWith({
    String? cityId,
    String? cityName,
    String? provinceId,
    String? provinceName,
  }) {
    return AuctionLocation(
      cityId: cityId ?? this.cityId,
      cityName: cityName ?? this.cityName,
      provinceId: provinceId ?? this.provinceId,
      provinceName: provinceName ?? this.provinceName,
    );
  }

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is AuctionLocation &&
        other.cityId == cityId &&
        other.provinceId == provinceId;
  }

  @override
  int get hashCode => cityId.hashCode ^ provinceId.hashCode;
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

  final String title;
  final String description;

  // Media
  final List<MediaEntity> media;

  // Koi Details
  final KoiDetails koiDetails;

  // Pricing
  final double openingBid; // OB - Opening Bid
  final double currentBid;
  final double bidIncrement; // KB - Kelipatan Bid
  final double? buyNowPrice; // BIN - Buy It Now (optional)

  // Item details (backend-aligned)
  final AuctionCondition? condition; // Item condition from backend

  // Timing (backend authority)
  final DateTime startTime;
  final DateTime endTime;
  final DateTime?
  startedAt; // Actual time when auction started (backend authority)
  final DateTime? endedAt; // Actual time when auction ended (backend authority)
  final DateTime?
  settlementDeadline; // Deadline for winner to complete purchase (waiting_settlement state only)
  final bool isScheduled;

  // Auction State
  final AuctionStatus status;
  final String? winnerId;
  final String? winnerUsername;
  final double? winningBid;
  final int totalBidders;
  final int totalWatchers;
  final int totalViews;
  final DateTime createdAt;
  final DateTime? updatedAt;
  final int? version; // Optimistic locking version (backend authority)

  // Location
  final AuctionLocation? location;

  // Shipping options
  final String? farmAddressId;

  // P11 Phase 2: Decision Contract from Backend
  final DecisionContract? decision;

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
    required this.title,
    required this.description,
    this.media = const [],
    required this.koiDetails,
    required this.openingBid,
    required this.currentBid,
    required this.bidIncrement,
    this.buyNowPrice,
    this.condition,
    required this.startTime,
    required this.endTime,
    this.startedAt,
    this.endedAt,
    this.settlementDeadline,
    this.isScheduled = false,
    required this.status,
    this.winnerId,
    this.winnerUsername,
    this.winningBid,
    this.totalBidders = 0,
    this.totalWatchers = 0,
    this.totalViews = 0,
    required this.createdAt,
    this.updatedAt,
    this.version,
    this.location,
    this.farmAddressId,
    this.decision,
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

  /// Calculate minimum next bid amount (display only)
  /// BOUNDARY NORMALIZATION: Use decision.display.minimumNextBid for business logic
  double get minimumNextBid => currentBid + bidIncrement;

  /// DERIVED PRESENTATION STATE (NOT a backend canonical state)
  /// True if auction ended with a winner (sold)
  /// Backend determines this via winnerId field, not status
  bool get isSold => status == AuctionStatus.ended && winnerId != null;

  /// DERIVED PRESENTATION STATE (NOT a backend canonical state)
  /// True if auction ended without a winner (expired with no bids)
  /// Backend determines this via winnerId field, not status
  bool get isExpired => status == AuctionStatus.ended && winnerId == null;

  /// Check if current user is the winner (requires winnerId comparison)
  bool isUserWinner(String userId) => winnerId != null && winnerId == userId;

  // P11 Phase 2: Business logic computed properties removed
  // BEFORE: bool get isActive => status == AuctionStatus.active (kept for BC);
  // AFTER: Use decision.state == 'active' or decision.allowed_actions.contains('bid')
  //
  // BEFORE: bool get hasEnded => DateTime.now().isAfter(endTime) (kept for BC);
  // AFTER: Use decision.state from backend
  //
  // BEFORE: double get minimumNextBid => currentBid + bidIncrement (kept for BC);
  // AFTER: Use decision.display.minimumNextBid from backend

  // Safe computed properties (data display only, no business logic)
  bool get hasBuyNow => buyNowPrice != null;
  double get startingBid => openingBid;

  // P11 Phase 2: minimumNextBid removed - use decision.display.minimumNextBid instead

  Auction copyWith({
    String? id,
    String? sellerId,
    String? sellerUsername,
    String? sellerFarmName,
    String? sellerAvatar,
    ContentLifecycle? sellerUserLifecycle,
    ContentLifecycle? sellerTrustLifecycle,
    String? sellerTier,
    String? title,
    String? description,
    List<MediaEntity>? media,
    KoiDetails? koiDetails,
    double? openingBid,
    double? currentBid,
    double? bidIncrement,
    double? buyNowPrice,
    AuctionCondition? condition,
    DateTime? startTime,
    DateTime? endTime,
    DateTime? startedAt,
    DateTime? endedAt,
    DateTime? settlementDeadline,
    bool? isScheduled,
    AuctionStatus? status,
    String? winnerId,
    String? winnerUsername,
    double? winningBid,
    int? totalBidders,
    int? totalWatchers,
    int? totalViews,
    DateTime? createdAt,
    DateTime? updatedAt,
    int? version,
    AuctionLocation? location,
    String? farmAddressId,
    DecisionContract? decision,
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
      title: title ?? this.title,
      description: description ?? this.description,
      media: media ?? this.media,
      koiDetails: koiDetails ?? this.koiDetails,
      openingBid: openingBid ?? this.openingBid,
      currentBid: currentBid ?? this.currentBid,
      bidIncrement: bidIncrement ?? this.bidIncrement,
      buyNowPrice: buyNowPrice ?? this.buyNowPrice,
      condition: condition ?? this.condition,
      startTime: startTime ?? this.startTime,
      endTime: endTime ?? this.endTime,
      startedAt: startedAt ?? this.startedAt,
      endedAt: endedAt ?? this.endedAt,
      settlementDeadline: settlementDeadline ?? this.settlementDeadline,
      isScheduled: isScheduled ?? this.isScheduled,
      status: status ?? this.status,
      winnerId: winnerId ?? this.winnerId,
      winnerUsername: winnerUsername ?? this.winnerUsername,
      winningBid: winningBid ?? this.winningBid,
      totalBidders: totalBidders ?? this.totalBidders,
      totalWatchers: totalWatchers ?? this.totalWatchers,
      totalViews: totalViews ?? this.totalViews,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
      version: version ?? this.version,
      location: location ?? this.location,
      farmAddressId: farmAddressId ?? this.farmAddressId,
      decision: decision ?? this.decision,
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
