/// For Sale Domain Entity
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// FOR SALE DOMAIN BUSINESS TRUTH
/// ═══════════════════════════════════════════════════════════════════════════════
/// PASS_21C: ForSale is the fixed-price sale mechanism over a Product — a
/// sibling of Auction, never its parent. Product (item/koi identity) is the
/// canonical root; ForSale and Auction are the two sale channels over it.
/// This entity still carries koi/item fields inline for display convenience
/// (no separate mobile Product model exists yet), but it must never be
/// treated as "the" product entity or as an umbrella over Auction.
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// LIFECYCLE STATES (canonical):
/// ═══════════════════════════════════════════════════════════════════════════════
/// - draft:      Workspace-only, NOT in market, seller can edit
/// - active:     Market-visible, purchasable if stock > 0 (ALWAYS PUBLIC)
/// - sold:       Successfully sold (TERMINAL for marketplace)
/// - withdrawn:  Seller removed from sale (TERMINAL)
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// HARD RULE: ACTIVE = PUBLIC ONLY
/// ═══════════════════════════════════════════════════════════════════════════════
/// - Draft forSales: MUST BE private (workspace-only)
/// - Active forSales: MUST BE public (enforced by backend)
/// - Terminal states: visibility irrelevant
/// - Backend automatically sets visibility to public on publish
/// - active + private combination is INVALID and rejected
/// ═══════════════════════════════════════════════════════════════════════════════
/// BUYABILITY (market operability):
/// ═══════════════════════════════════════════════════════════════════════════════
/// - Purchasable when: status == active AND stock > 0
/// - Visibility check is redundant (ACTIVE = PUBLIC ONLY)
/// - Shortlist does NOT reserve stock (first-come-first-served until order)
/// - Terminal states (sold, withdrawn) cannot be purchased
///
/// INTEGRATION POINTS:
/// - Direct backend API integration (NO collection dependency)
/// - Chat: ShareReference provides commerce context
/// - Order: Created with for_sale_id, stock deducted on order confirmation
/// - Shipping: Delivery coverage checked before buy action
/// ═══════════════════════════════════════════════════════════════════════════════
library;

import 'package:equatable/equatable.dart';

// Import MediaEntity from shared entities
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

// Import PreparationTime
import 'package:labuda/core/common/types/preparation_time.dart';

// =============================================================================
// Enums
// =============================================================================

/// For Sale visibility determines workspace vs market boundary
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// HARD RULE: STATUS → VISIBILITY MAPPING
/// ═══════════════════════════════════════════════════════════════════════════════
/// - Draft forSales: MUST BE private (workspace-only, NOT in market)
/// - Active forSales: MUST BE public (ACTIVE = PUBLIC ONLY invariant)
/// - Terminal states (sold/withdrawn): visibility field is irrelevant
///
/// The backend enforces this by automatically setting visibility=public on publish.
/// No manual setting of visibility for active listings is allowed.
/// ═══════════════════════════════════════════════════════════════════════════════
enum ForSaleVisibility {
  /// Seller-only visibility (workspace listing)
  /// This is the ONLY valid visibility for draft listings
  private,

  /// Market-visible (discoverable by buyers)
  /// This is the ONLY valid visibility for active listings
  /// Requires active seller subscription to create/update
  public;

  String get displayName {
    switch (this) {
      case ForSaleVisibility.private:
        return 'Private';
      case ForSaleVisibility.public:
        return 'Public';
    }
  }

  /// Returns true if this forSale is visible in the marketplace
  bool get isMarketVisible => this == ForSaleVisibility.public;

  /// Returns true if this forSale is workspace-only
  bool get isWorkspaceOnly => this == ForSaleVisibility.private;
}

// =============================================================================
// For Sale Entity
// =============================================================================

/// ForSale entity for marketplace items
/// Acts as an abstraction over Collection entity
class ForSale extends Equatable {
  final String forSaleId;
  final String? productId;
  final String title;
  final String description;

  /// **PRECISION NOTE:** Price is provided by backend as a numeric value.
  /// This field is for DISPLAY purposes only. All pricing calculations
  /// (subtotal, total, discounts) MUST be performed by the backend.
  final double price;

  final int stock;

  // Shipping Readiness - preparation time before item can be shipped
  // BUYER EXPECTATION: This is what buyers see before purchasing
  // ORDER SNAPSHOT: When order is created, this value is frozen as preparation_time_snapshot
  final PreparationTime preparationTime;
  final String? preparationNote;

  /// Rich media entities with metadata.
  /// Provides blurhash for placeholders, dimensions for aspect ratio,
  /// and variants for different sizes.
  final List<MediaEntity> media;
  final String sellerId;
  final String? sellerUsername;
  final String? sellerFarmName;
  final String? sellerAvatar;

  /// E8.2 — Canonical seller user-identity lifecycle ({active, unavailable,
  /// removed}). Sourced from the wire's nested
  /// `listing.seller.user.lifecycle` slot populated by E8.1.
  ///
  /// AXIS BOUNDARY: This field carries ONLY the USER axis (banned/deleted
  /// user). It does NOT capture seller verification/subscription state —
  /// that lives on the top-level seller.lifecycle slot ([sellerTrustLifecycle]).
  ///
  /// Defaults to active when wire is null/missing/unknown so legacy
  /// payloads keep rendering today's behavior.
  final ContentLifecycle sellerUserLifecycle;

  /// Expired-seller visibility — canonical seller-trust lifecycle
  /// ({active, unavailable}). Sourced from the wire's top-level
  /// `listing.seller.lifecycle` slot populated by backend coarsening of
  /// the latest `seller_subscriptions.status` row.
  ///
  /// AXIS BOUNDARY: SEPARATE from [sellerUserLifecycle]. User-axis
  /// degraded = "block / redact"; seller-trust degraded = "show + badge +
  /// disable transaction CTAs". The two are NEVER collapsed.
  ///
  /// Defaults to active when wire is null/missing so legacy payloads
  /// keep their existing render behavior.
  final ContentLifecycle sellerTrustLifecycle;

  /// Seller reputation tier — raw wire value from `listing.seller.tier`.
  ///
  /// Values: "pro", "elite". Null when backend emits no tier (flag
  /// disabled, user-identity/trust-axis degraded, or tier is Basic).
  ///
  /// RENDER RULE: SellerTierBadge hides for null/basic/unknown. Additional
  /// mobile gate: MUST NOT render when [sellerTrustLifecycle] is not active
  /// (expired subscription) — see _ListingSellerCard for enforcement.
  final String? sellerTier;

  final ForSaleStatus status;
  final ForSaleVisibility visibility;
  final bool isNegotiable;
  final int viewCount;
  final DateTime createdAt;
  final DateTime updatedAt;

  // Koi details - for checkout flow
  final String? variety;
  final double? sizeCm;
  final int? ageMonths;
  final String? gender;
  final String? breeder;
  final String? bloodline;

  const ForSale({
    required this.forSaleId,
    this.productId,
    required this.title,
    required this.description,
    required this.price,
    required this.stock,
    this.media = const [],
    required this.sellerId,
    this.sellerUsername,
    this.sellerFarmName,
    this.sellerAvatar,
    this.sellerUserLifecycle = ContentLifecycle.active,
    this.sellerTrustLifecycle = ContentLifecycle.active,
    this.sellerTier,
    required this.status,
    this.visibility = ForSaleVisibility.public,
    this.isNegotiable = false,
    this.viewCount = 0,
    this.preparationTime = PreparationTime.immediate,
    this.preparationNote,
    required this.createdAt,
    required this.updatedAt,
    this.variety,
    this.sizeCm,
    this.ageMonths,
    this.gender,
    this.breeder,
    this.bloodline,
  });

  // ═══════════════════════════════════════════════════════════════════════════════
  // MARKET OPERABILITY (buyability/purchasability)
  // ═══════════════════════════════════════════════════════════════════════════════

  /// Available for purchase: must be active (published) AND have stock
  /// Draft listings are NEVER available for purchase.
  ///
  /// NOTE: Visibility check is redundant because ACTIVE = PUBLIC ONLY (enforced by backend)
  /// This is the authoritative buyability check - use this everywhere
  bool get isAvailable => status == ForSaleStatus.active && stock > 0;

  /// Returns true if this is a workspace forSale (not in market)
  /// Workspace forSales: draft status only
  /// NOTE: active + private is INVALID (backend rejects this combination)
  bool get isWorkspaceForSale => status == ForSaleStatus.draft;

  /// Returns true if this is a market forSale (visible to buyers)
  /// Market forSales: active status (automatically public due to ACTIVE = PUBLIC ONLY)
  /// Draft forSales are NEVER market forSales
  bool get isMarketForSale => status == ForSaleStatus.active;

  /// Sold out or unavailable: no stock OR sold OR withdrawn
  /// Note: draft forSales are not "sold out", they're just not published
  bool get isSoldOut =>
      stock <= 0 ||
      status == ForSaleStatus.sold ||
      status == ForSaleStatus.withdrawn;

  // ═══════════════════════════════════════════════════════════════════════════════
  // EDITABILITY (mutability)
  // ═══════════════════════════════════════════════════════════════════════════════

  /// Returns true if this forSale can be edited by the seller
  /// Editable when: NOT in terminal state (sold/withdrawn)
  /// Draft and active forSales are editable
  bool get isEditable => !status.isTerminal;

  /// Returns true if this forSale is historical/reference only
  /// Historical forSales: terminal states (sold, withdrawn)
  bool get isHistorical => status.isTerminal;

  /// Returns true if this forSale can be withdrawn
  /// Can be withdrawn when: draft or active
  bool get canBeWithdrawn =>
      status == ForSaleStatus.draft || status == ForSaleStatus.active;

  /// Returns true if this forSale can be published
  /// Can be published when: draft status
  bool get canBePublished => status == ForSaleStatus.draft;

  String get formattedPrice {
    return 'Rp ${price.toStringAsFixed(0).replaceAllMapped(RegExp(r'(\d{1,3})(?=(\d{3})+(?!\d))'), (Match m) => '${m[1]}.')}';
  }

  ForSale copyWith({
    String? forSaleId,
    String? productId,
    String? title,
    String? description,
    double? price,
    int? stock,
    List<MediaEntity>? media,
    String? sellerId,
    String? sellerUsername,
    String? sellerFarmName,
    String? sellerAvatar,
    ContentLifecycle? sellerUserLifecycle,
    ContentLifecycle? sellerTrustLifecycle,
    String? sellerTier,
    ForSaleStatus? status,
    ForSaleVisibility? visibility,
    bool? isNegotiable,
    int? viewCount,
    PreparationTime? preparationTime,
    String? preparationNote,
    DateTime? createdAt,
    DateTime? updatedAt,
    String? variety,
    double? sizeCm,
    int? ageMonths,
    String? gender,
    String? breeder,
    String? bloodline,
  }) {
    return ForSale(
      forSaleId: forSaleId ?? this.forSaleId,
      productId: productId ?? this.productId,
      title: title ?? this.title,
      description: description ?? this.description,
      price: price ?? this.price,
      stock: stock ?? this.stock,
      media: media ?? this.media,
      sellerId: sellerId ?? this.sellerId,
      sellerUsername: sellerUsername ?? this.sellerUsername,
      sellerFarmName: sellerFarmName ?? this.sellerFarmName,
      sellerAvatar: sellerAvatar ?? this.sellerAvatar,
      sellerUserLifecycle: sellerUserLifecycle ?? this.sellerUserLifecycle,
      sellerTrustLifecycle: sellerTrustLifecycle ?? this.sellerTrustLifecycle,
      sellerTier: sellerTier ?? this.sellerTier,
      status: status ?? this.status,
      visibility: visibility ?? this.visibility,
      isNegotiable: isNegotiable ?? this.isNegotiable,
      viewCount: viewCount ?? this.viewCount,
      preparationTime: preparationTime ?? this.preparationTime,
      preparationNote: preparationNote ?? this.preparationNote,
      createdAt: createdAt ?? this.createdAt,
      updatedAt: updatedAt ?? this.updatedAt,
      variety: variety ?? this.variety,
      sizeCm: sizeCm ?? this.sizeCm,
      ageMonths: ageMonths ?? this.ageMonths,
      gender: gender ?? this.gender,
      breeder: breeder ?? this.breeder,
      bloodline: bloodline ?? this.bloodline,
    );
  }

  @override
  List<Object?> get props => [
    forSaleId,
    productId,
    title,
    description,
    price,
    stock,
    media,
    sellerId,
    sellerUsername,
    sellerFarmName,
    sellerAvatar,
    sellerUserLifecycle,
    sellerTrustLifecycle,
    sellerTier,
    status,
    visibility,
    isNegotiable,
    viewCount,
    preparationTime,
    preparationNote,
    createdAt,
    updatedAt,
    variety,
    sizeCm,
    ageMonths,
    gender,
    breeder,
    bloodline,
  ];
}

// =============================================================================
// ForSale Status Enum
// =============================================================================

/// ForSale status (lifecycle state)
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// CANONICAL LIFECYCLE (backend-authoritative):
/// ═══════════════════════════════════════════════════════════════════════════════
/// Lifecycle flow: draft -> active (published) -> sold/withdrawn
///
/// - draft:      Workspace-only, NOT yet published to market.
///               Seller can create/edit without active subscription.
///               NOT visible to buyers, NOT purchasable.
///               MUST BE private visibility.
///
/// - active:     Published and market-ready. ALWAYS PUBLIC (ACTIVE = PUBLIC ONLY).
///               Visible to buyers if stock > 0. Purchasable when all conditions met.
///               Requires active seller subscription to reach this state.
///               Automatically sets visibility to public on publish.
///
/// - sold:       Terminal state. All stock sold out. NOT editable, NOT purchasable.
///
/// - withdrawn:  Terminal state. Seller removed from sale. NOT editable, NOT purchasable.
///
/// ═══════════════════════════════════════════════════════════════════════════════
/// HARD RULE: STATUS → VISIBILITY MAPPING
/// ═══════════════════════════════════════════════════════════════════════════════
/// - Draft forSales: MUST BE private (workspace-only)
/// - Active forSales: MUST BE public (enforced by backend on publish)
/// - Terminal states: visibility irrelevant
///
/// A forSale can be:
/// - draft + private:    Workspace draft (ONLY valid draft state)
/// - draft + public:     Invalid (visibility ignored for draft)
/// - active + private:   INVALID (rejected by backend)
/// - active + public:    Full market forSale (ONLY valid active state)
/// ═══════════════════════════════════════════════════════════════════════════════
enum ForSaleStatus {
  /// Workspace-only, NOT yet published to market
  /// Seller can create/edit without active subscription
  /// MUST BE private visibility
  draft,

  /// Published and market-visible (purchasable if stock > 0)
  /// ALWAYS PUBLIC (ACTIVE = PUBLIC ONLY invariant)
  /// Requires active seller subscription to reach this state
  /// Backend automatically sets visibility to public on publish
  active,

  /// Seller removed from sale (TERMINAL state - cannot be reversed)
  withdrawn,

  /// Successfully sold (TERMINAL state - cannot be reversed)
  sold;

  String get displayName {
    switch (this) {
      case ForSaleStatus.draft:
        return 'Draft';
      case ForSaleStatus.active:
        return 'Active';
      case ForSaleStatus.withdrawn:
        return 'Withdrawn';
      case ForSaleStatus.sold:
        return 'Sold';
    }
  }

  /// Returns true if this forSale is available for commerce actions.
  /// Only active (published) forSales with positive stock can be purchased/negotiated.
  /// Draft forSales are NOT available for commerce.
  bool get isAvailableForCommerce => this == ForSaleStatus.active;

  /// Returns true if this forSale is in draft state (not yet published).
  bool get isDraft => this == ForSaleStatus.draft;

  /// Returns true if this forSale has been published (active state).
  bool get isPublished => this == ForSaleStatus.active;

  /// Returns true if this forSale is in a terminal state (cannot be changed).
  /// Terminal states: sold, withdrawn
  bool get isTerminal =>
      this == ForSaleStatus.sold || this == ForSaleStatus.withdrawn;
}

// =============================================================================
// Request DTOs (Domain Layer)
// =============================================================================

/// Get forSales filter parameters
class GetForSalesParams {
  final int page;
  final int limit;
  final ForSaleStatus? status;
  final String? searchQuery;
  final String? sellerId;
  final double? minPrice;
  final double? maxPrice;

  const GetForSalesParams({
    this.page = 1,
    this.limit = 20,
    this.status,
    this.searchQuery,
    this.sellerId,
    this.minPrice,
    this.maxPrice,
  });

  GetForSalesParams copyWith({
    int? page,
    int? limit,
    ForSaleStatus? status,
    String? searchQuery,
    String? sellerId,
    double? minPrice,
    double? maxPrice,
  }) {
    return GetForSalesParams(
      page: page ?? this.page,
      limit: limit ?? this.limit,
      status: status ?? this.status,
      searchQuery: searchQuery ?? this.searchQuery,
      sellerId: sellerId ?? this.sellerId,
      minPrice: minPrice ?? this.minPrice,
      maxPrice: maxPrice ?? this.maxPrice,
    );
  }

  @override
  String toString() {
    return 'GetForSalesParams(page: $page, limit: $limit, status: $status)';
  }
}

/// Create forSale request
class CreateForSaleRequest {
  final String title;
  final String description;
  final double price;
  final int quantity;
  final bool negotiationEnabled;
  final String visibility;
  final List<String> mediaUrls;
  final String? variety;
  final double? sizeCm;
  final int? ageMonths;
  final String? gender;
  final String? breeder;
  final String? bloodline;
  final List<String> certificates;
  final String? farmAddressId;
  // Shipping readiness
  final PreparationTime? preparationTime;
  final String? preparationNote;

  const CreateForSaleRequest({
    required this.title,
    required this.description,
    required this.price,
    required this.quantity,
    this.negotiationEnabled = false,
    this.visibility = 'public',
    this.mediaUrls = const [],
    this.variety,
    this.sizeCm,
    this.ageMonths,
    this.gender,
    this.breeder,
    this.bloodline,
    this.certificates = const [],
    this.farmAddressId,
    this.preparationTime,
    this.preparationNote,
  });
}

/// Update forSale request
class UpdateForSaleRequest {
  final String? title;
  final String? description;
  final double? price;
  final int? quantity;
  final bool? negotiationEnabled;
  final ForSaleStatus? status; // For publishing draft → active
  final List<String>? mediaUrls;
  final String? variety;
  final double? sizeCm;
  final int? ageMonths;
  final String? gender;
  final String? breeder;
  final String? bloodline;
  final List<String>? certificates;
  // Shipping readiness
  final PreparationTime? preparationTime;
  final String? preparationNote;

  const UpdateForSaleRequest({
    this.title,
    this.description,
    this.price,
    this.quantity,
    this.negotiationEnabled,
    this.status,
    this.mediaUrls,
    this.variety,
    this.sizeCm,
    this.ageMonths,
    this.gender,
    this.breeder,
    this.bloodline,
    this.certificates,
    this.preparationTime,
    this.preparationNote,
  });
}
