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
/// PASS_21B: auction creation no longer references a product/listing ID at
/// all — the backend creates the Product inline from the request's item
/// fields (see CreateAuctionDto below). `productId` still appears on the
/// *response* DTO ([AuctionDto]) once the backend has created it, purely as
/// read-only metadata; it is never sent on create.
/// ===========================================================================
library;

import 'package:equatable/equatable.dart';

// =============================================================================
// Request DTOs
// =============================================================================

/// Request to create a new auction.
///
/// PASS_21B: a Product is created inline by the backend from this request's
/// item fields (title/variety/size/age/gender/breeder/bloodline/media) — the
/// same pattern CreateFixedPriceSaleRequest already used. There is no
/// product_id/listing_id on this request: auction must never be sourced
/// from an existing Listing, and there is no "attach to existing product"
/// shape either.
///
/// TIMING (PASS_18C): the seller picks a start mode and duration; the
/// backend computes and enforces start_at/end_at server-side (1-7 day
/// bound). Anti-sniping soft-close (5-minute window, 5-minute extension,
/// 30-minute cap) is implemented backend-side in PlaceBid.
///
/// CONTRACT PARITY (PASS_18E/PASS_21B): field names and shape match the
/// backend's `CreateAuctionRequest` exactly
/// (`internal/commerce/auction/delivery/http`). `shippingOptionIds` is
/// REQUIRED — backend rejects creation with `min=1` binding, and auction is
/// still a physical fish that must ship. No stale `images`/`category`/
/// `condition`/`product_id`/`listing_id` keys are kept.
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
  final List<String> shippingOptionIds;

  final double startPrice;
  final double? bidIncrement;
  final double? buyNowPrice;

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
    required this.shippingOptionIds,
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
    'shipping_option_ids': shippingOptionIds,
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

/// Request to update an auction
///
/// autoExtend/autoExtendMinutes client fields are not part of this request —
/// anti-sniping soft-close is entirely backend-computed (PASS_18C, PlaceBid),
/// not client-configurable.
///
/// NOT YET ALIGNED (out of scope for PASS_18E): this DTO still uses raw
/// startTime/endTime and the old images/category keys, unlike
/// CreateAuctionDto. The auction edit route is currently unreachable
/// (no navigation ever passes auctionToEdit — see create_auction_screen.dart),
/// so this has not been prioritized. Align this DTO with CreateAuctionDto's
/// contract if/when the edit flow is actually wired up.
class UpdateAuctionDto {
  final String? title;
  final String? description;
  final List<String>? images;
  final String? category;
  final String? condition;
  final double? startPrice;
  final double? bidIncrement;
  final double? buyNowPrice;
  final DateTime? startTime;
  final DateTime? endTime;

  const UpdateAuctionDto({
    this.title,
    this.description,
    this.images,
    this.category,
    this.condition,
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
    if (images != null) map['images'] = images;
    if (category != null) map['category'] = category;
    if (condition != null) map['condition'] = condition;
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
  final double amount;
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

/// Auction winner response
class AuctionWinnerDto extends Equatable {
  final String id;
  final String auctionId;
  final String winnerId;
  final double winningBid;
  final String winMethod;
  final DateTime wonAt;
  final UserBriefDto? winner;

  const AuctionWinnerDto({
    required this.id,
    required this.auctionId,
    required this.winnerId,
    required this.winningBid,
    required this.winMethod,
    required this.wonAt,
    this.winner,
  });

  factory AuctionWinnerDto.fromJson(Map<String, dynamic> json) {
    return AuctionWinnerDto(
      id: json['id'] as String,
      auctionId: json['auction_id'] as String,
      winnerId: json['winner_id'] as String,
      winningBid: (json['winning_bid'] as num).toDouble(),
      winMethod: json['win_method'] as String,
      wonAt: DateTime.parse(json['won_at'] as String),
      winner: json['winner'] != null
          ? UserBriefDto.fromJson(json['winner'])
          : null,
    );
  }

  @override
  List<Object?> get props => [id, auctionId, winnerId, winningBid];
}

/// Auction response from API
///
/// SAFETY: productId is OPTIONAL metadata-only field.
/// Auction has independent inventory - no stock sharing with product.
///
/// FALSE FEATURE SIGNALS (PARKED): The following fields are PARSED from API
/// but are NOT used in domain logic because backend does NOT implement
/// anti-sniping in production code (only in tests):
/// - originalEndTime: Tracks original end time before auto-extend
/// - autoExtend: Whether auto-extend is enabled
/// - autoExtendMinutes: Minutes to extend when sniped
/// - autoExtendCount: Number of times auction has been extended
/// - remainingExtensions: Remaining extensions allowed
///
/// These fields are kept in DTO for API compatibility but are explicitly
/// NOT mapped to domain entity. See AuctionMapper.toEntity().
class AuctionDto extends Equatable {
  final String id;
  final String sellerId;

  /// OPTIONAL metadata field - traces origin only.
  /// Does NOT link inventory. Auction has independent stock.
  final String? productId;
  final String title;
  final String? description;
  final List<String> images;
  final String? category;
  final String? condition;
  final double startPrice;
  final double bidIncrement;
  final double? buyNowPrice;
  final double currentHighestBid;
  final String? highestBidderId;
  final int totalBids;
  final double minimumBid;
  final DateTime startTime;
  final DateTime endTime;
  final DateTime? originalEndTime;
  final DateTime? settlementDeadline;
  final int timeRemainingSeconds;
  final String status;
  final bool autoExtend;
  final int autoExtendMinutes;
  final int autoExtendCount;
  final int remainingExtensions;
  final int viewsCount;
  final int watchersCount;
  final bool canBid;
  final bool canBuyNow;
  final DateTime createdAt;
  final DateTime updatedAt;
  final DateTime? startedAt;
  final DateTime? endedAt;
  final UserBriefDto? seller;
  final UserBriefDto? highestBidder;
  final AuctionWinnerDto? winner;
  final bool isWatching;
  final BidDto? userBid;

  // ===========================================================================
  // STAGE 2 — IDENTITY PARSE-ONLY FIELDS (Phase 5)
  // ===========================================================================
  // Owner-truth identity scalars added by backend Stage 1 at auction top-level.
  // Receive-only plumbing: nested `seller` / `highestBidder` UserBriefDto
  // remains the field consumed by the auction mapper. Stage 3 will switch.
  // - seller_username   = account/user identity
  // - seller_farm_name  = seller/store identity (Owner Truth: farm name)
  // - seller_avatar_url = display avatar
  // - bidder_username   = highest bidder username scalar (auction-level)
  final String? sellerUsername;
  final String? sellerFarmName;
  final String? sellerAvatarUrl;
  final String? bidderUsername;

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

  const AuctionDto({
    required this.id,
    required this.sellerId,
    this.productId,
    required this.title,
    this.description,
    this.images = const [],
    this.category,
    this.condition,
    required this.startPrice,
    required this.bidIncrement,
    this.buyNowPrice,
    required this.currentHighestBid,
    this.highestBidderId,
    required this.totalBids,
    required this.minimumBid,
    required this.startTime,
    required this.endTime,
    this.originalEndTime,
    this.settlementDeadline,
    required this.timeRemainingSeconds,
    required this.status,
    required this.autoExtend,
    required this.autoExtendMinutes,
    required this.autoExtendCount,
    required this.remainingExtensions,
    required this.viewsCount,
    required this.watchersCount,
    required this.canBid,
    required this.canBuyNow,
    required this.createdAt,
    required this.updatedAt,
    this.startedAt,
    this.endedAt,
    this.seller,
    this.highestBidder,
    this.winner,
    this.isWatching = false,
    this.userBid,
    // Stage 2 identity parse-only fields
    this.sellerUsername,
    this.sellerFarmName,
    this.sellerAvatarUrl,
    this.bidderUsername,
    // E8.2 seller user-axis lifecycle (nested wire slot)
    this.sellerUserLifecycle,
    // Expired-seller visibility — top-level seller-trust lifecycle.
    this.sellerTrustLifecycle,
    // Stage 2 seller tier badge.
    this.sellerTier,
  });

  factory AuctionDto.fromJson(Map<String, dynamic> json) {
    final startAtRaw = json['start_at'] ?? json['start_time'];
    final endAtRaw = json['end_at'] ?? json['end_time'];
    final currentBidRaw =
        json['current_bid'] ??
        json['current_highest_bid'] ??
        json['start_price'];
    final winnerRaw = json['current_winner_id'] ?? json['highest_bidder_id'];
    final minimumBidRaw =
        json['minimum_bid'] ?? json['start_price'] ?? currentBidRaw;
    final imagesRaw = json['images'] as List<dynamic>?;
    final mediaUrlsRaw = json['media_urls'] as List<dynamic>?;
    final mediaRaw = json['media'] as List<dynamic>?;
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
      category: json['category'] as String?,
      condition: json['condition'] as String?,
      startPrice: (json['start_price'] as num).toDouble(),
      bidIncrement: (json['bid_increment'] as num).toDouble(),
      buyNowPrice: (json['buy_now_price'] as num?)?.toDouble(),
      currentHighestBid: (currentBidRaw as num).toDouble(),
      highestBidderId: winnerRaw as String?,
      totalBids: json['total_bids'] as int? ?? 0,
      minimumBid: (minimumBidRaw as num).toDouble(),
      startTime: DateTime.parse(startAtRaw as String),
      endTime: DateTime.parse(endAtRaw as String),
      originalEndTime: json['original_end_time'] != null
          ? DateTime.parse(json['original_end_time'] as String)
          : null,
      settlementDeadline: json['settlement_deadline'] != null
          ? DateTime.parse(json['settlement_deadline'] as String)
          : null,
      timeRemainingSeconds: json['time_remaining_seconds'] as int? ?? 0,
      status: json['status'] as String,
      autoExtend: json['auto_extend'] as bool? ?? false,
      autoExtendMinutes: json['auto_extend_minutes'] as int? ?? 10,
      autoExtendCount: json['auto_extend_count'] as int? ?? 0,
      remainingExtensions: json['remaining_extensions'] as int? ?? 3,
      viewsCount: json['views_count'] as int? ?? 0,
      watchersCount: json['watchers_count'] as int? ?? 0,
      canBid: json['can_bid'] as bool? ?? false,
      canBuyNow: json['can_buy_now'] as bool? ?? false,
      createdAt: DateTime.parse(json['created_at'] as String),
      updatedAt: DateTime.parse(json['updated_at'] as String),
      startedAt: json['started_at'] != null
          ? DateTime.parse(json['started_at'] as String)
          : null,
      endedAt: json['ended_at'] != null
          ? DateTime.parse(json['ended_at'] as String)
          : null,
      seller: json['seller'] != null
          ? UserBriefDto.fromJson(json['seller'])
          : null,
      highestBidder: json['highest_bidder'] != null
          ? UserBriefDto.fromJson(json['highest_bidder'])
          : null,
      winner: json['winner'] is Map<String, dynamic>
          ? AuctionWinnerDto.fromJson(json['winner'] as Map<String, dynamic>)
          : null,
      isWatching: json['is_watching'] as bool? ?? false,
      userBid: json['user_bid'] != null
          ? BidDto.fromJson(json['user_bid'])
          : null,
      // Stage 2 identity parse-only fields. Tolerate old payload (null) and
      // new payload. No fullName fallback — owner truth is username/farm.
      sellerUsername: json['seller_username'] as String?,
      sellerFarmName: json['seller_farm_name'] as String?,
      sellerAvatarUrl: json['seller_avatar_url'] as String?,
      bidderUsername: json['bidder_username'] as String?,
      // E8.2 — Walk the nested canonical PublicCard wire slot
      // (`auction.seller.user.lifecycle`). Pre-E8.1 payloads omit it →
      // null fall-through.
      sellerUserLifecycle: _readAuctionSellerUserLifecycle(json),
      // Expired-seller visibility — walk the top-level seller-trust slot
      // (`auction.seller.lifecycle`). Independent axis.
      sellerTrustLifecycle: _readAuctionSellerTrustLifecycle(json),
      // Stage 2 — seller reputation tier badge from `auction.seller.tier`.
      sellerTier: _readAuctionSellerTier(json),
    );
  }

  @override
  List<Object?> get props => [id, sellerId, title, status, settlementDeadline];
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
class BidDto extends Equatable {
  final String id;
  final String auctionId;
  final String bidderId;
  final double amount;
  final bool isWinning;
  final bool isOutbid;
  final DateTime bidTime;
  final DateTime createdAt;
  final UserBriefDto? bidder;

  // ===========================================================================
  // STAGE 2 — IDENTITY PARSE-ONLY FIELD (Phase 5)
  // ===========================================================================
  // Owner-truth bidder username scalar from backend Stage 1.
  // Receive-only plumbing: nested `bidder` UserBriefDto is still the field
  // consumed by AuctionMapper.toBidEntity. Stage 3 will switch the surface.
  // - bidder_username = bidder username scalar (bid-level)
  final String? bidderUsername;

  const BidDto({
    required this.id,
    required this.auctionId,
    required this.bidderId,
    required this.amount,
    required this.isWinning,
    required this.isOutbid,
    required this.bidTime,
    required this.createdAt,
    this.bidder,
    // Stage 2 identity parse-only field
    this.bidderUsername,
  });

  factory BidDto.fromJson(Map<String, dynamic> json) {
    final createdAtRaw = json['created_at'] ?? json['bid_time'];
    final createdAtString = createdAtRaw as String;
    return BidDto(
      id: json['id'] as String,
      auctionId: json['auction_id'] as String,
      bidderId: json['bidder_id'] as String,
      amount: (json['amount'] as num).toDouble(),
      isWinning: json['is_winning'] as bool? ?? false,
      isOutbid: json['is_outbid'] as bool? ?? false,
      bidTime: DateTime.parse(createdAtString),
      createdAt: DateTime.parse(createdAtString),
      bidder: json['bidder'] is Map<String, dynamic>
          ? UserBriefDto.fromJson(json['bidder'] as Map<String, dynamic>)
          : null,
      // Stage 2 identity parse-only field. Tolerate old payload (null) and
      // new payload. No fullName fallback — owner truth is username.
      bidderUsername: json['bidder_username'] as String?,
    );
  }

  @override
  List<Object?> get props => [id, auctionId, bidderId, amount];
}

/// Current bid info response
class CurrentBidDto extends Equatable {
  final String auctionId;
  final double currentHighestBid;
  final String? highestBidderId;
  final double minimumBid;
  final int totalBids;
  final int timeRemainingSeconds;
  final DateTime endTime;
  final bool isExtended;
  final String status;

  const CurrentBidDto({
    required this.auctionId,
    required this.currentHighestBid,
    this.highestBidderId,
    required this.minimumBid,
    required this.totalBids,
    required this.timeRemainingSeconds,
    required this.endTime,
    required this.isExtended,
    required this.status,
  });

  factory CurrentBidDto.fromJson(Map<String, dynamic> json) {
    return CurrentBidDto(
      auctionId: json['auction_id'] as String,
      currentHighestBid: (json['current_highest_bid'] as num).toDouble(),
      highestBidderId: json['highest_bidder_id'] as String?,
      minimumBid: (json['minimum_bid'] as num).toDouble(),
      totalBids: json['total_bids'] as int? ?? 0,
      timeRemainingSeconds: json['time_remaining_seconds'] as int? ?? 0,
      endTime: DateTime.parse(json['end_time'] as String),
      isExtended: json['is_extended'] as bool? ?? false,
      status: json['status'] as String,
    );
  }

  @override
  List<Object?> get props => [auctionId, currentHighestBid, status];
}
