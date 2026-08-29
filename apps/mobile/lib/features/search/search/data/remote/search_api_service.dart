import 'package:labuda/core/api/api_client.dart';
import 'package:labuda/core/src/interfaces/services/i_logger_service.dart';
import 'package:labuda/features/search/search/data/dto/search_dto.dart';

/// Search API Service
///
/// FEDERATED SEARCH CONTRACT:
/// - Content search: GET /api/v1/search/content
/// - Listing search: GET /api/v1/search/listings
/// - User search: GET /api/v1/search/users
/// - Auction search: GET /api/v1/search/auctions
/// - Search history: GET/POST/DELETE /api/v1/search/history
///
/// SEARCH CONTRACT:
/// - Supports: Listing, Auction, User, Content
/// - No AI/semantic search
/// - No hashtag search
///
/// Handles all search-related API calls to Go backend.
class SearchApiService {
  final ApiClient _apiClient;
  final ILoggerService? _logger;

  SearchApiService(this._apiClient, {ILoggerService? logger})
    : _logger = logger;

  // =====================
  // Content Search
  // =====================

  /// Search contents
  ///
  /// GET /api/v1/search/content?q={query}&limit={limit}&offset={offset}
  Future<ContentSearchResponseDto> searchContents({
    required String query,
    int limit = 20,
    int offset = 0,
  }) async {
    _logger?.info('Searching contents: query=$query');

    final queryParams = {
      'q': query,
      'limit': limit.toString(),
      'offset': offset.toString(),
    };

    final response = await _apiClient.get(
      '/search/content',
      queryParameters: queryParams,
    );

    return ContentSearchResponseDto.fromJson(
      response.data['data'] as Map<String, dynamic>,
    );
  }

  // =====================
  // Auction Search (PHASE 3.5 - AUCTION SEARCH TRUTH COMPLETION)
  // =====================

  /// Search auctions with full-text search
  ///
  /// GET /api/v1/search/auctions?q={query}&limit={limit}&offset={offset}&sort={sort}
  ///
  /// AUCTION SEARCH ELIGIBILITY:
  /// Only auctions with status IN ('scheduled', 'active', 'ended') are returned
  /// Draft and cancelled auctions are NOT discoverable via search
  ///
  /// Search fields: title, description
  /// Sort options: relevance (active first, then bid count), created_at, end_at
  Future<AuctionSearchResponseDto> searchAuctions({
    required String query,
    int limit = 20,
    int offset = 0,
    String sortBy = 'relevance',
    String sortDir = 'desc',
  }) async {
    _logger?.info('Searching auctions: query=$query');

    final queryParams = {
      'q': query,
      'limit': limit.toString(),
      'offset': offset.toString(),
      if (sortBy != 'relevance') 'sort': sortBy,
      if (sortDir != 'desc') 'sort_dir': sortDir,
    };

    final response = await _apiClient.get(
      '/search/auctions',
      queryParameters: queryParams,
    );

    return AuctionSearchResponseDto.fromJson(
      response.data['data'] as Map<String, dynamic>,
    );
  }

  // =====================
  // Listing Search (REAL LISTINGS TAB)
  // =====================

  /// Search listings with full-text search
  ///
  /// GET /api/v1/search/listings?q={query}&limit={limit}&offset={offset}&sort={sort}&sort_dir={sort_dir}
  ///
  /// Backend search-handler-bound parameters:
  /// - sort_by: "relevance" | "created_at"
  /// - sort_dir: "asc" | "desc"
  ///
  /// IMPORTANT: This is the discovery endpoint — it does NOT reuse the
  /// listing-detail datasource (ListingRemoteDatasource.searchListings)
  /// which returns the full ListingResponseDto and fabricates fields the
  /// search surface never emits.
  Future<ListingSearchResponseDto> searchListings({
    required String query,
    int limit = 20,
    int offset = 0,
    String sortBy = 'relevance',
    String sortDir = 'desc',
  }) async {
    _logger?.info('Searching listings: query=$query');

    final queryParams = {
      'q': query,
      'limit': limit.toString(),
      'offset': offset.toString(),
      if (sortBy != 'relevance') 'sort': sortBy,
      if (sortDir != 'desc') 'sort_dir': sortDir,
    };

    final response = await _apiClient.get(
      '/search/for-sale',
      queryParameters: queryParams,
    );

    return ListingSearchResponseDto.fromJson(
      response.data['data'] as Map<String, dynamic>,
    );
  }

  // =====================
  // User Search
  // =====================

  /// Search users
  ///
  /// GET /api/v1/search/users?q={query}&limit={limit}&offset={offset}
  Future<UserSearchResponseDto> searchUsers({
    required String query,
    int limit = 20,
    int offset = 0,
  }) async {
    _logger?.info('Searching users: query=$query');

    final response = await _apiClient.get(
      '/search/users',
      queryParameters: {
        'q': query,
        'limit': limit.toString(),
        'offset': offset.toString(),
      },
    );

    return UserSearchResponseDto.fromJson(
      response.data['data'] as Map<String, dynamic>,
    );
  }

  // =====================
  // Search History
  // =====================

  /// Get search history for current user
  ///
  /// GET /api/v1/search/history?limit={limit}
  Future<List<SearchHistoryDto>> getSearchHistory({int limit = 20}) async {
    _logger?.info('Getting search history');

    final response = await _apiClient.get(
      '/search/history',
      queryParameters: {'limit': limit.toString()},
    );

    final payload = response.data['data'] as Map<String, dynamic>? ?? {};
    final history =
        (payload['history'] as List?)
            ?.map(
              (json) => SearchHistoryDto.fromJson(json as Map<String, dynamic>),
            )
            .toList() ??
        [];

    return history;
  }

  /// Clear search history for current user
  ///
  /// DELETE /api/v1/search/history
  Future<void> clearSearchHistory() async {
    _logger?.info('Clearing search history');

    await _apiClient.delete('/search/history');
  }

  /// Save search to history
  ///
  /// POST /api/v1/search/history
  Future<void> saveSearchHistory({
    required String query,
    String? searchType,
    int? resultsCount,
  }) async {
    _logger?.info('Saving search to history: $query');

    final data = <String, dynamic>{'query': query};
    if (searchType != null) {
      data['search_type'] = searchType;
    }
    if (resultsCount != null) {
      data['results_count'] = resultsCount;
    }

    await _apiClient.post(
      '/search/history',
      data: data,
    );
  }

  /// Delete specific search history item
  ///
  /// DELETE /api/v1/search/history/{id}
  Future<void> deleteSearchHistoryItem(String historyId) async {
    _logger?.info('Deleting search history item: $historyId');

    await _apiClient.delete('/search/history/$historyId');
  }
}
