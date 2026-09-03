/// Auction Mapper
/// Converts between API DTOs and Domain Entities
///
/// SEMANTIC TRUTH:
/// - Auction entity has INDEPENDENT business data
/// - productId is OPTIONAL metadata for checkout integration
/// - Auction data is complete and self-contained
/// - Domain layer has no dependency on fixed-price sale entity
/// - Winner checkout flow requires productId to create order
library;

import 'package:labuda/domains/commerce/catalog/auction/data/dto/auction_dto.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

/// Mapper for Auction-related conversions
///
/// NOTE: productId from DTO is mapped as optional metadata for checkout integration.
/// The Auction entity is complete and independent.
class AuctionMapper {
  /// Convert AuctionDto to Auction domain entity
  ///
  /// SEMANTIC: productId from DTO is included as optional metadata for checkout.
  /// The Auction entity is complete and independent.
  ///
  /// NOTE: Anti-sniping fields (autoExtendCount, originalEndTime) are NOT
  /// mapped to domain entity as they are false feature signals - backend
  /// does not implement anti-sniping in production code.
  static Auction toEntity(AuctionDto dto) {
    // Convert backend images to MediaEntity
    final media = dto.images
        .map(
          (url) => MediaEntity(
            id: _generateMediaId(url),
            originalUrl: url,
            type: MediaType.image,
            createdAt: DateTime.now(),
          ),
        )
        .toList();

    // Owner Truth: username = account; farmName = seller/store; fullName = private/KYC.
    // Identity slots map directly from backend identity scalars:
    //   sellerUsername  ← seller_username
    //   sellerFarmName  ← seller_farm_name
    //   sellerAvatar    ← seller_avatar_url
    // No fullName fallback (KYC field).
    return Auction(
      id: dto.id,
      sellerId: dto.sellerId,
      sellerUsername: dto.sellerUsername,
      sellerFarmName: dto.sellerFarmName,
      sellerAvatar: dto.sellerAvatarUrl,
      // E8.2 — Canonical seller user-identity lifecycle parsed tolerantly.
      // Null / missing / unknown → active (legacy payloads stay backward
      // compatible).
      sellerUserLifecycle: ContentLifecycleParse.fromWire(
        dto.sellerUserLifecycle,
      ),
      // Expired-seller visibility — top-level seller-trust lifecycle.
      // Null wire field → active so pre-rollout payloads don't get
      // mis-classified as unavailable (the canonical parser fails CLOSED
      // on null which would disable CTAs everywhere on legacy data).
      sellerTrustLifecycle: dto.sellerTrustLifecycle == null
          ? ContentLifecycle.active
          : ContentLifecycleParse.fromWire(dto.sellerTrustLifecycle),
      // Stage 2 — seller reputation tier badge. Pass-through; SellerTierBadge
      // handles null/basic/unknown gracefully (renders nothing).
      sellerTier: dto.sellerTier,
      title: dto.title,
      description: dto.description ?? '',
      media: media,
      koiDetails: _createKoiDetails(dto),
      openingBid: dto.startPrice,
      currentBid: dto.currentHighestBid,
      bidIncrement: dto.bidIncrement,
      buyNowPrice: dto.buyNowPrice,
      condition: dto.condition != null
          ? parseAuctionCondition(dto.condition)
          : null,
      startTime: dto.startTime,
      endTime: dto.endTime,
      startedAt: dto.startedAt,
      endedAt: dto.endedAt,
      settlementDeadline: dto.settlementDeadline,
      isScheduled: dto.status == 'scheduled',
      status: parseAuctionStatus(dto.status),
      winnerId: dto.winner?.winnerId,
      winnerUsername: dto.winner?.winner?.username,
      winningBid: dto.winner?.winningBid,
      totalBidders: dto.totalBids,
      totalWatchers: dto.watchersCount,
      totalViews: dto.viewsCount,
      createdAt: dto.createdAt,
      updatedAt: dto.updatedAt,
      version: null,
      location: null,
      farmAddressId: null,
      decision: null,
      productId: dto.productId,
    );
  }

  /// Convert BidDto to AuctionBid domain entity
  ///
  /// Owner Truth: bidderUsername is the public bidder identity.
  /// No fullName/Anonymous/Bidder fake fallback — empty string indicates
  /// absence of identity.
  ///
  /// D14 — Bidder identity now arrives nested as `dto.bidder` (a
  /// `UserBriefDto` extended with avatar + lifecycle). The nested card
  /// is preferred over the legacy flat `dto.bidderUsername` scalar; the
  /// fallback is retained for rollback safety against pre-D14 payloads.
  static AuctionBid toBidEntity(BidDto dto) {
    final card = dto.bidder;
    final username = (card?.username.isNotEmpty ?? false)
        ? card!.username
        : (dto.bidderUsername ?? '');
    return AuctionBid(
      id: dto.id,
      auctionId: dto.auctionId,
      bidderId: dto.bidderId,
      bidderUsername: username,
      bidderAvatarUrl: card?.avatarUrl,
      bidderLifecycle: card?.lifecycle,
      amount: dto.amount,
      createdAt: dto.createdAt,
      isWinning: dto.isWinning,
      isOutbid: dto.isOutbid,
    );
  }

  /// Convert Auction entity to CreateAuctionDto
  ///
  /// SEMANTIC: productId is included at the request boundary even though the
  /// auction domain entity itself remains standalone.
  ///
  /// TIMING (PASS_18C): startMode/scheduledStartAt/durationHours replace the
  /// old raw startTime/endTime contract — the backend is the source of truth
  /// for the 1-7 day duration bound and computes start_at/end_at itself.
  ///
  /// CONTRACT PARITY (PASS_18E): every field the screen collects into
  /// [CreateAuctionParams] is now threaded through to the DTO with the exact
  /// backend key names (media_urls, variety, size_cm, age_months, ...).
  /// Previously `koiDetails` and `shippingSetupIds` were silently dropped
  /// here, so mobile auction creation either 400'd (missing required
  /// shipping_setup_ids) or created a product with no photos/variety.
  static CreateAuctionDto toCreateDto(CreateAuctionParams params) {
    final koi = params.koiDetails;
    return CreateAuctionDto(
      title: params.title,
      description: params.description,
      mediaUrls: params.mediaUrls,
      variety: koi.variety,
      sizeCm: koi.sizeInCm.round(),
      ageMonths: koi.ageInMonths,
      gender: koi.gender,
      breeder: koi.breeder,
      bloodline: koi.bloodline,
      certificates: koi.certificates.isEmpty ? null : koi.certificates,
      farmAddressId: params.farmAddressId,
      shippingSetupIds: params.shippingSetupIds,
      startPrice: params.openingBid,
      bidIncrement: params.bidIncrement,
      buyNowPrice: params.buyNowPrice,
      startMode: params.startMode,
      scheduledStartAt: params.scheduledStartAt,
      durationHours: params.durationHours,
      preparationNote: params.preparationNote,
    );
  }

  /// Convert Auction entity to UpdateAuctionDto
  ///
  /// NOTE: autoExtend and autoExtendMinutes are REMOVED - these are false
  /// feature signals. Backend does not implement anti-sniping in production.
  static UpdateAuctionDto toUpdateDto(Map<String, dynamic> updates) {
    return UpdateAuctionDto(
      title: updates['title'] as String?,
      description: updates['description'] as String?,
      images: (updates['images'] as List<dynamic>?)?.cast<String>(),
      category: updates['category'] as String?,
      startPrice: (updates['startPrice'] as num?)?.toDouble(),
      bidIncrement: (updates['bidIncrement'] as num?)?.toDouble(),
      buyNowPrice: (updates['buyNowPrice'] as num?)?.toDouble(),
      startTime: updates['startTime'] as DateTime?,
      endTime: updates['endTime'] as DateTime?,
    );
  }

  /// Convert Auction status enum to API string
  /// Uses the apiValue getter from AuctionStatus extension (backend-aligned)
  static String mapStatusToApi(AuctionStatus status) {
    return status.apiValue;
  }

  /// Create default KoiDetails when API doesn't provide full koi info
  static KoiDetails _createKoiDetails(AuctionDto dto) {
    // Category could be variety in some cases
    return KoiDetails(
      variety: dto.category ?? 'Unknown',
      sizeInCm: 0,
      ageInMonths: 0,
      gender: 'unknown',
      certificates: const [],
      breeder: null,
      bloodline: null,
    );
  }

  /// Generate a simple media ID from URL for mapping
  static String _generateMediaId(String url) {
    final uri = Uri.tryParse(url);
    if (uri != null && uri.pathSegments.isNotEmpty) {
      return uri.pathSegments.last.split('.').first;
    }
    return url.hashCode.toString();
  }
}
