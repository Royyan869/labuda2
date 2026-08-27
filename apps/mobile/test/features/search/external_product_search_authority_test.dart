import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/features/search/search/data/dto/search_dto.dart';
import 'package:labuda/features/search/search/data/remote/search_api_service.dart';
import 'package:labuda/features/search/search/data/search_repository_impl.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';
import 'package:labuda/features/search/search/presentation/screens/search_results_screen.dart';

class _ExternalProductSearchApiService implements SearchApiService {
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
          id: 'listing-1',
          title: 'Showa Koi 30cm',
          description: 'Organic listing',
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
      promotedItems: const [
        PromotedSearchItemDto(
          type: 'promoted_external',
          promotionInstanceId: 'pi-external',
          targetType: 'external_product',
          injectAt: 0,
          title: 'Promoted external product',
          imageUrl: 'https://example.com/external.jpg',
          externalUrl: 'https://example.com/product',
          externalMediaUrl: 'https://example.com/external-media.jpg',
        ),
      ],
    );
  }

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
    'external_product maps into listing surface as SearchResultType.externalProduct',
    () async {
      final repository = SearchRepositoryImpl(
        _ExternalProductSearchApiService(),
      );

      final result = await repository.searchAll(query: 'koi');

      expect(result.error, isNull);
      expect(result.data, isNotNull);

      final listingSurface = result.data!.listings;
      expect(listingSurface, hasLength(2));

      final external = listingSurface.singleWhere(
        (item) => item.promotionInstanceId == 'pi-external',
      );

      expect(external.type, SearchResultType.externalProduct);
      expect(external.isPromoted, isTrue);
      expect(external.title, 'Promoted external product');
      expect(external.imageUrl, 'https://example.com/external.jpg');
      expect(external.metadata['externalUrl'], 'https://example.com/product');
      expect(
        result.data!.getByType(SearchResultType.listing),
        contains(external),
      );
    },
  );

  testWidgets(
    'external product tap shows interstitial dialog before launching URL',
    (tester) async {
      const channel = MethodChannel('plugins.flutter.io/url_launcher');
      final calls = <MethodCall>[];

      final binding = TestWidgetsFlutterBinding.ensureInitialized();
      binding.defaultBinaryMessenger.setMockMethodCallHandler(channel, (
        MethodCall call,
      ) async {
        calls.add(call);
        return true;
      });

      final result = SearchResult(
        id: 'pi-external',
        type: SearchResultType.externalProduct,
        title: 'Promoted external product',
        imageUrl: 'https://example.com/external.jpg',
        metadata: const {'externalUrl': 'https://example.com/product'},
        createdAt: DateTime.parse('2026-01-01T00:00:00.000Z'),
        isPromoted: true,
        promotionInstanceId: 'pi-external',
      );

      late BuildContext context;
      late WidgetRef ref;

      await tester.pumpWidget(
        ProviderScope(
          child: MaterialApp(
            home: Consumer(
              builder: (contextValue, widgetRef, _) {
                context = contextValue;
                ref = widgetRef;
                return const SizedBox.shrink();
              },
            ),
          ),
        ),
      );

      // Do NOT await — the interstitial dialog blocks until user confirms.
      handleSearchResultTap(context, ref, result);
      await tester.pumpAndSettle();

      // Interstitial is now showing — URL not yet launched.
      expect(find.text('Buka tautan eksternal?'), findsOneWidget);
      expect(calls, isEmpty);

      // User confirms.
      await tester.tap(find.text('Buka'));
      await tester.pumpAndSettle();

      expect(calls, isNotEmpty);
      expect(calls.any((call) => call.method.contains('launch')), isTrue);
      expect(
        calls.any(
          (c) => c.arguments.toString().contains('https://example.com/product'),
        ),
        isTrue,
      );
    },
  );
}
