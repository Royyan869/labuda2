import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/features/search/search/data/dto/search_dto.dart';
import 'package:labuda/features/search/search/data/search_repository_impl.dart';
import 'package:labuda/features/search/search/domain/entities/search_filters.dart';
import 'package:labuda/features/search/search/data/remote/search_api_service.dart';

class _FakeSearchApiService implements SearchApiService {
  @override
  Future<UserSearchResponseDto> searchUsers({
    required String query,
    int limit = 20,
    int offset = 0,
  }) async {
    return const UserSearchResponseDto(
      query: '',
      users: [],
      total: 0,
      limit: 20,
      offset: 0,
    );
  }

  @override
  Future<ContentSearchResponseDto> searchContents({
    required String query,
    String? contentType,
    int limit = 20,
    int offset = 0,
  }) async {
    return const ContentSearchResponseDto(
      query: '',
      contents: [],
      total: 0,
      limit: 20,
      offset: 0,
    );
  }

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
      listings: const [],
      total: 0,
      limit: limit,
      offset: offset,
      promotedItems: const [
        PromotedSearchItemDto(
          type: 'promoted_for_sale',
          promotionInstanceId: 'pi-for-sale',
          targetType: 'for_sale',
          injectAt: 0,
          title: 'Promoted for-sale',
          imageUrl: 'https://example.com/for-sale.jpg',
          sellerUsername: 'seller_user',
          forSaleId: 'for-sale-1',
          pricePerUnit: 150000,
        ),
      ],
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
      promotedItems: const [
        PromotedSearchItemDto(
          type: 'promoted_auction',
          promotionInstanceId: 'pi-auction',
          targetType: 'auction',
          injectAt: 0,
          title: 'Promoted auction',
          imageUrl: 'https://example.com/auction.jpg',
          sellerUsername: 'seller_user',
          auctionId: 'auction-1',
          startPrice: 250000,
        ),
      ],
    );
  }

  @override
  Future<List<SearchHistoryDto>> getSearchHistory({int limit = 20}) async {
    return const [];
  }

  @override
  Future<void> clearSearchHistory() async {}

  @override
  Future<void> saveSearchHistory({
    required String query,
    String? searchType,
    int? resultsCount,
  }) async {}

  @override
  Future<void> deleteSearchHistoryItem(String historyId) async {}
}

void main() {
  test('promoted search uses split seller identity', () async {
    final repository = SearchRepositoryImpl(_FakeSearchApiService());

    final result = await repository.searchAll(
      query: 'koi',
      filters: const SearchFilters(),
      limit: 8,
    );

    expect(result.error, isNull);

    final data = result.data!;
    final promotedForSale = data.listings.singleWhere(
      (item) => item.promotionInstanceId == 'pi-for-sale',
    );
    final promotedAuction = data.auctions.singleWhere(
      (item) => item.promotionInstanceId == 'pi-auction',
    );

    expect(promotedForSale.subtitle, '@seller_user');
    expect(promotedAuction.subtitle, '@seller_user');
    expect(data.allResults.where((item) => item.isPromoted).length, 2);
  });
}
