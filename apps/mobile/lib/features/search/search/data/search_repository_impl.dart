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
/// - Listing search: /api/v1/search/listings (via FixedPriceSaleHandler)
/// - Search history: /api/v1/search/history
///
/// SEARCH CONTRACT:
/// - Supports: Listing, Auction, User, Content
/// - No AI/semantic search
/// - No hashtag search
///
/// PHASE 3.5 - AUCTION SEARCH TRUTH COMPLETION:
/// - Replaced workaround auction search (via content search filter)
/// - Now uses authoritative auction search endpoint
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
  Future<ApiResult<List<ListingSearchResult>>> searchListings({
    required String query,
    int page = 1,
    int pageSize = 20,
    String sortBy = 'relevance',
    String sortDir = 'desc',
  }) async {
    try {
      final bundleResult = await _fetchListingSearchBundle(
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
      return (data: null, error: 'Failed to search listings: ${e.toString()}');
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
    SearchSortBy sortBy = SearchSortBy.relevance,
    int limit = 20,
  }) async {
    try {
      final stopwatch = Stopwatch()..start();
      final limitPerType = (limit / 4).ceil();

      // Execute searches in parallel.
      // Promoted sidecar stays tied to the same response bundle that
      // produced the organic items.
      final results = await Future.wait<Object?>([
        searchUsers(query: query, pageSize: limitPerType),
        _fetchListingSearchBundle(
          query: query,
          pageSize: limitPerType,
          sortBy: _mapSearchSortByToBackendSortBy(sortBy),
        ),
        _fetchAuctionSearchBundle(
          query: query,
          pageSize: limitPerType,
          sortBy: _mapSearchSortByToBackendSortBy(sortBy),
        ),
        searchContents(query: query, pageSize: limitPerType),
      ]);

      stopwatch.stop();

      final usersResult = results[0] as ApiResult<List<UserSearchResult>>;
      final listingsBundleResult =
          results[1] as ApiResult<_SearchResultBundle<ListingSearchResult>>;
      final auctionsBundleResult =
          results[2] as ApiResult<_SearchResultBundle<AuctionSearchResult>>;
      final contentsResult = results[3] as ApiResult<List<ContentSearchResult>>;

      // Check for errors
      if (usersResult.error != null) {
        return (data: null, error: usersResult.error);
      }
      if (listingsBundleResult.error != null) {
        return (data: null, error: listingsBundleResult.error);
      }
      if (auctionsBundleResult.error != null) {
        return (data: null, error: auctionsBundleResult.error);
      }
      if (contentsResult.error != null) {
        return (data: null, error: contentsResult.error);
      }

      // Convert domain results to generic SearchResults, merge promoted sidecar
      final users = _mapUserResultsToGeneric(usersResult.data!);
      final listings = _mergePromotedSidecar(
        _mapListingResultsToGeneric(listingsBundleResult.data!.items),
        listingsBundleResult.data!.promotedItems,
      );
      final auctions = _mergePromotedSidecar(
        _mapAuctionResultsToGeneric(auctionsBundleResult.data!.items),
        auctionsBundleResult.data!.promotedItems,
      );
      final contents = _mapContentResultsToGeneric(contentsResult.data!);

      // Merge all results
      final allResults = <SearchResult>[
        ...users,
        ...listings,
        ...auctions,
        ...contents,
      ];

      // Sort by relevance or other criteria
      _sortResults(allResults, sortBy);

      return (
        data: UnifiedSearchResults(
          allResults: allResults.take(limit).toList(),
          users: users,
          listings: listings,
          auctions: auctions,
          contents: contents,
          totalCount: allResults.length,
          query: query,
          searchDuration: stopwatch.elapsed,
        ),
        error: null,
      );
    } catch (e) {
      return (data: null, error: 'Failed to perform search: ${e.toString()}');
    }
  }

  @override
  Future<ApiResult<List<SearchResult>>> searchByType({
    required String query,
    required SearchResultType type,
    SearchFilters? filters,
    SearchSortBy sortBy = SearchSortBy.relevance,
    int limit = 20,
    String? cursor,
  }) async {
    try {
      final ApiResult<List<SearchResult>> result;

      switch (type) {
        case SearchResultType.user:
          final userResult = await searchUsers(query: query, pageSize: limit);
          if (userResult.error != null) {
            return (data: null, error: userResult.error);
          }
          result = (
            data: _mapUserResultsToGeneric(userResult.data!),
            error: null,
          );
          break;

        case SearchResultType.listing:
        case SearchResultType.externalProduct:
          final listingBundleResult = await _fetchListingSearchBundle(
            query: query,
            pageSize: limit,
            sortBy: _mapSearchSortByToBackendSortBy(sortBy),
          );
          if (listingBundleResult.error != null) {
            return (data: null, error: listingBundleResult.error);
          }
          result = (
            data: _mergePromotedSidecar(
              _mapListingResultsToGeneric(listingBundleResult.data!.items),
              listingBundleResult.data!.promotedItems,
            ),
            error: null,
          );
          break;

        case SearchResultType.auction:
          final auctionBundleResult = await _fetchAuctionSearchBundle(
            query: query,
            pageSize: limit,
            sortBy: _mapSearchSortByToBackendSortBy(sortBy),
          );
          if (auctionBundleResult.error != null) {
            return (data: null, error: auctionBundleResult.error);
          }
          result = (
            data: _mergePromotedSidecar(
              _mapAuctionResultsToGeneric(auctionBundleResult.data!.items),
              auctionBundleResult.data!.promotedItems,
            ),
            error: null,
          );
          break;

        case SearchResultType.content:
          final contentResult = await searchContents(
            query: query,
            pageSize: limit,
          );
          if (contentResult.error != null) {
            return (data: null, error: contentResult.error);
          }
          result = (
            data: _mapContentResultsToGeneric(contentResult.data!),
            error: null,
          );
          break;
      }

      final results = result.data!;
      _sortResults(results, sortBy);

      return (data: results, error: null);
    } catch (e) {
      return (
        data: null,
        error: 'Failed to search ${type.name}: ${e.toString()}',
      );
    }
  }

  // Helper methods

  Future<ApiResult<_SearchResultBundle<ListingSearchResult>>>
  _fetchListingSearchBundle({
    required String query,
    int page = 1,
    int pageSize = 20,
    String sortBy = 'relevance',
    String sortDir = 'desc',
  }) async {
    try {
      final offset = (page - 1) * pageSize;
      final response = await _apiService.searchListings(
        query: query,
        limit: pageSize,
        offset: offset,
        sortBy: sortBy,
        sortDir: sortDir,
      );
      return (
        data: _SearchResultBundle(
          items: response.listings.map((dto) => dto.toDomain()).toList(),
          promotedItems: response.promotedItems,
        ),
        error: null,
      );
    } catch (e) {
      return (data: null, error: 'Failed to search listings: ${e.toString()}');
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

  /// REAL LISTINGS TAB: map ListingSearchResult to generic SearchResult.
  ///
  /// Owner Truth: subtitle prefers farmName, falling back to @username.
  /// When neither is present, subtitle is null (hide rather than fabricate).
  ///
  /// Mapping rules (no fabrication):
  /// - title    ← listing title
  /// - subtitle ← sellerFarmName ?? '@sellerUsername' ?? null
  /// - imageUrl ← first media_urls element, or null
  /// - metadata ← {'price': ...} ONLY when price is non-null;
  ///              {'sellerId': ...} for downstream consumers.
  ///   No quantity / status / visibility / listing_type / engagement
  ///   are emitted by /search/listings, so none are added here.
  List<SearchResult> _mapListingResultsToGeneric(
    List<ListingSearchResult> data,
  ) {
    return data
        .map(
          (r) => SearchResult(
            id: r.id,
            type: SearchResultType.listing,
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
  /// Owner-truth subtitle composition for listing/auction search rows.
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

  /// PHASE 3.5: Map SearchSortBy to backend sort_by parameter
  String _mapSearchSortByToBackendSortBy(SearchSortBy sortBy) {
    switch (sortBy) {
      case SearchSortBy.relevance:
        return 'relevance';
      case SearchSortBy.newest:
      case SearchSortBy.oldest:
        return 'created_at';
      case SearchSortBy.priceAsc:
      case SearchSortBy.priceDesc:
        return 'relevance'; // Use relevance for auctions (price sorting not supported)
      case SearchSortBy.popularity:
        return 'relevance'; // Use relevance (bid_count is already part of relevance)
    }
  }

  void _sortResults(List<SearchResult> results, SearchSortBy sortBy) {
    switch (sortBy) {
      case SearchSortBy.relevance:
        results.sort((a, b) => b.relevanceScore.compareTo(a.relevanceScore));
      case SearchSortBy.newest:
        results.sort((a, b) => b.createdAt.compareTo(a.createdAt));
      case SearchSortBy.oldest:
        results.sort((a, b) => a.createdAt.compareTo(b.createdAt));
      case SearchSortBy.priceAsc:
        results.sort((a, b) {
          final priceA = a.metadata['price'] as num? ?? 0;
          final priceB = b.metadata['price'] as num? ?? 0;
          return priceA.compareTo(priceB);
        });
      case SearchSortBy.priceDesc:
        results.sort((a, b) {
          final priceA = a.metadata['price'] as num? ?? 0;
          final priceB = b.metadata['price'] as num? ?? 0;
          return priceB.compareTo(priceA);
        });
      case SearchSortBy.popularity:
        results.sort((a, b) {
          final popA =
              (a.metadata['followersCount'] as int? ?? 0) +
              (a.metadata['likesCount'] as int? ?? 0);
          final popB =
              (b.metadata['followersCount'] as int? ?? 0) +
              (b.metadata['likesCount'] as int? ?? 0);
          return popB.compareTo(popA);
        });
    }
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
        type = SearchResultType.listing;
        id = dto.forSaleId ?? dto.promotionInstanceId;
      case 'auction':
        type = SearchResultType.auction;
        id = dto.auctionId ?? dto.promotionInstanceId;
      case 'external_product':
        type = SearchResultType.externalProduct;
        id = dto.promotionInstanceId;
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
      promotionInstanceId: dto.promotionInstanceId,
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
