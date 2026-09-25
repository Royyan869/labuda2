/// Auction Data Transfer Objects (DTOs)
/// These models match the API response structure from Go backend
///
/// ===========================================================================
/// SAFETY RULE: AUCTION INVENTORY INDEPENDENCE
/// ===========================================================================
/// Auction inventory is COMPLETELY INDEPENDENT from fixed-price sale inventory.
///
/// - Auction and fixed-price sale are SEPARATE commerce entities
/// - Winning an auction does NOT affect product stock
/// - Selling a product does NOT affect auction availability
///
/// PASS_21B: auction creation no longer references a product/forSale ID at
/// all — the backend creates the Product inline from the request's item
/// fields (see CreateAuctionDto below). `productId` still appears on the
/// *response* DTO ([AuctionDto]) once the backend has created it, purely as
/// read-only metadata; it is never sent on create.
/// ===========================================================================
library;

import 'package:equatable/equatable.dart';
import 'package:labuda/domains/commerce/catalog/shared/domain/entities/commerce_viewer_capabilities.dart';

// =============================================================================
// Request DTOs
// =============================================================================

/// Request to create a new auction.
///
/// PASS_21B: a Product is created inline by the backend from this request's
/// item fields (title/variety/size/age/gender/breeder/bloodline/media) — the
/// same pattern CreateFixedPriceSaleRequest already used. There is no
/// product_id/for_sale_id on this request: auction must never be sourced
/// from an existing ForSale, and there is no "attach to existing product"
/// shape either.
///
/// TIMING (PASS_18C): the seller picks a start mode and duration; the
/// backend computes and enforces start_at/end_at server-side (1-7 day
/// bound). Anti-sniping soft-close (5-minute window, 5-minute extension,
/// 30-minute cap) is implemented backend-side in PlaceBid.
///
/// CONTRACT PARITY (PASS_18E/PASS_21B): field names and shape match the
/// backend's `CreateAuctionRequest` exactly
/// (`internal/commerce/auction/delivery/http`). `shippingSetupIds` is
/// REQUIRED — backend rejects creation with `min=1` binding, and auction is
/// still a physical fish that must ship. No stale `images`/`category`/
/// `condition`/`product_id`/`for_sale_id` keys are kept.
class CreateAuctionDto {
  final String title;
  final String? description;
  final List<String> mediaUrls;
  final String? variety;
  final int? sizeCm;
  final int? ageMonths;
  final String? gender;
  final String? breeder;
  final String? bloodline;
  final List<String>? certificates;
  final String? farmAddressId;

  /// Required — backend rejects creation without at least one option.
  final List<String> shippingSetupIds;

  final int startPrice;
  final int? bidIncrement;
  final int? buyNowPrice;

  /// "now" (immediate start) or "scheduled" (custom future start).
  final String startMode;

  /// Required when [startMode] is "scheduled"; ignored otherwise.
  final DateTime? scheduledStartAt;

  /// How long the auction runs. Backend enforces 24-168 (1-7 days).
  final int durationHours;

  final String? preparationNote;

  const CreateAuctionDto({
    required this.title,
    this.description,
    required this.mediaUrls,
    this.variety,
    this.sizeCm,
    this.ageMonths,
    this.gender,
    this.breeder,
    this.bloodline,
    this.certificates,
    this.farmAddressId,
    required this.shippingSetupIds,
    required this.startPrice,
    this.bidIncrement,
    this.buyNowPrice,
    required this.startMode,
    this.scheduledStartAt,
    required this.durationHours,
    this.preparationNote,
  });

  Map<String, dynamic> toJson() => {
    'title': title,
    if (description != null) 'description': description,
    'media_urls': mediaUrls,
    if (variety != null) 'variety': variety,
    if (sizeCm != null) 'size_cm': sizeCm,
    if (ageMonths != null) 'age_months': ageMonths,
    if (gender != null) 'gender': gender,
    if (breeder != null) 'breeder': breeder,
    if (bloodline != null) 'bloodline': bloodline,
    if (certificates != null) 'certificates': certificates,
    if (farmAddressId != null) 'farm_address_id': farmAddressId,
    'shipping_option_ids': shippingSetupIds,
    'start_price': startPrice,
    if (bidIncrement != null) 'bid_increment': bidIncrement,
    if (buyNowPrice != null) 'buy_now_price': buyNowPrice,
    'start_mode': startMode,
    if (scheduledStartAt != null)
      'scheduled_start_at': scheduledStartAt!.toIso8601String(),
    'duration_hours': durationHours,
    if (preparationNote != null) 'preparation_note': preparationNote,
  };
}

/// Request to update an auction — CANONICAL UPDATE CONTRACT (F2.2B).
///
/// Draft allowed: title, description, startPrice, bidIncrement, buyNowPrice, startTime, endTime
/// Scheduled allowed: title, description, startTime, endTime
///
/// Backend persists title/description → products, and pricing/timing → auctions
/// in ONE transaction. Unsupported legacy fields (images/category/condition/
/// auto_extend*) are NOT part of this contract and will be rejected by the
/// backend if sent — the canonical client never sends them.
class UpdateAuctionDto {
  final String? title;
  final String? description;
  final int? startPrice;
  final int? bidIncrement;
  final int? buyNowPrice;
  final DateTime? startTime;
  final DateTime? endTime;

  const UpdateAuctionDto({
    this.title,
    this.description,
    this.startPrice,
    this.bidIncrement,
    this.buyNowPrice,
    this.startTime,
    this.endTime,
  });

  Map<String, dynamic> toJson() {
    final map = <String, dynamic>{};
    if (title != null) map['title'] = title;
    if (description != null) map['description'] = description;
    if (startPrice != null) map['start_price'] = startPrice;
    if (bidIncrement != null) map['bid_increment'] = bidIncrement;
    if (buyNowPrice != null) map['buy_now_price'] = buyNowPrice;
    if (startTime != null) map['start_at'] = startTime!.toIso8601String();
    if (endTime != null) map['end_at'] = endTime!.toIso8601String();
    return map;
  }
}

/// Request to place a bid
class PlaceBidDto {
  final int amount;
  final String? idempotencyKey;

  const PlaceBidDto({required this.amount, this.idempotencyKey});

  Map<String, dynamic> toJson() => {
    'amount': amount,
    'idempotency_key':
        idempotencyKey ?? 'bid_${DateTime.now().microsecondsSinceEpoch}',
  };
}

/// Request to cancel auction
class CancelAuctionDto {
  final String reason;

  const CancelAuctionDto({required this.reason});

  Map<String, dynamic> toJson() => {'reason': reason};
}

// =============================================================================
// Response DTOs
// =============================================================================

/// User brief info from API
///
/// Owner Truth: public account identity is `username`. `full_name` is KYC
/// and is NEVER consumed on this surface.
///
/// D14 — Auction bid discovery governance convergence.
/// Backend `/auctions/:id/bids` now emits this DTO as a nested
/// `publiccard.UserCard` carrying coarsened lifecycle. The additive
/// fields `avatarUrl` and `lifecycle` are tolerated when present; older
/// payloads that omit them continue to parse as before.
class UserBriefDto extends Equatable {
  final String id;
  final String username;
  final String? avatarUrl;
  // D14 — coarsened public lifecycle: "active" | "unavailable" | "removed".
  // Null on older payloads (rollback-safe).
  final String? lifecycle;

  const UserBriefDto({
    required this.id,
    required this.username,
    this.avatarUrl,
    this.lifecycle,
  });

  factory UserBriefDto.fromJson(Map<String, dynamic> json) {
    final lc = json['lifecycle'];
    final avatar = json['avatar_url'];
    return UserBriefDto(
      id: json['id'] as String,
      username: (json['username'] as String?) ?? '',
      avatarUrl: avatar is String && avatar.isNotEmpty ? avatar : null,
      lifecycle: lc is String && lc.isNotEmpty ? lc : null,
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'username': username,
    if (avatarUrl != null) 'avatar_url': avatarUrl,
    if (lifecycle != null) 'lifecycle': lifecycle,
  };

  @override
  List<Object?> get props => [id, username, avatarUrl, lifecycle];
}

/// Auction response from API
///
/// SAFETY: productId is OPTIONAL metadata-only field.
/// Auction has independent inventory - no stock sharing with product.
///
/// CANONICAL WIRE (backend auctionToResponseWithSeller):
/// Only backend-emitted keys are parsed here. Phantom keys that the backend
/// never emits (total_bids, views_count, settlement_deadline, started_at,
/// ended_at, auto_extend*, original_end_time, user_bid, highest_bidder,
/// category, condition, legacy flat seller/bidder scalars) are PURGED —
/// parsing them recreated fake truths (always-zero counters, dead state).
/// The settlement deadline is DERIVED downstream from end_at + 24h
/// (canonical backend rule: Auction.SettlementDeadline()).
/// Typed media item from the backend detail wire — CONVERGED shape,
/// identical to for_sale's ForSaleMediaItemDto (same fields, same parser
/// tolerance) so both Product surfaces render through one model contract.
class AuctionMediaItemDto {
  final String id;
  final String type; // "image" or "video"
  final String url;
  final int position;
  final String? thumbnailUrl;
  final int? width;
  final int? height;
  final int? duration;
  final DateTime? createdAt;

  const AuctionMediaItemDto({
    required this.id,
    required this.type,
    required this.url,
    required this.position,
    this.thumbnailUrl,
    this.width,
    this.height,
    this.duration,
    this.createdAt,
  });

  factory AuctionMediaItemDto.fromJson(Map<String, dynamic> json) {
    final thumbnail = json['thumbnail_url'];
    final createdAtRaw = json['created_at'];
    return AuctionMediaItemDto(
      id: json['id'] as String? ?? '',
      type: json['type'] as String? ?? 'image',
      url: json['url'] as String? ?? '',
      position: json['position'] as int? ?? 0,
      // Backend renders thumbnail_url as "" when absent — normalize to null
      // so callers treat it as genuinely absent instead of an empty URL.
      thumbnailUrl: (thumbnail is String && thumbnail.isNotEmpty) ? thumbnail : null,
      width: (json['width'] as num?)?.toInt(),
      height: (json['height'] as num?)?.toInt(),
      duration: (json['duration'] as num?)?.toInt(),
      createdAt:
          (createdAtRaw is String && createdAtRaw.isNotEmpty)
          ? DateTime.tryParse(createdAtRaw)
          : null,
    );
  }

  bool get isVideo => type == 'video';
}

class AuctionDto extends Equatable {
  final String id;
  final String sellerId;

  /// OPTIONAL metadata field - traces origin only.
  /// Does NOT link inventory. Auction has independent stock.
  final String? productId;
  final String title;
  final String? description;
  final List<String> images;

  /// Typed media block from the detail wire (backend commerce/shared
  /// MediaWireItems) — the converged projection of Product.MediaURLs.
  /// Empty/absent on list payloads; string [images] remains the universal
  /// fallback so the parser stays shape-agnostic.
  final List<AuctionMediaItemDto> mediaItems;

  // =========================================================================
  // CANONICAL DETAIL CONTENT (backend Product projection on the detail wire)
  // =========================================================================
  // Present on GET /api/v1/auctions/:id (auctionToDetailResponseWithSeller);
  // absent/empty on list payloads — tolerated as null/[] so the parser is
  // shape-agnostic. Parsed here and mapped into the Auction read model so no
  // canonical Product content is dropped or replaced by synthetic defaults.
  final String? variety;
  final int? sizeCm;
  final int? ageMonths;
  final String? gender;
  final String? breeder;
  final String? bloodline;
  final List<String> certificates;
  final String? preparationTime;
  final String? preparationNote;

  // Canonical numeric read representation: backend emits int64/bigint JSON
  // integer literals (auctionToResponseWithSeller); int is the single
  // canonical representation — no double conversion on this chain.
  // currentBid is nullable — null means no bids yet (backend current_bid null).
  // Presentation fallback to startPrice is derived explicitly in the mapper from
  // factual fields, not hidden in DTO wire parsing.
  final int startPrice;
  final int bidIncrement;
  final int? buyNowPrice;
  final int? currentBid;
  final String? currentWinnerId;
  final DateTime startTime;
  final DateTime endTime;
  final String status;
  final DateTime createdAt;
  final DateTime updatedAt;

  // ===========================================================================
  // STAGE 2 — IDENTITY PARSE-ONLY FIELDS (Phase 5)
  // ===========================================================================
  // Owner-truth identity scalars emitted by the backend at auction top-level.
  // - seller_username   = account/user identity
  // - seller_farm_name  = seller/store identity (Owner Truth: farm name)
  // - seller_avatar_url = display avatar
  final String? sellerUsername;
  final String? sellerFarmName;
  final String? sellerAvatarUrl;

  /// E8.2 — Canonical seller user-identity lifecycle.
  ///
  /// Sourced at parse time from the wire's nested
  /// `auction.seller.user.lifecycle` slot populated by E8.1's
  /// publiccard.NewSellerCardWithUserLifecycle. Tolerant: null / missing /
  /// unknown / empty → null here (mapper converts to ContentLifecycle.active).
  ///
  /// AXIS BOUNDARY: This field carries ONLY the user-identity axis
  /// (banned/deleted user). Seller-trust/capability axis is on
  /// [sellerTrustLifecycle].
  final String? sellerUserLifecycle;

  /// Expired-seller visibility — seller-trust lifecycle from the wire's
  /// top-level `auction.seller.lifecycle` slot. Populated by backend
  /// coarsening of the latest `seller_subscriptions.status` row.
  /// Tolerant: null when missing — mapper defaults to active so legacy
  /// payloads keep current render behavior.
  final String? sellerTrustLifecycle;

  /// Seller tier — raw wire value from `auction.seller.tier`.
  /// Populated by backend GatedSellerTier when
  /// ENABLE_PUBLIC_SELLER_TIER_PROFILE is true and all lifecycle gates pass.
  /// Values: "pro", "elite". Null when gated out or flag disabled.
  final String? sellerTier;

  // =========================================================================
  // CANONICAL DETAIL ACTION AUTHORITY — viewer_capabilities
  // =========================================================================
  // Present on GET /api/v1/auctions/:id (auctionToDetailResponseWithSeller →
  // commerceshared.EvaluateAuctionViewerCapabilities). Absent on list/discovery
  // payloads — tolerated as null so the parser stays shape-agnostic and the
  // shared Auction read model keeps working on list surfaces.
  final CommerceViewerCapabilities? viewerCapabilities;

  const AuctionDto({
    required this.id,
    required this.sellerId,
    this.productId,
    required this.title,
    this.description,
    this.images = const [],
    this.mediaItems = const [],
    this.variety,
    this.sizeCm,
    this.ageMonths,
    this.gender,
    this.breeder,
    this.bloodline,
    this.certificates = const [],
    this.preparationTime,
    this.preparationNote,
    required this.startPrice,
    required this.bidIncrement,
    this.buyNowPrice,
    this.currentBid,
    this.currentWinnerId,
    required this.startTime,
    required this.endTime,
    required this.status,
    required this.createdAt,
    required this.updatedAt,
    // Stage 2 identity parse-only fields
    this.sellerUsername,
    this.sellerFarmName,
    this.sellerAvatarUrl,
    // E8.2 seller user-axis lifecycle (nested wire slot)
    this.sellerUserLifecycle,
    // Expired-seller visibility — top-level seller-trust lifecycle.
    this.sellerTrustLifecycle,
    // Stage 2 seller tier badge.
    this.sellerTier,
    // Canonical detail action authority.
    this.viewerCapabilities,
  });

  factory AuctionDto.fromJson(Map<String, dynamic> json) {
    final startAtRaw = json['start_at'] ?? json['start_time'];
    final endAtRaw = json['end_at'] ?? json['end_time'];
    // Canonical: only backend-emitted keys. No phantom fallback.
    // current_bid is nullable — null means no bids yet; presentation
    // derivation lives in mapper from factual fields.
    final currentBidRaw = json['current_bid'];
    // Canonical winner authority is current_winner_id only.
    final winnerRaw = json['current_winner_id'];
    final imagesRaw = json['images'] as List<dynamic>?;
    final mediaUrlsRaw = json['media_urls'] as List<dynamic>?;
    final mediaRaw = json['media'] as List<dynamic>?;
    final mediaItems =
        (mediaRaw ?? const [])
            .whereType<Map<String, dynamic>>()
            .map(AuctionMediaItemDto.fromJson)
            .toList();
    final normalizedImages = (imagesRaw ?? mediaUrlsRaw ?? mediaRaw ?? const [])
        .map((e) {
          if (e is String) return e;
          if (e is Map<String, dynamic>) {
            final url = e['url'] ?? e['original_url'] ?? e['image_url'];
            if (url is String) return url;
          }
          return null;
        })
        .whereType<String>()
        .toList();

    return AuctionDto(
      id: json['id'] as String,
      sellerId: json['seller_id'] as String,
      productId: json['product_id'] as String?,
      title: json['title'] as String,
      description: json['description'] as String?,
      images: normalizedImages,
      mediaItems: mediaItems,
      variety: json['variety'] as String?,
      sizeCm: (json['size_cm'] as num?)?.toInt(),
      ageMonths: (json['age_months'] as num?)?.toInt(),
      gender: json['gender'] as String?,
      breeder: json['breeder'] as String?,
      bloodline: json['bloodline'] as String?,
      certificates:
          (json['certificates'] as List?)?.whereType<String>().toList() ??
          const [],
      preparationTime: json['preparation_time'] as String?,
      preparationNote: json['preparation_note'] as String?,
      startPrice: (json['start_price'] as num).toInt(),
      bidIncrement: (json['bid_increment'] as num).toInt(),
      buyNowPrice: (json['buy_now_price'] as num?)?.toInt(),
      currentBid: (currentBidRaw as num?)?.toInt(),
      currentWinnerId: winnerRaw as String?,
      startTime: DateTime.parse(startAtRaw as String),
      endTime: DateTime.parse(endAtRaw as String),
      status: json['status'] as String,
      createdAt: DateTime.parse(json['created_at'] as String),
      updatedAt: DateTime.parse(json['updated_at'] as String),
      // Stage 2 identity parse-only fields. Tolerate old payload (null) and
      // new payload. No fullName fallback — owner truth is username/farm.
      sellerUsername: json['seller_username'] as String?,
      sellerFarmName: json['seller_farm_name'] as String?,
      sellerAvatarUrl: json['seller_avatar_url'] as String?,
      // E8.2 — Walk the nested canonical PublicCard wire slot
      // (`auction.seller.user.lifecycle`). Pre-E8.1 payloads omit it →
      // null fall-through.
      sellerUserLifecycle: _readAuctionSellerUserLifecycle(json),
      // Expired-seller visibility — walk the top-level seller-trust slot
      // (`auction.seller.lifecycle`). Independent axis.
      sellerTrustLifecycle: _readAuctionSellerTrustLifecycle(json),
      // Stage 2 — seller reputation tier badge from `auction.seller.tier`.
      sellerTier: _readAuctionSellerTier(json),
      // Canonical detail action authority. Null when the payload is a
      // list/discovery item that does not carry viewer-scoped capabilities.
      viewerCapabilities: json['viewer_capabilities'] is Map<String, dynamic>
          ? CommerceViewerCapabilities.fromJson(
              json['viewer_capabilities'] as Map<String, dynamic>,
            )
          : null,
    );
  }

  @override
  List<Object?> get props => [id, sellerId, title, status, images, mediaItems];
}

/// E8.2 — Extract the embedded seller user-identity lifecycle string from
/// the auction wire shape. Walks `auction.seller.user.lifecycle`; returns
/// null when any segment is absent / empty / not a string.
///
/// AXIS BOUNDARY: This walker only reaches the USER axis. The top-level
/// `auction.seller.lifecycle` slot is doctrine-reserved (seller trust /
/// capability axis) and is NEVER read here.
String? _readAuctionSellerUserLifecycle(Map<String, dynamic> json) {
  final auction = json['auction'];
  if (auction is Map<String, dynamic>) {
    final seller = auction['seller'];
    if (seller is Map<String, dynamic>) {
      final user = seller['user'];
      if (user is Map<String, dynamic>) {
        final lc = user['lifecycle'];
        if (lc is String && lc.isNotEmpty) return lc;
      }
    }
  }
  return null;
}

/// Expired-seller visibility — Extract the top-level seller-trust lifecycle
/// string from the auction wire shape. Walks `auction.seller.lifecycle`
/// (NOT `auction.seller.user.lifecycle`); returns null when any segment is
/// absent / not a non-empty string. Mapper defaults null to
/// [ContentLifecycle.active] for legacy-payload compatibility.
String? _readAuctionSellerTrustLifecycle(Map<String, dynamic> json) {
  final auction = json['auction'];
  if (auction is Map<String, dynamic>) {
    final seller = auction['seller'];
    if (seller is Map<String, dynamic>) {
      final lc = seller['lifecycle'];
      if (lc is String && lc.isNotEmpty) return lc;
    }
  }
  return null;
}

/// Stage 2 — Extract the seller tier string from the auction wire shape.
/// Walks `auction.seller.tier`; returns null when any segment is absent /
/// not a non-empty string. Mobile renders via SellerTierBadge which hides
/// for null/basic/unknown — no additional mapping needed at DTO level.
String? _readAuctionSellerTier(Map<String, dynamic> json) {
  final auction = json['auction'];
  if (auction is Map<String, dynamic>) {
    final seller = auction['seller'];
    if (seller is Map<String, dynamic>) {
      final tier = seller['tier'];
      if (tier is String && tier.isNotEmpty) return tier;
    }
  }
  return null;
}

/// Bid response from API
///
/// Canonical wire: bidToResponseWithBidderCard — {id, auction_id, bidder_id,
/// amount, created_at, bidder: UserCard}. Phantom keys the backend never
/// emits (is_winning, is_outbid, bid_time, bidder_username) are PURGED;
/// buyer bid-position authority lives in GET /api/v1/bidding, not here.
class BidDto extends Equatable {
  final String id;
  final String auctionId;
  final String bidderId;
  final int amount;
  final DateTime createdAt;
  final UserBriefDto? bidder;

  const BidDto({
    required this.id,
    required this.auctionId,
    required this.bidderId,
    required this.amount,
    required this.createdAt,
    this.bidder,
  });

  factory BidDto.fromJson(Map<String, dynamic> json) {
    return BidDto(
      id: json['id'] as String,
      auctionId: json['auction_id'] as String,
      bidderId: json['bidder_id'] as String,
      amount: (json['amount'] as num).toInt(),
      createdAt: DateTime.parse(json['created_at'] as String),
      bidder: json['bidder'] is Map<String, dynamic>
          ? UserBriefDto.fromJson(json['bidder'] as Map<String, dynamic>)
          : null,
    );
  }

  @override
  List<Object?> get props => [id, auctionId, bidderId, amount];
}
