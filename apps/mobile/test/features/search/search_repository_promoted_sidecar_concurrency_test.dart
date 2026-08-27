import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/features/search/search/data/dto/search_dto.dart';
import 'package:labuda/features/search/search/data/remote/search_api_service.dart';
import 'package:labuda/features/search/search/data/search_repository_impl.dart';
import 'package:labuda/features/search/search/domain/entities/search_filters.dart';

class _OverlappingPromotedSearchApiService implements SearchApiService {
  final Completer<void> fixedPriceSaleASeen = Completer<void>();
  final Completer<void> fixedPriceSaleBSeen = Completer<void>();
  final Completer<void> releaseAuctionA = Completer<void>();
  final Completer<void> releaseAuctionB = Completer<void>();

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
  Future<ContentSearchResponseDto> searchContents({
    required String query,
    String? contentType,
    int limit = 20,
    int offset = 0,
  }) async {
    return ContentSearchResponseDto(
      query: query,
      contents: const [],
      total: 0,
      limit: limit,
      offset: offset,
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
    final promotedId = query == 'A'
        ? 'pi-A-fixed-price-sale'
        : 'pi-B-fixed-price-sale';
    final seen = query == 'A' ? fixedPriceSaleASeen : fixedPriceSaleBSeen;
    if (!seen.isCompleted) {
      seen.complete();
    }

    return ListingSearchResponseDto(
      query: query,
      listings: const [],
      total: 0,
      limit: limit,
      offset: offset,
      promotedItems: [
        PromotedSearchItemDto(
          type: 'promoted_fixed_price_sale',
          promotionInstanceId: promotedId,
          targetType: 'fixed_price_sale',
          injectAt: 0,
          title: 'Promoted fixed-price-sale $query',
          imageUrl: 'https://example.com/$query-fixed-price-sale.jpg',
          fixedPriceSaleId: 'fixed-price-sale-$query',
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
    if (query == 'A') {
      await releaseAuctionA.future;
    } else {
      await releaseAuctionB.future;
    }

    final promotedId = query == 'A' ? 'pi-A-auction' : 'pi-B-auction';
    return AuctionSearchResponseDto(
      query: query,
      auctions: const [],
      total: 0,
      limit: limit,
      offset: offset,
      promotedItems: [
        PromotedSearchItemDto(
          type: 'promoted_auction',
          promotionInstanceId: promotedId,
          targetType: 'auction',
          injectAt: 0,
          title: 'Promoted auction $query',
          imageUrl: 'https://example.com/$query-auction.jpg',
          auctionId: 'auction-$query',
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
  test(
    'overlapping searchAll calls keep promoted sidecar request-local',
    () async {
      final api = _OverlappingPromotedSearchApiService();
      final repository = SearchRepositoryImpl(api);

      final searchAFuture = repository.searchAll(
        query: 'A',
        filters: const SearchFilters(),
      );
      await api.fixedPriceSaleASeen.future;

      final searchBFuture = repository.searchAll(
        query: 'B',
        filters: const SearchFilters(),
      );
      await api.fixedPriceSaleBSeen.future;

      api.releaseAuctionB.complete();
      final resultB = await searchBFuture;

      api.releaseAuctionA.complete();
      final resultA = await searchAFuture;

      expect(resultA.error, isNull);
      expect(resultB.error, isNull);

      final fixedPriceSaleA = resultA.data!.listings.singleWhere(
        (item) => item.isPromoted,
      );
      final auctionA = resultA.data!.auctions.singleWhere(
        (item) => item.isPromoted,
      );
      final fixedPriceSaleB = resultB.data!.listings.singleWhere(
        (item) => item.isPromoted,
      );
      final auctionB = resultB.data!.auctions.singleWhere(
        (item) => item.isPromoted,
      );

      expect(fixedPriceSaleA.promotionInstanceId, 'pi-A-fixed-price-sale');
      expect(auctionA.promotionInstanceId, 'pi-A-auction');
      expect(fixedPriceSaleB.promotionInstanceId, 'pi-B-fixed-price-sale');
      expect(auctionB.promotionInstanceId, 'pi-B-auction');
    },
  );
}
