import 'package:labuda/features/search/search/data/dto/search_dto.dart';
import 'package:labuda/features/search/search/domain/entities/search_history.dart';
import 'package:labuda/features/search/search/domain/entities/user_search.dart';
import 'package:labuda/features/search/search/domain/repositories/search_repository.dart'
    show ContentSearchResult, ListingSearchResult, UserSearchResult;
import 'package:labuda/shared/governance/content_lifecycle.dart';

/// Extension to convert ContentSearchResultDto to domain entity
///
/// FEDERATED SEARCH CONTRACT REALIGN PACK V1:
/// Aligned with backend search response format
/// Backend provides: id, author_id, caption, media_urls, created_at
extension ContentSearchResultDtoX on ContentSearchResultDto {
  ContentSearchResult toDomain() {
    return ContentSearchResult(
      id: id,
      title: _getTitle(),
      description: caption,
      contentType: 'content',
      // Raw entity status placeholder. Historically /search/content only
      // surfaced active content, so this scalar stayed 'active'. After
      // governance enforce promotion the visibility decision lives on
      // `lifecycle` below — DO NOT erase lifecycle into status, and
      // DO NOT branch UX on this `status` field.
      status: 'active',
      lifecycle: ContentLifecycleParse.fromWire(lifecycle),
      thumbnailUrl: _getThumbnailUrl(),
      price: price,
      authorId: authorId,
      authorUsername: authorUsername ?? '',
      authorAvatarUrl: authorAvatarUrl,
      // likesCount / commentsCount are intentionally omitted: /search/content
      // does not emit engagement counts, and surfacing 0 was rendering as a
      // visible "♥ 0  💬 0" lie on every content row.
      createdAt: createdAt,
      // E9.1 — null / missing / empty / unknown → active (forward-compat).
      authorLifecycle: ContentLifecycleParse.fromWire(authorLifecycle),
    );
  }

  String _getTitle() {
    final projectionTitle = resourceProjection?.titleText;
    if (projectionTitle != null && projectionTitle.isNotEmpty) {
      return projectionTitle;
    }
    // Extract title from caption or generate one
    if (caption != null && caption!.isNotEmpty) {
      // Use first line of caption as title
      final lines = caption!.split('\n');
      return lines.first.length > 50
          ? '${lines.first.substring(0, 47)}...'
          : lines.first;
    }
    return 'Content';
  }

  String? _getThumbnailUrl() {
    final projectionImage = resourceProjection?.imageUrl;
    if (projectionImage != null && projectionImage.isNotEmpty) {
      return projectionImage;
    }
    // Return first media URL as thumbnail
    if (mediaUrls.isNotEmpty) {
      return mediaUrls.first;
    }
    return null;
  }
}

/// Extension to convert ListingSearchResultDto to domain entity
///
/// SKINNY TRUTHFUL MAPPER — passes through ONLY fields the
/// /search/listings endpoint emits. No fabricated quantity / status /
/// visibility / listing_type / updated_at.
///
/// Owner Truth: username / farmName / fullName(KYC). Backend identity
/// scalars (`seller_username`, `seller_farm_name`, `seller_avatar_url`)
/// are passed straight through to the domain. No fullName fallback.
extension ListingSearchResultDtoX on ListingSearchResultDto {
  ListingSearchResult toDomain() {
    return ListingSearchResult(
      id: id,
      title: title,
      description: description,
      variety: variety,
      price: price,
      mediaUrls: mediaUrls,
      sellerId: sellerId,
      sellerUsername: sellerUsername,
      sellerFarmName: sellerFarmName,
      sellerAvatarUrl: sellerAvatarUrl,
      createdAt: createdAt,
      // E8.4 — null / missing / empty / unknown → active (forward-compat).
      sellerUserLifecycle: ContentLifecycleParse.fromWire(sellerUserLifecycle),
      // Trust-axis — null / missing / unknown → active (forward-compat).
      sellerTrustLifecycle: ContentLifecycleParse.fromWire(
        sellerTrustLifecycle,
      ),
    );
  }
}

/// Extension to convert UserSearchResultDto to domain entity
///
/// FEDERATED SEARCH CONTRACT REALIGN PACK V1:
/// Aligned with backend search response format
/// Backend provides: id, username, display_name, avatar_url
extension UserSearchResultDtoX on UserSearchResultDto {
  UserSearchResult toDomain() {
    return UserSearchResult(
      id: id,
      username: username,
      avatarUrl: avatarUrl,
      bio: null, // Not provided by backend search response
      isFollowing: null, // Not provided by backend search response
    );
  }

  /// Convert to UserSearch entity
  UserSearch toUserSearch() {
    return UserSearch(userId: id, username: username, avatarUrl: avatarUrl);
  }
}

/// SEARCH SURFACE PURGE V1: HashtagSearchResultDtoX removed
/// Hashtag search is not in the final search contract

/// Extension to convert SearchHistoryDto to domain entity
///
/// FEDERATED SEARCH CONTRACT REALIGN PACK V1:
/// Simplified to match backend response (id, query, created_at only)
extension SearchHistoryDtoX on SearchHistoryDto {
  SearchHistory toDomain(String userId) {
    return SearchHistory(
      id: id,
      userId: userId,
      query: query,
      searchedAt: createdAt,
      resultCount: 0, // Not provided by backend
    );
  }
}

/// Extension to convert SearchHistory entity to DTO
extension SearchHistoryX on SearchHistory {
  SearchHistoryDto toDto() {
    return SearchHistoryDto(id: id, query: query, createdAt: searchedAt);
  }
}
