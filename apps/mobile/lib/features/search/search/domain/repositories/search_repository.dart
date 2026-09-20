import 'package:labuda/features/search/search/domain/entities/search_filters.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

/// Result type for operations that can fail
typedef ApiResult<T> = ({T? data, String? error});

/// Search Repository Interface
///
/// Defines operations for searching across the application:
/// - Content search (universal content rows)
/// - Auction search (authoritative auction search)
/// - User search
/// - Search history management
///
/// SEARCH CONTRACT:
/// - Supports: ForSale, Auction, User, Content
/// - No AI/semantic search
/// - No hashtag search
abstract class SearchRepository {
  // =====================
  // Content Search
  // =====================

  /// Search contents
  Future<ApiResult<List<ContentSearchResult>>> searchContents({
    required String query,
    int page = 1,
    int pageSize = 20,
  });

  // =====================
  // Auction Search (PHASE 3.5 - AUCTION SEARCH TRUTH COMPLETION)
  // =====================

  /// Search auctions with full-text search
  ///
  /// Uses authoritative auction search endpoint:
  /// GET /api/v1/search/auctions
  ///
  /// Only returns auctions with status IN ('scheduled', 'active', 'ended')
  /// Draft and cancelled auctions are NOT discoverable via search
  Future<ApiResult<List<AuctionSearchResult>>> searchAuctions({
    required String query,
    int page = 1,
    int pageSize = 20,
    String sortBy = 'relevance',
    String sortDir = 'desc',
  });

  // =====================
  // For Sale Search (REAL FOR SALE TAB)
  // =====================

  /// Search for-sale items with full-text search
  ///
  /// Uses the discovery endpoint:
  /// GET /api/v1/search/for-sale
  ///
  /// Returns ONLY the fields actually emitted by /search/for-sale —
  /// no fabricated quantity / status / visibility / for_sale_type.
  Future<ApiResult<List<ForSaleSearchResult>>> searchForSale({
    required String query,
    String? cursor,
    int limit = 20,
    String sortBy = 'relevance',
    String sortDir = 'desc',
  });

  // =====================
  // User Search
  // =====================

  /// Search users by username or name
  Future<ApiResult<List<UserSearchResult>>> searchUsers({
    required String query,
    int page = 1,
    int pageSize = 20,
  });

  // =====================
  // Unified Search (All Types)
  // =====================

  /// Unified search across all content types.
  ///
  /// SECTION-BASED ALL (canonical): this is the single execution authority
  /// for a search query. It runs each canonical domain search in parallel
  /// and returns the results as separate domain collections ([users],
  /// [forSales], [auctions], [contents]), each in its own canonical backend
  /// order. There is no cross-domain flattening and no unified relevance
  /// ranking.
  ///
  /// [limit] is the per-domain page size (default 20).
  Future<ApiResult<UnifiedSearchResults>> searchAll({
    required String query,
    SearchFilters? filters,
    int limit = 20,
  });
}

/// Content search result (from API)
///
/// Engagement counts (likesCount, commentsCount) are nullable because
/// /search/content does NOT emit them. They are kept on the entity only
/// to leave room for a future backend contract; mappers must pass null
/// rather than fabricate a 0.
///
/// `lifecycle` carries the canonical governance lifecycle ({active,
/// unavailable, removed}) and is SEPARATE from raw `status`. Discovery
/// surfaces branch UX on this field; tombstones drop from the list,
/// unavailable items grey out. Default is active for backward compat.
class ContentSearchResult {
  final String id;
  final String title;
  final String? description;
  final String contentType;
  final String status;
  final ContentLifecycle lifecycle;
  final String? thumbnailUrl;
  final double? price;
  final String authorId;
  final String authorUsername;
  final String? authorAvatarUrl;
  final int? likesCount;
  final int? commentsCount;
  final DateTime createdAt;

  /// E9.1 — Author user-identity lifecycle (banned/deleted user).
  /// Default active when wire omits the nested slot (pre-E9.1 / unknown).
  /// AXIS BOUNDARY: `lifecycle` above is the item-axis; this field is the
  /// author-identity axis. They are independent and must never be conflated.
  final ContentLifecycle authorLifecycle;

  const ContentSearchResult({
    required this.id,
    required this.title,
    this.description,
    required this.contentType,
    required this.status,
    this.lifecycle = ContentLifecycle.active,
    this.thumbnailUrl,
    this.price,
    required this.authorId,
    required this.authorUsername,
    this.authorAvatarUrl,
    this.likesCount,
    this.commentsCount,
    required this.createdAt,
    this.authorLifecycle = ContentLifecycle.active,
  });
}

/// User search result (from API)
class UserSearchResult {
  final String id;
  final String username;
  final String? avatarUrl;
  final String? bio;
  final bool? isFollowing;

  const UserSearchResult({
    required this.id,
    required this.username,
    this.avatarUrl,
    this.bio,
    this.isFollowing,
  });
}

/// For Sale search result (from /api/v1/search/for-sale)
///
/// SKINNY TRUTHFUL ENTITY — only fields the discovery endpoint emits.
/// DO NOT ADD: quantity, status, visibility, or any fabricated type field,
/// updated_at, engagement counts — none of these are emitted by
/// /search/for-sale.
///
/// Owner-truth identity (`sellerUsername`, `sellerFarmName`,
/// `sellerAvatarUrl`) is the public seller identity surface.
///
/// `sellerUserLifecycle` is the user-identity axis (E8.4). It is
/// SEPARATE from any seller-trust axis (`seller.lifecycle` is reserved
/// and never consumed). Default = active for pre-E8.1 payloads.
class ForSaleSearchResult {
  final String id;
  final String title;
  final String description;
  final String variety;
  final num? price;
  final List<String> mediaUrls;
  final String sellerId;
  final DateTime createdAt;

  // Owner Truth identity fields (username/farmName/avatar).
  final String? sellerUsername;
  final String? sellerFarmName;
  final String? sellerAvatarUrl;

  // E8.4 — Seller user-axis lifecycle (banned/deleted user). Default
  // active when wire omits the nested slot (pre-E8.1 / unknown values).
  final ContentLifecycle sellerUserLifecycle;

  // Seller-trust axis lifecycle (subscription expired/lapsed). Default
  // active when wire omits the field (pre-convergence / unknown values).
  final ContentLifecycle sellerTrustLifecycle;

  const ForSaleSearchResult({
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
    this.sellerUserLifecycle = ContentLifecycle.active,
    this.sellerTrustLifecycle = ContentLifecycle.active,
  });
}

/// Auction search result (from API)
///
/// Authoritative auction search result from GET /api/v1/search/auctions.
///
/// Owner-truth identity (`sellerUsername`, `sellerFarmName`,
/// `sellerAvatarUrl`) is the public seller identity surface.
class AuctionSearchResult {
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

  // Owner Truth identity fields (username/farmName/avatar).
  final String? sellerUsername;
  final String? sellerFarmName;
  final String? sellerAvatarUrl;

  // E8.4 — Seller user-axis lifecycle (banned/deleted user). Default
  // active when wire omits the nested slot (pre-E8.1 / unknown values).
  final ContentLifecycle sellerUserLifecycle;

  // Seller-trust axis lifecycle (subscription expired/lapsed). Default
  // active when wire omits the field (pre-convergence / unknown values).
  final ContentLifecycle sellerTrustLifecycle;

  const AuctionSearchResult({
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
    this.sellerUserLifecycle = ContentLifecycle.active,
    this.sellerTrustLifecycle = ContentLifecycle.active,
  });

  /// Returns the display price (current bid if exists, otherwise start price)
  int get displayPrice => currentBid ?? startPrice;

  /// Check if auction is currently active
  bool get isActive => status == 'active';

  /// Check if auction is scheduled (upcoming)
  bool get isScheduled => status == 'scheduled';

  /// Check if auction has ended
  bool get isEnded => status == 'ended';
}
