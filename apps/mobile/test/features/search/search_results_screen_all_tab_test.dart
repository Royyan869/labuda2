// SECTION-BASED ALL — SearchResultsScreen runtime widget tests.
//
// Pumps the REAL SearchResultsScreen + REAL search notifier/usecase/
// repository against a fake SearchApiService, so the whole canonical
// authority chain (screen -> notifier -> usecase -> repository ->
// per-domain API) executes for real. Proves:
// - Gate B: per-domain All preview caps (Users 3, For Sale 5, Auctions 5,
//   Content 5) while the full domain collections are in state.
// - Gate C: empty domains render no section; all-empty -> global empty state.
// - Gate D: "Lihat Semua" moves to the correct domain tab WITHOUT issuing
//   a new search.
// - Gate E: per-type tabs show the full canonical domain collection, never
//   the All preview slice.
// - Gate F: one search query = exactly one call per canonical domain
//   endpoint; tab/All navigation adds zero requests.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/auth_controller.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/auth_state.dart';
import 'package:labuda/features/search/search/data/dto/search_dto.dart';
import 'package:labuda/features/search/search/data/remote/search_api_service.dart';
import 'package:labuda/features/search/search/presentation/providers/providers.dart';
import 'package:labuda/features/search/search/presentation/screens/search_results_screen.dart';

const _guestAuth = AuthStateUnauthenticated();

class _FakeAuthController extends AuthController {
  @override
  AuthState build() => _guestAuth;
}

/// Counts every canonical domain call so tests can prove zero duplicate
/// requests. Content order is intentionally reversed to prove the All view
/// preserves the exact canonical order returned by the backend.
class _CountingSearchApiService implements SearchApiService {
  final int userCount;
  final int forSaleCount;
  final int auctionCount;
  final int contentCount;

  int userCalls = 0;
  int forSaleCalls = 0;
  int auctionCalls = 0;
  int contentCalls = 0;

  _CountingSearchApiService({
    this.userCount = 0,
    this.forSaleCount = 0,
    this.auctionCount = 0,
    this.contentCount = 0,
  });

  int get totalCalls => userCalls + forSaleCalls + auctionCalls + contentCalls;

  @override
  Future<UserSearchResponseDto> searchUsers({
    required String query,
    int limit = 20,
    int offset = 0,
  }) async {
    userCalls++;
    return UserSearchResponseDto(
      query: query,
      users: [
        for (var i = 1; i <= userCount; i++)
          UserSearchResultDto(id: 'u-$i', username: 'u-$i'),
      ],
      total: userCount,
      limit: limit,
      offset: offset,
    );
  }

  @override
  Future<ForSaleSearchResponseDto> searchForSale({
    required String query,
    String? cursor,
    int limit = 20,
    String sortBy = 'relevance',
    String sortDir = 'desc',
  }) async {
    forSaleCalls++;
    return ForSaleSearchResponseDto(
      forSales: [
        for (var i = 1; i <= forSaleCount; i++)
          ForSaleSearchResultDto(
            id: 'fs-$i',
            title: 'fs-$i',
            description: '',
            variety: 'Showa',
            mediaUrls: const [],
            sellerId: 'seller-$i',
            createdAt: DateTime.utc(2026, 1, i),
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
    auctionCalls++;
    return AuctionSearchResponseDto(
      query: query,
      auctions: [
        for (var i = 1; i <= auctionCount; i++)
          AuctionSearchResultDto(
            id: 'a-$i',
            sellerId: 'seller-$i',
            productId: 'p-$i',
            title: 'a-$i',
            description: '',
            startPrice: 100000 * i,
            startAt: DateTime.utc(2026, 1, 1),
            endAt: DateTime.utc(2026, 1, 10),
            status: 'scheduled',
            bidCount: 0,
            createdAt: DateTime.utc(2026, 1, i),
          ),
      ],
      total: auctionCount,
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
    contentCalls++;
    // Content is returned in reverse index order (c-N..c-1). The All view
    // must keep this exact canonical order inside the Content section.
    final reversed = [for (var i = contentCount; i >= 1; i--) i];
    return ContentSearchResponseDto(
      query: query,
      contents: [
        for (final i in reversed)
          ContentSearchResultDto(
            id: 'c-$i',
            authorId: 'author-$i',
            authorUsername: 'author-$i',
            type: 'content',
            caption: 'c-$i',
            mediaUrls: const [],
            createdAt: DateTime.utc(2026, 1, i),
            lifecycle: 'active',
            authorLifecycle: 'active',
          ),
      ],
      total: contentCount,
      limit: limit,
      offset: offset,
    );
  }

  @override
  Future<List<SearchHistoryDto>> getSearchHistory({int limit = 20}) async =>
      const [];

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

Future<void> _pumpSearchResults(
  WidgetTester tester,
  _CountingSearchApiService api,
) async {
  await tester.pumpWidget(
    ProviderScope(
      // Unique key so a re-pump inside one test creates a fresh container
      // and applies the new overrides.
      key: UniqueKey(),
      overrides: [
        searchApiServiceProvider.overrideWithValue(api),
        authControllerProvider.overrideWith(() => _FakeAuthController()),
      ],
      child: const MaterialApp(
        home: Scaffold(body: SearchResultsScreen(query: 'koi')),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

Finder _seeAll(String domainTitle) => find.byKey(ValueKey('seeAll-$domainTitle'));

Finder _tab(String label) => find.widgetWithText(Tab, label);

void main() {
  // User rows render the username as both title and subtitle, so presence
  // means >= 1 Text widget; absence means findsNothing.
  Future<void> useTallSurface(WidgetTester tester) async {
    tester.view.physicalSize = const Size(900, 3200);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
  }

  TabController tabControllerOf(WidgetTester tester) =>
      tester.widget<TabBar>(find.byType(TabBar)).controller!;

  testWidgets('All overview: per-domain caps, canonical order, one fetch per domain', (
    tester,
  ) async {
    await useTallSurface(tester);
    final api = _CountingSearchApiService(
      userCount: 5,
      forSaleCount: 7,
      auctionCount: 6,
      contentCount: 8,
    );
    await _pumpSearchResults(tester, api);

    // Every domain has results -> every section header + Lihat Semua exists.
    for (final title in ['User', 'For Sale', 'Auctions', 'Content']) {
      expect(_seeAll(title), findsOneWidget, reason: '$title section missing');
    }

    // Users preview capped at 3 of 5.
    for (var i = 1; i <= 3; i++) {
      expect(find.text('@u-$i'), findsWidgets);
    }
    expect(find.text('@u-4'), findsNothing);
    expect(find.text('@u-5'), findsNothing);

    // For Sale preview capped at 5 of 7.
    for (var i = 1; i <= 5; i++) {
      expect(find.text('fs-$i'), findsOneWidget);
    }
    expect(find.text('fs-6'), findsNothing);
    expect(find.text('fs-7'), findsNothing);

    // Auctions preview capped at 5 of 6.
    for (var i = 1; i <= 5; i++) {
      expect(find.text('a-$i'), findsOneWidget);
    }
    expect(find.text('a-6'), findsNothing);

    // Content preview capped at 5 of 8, in exact canonical order.
    // Fixture order is c-8..c-1 -> the section shows c-8..c-4.
    for (var i = 8; i >= 4; i--) {
      expect(find.text('c-$i'), findsOneWidget);
    }
    expect(find.text('c-3'), findsNothing);
    expect(find.text('c-2'), findsNothing);
    expect(find.text('c-1'), findsNothing);

    // Ordering proof: the section renders c-8 above c-7 above c-6 above c-5.
    final dyC8 = tester.getTopLeft(find.text('c-8')).dy;
    final dyC7 = tester.getTopLeft(find.text('c-7')).dy;
    final dyC6 = tester.getTopLeft(find.text('c-6')).dy;
    final dyC5 = tester.getTopLeft(find.text('c-5')).dy;
    expect(dyC7, greaterThan(dyC8));
    expect(dyC6, greaterThan(dyC7));
    expect(dyC5, greaterThan(dyC6));

    // No SearchState.error.
    expect(find.textContaining('Failed'), findsNothing);

    // Gate F: exactly one call per canonical domain endpoint.
    expect(api.userCalls, 1);
    expect(api.forSaleCalls, 1);
    expect(api.auctionCalls, 1);
    expect(api.contentCalls, 1);
    expect(api.totalCalls, 4);
  });

  testWidgets('empty domain renders NO section; all empty renders global empty state', (
    tester,
  ) async {
    await useTallSurface(tester);
    final api = _CountingSearchApiService(
      userCount: 4,
      forSaleCount: 4,
      auctionCount: 0, // empty domain
      contentCount: 4,
    );
    await _pumpSearchResults(tester, api);

    // No Auctions section: 'Auctions' appears only as the tab label and no
    // auction row is rendered.
    expect(_seeAll('Auctions'), findsNothing);
    expect(find.text('Auctions'), findsOneWidget);
    expect(find.text('a-1'), findsNothing);
    // Other domains still render.
    expect(_seeAll('User'), findsOneWidget);
    expect(_seeAll('For Sale'), findsOneWidget);
    expect(_seeAll('Content'), findsOneWidget);

    // All-empty -> canonical global empty state (no per-domain placeholders).
    final emptyApi = _CountingSearchApiService();
    await _pumpSearchResults(tester, emptyApi);
    expect(find.text('No results found'), findsOneWidget);
    expect(find.text('No Users'), findsNothing);
    expect(find.text('No For Sale'), findsNothing);
    expect(find.text('No Auctions'), findsNothing);
    expect(find.text('No Content'), findsNothing);
    expect(_seeAll('User'), findsNothing);
  });

  group('Lihat Semua — each section jumps to its domain tab, zero new requests', () {
    Future<_CountingSearchApiService> pumpMultiDomain(
      WidgetTester tester,
    ) async {
      await useTallSurface(tester);
      final api = _CountingSearchApiService(
        userCount: 5,
        forSaleCount: 7,
        auctionCount: 6,
        contentCount: 6,
      );
      await _pumpSearchResults(tester, api);
      expect(tabControllerOf(tester).index, 0);
      return api;
    }

    testWidgets('User section -> User tab (index 3) with full user domain', (tester) async {
      final api = await pumpMultiDomain(tester);
      await tester.tap(_seeAll('User'));
      await tester.pumpAndSettle();

      expect(tabControllerOf(tester).index, 3);
      // Full canonical user domain (5), not the All preview of 3.
      expect(find.text('@u-4'), findsWidgets);
      expect(find.text('@u-5'), findsWidgets);
      expect(api.totalCalls, 4);
    });

    testWidgets('For Sale section -> For Sale tab (index 1) with full for-sale domain', (
      tester,
    ) async {
      final api = await pumpMultiDomain(tester);
      await tester.tap(_seeAll('For Sale'));
      await tester.pumpAndSettle();

      expect(tabControllerOf(tester).index, 1);
      expect(find.text('fs-6'), findsOneWidget);
      expect(find.text('fs-7'), findsOneWidget);
      expect(api.totalCalls, 4);
    });

    testWidgets('Auctions section -> Auctions tab (index 2) with full auction domain', (
      tester,
    ) async {
      final api = await pumpMultiDomain(tester);
      await tester.tap(_seeAll('Auctions'));
      await tester.pumpAndSettle();

      expect(tabControllerOf(tester).index, 2);
      expect(find.text('a-6'), findsOneWidget);
      expect(api.totalCalls, 4);
    });

    testWidgets('Content section -> Content tab (index 4) with full content domain', (
      tester,
    ) async {
      final api = await pumpMultiDomain(tester);
      await tester.tap(_seeAll('Content'));
      await tester.pumpAndSettle();

      expect(tabControllerOf(tester).index, 4);
      // Full canonical content domain in backend order: c-6..c-1 all shown.
      expect(find.text('c-6'), findsOneWidget);
      expect(find.text('c-1'), findsOneWidget);
      expect(api.totalCalls, 4);
    });
  });

  testWidgets('All -> per-type tab -> back to All keeps sections and adds no requests', (
    tester,
  ) async {
    await useTallSurface(tester);
    final api = _CountingSearchApiService(
      userCount: 6,
      forSaleCount: 6,
      auctionCount: 6,
      contentCount: 6,
    );
    await _pumpSearchResults(tester, api);

    // All caps users at 3 of 6.
    expect(find.text('@u-4'), findsNothing);

    // Manual tab switch to User: full canonical domain (6), no All cap.
    await tester.tap(_tab('User'));
    await tester.pumpAndSettle();
    expect(tabControllerOf(tester).index, 3);
    expect(find.text('@u-6'), findsWidgets);
    expect(find.text('fs-1'), findsNothing);

    // Back to All: sections reappear (no stale per-type projection).
    await tester.tap(_tab('All'));
    await tester.pumpAndSettle();
    expect(tabControllerOf(tester).index, 0);
    expect(_seeAll('User'), findsOneWidget);
    expect(_seeAll('Content'), findsOneWidget);
    expect(find.text('@u-4'), findsNothing);

    // Auctions tab still shows the full domain after returning to All.
    await tester.tap(_tab('Auctions'));
    await tester.pumpAndSettle();
    expect(tabControllerOf(tester).index, 2);
    expect(find.text('a-6'), findsOneWidget);

    // Zero extra requests across all tab hops.
    expect(api.userCalls, 1);
    expect(api.forSaleCalls, 1);
    expect(api.auctionCalls, 1);
    expect(api.contentCalls, 1);
  });
}
