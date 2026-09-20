import 'package:equatable/equatable.dart';
import 'package:labuda/domains/social/content/domain/entities/content_resource_projection.dart';
import 'package:labuda/features/search/search/domain/entities/user_search.dart';

// E9.1 — Walk the canonical PublicCard author lifecycle slot on content
// search rows. Primary path: `card.author.lifecycle` (nested PublicCard
// ContentCard shape emitted since E9.1). Fallback: `author.lifecycle`
// (flat authorCard emitted at the same site). Pre-E9.1 payloads omit the
// lifecycle field entirely → null fall-through (mapper coerces to active).
String? _readContentSearchAuthorLifecycle(Map<String, dynamic> json) {
  final card = json['card'];
  if (card is Map<String, dynamic>) {
    final author = card['author'];
    if (author is Map<String, dynamic>) {
      final lc = author['lifecycle'];
      if (lc is String && lc.isNotEmpty) return lc;
    }
  }
  // Fallback: flat author card at the response item root.
  final author = json['author'];
  if (author is Map<String, dynamic>) {
    final lc = author['lifecycle'];
    if (lc is String && lc.isNotEmpty) return lc;
  }
  return null;
}

// E8.4 — Walk the nested canonical PublicCard wire slot
// (`for_sale.seller.user.lifecycle`). Pre-E8.1 payloads omit it →
// null fall-through. Top-level `for_sale.seller.lifecycle` is the
// TRUST axis and is read by a separate helper below.
String? _readFixedPriceSaleSearchSellerUserLifecycle(
  Map<String, dynamic> json,
) {
  final fixedPriceSale = json['for_sale'];
  if (fixedPriceSale is Map<String, dynamic>) {
    final seller = fixedPriceSale['seller'];
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

// Seller-trust axis for for-sale search rows. Reads top-level
// `for_sale.seller.lifecycle` populated by NewSellerCardWithBothLifecycles
// (search_handler.go fixedPriceSalePreviewsToResponse). Null → active (forward-compat).
String? _readFixedPriceSaleSearchSellerTrustLifecycle(
  Map<String, dynamic> json,
) {
  final fixedPriceSale = json['for_sale'];
  if (fixedPriceSale is Map<String, dynamic>) {
    final seller = fixedPriceSale['seller'];
    if (seller is Map<String, dynamic>) {
      final lc = seller['lifecycle'];
      if (lc is String && lc.isNotEmpty) return lc;
    }
  }
  return null;
}

ContentResourceProjection? _readContentSearchResourceProjection(
  Map<String, dynamic> json,
) {
  final raw = json['resource_projection'];
  if (raw is Map<String, dynamic>) {
    return ContentResourceProjection.fromJson(raw);
  }
  return null;
}

// E8.4 — Walk the nested canonical PublicCard wire slot
// (`auction.seller.user.lifecycle`). Pre-E8.1 payloads omit it →
// null fall-through. Top-level `auction.seller.lifecycle` is the
// TRUST axis and is read by a separate helper below.
String? _readAuctionSearchSellerUserLifecycle(Map<String, dynamic> json) {
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

// Seller-trust axis for auction search rows. Reads top-level
// `auction.seller.lifecycle` populated by NewSellerCardWithBothLifecycles
// (search_handler.go auctionPreviewsToResponse). Null → active (forward-compat).
String? _readAuctionSearchSellerTrustLifecycle(Map<String, dynamic> json) {
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

// =====================
// Content Search DTOs
// =====================

/// Content search response from backend
///
/// FEDERATED SEARCH CONTRACT REALIGN PACK V1:
/// Aligned with backend /api/v1/search/content response format
/// {
///   "query": "...",
///   "contents": [...],
///   "total": 123,
///   "limit": 20,
///   "offset": 0
/// }
class ContentSearchResponseDto {
  final String query;
  final List<ContentSearchResultDto> contents;
  final int total;
  final int limit;
  final int offset;

  const ContentSearchResponseDto({
    required this.query,
    required this.contents,
    required this.total,
    required this.limit,
    required this.offset,
  });

  factory ContentSearchResponseDto.fromJson(Map<String, dynamic> json) {
    return ContentSearchResponseDto(
      query: json['query'] as String? ?? '',
      contents:
          (json['contents'] as List<dynamic>?)
              ?.map(
                (e) =>
                    ContentSearchResultDto.fromJson(e as Map<String, dynamic>),
              )
              .toList() ??
          [],
      total: (json['total'] as num?)?.toInt() ?? 0,
      limit: (json['limit'] as num?)?.toInt() ?? 20,
      offset: (json['offset'] as num?)?.toInt() ?? 0,
    );
  }

  Map<String, dynamic> toJson() => {
    'query': query,
    'contents': contents.map((e) => e.toJson()).toList(),
    'total': total,
    'limit': limit,
    'offset': offset,
  };

  /// Helper to get page number from offset/limit
  int get page => (offset / limit).floor() + 1;

  /// Helper to check if there are more results
  bool get hasMore => offset + contents.length < total;
}

/// Content search result DTO
///
/// Backend response format from SearchHandler:
/// {
///   "id": "...",
///   "author_id": "...",
///   "author": { "id": "...", "username": "...", "avatar_url": "..." },
///   "type": "content|forSale|auction",
///   "caption": "...",
///   "media_urls": ["..."],
///   "created_at": "..."
/// }
class ContentSearchResultDto extends Equatable {
  final String id;
  final String authorId;
  final String? authorUsername;
  final String? authorAvatarUrl;
  final String type;
  final String? caption;

  /// Media URLs for the row. The backend already projected these through the
  /// canonical mediaresolve read authority, so they arrive as readable URLs for
  /// the search surface. This layer MUST NOT build, rewrite, or type-infer
  /// media; rendering goes through the canonical `StableNetworkImage` /
  /// `resolveNetworkImageUrl` path.
  final List<String> mediaUrls;

  final DateTime createdAt;
  final double? price;
  final ContentResourceProjection? resourceProjection;
  // Canonical governance lifecycle ({active, unavailable, removed}). Wire field
  // is optional; absence is parsed as active by the mapper. Separate from any
  // raw entity status — never coerce one into the other.
  final String? lifecycle;

  /// E9.1 — Canonical author user-identity lifecycle on content search rows.
  ///
  /// Sourced from `card.author.lifecycle` (primary) or `author.lifecycle`
  /// (fallback) via `_readContentSearchAuthorLifecycle`. Pre-E9.1 payloads
  /// omit the field → null (mapper coerces to ContentLifecycle.active).
  ///
  /// AXIS BOUNDARY: content item lifecycle (`lifecycle` field above) is the
  /// item-axis and governs opacity/tap. This field is the AUTHOR user-identity
  /// axis and governs only the subtitle placeholder. The two axes are
  /// completely independent and must never be conflated.
  final String? authorLifecycle;

  const ContentSearchResultDto({
    required this.id,
    required this.authorId,
    this.authorUsername,
    this.authorAvatarUrl,
    required this.type,
    this.caption,
    required this.mediaUrls,
    required this.createdAt,
    this.price,
    this.resourceProjection,
    this.lifecycle,
    this.authorLifecycle,
  });

  factory ContentSearchResultDto.fromJson(Map<String, dynamic> json) {
    final authorRef = json['author'] as Map<String, dynamic>?;
    final resourceProjection = _readContentSearchResourceProjection(json);
    return ContentSearchResultDto(
      id: json['id'] as String,
      authorId: json['author_id'] as String,
      authorUsername:
          (authorRef?['username'] as String?) ??
          (json['author_username'] as String?),
      authorAvatarUrl:
          (authorRef?['avatar_url'] as String?) ??
          (json['author_avatar_url'] as String?),
      type: json['type'] as String? ?? 'content',
      caption: json['caption'] as String?,
      mediaUrls:
          (json['media_urls'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          [],
      createdAt: DateTime.parse(json['created_at'] as String),
      price:
          resourceProjection?.fixedPriceSale?.price.toDouble() ??
          (json['price'] as num?)?.toDouble(),
      resourceProjection: resourceProjection,
      lifecycle: json['lifecycle'] as String?,
      authorLifecycle: _readContentSearchAuthorLifecycle(json),
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'author_id': authorId,
    if (authorUsername != null) 'author_username': authorUsername,
    if (authorAvatarUrl != null) 'author_avatar_url': authorAvatarUrl,
    'type': type,
    if (caption != null) 'caption': caption,
    'media_urls': mediaUrls,
    'created_at': createdAt.toIso8601String(),
    if (price != null) 'price': price,
    if (resourceProjection != null)
      'resource_projection': resourceProjection!.toJson(),
    if (lifecycle != null) 'lifecycle': lifecycle,
    if (authorLifecycle != null) 'author_lifecycle': authorLifecycle,
  };

  @override
  List<Object?> get props => [
    id,
    type,
    authorId,
    createdAt,
    price,
    resourceProjection,
    lifecycle,
    authorLifecycle,
  ];
}

// =====================
// User Search DTOs
// =====================

/// User search response from backend
///
/// FEDERATED SEARCH CONTRACT REALIGN PACK V1:
/// Aligned with backend /api/v1/search/users response format
/// {
///   "query": "...",
///   "users": [...],
///   "total": 123,
///   "limit": 20,
///   "offset": 0
/// }
class UserSearchResponseDto {
  final String query;
  final List<UserSearchResultDto> users;
  final int total;
  final int limit;
  final int offset;

  const UserSearchResponseDto({
    required this.query,
    required this.users,
    required this.total,
    required this.limit,
    required this.offset,
  });

  factory UserSearchResponseDto.fromJson(Map<String, dynamic> json) {
    return UserSearchResponseDto(
      query: json['query'] as String? ?? '',
      users:
          (json['users'] as List<dynamic>?)
              ?.map(
                (e) => UserSearchResultDto.fromJson(e as Map<String, dynamic>),
              )
              .toList() ??
          [],
      total: (json['total'] as num?)?.toInt() ?? 0,
      limit: (json['limit'] as num?)?.toInt() ?? 20,
      offset: (json['offset'] as num?)?.toInt() ?? 0,
    );
  }

  Map<String, dynamic> toJson() => {
    'query': query,
    'users': users.map((e) => e.toJson()).toList(),
    'total': total,
    'limit': limit,
    'offset': offset,
  };

  /// Helper to get page number from offset/limit
  int get page => (offset / limit).floor() + 1;

  /// Helper to check if there are more results
  bool get hasMore => offset + users.length < total;
}

/// User search result DTO
///
/// Backend response format from SearchHandler:
/// {
///   "id": "...",
///   "username": "...",
///   "avatar_url": "..."
/// }
class UserSearchResultDto extends Equatable {
  final String id;
  final String username;
  final String? avatarUrl;

  const UserSearchResultDto({
    required this.id,
    required this.username,
    this.avatarUrl,
  });

  factory UserSearchResultDto.fromJson(Map<String, dynamic> json) {
    return UserSearchResultDto(
      id: json['id'] as String,
      username: json['username'] as String? ?? '',
      avatarUrl: json['avatar_url'] as String?,
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'username': username,
    if (avatarUrl != null) 'avatar_url': avatarUrl,
  };

  @override
  List<Object?> get props => [id, username];
}

/// Extension to convert UserSearchResultDto to UserSearch entity
/// R3.1: Added for user_search_bottom_sheet.dart alignment
extension UserSearchResultDtoMapper on UserSearchResultDto {
  /// Convert DTO to UserSearch domain entity.
  ///
  /// OWNER TRUTH (Phase 2 + 4): public identity is username only.
  /// Username is the public identity truth. UI degrades to '@username'.
  UserSearch toUserSearch() {
    return UserSearch(userId: id, username: username, avatarUrl: avatarUrl);
  }
}

// =====================
// Auction Search DTOs (PHASE 3.5 - AUCTION SEARCH TRUTH COMPLETION)
// =====================

/// Auction search response from backend
///
/// PHASE 3.5: Aligned with backend /api/v1/search/auctions response format
/// {
///   "query": "...",
///   "auctions": [...],
///   "total": 123,
///   "limit": 20,
///   "offset": 0
/// }
///
/// AUCTION SEARCH ELIGIBILITY:
/// Only auctions with status IN ('scheduled', 'active', 'ended') are returned
/// Draft and cancelled auctions are NOT discoverable via search
class AuctionSearchResponseDto {
  final String query;
  final List<AuctionSearchResultDto> auctions;
  final int total;
  final int limit;
  final int offset;

  /// P3B — Promoted items sidecar from backend.
  final List<PromotedSearchItemDto> promotedItems;

  const AuctionSearchResponseDto({
    required this.query,
    required this.auctions,
    required this.total,
    required this.limit,
    required this.offset,
    this.promotedItems = const [],
  });

  factory AuctionSearchResponseDto.fromJson(Map<String, dynamic> json) {
    return AuctionSearchResponseDto(
      query: json['query'] as String? ?? '',
      auctions:
          (json['auctions'] as List<dynamic>?)
              ?.map(
                (e) =>
                    AuctionSearchResultDto.fromJson(e as Map<String, dynamic>),
              )
              .toList() ??
          [],
      total: (json['total'] as num?)?.toInt() ?? 0,
      limit: (json['limit'] as num?)?.toInt() ?? 20,
      offset: (json['offset'] as num?)?.toInt() ?? 0,
      promotedItems:
          (json['promoted_items'] as List<dynamic>?)
              ?.map(
                (e) =>
                    PromotedSearchItemDto.fromJson(e as Map<String, dynamic>),
              )
              .toList() ??
          [],
    );
  }

  Map<String, dynamic> toJson() => {
    'query': query,
    'auctions': auctions.map((e) => e.toJson()).toList(),
    'total': total,
    'limit': limit,
    'offset': offset,
  };

  /// Helper to get page number from offset/limit
  int get page => (offset / limit).floor() + 1;

  /// Helper to check if there are more results
  bool get hasMore => offset + auctions.length < total;
}

/// Auction search result DTO
///
/// Backend response format from SearchHandler:
/// {
///   "id": "...",
///   "seller_id": "...",
///   "product_id": "...",
///   "title": "...",
///   "description": "...",
///   "start_price": 1000000,
///   "current_bid": 1500000,
///   "buy_now_price": 2000000,
///   "start_at": "...",
///   "end_at": "...",
///   "status": "scheduled|active|ended",
///   "thumbnail_url": "...",
///   "seller_username": "...",
///   "seller_farm_name": "...",
///   "seller_avatar_url": "...",
///   "bid_count": 5,
///   "created_at": "..."
/// }
class AuctionSearchResultDto extends Equatable {
  final String id;
  final String sellerId;
  final String productId;
  final String title;
  final String description;
  final int startPrice;
  final int? currentBid;
  final int? buyNowPrice;
  final DateTime startAt;
  final DateTime endAt;
  final String status; // "scheduled", "active", "ended"
  final String? thumbnailUrl;
  final int bidCount;
  final DateTime createdAt;

  // Owner Truth identity scalars (account / store / avatar).
  // No fullName field — public identity is username/farm.
  final String? sellerUsername;
  final String? sellerFarmName;
  final String? sellerAvatarUrl;

  /// E8.4 — Canonical seller user-identity lifecycle on auction search rows.
  ///
  /// Sourced ONLY from the nested wire slot `auction.seller.user.lifecycle`.
  /// Tolerant: null / missing / empty → null (mapper coerces to active).
  ///
  /// AXIS BOUNDARY: this field carries the user-identity axis only.
  final String? sellerUserLifecycle;

  /// Seller-trust axis lifecycle on auction search rows.
  ///
  /// Sourced from top-level `auction.seller.lifecycle` populated by
  /// NewSellerCardWithBothLifecycles (search_handler.go). Null → active.
  final String? sellerTrustLifecycle;

  const AuctionSearchResultDto({
    required this.id,
    required this.sellerId,
    required this.productId,
    required this.title,
    required this.description,
    required this.startPrice,
    this.currentBid,
    this.buyNowPrice,
    required this.startAt,
    required this.endAt,
    required this.status,
    this.thumbnailUrl,
    required this.bidCount,
    required this.createdAt,
    this.sellerUsername,
    this.sellerFarmName,
    this.sellerAvatarUrl,
    this.sellerUserLifecycle,
    this.sellerTrustLifecycle,
  });

  factory AuctionSearchResultDto.fromJson(Map<String, dynamic> json) {
    final auctionRef = json['auction'] as Map<String, dynamic>?;
    return AuctionSearchResultDto(
      id: json['id'] as String,
      sellerId: json['seller_id'] as String,
      productId: json['product_id'] as String,
      title: json['title'] as String? ?? '',
      description: json['description'] as String? ?? '',
      startPrice: (json['start_price'] as num?)?.toInt() ?? 0,
      currentBid:
          (auctionRef?['current_price'] as num?)?.toInt() ??
          (json['current_bid'] as num?)?.toInt(),
      buyNowPrice: (json['buy_now_price'] as num?)?.toInt(),
      startAt: DateTime.parse(json['start_at'] as String),
      endAt: DateTime.parse(
        (auctionRef?['end_at'] as String?) ?? (json['end_at'] as String),
      ),
      status:
          (auctionRef?['status'] as String?) ??
          (json['status'] as String? ?? 'scheduled'),
      thumbnailUrl:
          (auctionRef?['thumbnail'] as String?) ??
          (json['thumbnail_url'] as String?),
      bidCount: (json['bid_count'] as num?)?.toInt() ?? 0,
      createdAt: DateTime.parse(json['created_at'] as String),
      sellerUsername: json['seller_username'] as String?,
      sellerFarmName: json['seller_farm_name'] as String?,
      sellerAvatarUrl: json['seller_avatar_url'] as String?,
      sellerUserLifecycle: _readAuctionSearchSellerUserLifecycle(json),
      sellerTrustLifecycle: _readAuctionSearchSellerTrustLifecycle(json),
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'seller_id': sellerId,
    'product_id': productId,
    'title': title,
    'description': description,
    'start_price': startPrice,
    if (currentBid != null) 'current_bid': currentBid,
    if (buyNowPrice != null) 'buy_now_price': buyNowPrice,
    'start_at': startAt.toIso8601String(),
    'end_at': endAt.toIso8601String(),
    'status': status,
    if (thumbnailUrl != null) 'thumbnail_url': thumbnailUrl,
    'bid_count': bidCount,
    'created_at': createdAt.toIso8601String(),
    if (sellerUsername != null) 'seller_username': sellerUsername,
    if (sellerFarmName != null) 'seller_farm_name': sellerFarmName,
    if (sellerAvatarUrl != null) 'seller_avatar_url': sellerAvatarUrl,
  };

  @override
  List<Object?> get props => [id, status, sellerId, startAt];
}

// =====================
// For Sale Search DTOs (REAL FOR SALE TAB - /search/for-sale)
// =====================
//
// SEMANTIC CONTRACT:
// - For Sale tab = REAL commerce For Sale items only.
// - These DTOs parse ONLY the fields actually emitted by
//   GET /api/v1/search/for-sale.
// - DO NOT reuse For Sale detail DTOs/mappers: those fabricate
//   quantity/status/visibility fields the search surface never emits.
// - Optional `author`, `media`, `forSale` blocks are tolerated
//   (Phase C horizontal additive refs) but never depended on.

/// For Sale search response — canonical backend contract for
/// GET /api/v1/search/for-sale: { for_sales, next_cursor, has_more }.
class ForSaleSearchResponseDto {
  final List<ForSaleSearchResultDto> forSales;

  /// P3B — Promoted items sidecar from backend.
  final List<PromotedSearchItemDto> promotedItems;

  /// Opaque backend cursor; null on the first page.
  final String? nextCursor;

  /// Backend-driven continuation flag (single pagination authority).
  final bool hasMore;

  const ForSaleSearchResponseDto({
    required this.forSales,
    this.promotedItems = const [],
    this.nextCursor,
    this.hasMore = false,
  });

  factory ForSaleSearchResponseDto.fromJson(Map<String, dynamic> json) {
    return ForSaleSearchResponseDto(
      forSales:
          (json['for_sales'] as List<dynamic>?)
              ?.map(
                (e) =>
                    ForSaleSearchResultDto.fromJson(e as Map<String, dynamic>),
              )
              .toList() ??
          [],
      promotedItems:
          (json['promoted_items'] as List<dynamic>?)
              ?.map(
                (e) =>
                    PromotedSearchItemDto.fromJson(e as Map<String, dynamic>),
              )
              .toList() ??
          [],
      nextCursor: json['next_cursor'] as String?,
      hasMore: json['has_more'] as bool? ?? false,
    );
  }

  Map<String, dynamic> toJson() => {
    'for_sales': forSales.map((e) => e.toJson()).toList(),
    if (nextCursor != null) 'next_cursor': nextCursor,
    'has_more': hasMore,
  };
}

/// For Sale search result DTO
///
/// SKINNY TRUTHFUL DTO — only parses fields the backend ACTUALLY emits.
///
/// Backend response format from search_handler.go fixedPriceSalePreviewsToResponse:
/// {
///   "id": "...",
///   "title": "...",
///   "description": "...",
///   "variety": "...",
///   "price": 1500000,
///   "media_urls": ["..."],
///   "seller_id": "...",
///   "seller_username": "...",
///   "seller_farm_name": "...",
///   "seller_avatar_url": "...",
///   "created_at": "2026-..."
/// }
///
/// DELIBERATELY NOT PARSED (backend does not emit on this surface):
/// - quantity, status, visibility, for_sale_type, updated_at
/// - engagement counts
class ForSaleSearchResultDto extends Equatable {
  final String id;
  final String title;
  final String description;
  final String variety;
  final num? price;
  final List<String> mediaUrls;
  final String sellerId;
  final DateTime createdAt;

  // Owner Truth identity scalars (account / store / avatar).
  // No fullName field — public identity is username/farm.
  final String? sellerUsername;
  final String? sellerFarmName;
  final String? sellerAvatarUrl;

  /// E8.4 — Canonical seller user-identity lifecycle on forSale search rows.
  ///
  /// Sourced ONLY from the nested wire slot `forSale.seller.user.lifecycle`.
  /// Tolerant: null / missing / empty → null (mapper coerces to active).
  ///
  /// AXIS BOUNDARY: this field carries the user-identity axis only.
  final String? sellerUserLifecycle;

  /// Seller-trust axis lifecycle on forSale search rows.
  ///
  /// Sourced from top-level `forSale.seller.lifecycle` populated by
  /// NewSellerCardWithBothLifecycles (search_handler.go). Null → active.
  final String? sellerTrustLifecycle;

  const ForSaleSearchResultDto({
    required this.id,
    required this.title,
    required this.description,
    required this.variety,
    this.price,
    required this.mediaUrls,
    required this.sellerId,
    required this.createdAt,
    this.sellerUsername,
    this.sellerFarmName,
    this.sellerAvatarUrl,
    this.sellerUserLifecycle,
    this.sellerTrustLifecycle,
  });

  factory ForSaleSearchResultDto.fromJson(Map<String, dynamic> json) {
    return ForSaleSearchResultDto(
      id: json['id'] as String,
      title: json['title'] as String? ?? '',
      description: json['description'] as String? ?? '',
      variety: json['variety'] as String? ?? '',
      price: json['price'] as num?,
      mediaUrls:
          (json['media_urls'] as List<dynamic>?)
              ?.map((e) => e.toString())
              .toList() ??
          const <String>[],
      sellerId: json['seller_id'] as String? ?? '',
      createdAt: DateTime.parse(json['created_at'] as String),
      sellerUsername: json['seller_username'] as String?,
      sellerFarmName: json['seller_farm_name'] as String?,
      sellerAvatarUrl: json['seller_avatar_url'] as String?,
      sellerUserLifecycle: _readFixedPriceSaleSearchSellerUserLifecycle(json),
      sellerTrustLifecycle: _readFixedPriceSaleSearchSellerTrustLifecycle(json),
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'title': title,
    'description': description,
    'variety': variety,
    if (price != null) 'price': price,
    'media_urls': mediaUrls,
    'seller_id': sellerId,
    'created_at': createdAt.toIso8601String(),
    if (sellerUsername != null) 'seller_username': sellerUsername,
    if (sellerFarmName != null) 'seller_farm_name': sellerFarmName,
    if (sellerAvatarUrl != null) 'seller_avatar_url': sellerAvatarUrl,
  };

  @override
  List<Object?> get props => [id, title, sellerId, createdAt];
}

// =====================
// Search History DTOs
// =====================

/// Search history entry DTO
///
/// Backend response format:
/// {
///   "id": "...",
///   "query": "...",
///   "created_at": "..."
/// }
class SearchHistoryDto {
  final String id;
  final String query;
  final DateTime createdAt;

  const SearchHistoryDto({
    required this.id,
    required this.query,
    required this.createdAt,
  });

  factory SearchHistoryDto.fromJson(Map<String, dynamic> json) {
    return SearchHistoryDto(
      id: json['id'] as String,
      query: json['query'] as String,
      createdAt: DateTime.parse(json['created_at'] as String),
    );
  }

  Map<String, dynamic> toJson() => {
    'id': id,
    'query': query,
    'created_at': createdAt.toIso8601String(),
  };
}

// ============================================================================
// P3B — Promoted Search Item DTO
// ============================================================================

/// DTO for promoted items in the search sidecar (`promoted_items` array).
///
/// Hand-written (no json_serializable) because the wire shape is different
/// from organic search result DTOs. Same flat shape as feed's
/// PromotedFeedItemDto plus an `inject_at` field for client-side positioning.
///
/// Covers three target types:
/// - promoted_for_sale: for-sale card with price
/// - promoted_auction: auction card with bidding info
/// - promoted_external: external product with URL
class PromotedSearchItemDto {
  final String
  type; // promoted_for_sale, promoted_auction, promoted_external
  /// Canonical contract identity (promotion_contracts.id) carried on the
  /// wire as `contract_id`. The legacy `promotion_instance_id` key is purged.
  final String contractId;
  final String targetType; // for_sale, auction, external_product
  final int injectAt; // 0-based index to insert before in organic list

  // Common
  final String? title;
  final String? imageUrl;
  final String? sellerUsername;
  final String? sellerFarmName;
  final String? sellerLifecycle;

  // ForSale-specific (canonical: for_sale_id from backend)
  final String? forSaleId;
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

  const PromotedSearchItemDto({
    required this.type,
    required this.contractId,
    required this.targetType,
    required this.injectAt,
    this.title,
    this.imageUrl,
    this.sellerUsername,
    this.sellerFarmName,
    this.sellerLifecycle,
    this.forSaleId,
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

  factory PromotedSearchItemDto.fromJson(Map<String, dynamic> json) {
    return PromotedSearchItemDto(
      type: json['type'] as String? ?? '',
      contractId: json['contract_id'] as String? ?? '',
      targetType: json['target_type'] as String? ?? '',
      injectAt: (json['inject_at'] as num?)?.toInt() ?? 0,
      title: json['title'] as String?,
      imageUrl: json['image_url'] as String?,
      sellerUsername: json['seller_username'] as String?,
      sellerFarmName: json['seller_farm_name'] as String?,
      sellerLifecycle: json['seller_lifecycle'] as String?,
      forSaleId: json['for_sale_id'] as String?,
      pricePerUnit: (json['price_per_unit'] as num?)?.toInt(),
      auctionId: json['auction_id'] as String?,
      startPrice: (json['start_price'] as num?)?.toInt(),
      currentBid: (json['current_bid'] as num?)?.toInt(),
      buyNowPrice: (json['buy_now_price'] as num?)?.toInt(),
      endAt: json['end_at'] as String?,
      bidCount: (json['bid_count'] as num?)?.toInt(),
      status: json['status'] as String?,
      externalUrl: json['external_url'] as String?,
      externalMediaUrl: json['external_media_url'] as String?,
    );
  }
}
