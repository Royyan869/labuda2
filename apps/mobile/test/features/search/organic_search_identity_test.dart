import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/features/search/search/data/dto/search_dto.dart';
import 'package:labuda/features/search/search/data/remote/search_api_service.dart';
import 'package:labuda/features/search/search/data/search_repository_impl.dart';

class _FakeOrganicSearchApiService implements SearchApiService {
  @override
  Future<ForSaleSearchResponseDto> searchForSale({
    required String query,
    String? cursor,
    int limit = 20,
    String sortBy = 'relevance',
    String sortDir = 'desc',
  }) async {
    return ForSaleSearchResponseDto(
      forSales: [
        ForSaleSearchResultDto(
          id: 'l1',
          title: 'Showa Koi 30cm',
          description: 'Beautiful showa',
          variety: 'Showa',
          price: 1500000,
          mediaUrls: const ['https://example.com/forSale.jpg'],
          sellerId: 'seller-1',
          createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
          sellerUsername: 'seller_user',
          sellerFarmName: 'Farm Name',
          sellerAvatarUrl: 'https://example.com/avatar.jpg',
        ),
      ],
      nextCursor: null,
      hasMore: false,
    );
  }

  @override
  Future<AuctionSearchResponseDto> searchAuctions({
    required String query,
    int limit = 20,
    int offset = 0,
    String sortBy = 'relevance',
    String sortDir = 'desc',
  }) async {
    return AuctionSearchResponseDto(
      query: query,
      auctions: [
        AuctionSearchResultDto(
          id: 'a1',
          sellerId: 'seller-2',
          productId: 'product-1',
          title: 'Sanke Auction',
          description: 'Rare sanke',
          startPrice: 2500000,
          startAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
          endAt: DateTime.parse('2026-01-02T00:00:00.000Z'),
          status: 'active',
          bidCount: 3,
          createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
          sellerUsername: 'auction_user',
          sellerFarmName: 'Auction Farm',
          sellerAvatarUrl: 'https://example.com/auction-avatar.jpg',
        ),
      ],
      total: 1,
      limit: limit,
      offset: offset,
    );
  }

  @override
  Future<ContentSearchResponseDto> searchContents({
    required String query,
    String? contentType,
    int limit = 20,
    int offset = 0,
  }) async {
    // Not under test: this domain only has to succeed so `searchAll` can map
    // the for-sale/auction rows the identity rule is asserted on.
    return ContentSearchResponseDto(
      query: query,
      contents: const [],
      total: 0,
      limit: limit,
      offset: offset,
    );
  }

  @override
  Future<UserSearchResponseDto> searchUsers({
    required String query,
    int limit = 20,
    int offset = 0,
  }) async {
    return UserSearchResponseDto(
      query: query,
      users: const [],
      total: 0,
      limit: limit,
      offset: offset,
    );
  }

  @override
  Future<List<SearchHistoryDto>> getSearchHistory({int limit = 20}) {
    throw UnimplementedError();
  }

  @override
  Future<void> clearSearchHistory() => throw UnimplementedError();

  @override
  Future<void> saveSearchHistory({
    required String query,
    String? searchType,
    int? resultsCount,
  }) => throw UnimplementedError();

  @override
  Future<void> deleteSearchHistoryItem(String historyId) =>
      throw UnimplementedError();
}

void main() {
  test(
    'organic search rows pair the store name with the handle',
    () async {
      final repository = SearchRepositoryImpl(_FakeOrganicSearchApiService());

      final results = await repository.searchAll(query: 'koi');

      expect(results.error, isNull);
      final unified = results.data!;
      expect(unified.forSales, hasLength(1));
      expect(unified.auctions, hasLength(1));

      // ONE identity rule, applied by the repository itself: the store name is
      // the primary line, the handle is secondary.
      expect(unified.forSales.single.subtitle, 'Farm Name\n@seller_user');
      expect(unified.auctions.single.subtitle, 'Auction Farm\n@auction_user');
    },
  );

  test('organic search falls back to @username when farm is missing', () async {
    final repository = SearchRepositoryImpl(
      _FakeOrganicSearchApiServiceMissingFarm(),
    );

    final results = await repository.searchAll(query: 'koi');

    expect(results.error, isNull);
    expect(results.data!.forSales, hasLength(1));
    // No store: the handle is the primary line on its own.
    expect(results.data!.forSales.single.subtitle, '@seller_user');
  });
}

class _FakeOrganicSearchApiServiceMissingFarm implements SearchApiService {
  @override
  Future<ForSaleSearchResponseDto> searchForSale({
    required String query,
    String? cursor,
    int limit = 20,
    String sortBy = 'relevance',
    String sortDir = 'desc',
  }) async {
    return ForSaleSearchResponseDto(
      forSales: [
        ForSaleSearchResultDto(
          id: 'l1',
          title: 'Showa Koi 30cm',
          description: 'Beautiful showa',
          variety: 'Showa',
          price: 1500000,
          mediaUrls: const ['https://example.com/forSale.jpg'],
          sellerId: 'seller-1',
          createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
          sellerUsername: 'seller_user',
          sellerFarmName: null,
          sellerAvatarUrl: 'https://example.com/avatar.jpg',
        ),
      ],
      nextCursor: null,
      hasMore: false,
    );
  }

  @override
  Future<AuctionSearchResponseDto> searchAuctions({
    required String query,
    int limit = 20,
    int offset = 0,
    String sortBy = 'relevance',
    String sortDir = 'desc',
  }) async {
    return AuctionSearchResponseDto(
      query: query,
      auctions: const [],
      total: 0,
      limit: limit,
      offset: offset,
    );
  }

  @override
  Future<ContentSearchResponseDto> searchContents({
    required String query,
    String? contentType,
    int limit = 20,
    int offset = 0,
  }) async {
    // Not under test: this domain only has to succeed so `searchAll` can map
    // the for-sale/auction rows the identity rule is asserted on.
    return ContentSearchResponseDto(
      query: query,
      contents: const [],
      total: 0,
      limit: limit,
      offset: offset,
    );
  }

  @override
  Future<UserSearchResponseDto> searchUsers({
    required String query,
    int limit = 20,
    int offset = 0,
  }) async {
    return UserSearchResponseDto(
      query: query,
      users: const [],
      total: 0,
      limit: limit,
      offset: offset,
    );
  }

  @override
  Future<List<SearchHistoryDto>> getSearchHistory({int limit = 20}) {
    throw UnimplementedError();
  }

  @override
  Future<void> clearSearchHistory() => throw UnimplementedError();

  @override
  Future<void> saveSearchHistory({
    required String query,
    String? searchType,
    int? resultsCount,
  }) => throw UnimplementedError();

  @override
  Future<void> deleteSearchHistoryItem(String historyId) =>
      throw UnimplementedError();
}
