// Feed Mapper
// Converts Feed DTOs to domain FeedItem entities (PROJECTION LAYER)
//
// FEED / DISCOVERY QUALITY PASS V1
//
// CONTRACT:
// - PROJECTION MAPPER: Converts API DTOs to UI projection entities
// - This mapper handles ONLY universal social content and reposts
// - Commerce objects (auction) should NOT appear in home feed (filtered by backend)
// - Engagement counts are NOT provided by backend Feed domain
//
// NO FAKE LIVELINESS:
// - likes: 0, comments: 0 are honest placeholders
// - UI layer should hide these counts, not show "0 likes"
// - Engagement will be added when backend supports it
//
// MEDIA INTEGRATION: Maps media array from FeedItemDto to MediaEntity list

import 'package:labuda/features/home/data/dto/feed_dto.dart';
import 'package:labuda/features/home/domain/domain.dart'; // R3.1: Import FeedItem entity
import 'package:labuda/shared/governance/content_lifecycle.dart';

/// Extension to convert [FeedItemDto] to [FeedItem]
///
/// This is a pure UI mapper - NO business logic.
/// All feed logic (following, blocking, filtering) is handled by backend Feed domain.
extension FeedItemMapper on FeedItemDto {
  /// Convert DTO to domain FeedItem
  ///
  /// QUALITY PASS NOTES:
  /// - Canonical resourceProjection and repost attribution are preserved
  /// - Status is passed through for lifecycle display
  /// - Engagement counts are 0 (backend doesn't provide them)
  /// - Media is properly mapped from FeedMediaDto to MediaEntity
  FeedItem toFeedItem() {
    // Map backend type to FeedItemType
    final feedType = _mapFeedItemType(type);

    // Check if this is a repost (has original_author_id)
    final isRepost = originalAuthorId != null;

    // MEDIA INTEGRATION: Map FeedMediaDto list to MediaEntity list
    final mediaList = media.map((dto) => dto.toMediaEntity()).toList();

    return FeedItem(
      id: id,
      content: body,
      authorId: authorId,
      authorUsername: authorUsername,
      authorAvatarUrl: authorAvatar,
      type: feedType,
      createdAt: createdAt,
      // MEDIA INTEGRATION: Use mapped MediaEntity list from backend
      media: mediaList,
      // Note: Backend Feed domain doesn't return engagement counts
      // UI layer handles this by not displaying fake "0" counts
      likes: 0,
      comments: 0,
      likedByUsers: const [],
      // Governance lifecycle is its own field, separate from raw status.
      // Tolerant parse: null / unknown / missing → active.
      lifecycle: ContentLifecycleParse.fromWire(lifecycle),
      // E2.1 — Author identity governance lifecycle. Independent of the
      // content lifecycle above; the author may be unavailable/removed
      // while the content itself is active (in which case the renderer
      // redacts the author block without dropping the card).
      authorLifecycle: ContentLifecycleParse.fromWire(authorLifecycle),
      // FIX-3 — Original-author governance lifecycle for reposts. Tolerant:
      // null / missing / unknown → active (non-reposts default safely).
      originalAuthorLifecycle: ContentLifecycleParse.fromWire(
        originalAuthorLifecycle,
      ),
      additionalData: {
        'title': title,
        'caption': caption,
        'isHidden': isHidden,
        'status': status,
        // SHARE CONTRACT V1: Include repost attribution data
        // This enables proper canonical repost rendering
        'isRepost': isRepost,
        'originalAuthorId': originalAuthorId,
        'resourceProjection': resourceProjection,
      },
    );
  }

  /// Map backend type string to FeedItemType enum
  ///
  /// PROJECTION CONTRACT: Only universal social content is mapped.
  /// Backend filters out commerce types (auction) from home feed.
  /// Falls back to content for unknown types (protects against unexpected backend changes).
  FeedItemType _mapFeedItemType(String backendType) {
    switch (backendType.toLowerCase()) {
      case 'content':
        return FeedItemType.content;
      default:
        // Default to content for unknown types
        // This protects against unexpected backend changes
        return FeedItemType.content;
    }
  }
}

/// Extension to convert list of FeedItemDto to list of FeedItem
extension FeedItemListMapper on List<FeedItemDto> {
  /// Convert list of DTOs to list of domain FeedItems
  List<FeedItem> toFeedItems() => map((dto) => dto.toFeedItem()).toList();
}

// ============================================================================
// P3A — Promoted item mapper
// ============================================================================

/// Maps a [PromotedFeedItemDto] to the unified [FeedItem] entity.
///
/// Promoted items reuse the FeedItem entity with FeedItemType.promoted*
/// and target-specific data packed into [FeedItem.additionalData].
extension PromotedFeedItemMapper on PromotedFeedItemDto {
  FeedItem toFeedItem() {
    final feedType = _mapPromotedType(type);
    final sellerUsername = this.sellerUsername;
    final sellerFarmName = this.sellerFarmName;
    final sellerLabel = _formatSellerLabel(sellerUsername, sellerFarmName);

    return FeedItem(
      id: promotionInstanceId,
      content: title ?? '',
      authorId: '',
      authorUsername: sellerUsername ?? '',
      type: feedType,
      createdAt: DateTime.now(),
      additionalData: {
        'isPromoted': true,
        'targetType': targetType,
        'promotionInstanceId': promotionInstanceId,
        'title': title,
        'imageUrl': imageUrl,
        'sellerUsername': sellerUsername,
        'sellerFarmName': sellerFarmName,
        'sellerLabel': sellerLabel,
        'sellerLifecycle': sellerLifecycle,
        // Listing
        'fixedPriceSaleId': fixedPriceSaleId,
        'pricePerUnit': pricePerUnit,
        // Auction
        'auctionId': auctionId,
        'startPrice': startPrice,
        'currentBid': currentBid,
        'buyNowPrice': buyNowPrice,
        'endAt': endAt,
        'bidCount': bidCount,
        'status': status,
        // External
        'externalUrl': externalUrl,
        'externalMediaUrl': externalMediaUrl,
      },
    );
  }

  String _formatSellerLabel(String? sellerUsername, String? sellerFarmName) {
    final username = (sellerUsername ?? '').trim();
    final farmName = (sellerFarmName ?? '').trim();
    if (username.isNotEmpty && farmName.isNotEmpty) {
      return '@$username • $farmName';
    }
    if (username.isNotEmpty) {
      return '@$username';
    }
    return '';
  }

  FeedItemType _mapPromotedType(String wireType) {
    switch (wireType) {
      case 'promoted_listing':
        return FeedItemType.promotedListing;
      case 'promoted_auction':
        return FeedItemType.promotedAuction;
      case 'promoted_external':
        return FeedItemType.promotedExternal;
      default:
        return FeedItemType.promotedListing;
    }
  }
}

/// Merges organic [FeedItem]s with promoted items at their original slot
/// positions from the wire data array.
List<FeedItem> mergeFeedItems(
  List<FeedItem> organic,
  List<PromotedFeedItemDto> promoted,
  List<int> slotIndices,
) {
  if (promoted.isEmpty) return organic;

  final result = <FeedItem>[];
  int organicIdx = 0;
  int promoIdx = 0;

  for (
    int i = 0;
    organicIdx < organic.length || promoIdx < promoted.length;
    i++
  ) {
    if (promoIdx < promoted.length &&
        promoIdx < slotIndices.length &&
        i == slotIndices[promoIdx]) {
      result.add(promoted[promoIdx].toFeedItem());
      promoIdx++;
    } else if (organicIdx < organic.length) {
      result.add(organic[organicIdx]);
      organicIdx++;
    } else {
      break;
    }
  }

  // Append remaining promoted items at the end.
  for (; promoIdx < promoted.length; promoIdx++) {
    result.add(promoted[promoIdx].toFeedItem());
  }

  return result;
}
