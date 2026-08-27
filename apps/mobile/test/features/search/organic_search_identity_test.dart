import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/features/search/search/data/dto/search_dto.dart';
import 'package:labuda/features/search/search/data/remote/search_api_service.dart';
import 'package:labuda/features/search/search/data/search_repository_impl.dart';
import 'package:labuda/shared/utils/commerce_seller_identity.dart';

class _FakeOrganicSearchApiService implements SearchApiService {
  @override
  Future<ListingSearchResponseDto> searchListings({
    required String query,
    int limit = 20,
    int offset = 0,
    String sortBy = 'relevance',
    String sortDir = 'desc',
  }) async {
    return ListingSearchResponseDto(
      query: query,
      listings: [
        ListingSearchResultDto(
          id: 'l1',
          title: 'Showa Koi 30cm',
          description: 'Beautiful showa',
          variety: 'Showa',
          price: 1500000,
          mediaUrls: const ['https://example.com/listing.jpg'],
          sellerId: 'seller-1',
          createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
          sellerUsername: 'seller_user',
          sellerFarmName: 'Farm Name',
          sellerAvatarUrl: 'https://example.com/avatar.jpg',
        ),
      ],
      total: 1,
      limit: limit,
      offset: offset,
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
  }) {
    throw UnimplementedError();
  }

  @override
  Future<UserSearchResponseDto> searchUsers({
    required String query,
    int limit = 20,
    int offset = 0,
  }) {
    throw UnimplementedError();
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
    'organic listing and auction search render split seller identity',
    () async {
      final repository = SearchRepositoryImpl(_FakeOrganicSearchApiService());

      final listings = await repository.searchListings(query: 'koi');
      final auctions = await repository.searchAuctions(query: 'koi');

      expect(listings.error, isNull);
      expect(auctions.error, isNull);

      expect(listings.data, hasLength(1));
      expect(auctions.data, hasLength(1));

      final listingIdentity = buildCommerceSellerIdentity(
        username: listings.data!.single.sellerUsername,
        storeName: listings.data!.single.sellerFarmName,
      );
      final auctionIdentity = buildCommerceSellerIdentity(
        username: auctions.data!.single.sellerUsername,
        storeName: auctions.data!.single.sellerFarmName,
      );

      expect(listingIdentity?.multilineLabel, '@seller_user\nFarm Name');
      expect(auctionIdentity?.multilineLabel, '@auction_user\nAuction Farm');
    },
  );

  test('organic search falls back to @username when farm is missing', () async {
    final repository = SearchRepositoryImpl(
      _FakeOrganicSearchApiServiceMissingFarm(),
    );

    final listings = await repository.searchListings(query: 'koi');

    expect(listings.error, isNull);
    expect(listings.data, hasLength(1));
    final listingIdentity = buildCommerceSellerIdentity(
      username: listings.data!.single.sellerUsername,
      storeName: listings.data!.single.sellerFarmName,
    );
    expect(listingIdentity?.multilineLabel, '@seller_user');
  });
}

class _FakeOrganicSearchApiServiceMissingFarm implements SearchApiService {
  @override
  Future<ListingSearchResponseDto> searchListings({
    required String query,
    int limit = 20,
    int offset = 0,
    String sortBy = 'relevance',
    String sortDir = 'desc',
  }) async {
    return ListingSearchResponseDto(
      query: query,
      listings: [
        ListingSearchResultDto(
          id: 'l1',
          title: 'Showa Koi 30cm',
          description: 'Beautiful showa',
          variety: 'Showa',
          price: 1500000,
          mediaUrls: const ['https://example.com/listing.jpg'],
          sellerId: 'seller-1',
          createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
          sellerUsername: 'seller_user',
          sellerFarmName: null,
          sellerAvatarUrl: 'https://example.com/avatar.jpg',
        ),
      ],
      total: 1,
      limit: limit,
      offset: offset,
    );
  }

  @override
  Future<AuctionSearchResponseDto> searchAuctions({
    required String query,
    int limit = 20,
    int offset = 0,
    String sortBy = 'relevance',
    String sortDir = 'desc',
  }) {
    throw UnimplementedError();
  }

  @override
  Future<ContentSearchResponseDto> searchContents({
    required String query,
    String? contentType,
    int limit = 20,
    int offset = 0,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<UserSearchResponseDto> searchUsers({
    required String query,
    int limit = 20,
    int offset = 0,
  }) {
    throw UnimplementedError();
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
