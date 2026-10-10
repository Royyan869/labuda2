// REAL-ROUTER REGRESSION — PASS 2.
//
// Pass 1 found a production defect: the comment composer called
// `context.pushNamed(RoutePaths.createForSale)` — passing a PATH where a route
// NAME is required. The production route is registered under
// `RouteNames.createForSale` (see for_sale_module.dart), so the old call could
// never resolve. The former test masked this by naming its fake route with the
// path string.
//
// These tests use the REAL `ForSaleModule` registration (real path + real name)
// so they fail if the composer and the router ever disagree again.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/src/router/modules/for_sale_module.dart';
import 'package:hishumi/core/src/router/route_paths.dart';
import 'package:hishumi/domains/commerce/catalog/auction/presentation/providers/seller_auctions_pager.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/presentation/providers/seller_fps_pager.dart';
import 'package:hishumi/domains/social/comment/presentation/widgets/comment_input_with_commerce_reference.dart';

// ── Fake pagers (the picker renders immediately, no API calls) ─────────────

class _FakeSellerFPSPagerController extends SellerFPSPagerController {
  @override
  SellerFPSPagerState build() => SellerFPSPagerState(
        items: const [],
        hasMore: false,
        isInitialLoading: false,
        isLoadingMore: false,
        initialError: null,
        loadMoreError: null,
        ownerId: 'test-seller',
        pageSize: 20,
      );
}

class _FakeSellerAuctionsPagerController extends SellerAuctionsPagerController {
  @override
  SellerAuctionsPagerState build() => const SellerAuctionsPagerState(
        activeFilter: null,
        auctions: [],
        pageSize: 20,
        hasMore: false,
        isInitialLoading: false,
        isLoadMoreLoading: false,
        isRefreshing: false,
        initialError: null,
        loadMoreError: null,
        refreshError: null,
        ownerId: 'test-seller',
      );
}

void main() {
  group('Create For Sale — real router registration', () {
    test('ForSaleModule registers the route under RouteNames.createForSale', () {
      final route = ForSaleModule().routes.singleWhere(
        (r) => r.path == RoutePaths.createForSale,
      );

      expect(route.name, RouteNames.createForSale);
      expect(
        route.name,
        isNot(RoutePaths.createForSale),
        reason: 'the route must NOT be named by its path',
      );
    });

    test('the real router resolves pushNamed(RouteNames.createForSale)', () {
      final router = GoRouter(routes: ForSaleModule().routes);

      // The canonical call now resolves to the canonical path.
      expect(
        router.namedLocation(RouteNames.createForSale),
        RoutePaths.createForSale,
      );

      // The pre-fix call (path used as a name) is proven invalid — GoRouter
      // cannot resolve a path as a route name.
      expect(
        () => router.namedLocation(RoutePaths.createForSale),
        throwsA(anything),
        reason: 'a path is not a route name — the old call could not resolve',
      );
    });

    testWidgets(
      'composer "Buat Produk Baru" reaches the REAL Create For Sale route',
      (tester) async {
        // The route's path + name are taken from the real module registration;
        // only the destination widget is a test double (the production screen
        // has unrelated seller/shipping provider dependencies). Reaching this
        // widget proves the composer's pushNamed resolved the real named route.
        final realRoute = ForSaleModule().routes.singleWhere(
          (r) => r.name == RouteNames.createForSale,
        );

        final router = GoRouter(
          initialLocation: '/',
          routes: [
            GoRoute(
              path: '/',
              builder: (context, state) => Scaffold(
                body: CommentInputWithCommerceReference(
                  key: const ValueKey('composer-under-test'),
                  onSubmit: (_, _) async => true,
                  isSeller: true,
                  sellerId: 'test-seller',
                ),
              ),
            ),
            GoRoute(
              path: realRoute.path,
              name: realRoute.name,
              pageBuilder: (context, state) => MaterialPage(
                key: state.pageKey,
                child: Scaffold(
                  appBar: AppBar(title: const Text('Create ForSale')),
                  body: const SizedBox.shrink(),
                ),
              ),
            ),
          ],
        );

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              sellerFPSPagerProvider.overrideWith(
                _FakeSellerFPSPagerController.new,
              ),
              sellerAuctionsPagerProvider.overrideWith(
                _FakeSellerAuctionsPagerController.new,
              ),
            ],
            child: MaterialApp.router(routerConfig: router),
          ),
        );
        await tester.pump();

        // Open the attach sheet → commerce picker.
        await tester.tap(find.byIcon(Icons.add_circle));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 400));
        await tester.tap(find.text('Lampirkan Produk'));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 400));

        expect(find.text('Pilih Produk'), findsOneWidget);

        // "Buat Produk Baru" → pushNamed(RouteNames.createForSale).
        await tester.tap(find.text('Buat Produk Baru'));
        await tester.pump();
        await tester.pump(const Duration(seconds: 2));

        expect(
          find.text('Create ForSale'),
          findsOneWidget,
          reason:
              'composer navigation must reach the real Create For Sale route',
        );
      },
    );
  });
}
