import 'package:labuda/features/search/search/data/mappers/search_mapper.dart';
import 'package:labuda/features/search/search/data/remote/search_api_service.dart';
import 'package:labuda/features/search/search/data/dto/search_dto.dart';
import 'package:labuda/features/search/search/domain/entities/search_filters.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart'
    show SearchResult, SearchResultType, UnifiedSearchResults;
import 'package:labuda/features/search/search/domain/repositories/search_repository.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/utils/commerce_seller_identity.dart';

/// Search Repository Implementation using API backend
///
/// FEDERATED SEARCH CONTRACT REALIGN PACK V1:
/// - Aligned with federated search backend endpoints
/// - Uses limit/offset pagination instead of page/pageSize
/// - Content search: /api/v1/search/content
/// - Auction search: /api/v1/search/auctions (PHASE 3.5 - authoritative)
/// - User search: /api/v1/search/users
/// - For Sale search: /api/v1/search/for-sale (via FixedPriceSaleHandler)
/// - Search history: /api/v1/search/history
///
/// SEARCH CONTRACT:
/// - Supports: For Sale, Auction, User, Content
/// - No AI/semantic search
/// - No hashtag search
///
/// SECTION-BASED ALL (canonical):
/// [searchAll] executes the four canonical domain searches in parallel and
/// keeps the results as separate domain collections. Each collection keeps
/// its canonical backend ordering. There is NO flattening into a
/// cross-domain list, NO client-side relevance sort, and NO unified
/// ranking — the All tab renders these collections as independent sections.
class SearchRepositoryImpl implements SearchRepository {
  final SearchApiService _apiService;

  SearchRepositoryImpl(this._apiService);

  @override
  Future<ApiResult<List<ContentSearchResult>>> searchContents({
    required String query,
    int page = 1,
    int pageSize = 20,
  }) async {
    try {
      final offset = (page - 1) * pageSize;
      final response = await _apiService.searchContents(
        query: query,
        limit: pageSize,
        offset: offset,
      );

      final results = response.contents.map((dto) => dto.toDomain()).toList();

      return (data: results, error: null);
    } catch (e) {
      return (data: null, error: 'Failed to search contents: ${e.toString()}');
    }
  }

  @override
  Future<ApiResult<List<ForSaleSearchResult>>> searchForSale({
    required String query,
    String? cursor,
    int limit = 20,
    String sortBy = 'relevance',
    String sortDir = 'desc',
  }) async {
    try {
      final bundleResult = await _fetchForSaleSearchBundle(
        query: query,
        cursor: cursor,
        limit: limit,
        sortBy: sortBy,
        sortDir: sortDir,
      );
      if (bundleResult.error != null) {
        return (data: null, error: bundleResult.error);
      }
      return (data: bundleResult.data!.items, error: null);
    } catch (e) {
      return (data: null, error: 'Failed to search for-sale: ${e.toString()}');
    }
  }

  @override
  Future<ApiResult<List<AuctionSearchResult>>> searchAuctions({
    required String query,
    int page = 1,
    int pageSize = 20,
    String sortBy = 'relevance',
    String sortDir = 'desc',
  }) async {
    try {
      final bundleResult = await _fetchAuctionSearchBundle(
        query: query,
        page: page,
        pageSize: pageSize,
        sortBy: sortBy,
        sortDir: sortDir,
      );
      if (bundleResult.error != null) {
        return (data: null, error: bundleResult.error);
      }
      return (data: bundleResult.data!.items, error: null);
    } catch (e) {
      return (data: null, error: 'Failed to search auctions: ${e.toString()}');
    }
  }

  @override
  Future<ApiResult<List<UserSearchResult>>> searchUsers({
    required String query,
    int page = 1,
    int pageSize = 20,
  }) async {
    try {
      final offset = (page - 1) * pageSize;
      final response = await _apiService.searchUsers(
        query: query,
        limit: pageSize,
        offset: offset,
      );

      final results = response.users.map((dto) => dto.toDomain()).toList();

      return (data: results, error: null);
    } catch (e) {
      return (data: null, error: 'Failed to search users: ${e.toString()}');
    }
  }

  @override
  Future<ApiResult<UnifiedSearchResults>> searchAll({
    required String query,
    SearchFilters? filters,
    int limit = 20,
  }) async {
    try {
      final stopwatch = Stopwatch()..start();

      // Execute the canonical domain searches in parallel — the single
      // execution authority for a query. [limit] is the per-domain page
      // size; each domain response keeps its canonical backend ordering.
      // Promoted sidecar stays tied to the same response bundle that
      // produced the organic items.
      final results = await Future.wait<Object?>([
        searchUsers(query: query, pageSize: limit),
        _fetchForSaleSearchBundle(query: query, limit: limit),
        _fetchAuctionSearchBundle(query: query, pageSize: limit),
        searchContents(query: query, pageSize: limit),
      ]);

      stopwatch.stop();

      final usersResult = results[0] as ApiResult<List<UserSearchResult>>;
      final forSalesBundleResult =
          results[1] as ApiResult<_SearchResultBundle<ForSaleSearchResult>>;
      final auctionsBundleResult =
          results[2] as ApiResult<_SearchResultBundle<AuctionSearchResult>>;
      final contentsResult = results[3] as ApiResult<List<ContentSearchResult>>;

      // Check for errors
      if (usersResult.error != null) {
        return (data: null, error: usersResult.error);
      }
      if (forSalesBundleResult.error != null) {
        return (data: null, error: forSalesBundleResult.error);
      }
      if (auctionsBundleResult.error != null) {
        return (data: null, error: auctionsBundleResult.error);
      }
      if (contentsResult.error != null) {
        return (data: null, error: contentsResult.error);
      }

      // Convert domain results to generic SearchResults, merge promoted
      // sidecar. Each collection keeps its canonical domain order.
      final users = _mapUserResultsToGeneric(usersResult.data!);
      final forSales = _mergePromotedSidecar(
        _mapForSaleResultsToGeneric(forSalesBundleResult.data!.items),
        forSalesBundleResult.data!.promotedItems,
      );
      final auctions = _mergePromotedSidecar(
        _mapAuctionResultsToGeneric(auctionsBundleResult.data!.items),
        auctionsBundleResult.data!.promotedItems,
      );
      final contents = _mapContentResultsToGeneric(contentsResult.data!);

      return (
        data: UnifiedSearchResults(
          users: users,
          forSales: forSales,
          auctions: auctions,
          contents: contents,
          totalCount:
              users.length + forSales.length + auctions.length + contents.length,
          query: query,
          searchDuration: stopwatch.elapsed,
        ),
        error: null,
      );
    } catch (e) {
      return (data: null, error: 'Failed to perform search: ${e.toString()}');
    }
  }

  // Helper methods

  Future<ApiResult<_SearchResultBundle<ForSaleSearchResult>>>
  _fetchForSaleSearchBundle({
    required String query,
    String? cursor,
    int limit = 20,
    String sortBy = 'relevance',
    String sortDir = 'desc',
  }) async {
    try {
      final response = await _apiService.searchForSale(
        query: query,
        cursor: cursor,
        limit: limit,
        sortBy: sortBy,
        sortDir: sortDir,
      );
      return (
        data: _SearchResultBundle(
          items: response.forSales.map((dto) => dto.toDomain()).toList(),
          promotedItems: response.promotedItems,
        ),
        error: null,
      );
    } catch (e) {
      return (data: null, error: 'Failed to search for-sale: ${e.toString()}');
    }
  }

  Future<ApiResult<_SearchResultBundle<AuctionSearchResult>>>
  _fetchAuctionSearchBundle({
    required String query,
    int page = 1,
    int pageSize = 20,
    String sortBy = 'relevance',
    String sortDir = 'desc',
  }) async {
    try {
      final offset = (page - 1) * pageSize;
      final response = await _apiService.searchAuctions(
        query: query,
        limit: pageSize,
        offset: offset,
        sortBy: sortBy,
        sortDir: sortDir,
      );
      return (
        data: _SearchResultBundle(
          items: response.auctions.map(_mapAuctionDtoToDomain).toList(),
          promotedItems: response.promotedItems,
        ),
        error: null,
      );
    } catch (e) {
      return (data: null, error: 'Failed to search auctions: ${e.toString()}');
    }
  }

  List<SearchResult> _mapUserResultsToGeneric(List<UserSearchResult> data) {
    return data
        .map(
          (r) => SearchResult(
            id: r.id,
            type: SearchResultType.user,
            title: '@${r.username}',
            subtitle: '@${r.username}',
            imageUrl: r.avatarUrl,
            metadata: {'username': r.username, 'bio': r.bio},
            createdAt: DateTime.now(),
          ),
        )
        .toList();
  }

  /// REAL FOR SALE TAB: map ForSaleSearchResult to generic SearchResult.
  ///
  /// Owner Truth: subtitle prefers farmName, falling back to @username.
  /// When neither is present, subtitle is null (hide rather than fabricate).
  ///
  /// Mapping rules (no fabrication):
  /// - title    ← forSale title
  /// - subtitle ← sellerFarmName ?? '@sellerUsername' ?? null
  /// - imageUrl ← first media_urls element, or null
  /// - metadata ← {'price': ...} ONLY when price is non-null;
  ///              {'sellerId': ...} for downstream consumers.
  ///   No quantity / status / visibility / for_sale_type / engagement
  ///   are emitted by /search/for-sale, so none are added here.
  List<SearchResult> _mapForSaleResultsToGeneric(
    List<ForSaleSearchResult> data,
  ) {
    return data
        .map(
          (r) => SearchResult(
            id: r.id,
            type: SearchResultType.forSale,
            title: r.title,
            subtitle: buildCommerceSellerIdentity(
              username: r.sellerUsername,
              storeName: r.sellerFarmName,
            )?.multilineLabel,
            imageUrl: r.mediaUrls.isNotEmpty ? r.mediaUrls.first : null,
            description: r.description.isEmpty ? null : r.description,
            metadata: {
              if (r.price != null) 'price': r.price,
              'sellerId': r.sellerId,
              if (r.variety.isNotEmpty) 'variety': r.variety,
              // E8.4 — Seller user-axis lifecycle as wire string for the
              // renderer. Carries the user-identity axis ONLY.
              'sellerLifecycle': r.sellerUserLifecycle.name,
              // Seller-trust axis (subscription expired/lapsed).
              'sellerTrustLifecycle': r.sellerTrustLifecycle.name,
            },
            createdAt: r.createdAt,
          ),
        )
        .toList();
  }

  /// STAGE 4 — UI MIGRATION (Phase 5)
  /// Owner-truth subtitle composition for forSale/auction search rows.
  /// - prefer farmName
  /// - else @username
  /// - else null (hide rather than fabricate; no Unknown / Seller / etc.)
  List<SearchResult> _mapContentResultsToGeneric(
    List<ContentSearchResult> data,
  ) {
    // Governance discovery rule: TOMBSTONE (lifecycle == removed) drops from
    // the list at the projection boundary. unavailable items continue through
    // and are greyed by the renderer using metadata['lifecycle'].
    return data
        .where((r) => !r.lifecycle.shouldDropFromList)
        .map(
          (r) => SearchResult(
            id: r.id,
            type: SearchResultType.content,
            title: r.title,
            subtitle: r.authorUsername.isEmpty ? 'Content' : r.authorUsername,
            imageUrl: r.thumbnailUrl ?? r.authorAvatarUrl,
            description: r.description,
            metadata: {
              'price': r.price,
              'authorId': r.authorId,
              'authorUsername': r.authorUsername,
              'lifecycle': r.lifecycle.name,
              'authorLifecycle': r.authorLifecycle.name,
            },
            createdAt: r.createdAt,
          ),
        )
        .toList();
  }

  /// Map auction results to generic SearchResult format.
  ///
  /// Owner Truth: subtitle prefers farmName, falling back to @username,
  /// then null (hide rather than fabricate).
  List<SearchResult> _mapAuctionResultsToGeneric(
    List<AuctionSearchResult> data,
  ) {
    return data
        .map(
          (r) => SearchResult(
            id: r.id,
            type: SearchResultType.auction,
            title: r.title,
            subtitle: buildCommerceSellerIdentity(
              username: r.sellerUsername,
              storeName: r.sellerFarmName,
            )?.multilineLabel,
            imageUrl: r.thumbnailUrl,
            description: r.description,
            metadata: {
              'price': r.displayPrice,
              'startPrice': r.startPrice,
              'currentBid': r.currentBid,
              'buyNowPrice': r.buyNowPrice,
              'displayPrice': r.displayPrice,
              'sellerId': r.sellerId,
              'bidCount': r.bidCount,
              'startAt': r.startAt.toIso8601String(),
              'endAt': r.endAt.toIso8601String(),
              'status': r.status,
              'isActive': r.isActive,
              'isScheduled': r.isScheduled,
              'isEnded': r.isEnded,
              // E8.4 — Seller user-axis lifecycle as wire string.
              'sellerLifecycle': r.sellerUserLifecycle.name,
              // Seller-trust axis (subscription expired/lapsed).
              'sellerTrustLifecycle': r.sellerTrustLifecycle.name,
            },
            createdAt: r.createdAt,
          ),
        )
        .toList();
  }

  /// Map AuctionSearchResultDto to AuctionSearchResult domain.
  ///
  /// Owner Truth: backend identity scalars (`seller_username`,
  /// `seller_farm_name`, `seller_avatar_url`) pass straight through.
  /// No fullName fallback.
  AuctionSearchResult _mapAuctionDtoToDomain(AuctionSearchResultDto dto) {
    return AuctionSearchResult(
      id: dto.id,
      sellerId: dto.sellerId,
      productId: dto.productId,
      title: dto.title,
      description: dto.description,
      startPrice: dto.startPrice,
      currentBid: dto.currentBid,
      buyNowPrice: dto.buyNowPrice,
      startAt: dto.startAt,
      endAt: dto.endAt,
      status: dto.status,
      thumbnailUrl: dto.thumbnailUrl,
      sellerUsername: dto.sellerUsername,
      sellerFarmName: dto.sellerFarmName,
      sellerAvatarUrl: dto.sellerAvatarUrl,
      bidCount: dto.bidCount,
      createdAt: dto.createdAt,
      // E8.4 — null / missing / empty / unknown → active (forward-compat).
      sellerUserLifecycle: ContentLifecycleParse.fromWire(
        dto.sellerUserLifecycle,
      ),
      // Trust-axis — null / missing / unknown → active (forward-compat).
      sellerTrustLifecycle: ContentLifecycleParse.fromWire(
        dto.sellerTrustLifecycle,
      ),
    );
  }

  // =====================
  // P3B — Server-side promoted sidecar merge
  // =====================

  /// Convert a promoted sidecar DTO to a generic SearchResult.
  SearchResult? _promotedSearchItemToSearchResult(PromotedSearchItemDto dto) {
    final SearchResultType type;
    final String id;
    switch (dto.targetType) {
      case 'for_sale':
        type = SearchResultType.forSale;
        id = dto.forSaleId ?? dto.contractId;
      case 'auction':
        type = SearchResultType.auction;
        id = dto.auctionId ?? dto.contractId;
      case 'external_product':
        type = SearchResultType.externalProduct;
        id = dto.contractId;
      default:
        return null;
    }

    final sellerLabel = _formatSellerLabel(
      dto.sellerUsername,
      dto.sellerFarmName,
    );
    return SearchResult(
      id: id,
      type: type,
      title: dto.title ?? '',
      subtitle: sellerLabel,
      imageUrl: dto.imageUrl ?? dto.externalMediaUrl,
      metadata: {
        if (dto.pricePerUnit != null) 'price': dto.pricePerUnit,
        if (dto.sellerUsername != null) 'sellerUsername': dto.sellerUsername,
        if (dto.sellerFarmName != null) 'sellerFarmName': dto.sellerFarmName,
        if (dto.sellerLifecycle != null) 'sellerLifecycle': dto.sellerLifecycle,
        if (dto.startPrice != null) 'startPrice': dto.startPrice,
        if (dto.currentBid != null) 'currentBid': dto.currentBid,
        if (dto.buyNowPrice != null) 'buyNowPrice': dto.buyNowPrice,
        if (dto.bidCount != null) 'bidCount': dto.bidCount,
        if (dto.endAt != null) 'endAt': dto.endAt,
        if (dto.status != null) 'status': dto.status,
        if (dto.externalUrl != null) 'externalUrl': dto.externalUrl,
        if (dto.targetType == 'external_product') 'isExternal': true,
      },
      createdAt: DateTime.now(),
      isPromoted: true,
      contractId: dto.contractId,
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

  /// Insert promoted sidecar items at their inject_at positions.
  List<SearchResult> _mergePromotedSidecar(
    List<SearchResult> organic,
    List<PromotedSearchItemDto> promoted,
  ) {
    if (promoted.isEmpty) return organic;
    final result = List<SearchResult>.from(organic);
    for (final item in promoted) {
      final sr = _promotedSearchItemToSearchResult(item);
      if (sr == null) continue;
      final insertIdx = item.injectAt.clamp(0, result.length);
      result.insert(insertIdx, sr);
    }
    return result;
  }
}

class _SearchResultBundle<T> {
  final List<T> items;
  final List<PromotedSearchItemDto> promotedItems;

  const _SearchResultBundle({required this.items, required this.promotedItems});
}
