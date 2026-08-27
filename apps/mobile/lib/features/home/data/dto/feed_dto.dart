// Feed DTOs for /api/v1/feed endpoint
// Data Transfer Objects for Feed domain API communication

import 'package:json_annotation/json_annotation.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/social/content/domain/entities/content_resource_projection.dart';

part 'feed_dto.g.dart';

// ============================================================================
// Response DTOs
// ============================================================================

/// Feed response from /api/v1/feed endpoint
///
/// This represents the canonical feed from Feed domain:
/// - Content from followed users only
/// - Excludes blocked users
/// - Cursor-based pagination
/// - P3A: promoted items interleaved by backend
@JsonSerializable(createFactory: false)
class FeedResponseDto {
  final List<FeedItemDto> data;
  @JsonKey(name: 'next_cursor')
  final String? nextCursor;
  @JsonKey(name: 'has_more')
  final bool hasMore;

  /// P3A — Promoted items extracted from the interleaved data array.
  /// NOT round-tripped through json_serializable.
  @JsonKey(includeFromJson: false, includeToJson: false)
  final List<PromotedFeedItemDto> promotedItems;

  /// P3A — Original indices of promoted items in the wire data array.
  /// Used to reconstruct the interleaved order during mapping.
  @JsonKey(includeFromJson: false, includeToJson: false)
  final List<int> promotedSlotIndices;

  const FeedResponseDto({
    required this.data,
    this.nextCursor,
    required this.hasMore,
    this.promotedItems = const [],
    this.promotedSlotIndices = const [],
  });

  /// Hand-written factory that parses the heterogeneous data array.
  /// Organic items (type: post/request) → FeedItemDto.
  /// Promoted items (type: promoted_*) → PromotedFeedItemDto with slot index.
  factory FeedResponseDto.fromJson(Map<String, dynamic> json) {
    final rawData = json['data'] as List<dynamic>? ?? [];
    final organicItems = <FeedItemDto>[];
    final promotedItems = <PromotedFeedItemDto>[];
    final promotedSlots = <int>[];

    for (int i = 0; i < rawData.length; i++) {
      final item = rawData[i];
      if (item is! Map<String, dynamic>) continue;
      final type = item['type'] as String? ?? '';
      if (type.startsWith('promoted_')) {
        promotedItems.add(PromotedFeedItemDto.fromJson(item));
        promotedSlots.add(i);
      } else {
        organicItems.add(FeedItemDto.fromJson(item));
      }
    }

    return FeedResponseDto(
      data: organicItems,
      nextCursor: json['next_cursor'] as String?,
      hasMore: json['has_more'] as bool? ?? false,
      promotedItems: promotedItems,
      promotedSlotIndices: promotedSlots,
    );
  }

  Map<String, dynamic> toJson() => _$FeedResponseDtoToJson(this);
}

/// Single feed item from /api/v1/feed
///
/// PROJECTION DTO: Simplified content representation for Feed API.
/// This is NOT the canonical Content structure - use Content API for full entity data.
///
/// The backend Feed domain returns only essential fields for feed display.
///
/// MEDIA INTEGRATION: Includes media array from backend Feed domain.
/// Source of truth: canonical Content.media (may be simplified in feed projection)
@JsonSerializable()
class FeedItemDto {
  final String id;
  @JsonKey(name: 'author_id')
  final String authorId;
  final String type;
  final String status;
  // Canonical governance lifecycle ({active, unavailable, removed}). Separate
  // from raw `status` — never coerce one into the other. Tolerant of null /
  // missing / unknown (parsed at the UI boundary via ContentLifecycle.fromWire).
  final String? lifecycle;
  final String body;
  final String? title;
  final String? caption;
  // Backend intentionally omits `is_hidden` at the public boundary
  // (feed_handler.go). Default to false so a missing key does not throw.
  @JsonKey(name: 'is_hidden', defaultValue: false)
  final bool isHidden;
  @JsonKey(name: 'created_at')
  final DateTime createdAt;
  @JsonKey(name: 'updated_at')
  final DateTime updatedAt;
  @JsonKey(name: 'author_username')
  final String? authorUsername;
  @JsonKey(name: 'author_avatar')
  final String? authorAvatar;

  // MEDIA INTEGRATION: Media from backend Feed domain
  // Contract: Sourced from content_media table
  final List<FeedMediaDto> media;

  // SHARE CONTRACT V1: Repost attribution fields
  /// If present, this item is a repost of the original author's content
  @JsonKey(name: 'original_author_id')
  final String? originalAuthorId;
  @JsonKey(includeFromJson: false, includeToJson: false)
  final ContentResourceProjection? resourceProjection;

  /// E2.1 — Embedded author lifecycle (canonical PublicCard.UserCard.Lifecycle).
  ///
  /// Sourced at parse time from the wire's nested `author.lifecycle` slot,
  /// with a fallback to `card.author.lifecycle` for envelope shapes that only
  /// carry the canonical ContentCard. Tolerant: null / missing / unknown
  /// values are converted to [ContentLifecycle.active] at the mapper layer.
  ///
  /// This field is NOT round-tripped through the json_serializable contract;
  /// it lives outside the generated [`feed_dto.g.dart`] surface to avoid a
  /// build_runner regeneration. The hand-written [FeedItemDto.fromJson]
  /// extracts it explicitly via [_readAuthorLifecycle].
  @JsonKey(includeFromJson: false, includeToJson: false)
  final String? authorLifecycle;

  /// FIX-3 — Embedded original-author lifecycle for content-type reposts.
  ///
  /// Sourced from the top-level `original_author_lifecycle` wire key emitted
  /// by feed_handler.go:hydrateOriginalAuthorLifecycles. Only present on
  /// repost rows (original_author_id != null). Tolerant: null / missing /
  /// unknown → [ContentLifecycle.active] at the mapper layer.
  ///
  /// NOT round-tripped through json_serializable — extracted by the
  /// hand-written [FeedItemDto.fromJson] via [_readOriginalAuthorLifecycle].
  @JsonKey(includeFromJson: false, includeToJson: false)
  final String? originalAuthorLifecycle;

  const FeedItemDto({
    required this.id,
    required this.authorId,
    required this.type,
    required this.status,
    this.lifecycle,
    required this.body,
    this.title,
    this.caption,
    this.isHidden = false,
    required this.createdAt,
    required this.updatedAt,
    this.authorUsername,
    this.authorAvatar,
    this.media = const [],
    this.originalAuthorId,
    this.resourceProjection,
    this.authorLifecycle,
    this.originalAuthorLifecycle,
  });

  /// Hand-written factory: delegates to the generated parser for every
  /// existing field, then layers on the E2.1 author lifecycle and the
  /// FIX-3 original-author lifecycle extracted from the wire shape.
  /// The generated parser remains the single source of truth for the
  /// contract that build_runner owns.
  factory FeedItemDto.fromJson(Map<String, dynamic> json) {
    final base = _$FeedItemDtoFromJson(json);
    final authorLc = _readAuthorLifecycle(json);
    final origAuthorLc = _readOriginalAuthorLifecycle(json);
    if (authorLc == null && origAuthorLc == null) return base;
    return FeedItemDto(
      id: base.id,
      authorId: base.authorId,
      type: base.type,
      status: base.status,
      lifecycle: base.lifecycle,
      body: base.body,
      title: base.title,
      caption: base.caption,
      isHidden: base.isHidden,
      createdAt: base.createdAt,
      updatedAt: base.updatedAt,
      authorUsername: base.authorUsername,
      authorAvatar: base.authorAvatar,
      media: base.media,
      originalAuthorId: base.originalAuthorId,
      resourceProjection: _readResourceProjection(json),
      authorLifecycle: authorLc,
      originalAuthorLifecycle: origAuthorLc,
    );
  }

  Map<String, dynamic> toJson() => _$FeedItemDtoToJson(this);
}

/// E2.1 — Extract the embedded author lifecycle string from the feed wire.
///
/// Preference order (per E2 backend hydration topology):
///   1. `author.lifecycle`           — populated by feed_handler.go via
///                                      publiccard.NewWithLifecycle
///   2. `card.author.lifecycle`      — same value, mirrored inside the
///                                      canonical ContentCard envelope
///
/// Returns null when both paths are absent / empty / not a string. The
/// mapper layer converts null into [ContentLifecycle.active] via the
/// canonical [ContentLifecycleParse.fromWire] helper.
String? _readAuthorLifecycle(Map<String, dynamic> json) {
  final author = json['author'];
  if (author is Map<String, dynamic>) {
    final lc = author['lifecycle'];
    if (lc is String && lc.isNotEmpty) return lc;
  }
  final card = json['card'];
  if (card is Map<String, dynamic>) {
    final cardAuthor = card['author'];
    if (cardAuthor is Map<String, dynamic>) {
      final lc = cardAuthor['lifecycle'];
      if (lc is String && lc.isNotEmpty) return lc;
    }
  }
  return null;
}

/// FIX-3 — Extract the original-author lifecycle string from the feed wire.
///
/// Reads the top-level `original_author_lifecycle` key emitted by
/// feed_handler.go via hydrateOriginalAuthorLifecycles. Only present on
/// repost rows (original_author_id != null). Returns null when absent /
/// empty / not a string. The mapper layer converts null into
/// [ContentLifecycle.active] via [ContentLifecycleParse.fromWire].
String? _readOriginalAuthorLifecycle(Map<String, dynamic> json) {
  final lc = json['original_author_lifecycle'];
  if (lc is String && lc.isNotEmpty) return lc;
  return null;
}

ContentResourceProjection? _readResourceProjection(Map<String, dynamic> json) {
  final raw = json['resource_projection'];
  if (raw is Map<String, dynamic>) {
    return ContentResourceProjection.fromJson(raw);
  }
  return null;
}

/// Feed media DTO from backend Feed domain
///
/// PROJECTION DTO: Simplified media for feed display.
/// Convert to MediaEntity for UI rendering, but canonical Content.media is source of truth.
@JsonSerializable()
class FeedMediaDto {
  final String url;
  final String type; // "image" or "video"
  final int position;

  const FeedMediaDto({
    required this.url,
    required this.type,
    required this.position,
  });

  factory FeedMediaDto.fromJson(Map<String, dynamic> json) =>
      _$FeedMediaDtoFromJson(json);
  Map<String, dynamic> toJson() => _$FeedMediaDtoToJson(this);

  /// Convert to domain MediaEntity
  MediaEntity toMediaEntity() {
    return MediaEntity(
      id: position.toString(), // Use position as ID for feed media
      originalUrl: url,
      type: type == 'image' ? MediaType.image : MediaType.video,
      createdAt: DateTime.now(),
    );
  }
}

// Legacy linked-item transport DTO removed.

// ============================================================================
// P3A — Promoted Feed Item DTO
// ============================================================================

/// DTO for promoted items injected into the feed by the backend.
///
/// Hand-written (no json_serializable) because the wire shape is entirely
/// different from organic FeedItemDto. Covers three target types:
/// - promoted_listing: listing card with price
/// - promoted_auction: auction card with bidding info
/// - promoted_external: external product with URL
class PromotedFeedItemDto {
  final String type; // promoted_listing, promoted_auction, promoted_external
  final String promotionInstanceId;
  final String targetType; // listing, auction, external_product

  // Common
  final String? title;
  final String? imageUrl;
  final String? sellerUsername;
  final String? sellerFarmName;
  final String? sellerLifecycle;

  // Listing-specific
  final String? fixedPriceSaleId;
  final int? pricePerUnit;

  // Auction-specific
  final String? auctionId;
  final int? startPrice;
  final int? currentBid;
  final int? buyNowPrice;
  final String? endAt;
  final int? bidCount;
  final String? status;

  // External-specific
  final String? externalUrl;
  final String? externalMediaUrl;

  const PromotedFeedItemDto({
    required this.type,
    required this.promotionInstanceId,
    required this.targetType,
    this.title,
    this.imageUrl,
    this.sellerUsername,
    this.sellerFarmName,
    this.sellerLifecycle,
    this.fixedPriceSaleId,
    this.pricePerUnit,
    this.auctionId,
    this.startPrice,
    this.currentBid,
    this.buyNowPrice,
    this.endAt,
    this.bidCount,
    this.status,
    this.externalUrl,
    this.externalMediaUrl,
  });

  factory PromotedFeedItemDto.fromJson(Map<String, dynamic> json) {
    return PromotedFeedItemDto(
      type: json['type'] as String? ?? '',
      promotionInstanceId: json['promotion_instance_id'] as String? ?? '',
      targetType: json['target_type'] as String? ?? '',
      title: json['title'] as String?,
      imageUrl: json['image_url'] as String?,
      sellerUsername: json['seller_username'] as String?,
      sellerFarmName: json['seller_farm_name'] as String?,
      sellerLifecycle: json['seller_lifecycle'] as String?,
      fixedPriceSaleId: json['fixed_price_sale_id'] as String?,
      pricePerUnit: json['price_per_unit'] as int?,
      auctionId: json['auction_id'] as String?,
      startPrice: json['start_price'] as int?,
      currentBid: json['current_bid'] as int?,
      buyNowPrice: json['buy_now_price'] as int?,
      endAt: json['end_at'] as String?,
      bidCount: json['bid_count'] as int?,
      status: json['status'] as String?,
      externalUrl: json['external_url'] as String?,
      externalMediaUrl: json['external_media_url'] as String?,
    );
  }
}
