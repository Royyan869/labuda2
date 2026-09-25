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
import 'package:labuda/core/common/types/preparation_time.dart';
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
  /// Phantom purge: fields the backend never emits are not mapped. The
  /// settlement deadline is DERIVED from end_at + 24h (canonical backend
  /// rule: Auction.SettlementDeadline()).
  static Auction toEntity(AuctionDto dto) {
    // CONVERGED media parsing: prefer the typed media block (backend detail
    // wire carries id/type/dimensions/thumbnail — identical shape to
    // for_sale); fall back to string images (list wire) as image-only.
    final media =
        dto.mediaItems.isNotEmpty
        ? dto.mediaItems
              .map(
                (item) => MediaEntity(
                  id: item.id.isNotEmpty
                      ? item.id
                      : _generateMediaId(item.url),
                  originalUrl: item.url,
                  type: item.isVideo ? MediaType.video : MediaType.image,
                  dimensions:
                      (item.width != null && item.height != null)
                      ? MediaDimensions(width: item.width!, height: item.height!)
                      : null,
                  createdAt: item.createdAt ?? DateTime.now(),
                ),
              )
              .toList()
        : dto.images
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
    // Phantom purge: currentBid is nullable factual (current_bid null = no bids).
    // Presentation fallback to startPrice is explicit derivation from factual
    // fields, not a hidden DTO wire fallback.
    final factualCurrentBid = dto.currentBid ?? dto.startPrice;
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
      // Canonical per-viewer detail action authority. Null on list/discovery
      // payloads; present on the detail wire (viewer_capabilities).
      viewerCapabilities: dto.viewerCapabilities,
      title: dto.title,
      description: dto.description ?? '',
      media: media,
      koiDetails: _createKoiDetails(dto),
      openingBid: dto.startPrice,
      currentBid: factualCurrentBid,
      bidIncrement: dto.bidIncrement,
      buyNowPrice: dto.buyNowPrice,
      // Shipping readiness — canonical Product content from the detail wire.
      // Null/empty wire value stays null (absence is NOT defaulted to
      // immediate) so no canonical absence is masked.
      preparationTime:
          (dto.preparationTime == null || dto.preparationTime!.isEmpty)
          ? null
          : PreparationTime.fromJson(dto.preparationTime),
      preparationNote: dto.preparationNote,
      startTime: dto.startTime,
      endTime: dto.endTime,
      status: parseAuctionStatus(dto.status),
      // Canonical winner authority is current_winner_id only — phantom winner
      // object purged.
      winnerId: dto.currentWinnerId,
      // Canonical settlement deadline derivation (backend rule:
      // Auction.SettlementDeadline() = end_at + 24h). Derived from factual
      // wire fields — never parsed from a wire field.
      createdAt: dto.createdAt,
      updatedAt: dto.updatedAt,
      farmAddressId: null,
      productId: dto.productId,
    );
  }

  /// Convert BidDto to AuctionBid domain entity
  ///
  /// Owner Truth: bidder identity arrives nested as `dto.bidder` (a
  /// `UserBriefDto` carrying username + avatar + coarsened lifecycle).
  /// No fullName fallback (KYC field), no phantom winner flags — buyer
  /// bid-position authority lives in GET /api/v1/bidding, not on the bid wire.
  static AuctionBid toBidEntity(BidDto dto) {
    final card = dto.bidder;
    return AuctionBid(
      id: dto.id,
      auctionId: dto.auctionId,
      bidderId: dto.bidderId,
      bidderUsername: card?.username ?? '',
      bidderAvatarUrl: card?.avatarUrl,
      bidderLifecycle: card?.lifecycle,
      amount: dto.amount,
      createdAt: dto.createdAt,
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
  /// shipping_option_ids) or created a product with no photos/variety.
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

  /// Convert Auction entity to UpdateAuctionDto — CANONICAL UPDATE CONTRACT.
  ///
  /// Only supported keys are mapped; legacy keys (images/category/condition)
  /// are intentionally NOT propagated — the canonical client never sends them
  /// and the backend now rejects them.
  static UpdateAuctionDto toUpdateDto(Map<String, dynamic> updates) {
    return UpdateAuctionDto(
      title: updates['title'] as String?,
      description: updates['description'] as String?,
      startPrice: (updates['startPrice'] as num?)?.toInt(),
      bidIncrement: (updates['bidIncrement'] as num?)?.toInt(),
      buyNowPrice: (updates['buyNowPrice'] as num?)?.toInt(),
      startTime: updates['startTime'] as DateTime?,
      endTime: updates['endTime'] as DateTime?,
    );
  }

  /// Convert Auction status enum to API string
  /// Uses the apiValue getter from AuctionStatus extension (backend-aligned)
  static String mapStatusToApi(AuctionStatus status) {
    return status.apiValue;
  }

  /// Map koi content from the canonical detail wire into [KoiDetails].
  ///
  /// Canonical values (variety/size_cm/age_months/gender/breeder/bloodline/
  /// certificates) are mapped verbatim. The 'Unknown' / 0 / 'unknown' /
  /// empty placeholders are LEGITIMATE ABSENCE DEFAULTS applied ONLY when the
  /// wire truly omits a value (list payloads, non-koi products) — they never
  /// replace a canonical value that is present.
  static KoiDetails _createKoiDetails(AuctionDto dto) {
    final variety = dto.variety;
    final gender = dto.gender;
    return KoiDetails(
      variety: (variety == null || variety.isEmpty) ? 'Unknown' : variety,
      sizeInCm: (dto.sizeCm ?? 0).toDouble(),
      ageInMonths: dto.ageMonths ?? 0,
      gender: (gender == null || gender.isEmpty) ? 'unknown' : gender,
      certificates: dto.certificates,
      breeder: dto.breeder,
      bloodline: dto.bloodline,
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
