import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/seller_auctions_pager.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'package:labuda/domains/commerce/catalog/for_sale/presentation/providers/seller_fps_pager.dart';
import 'package:labuda/domains/social/comment/domain/entities/comment.dart';
import 'package:labuda/domains/social/comment/presentation/widgets/comment_input_with_commerce_reference.dart';
// ── Fake controllers ─────────────────────────────────────────────────────
// TEST-ONLY: override the pager providers so the CommerceResourcePicker
// renders immediately without making API calls.

class FakeSellerFPSPagerController extends SellerFPSPagerController {
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

class FakeSellerAuctionsPagerController extends SellerAuctionsPagerController {
  @override
  SellerAuctionsPagerState build() => const SellerAuctionsPagerState(
        activeFilter: SellerAuctionFilter.all,
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

// ── Test ForSale factory ─────────────────────────────────────────────────

ForSale _testForSale(String id) => ForSale(
      forSaleId: id,
      title: 'Test ForSale $id',
      description: 'A test listing',
      price: 50000,
      stock: 1,
      sellerId: 'seller-1',
      status: ForSaleStatus.active,
      createdAt: DateTime(2026, 8, 1),
      updatedAt: DateTime(2026, 8, 1),
    );

// ── GoRouter factory ─────────────────────────────────────────────────────
// mode: 'cancel' → route pops without result
//        'success' → route pops with a ForSale

GoRouter _testRouter({required String mode}) {
  return GoRouter(
    initialLocation: '/',
    routes: [
      GoRoute(
        path: RoutePaths.createForSale,
        name: RoutePaths.createForSale, // Match RoutePaths.createForSale used in pushNamed
        pageBuilder: (context, state) => MaterialPage(
          key: state.pageKey,
          child: Scaffold(
            appBar: AppBar(title: const Text('Create ForSale')),
            body: Center(
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  ElevatedButton(
                    key: const ValueKey('cancel-create-listing'),
                    onPressed: () => GoRouter.of(context).pop(),
                    child: const Text('Cancel'),
                  ),
                  ElevatedButton(
                    key: const ValueKey('success-create-listing'),
                    onPressed: () =>
                        GoRouter.of(context).pop(_testForSale('new-fps-id')),
                    child: const Text('Create'),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
      GoRoute(
        path: '/',
        pageBuilder: (context, state) => MaterialPage(
          key: state.pageKey,
          child: Builder(
            builder: (context) {
              // Retrieve test callbacks from the widget tree.
              final testData = _TestData.of(context);
              return Scaffold(
                body: CommentInputWithCommerceReference(
                  key: const ValueKey('composer-under-test'),
                  onSubmit: testData.onSubmit,
                  initialResource: testData.initialResource,
                  isSeller: true,
                  sellerId: 'test-seller',
                ),
              );
            },
          ),
        ),
      ),
    ],
  );
}

// ── Test data carrier ────────────────────────────────────────────────────

class _TestData extends InheritedWidget {
  final Future<bool> Function(String body, ResourceIdentity? resource) onSubmit;
  final ResourceIdentity? initialResource;

  const _TestData({
    super.key,
    required this.onSubmit,
    required this.initialResource,
    required super.child,
  });

  static _TestData of(BuildContext context) {
    final result = context.dependOnInheritedWidgetOfExactType<_TestData>();
    assert(result != null, 'No _TestData found in context');
    return result!;
  }

  @override
  bool updateShouldNotify(_TestData oldWidget) =>
      onSubmit != oldWidget.onSubmit || initialResource != oldWidget.initialResource;
}

// ── Test wrapper ─────────────────────────────────────────────────────────

Widget _wrapTest({
  required String mode,
  required Future<bool> Function(String body, ResourceIdentity? resource) onSubmit,
  ResourceIdentity? initialResource,
}) {
  return ProviderScope(
    overrides: [
      sellerFPSPagerProvider.overrideWith(() => FakeSellerFPSPagerController()),
      sellerAuctionsPagerProvider.overrideWith(
        () => FakeSellerAuctionsPagerController(),
      ),
    ],
    child: _TestData(
      onSubmit: onSubmit,
      initialResource: initialResource,
      child: MaterialApp.router(
        routerConfig: _testRouter(mode: mode),
        title: 'Test',
      ),
    ),
  );
}

// ── Tests ────────────────────────────────────────────────────────────────

void main() {
  group('Create ForSale GoRouter navigation', () {
    // ── CANCEL ──────────────────────────────────────────────────────────

    testWidgets(
      'CANCEL: pop without result preserves draft and selection, no send',
      (tester) async {
        int submitCount = 0;
        final initialResource = ResourceIdentity(
          resourceType: ResourceType.forSale,
          resourceId: 'pre-existing-fps',
        );

        await tester.pumpWidget(_wrapTest(
          mode: 'cancel',
          onSubmit: (_, _) async {
            submitCount++;
            return true;
          },
          initialResource: initialResource,
        ));
        await tester.pump();

        // Draft text
        await tester.enterText(find.byType(TextField), 'draft before create');
        await tester.pump();

        // Tap attach button to open CommerceResourcePicker
        await tester.tap(find.byIcon(Icons.add_circle_outline));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 400));

        // Picker should be visible with "Pilih Produk" title
        expect(find.text('Pilih Produk'), findsOneWidget);

        // Tap "Buat Produk Baru" in the FPS tab
        await tester.tap(find.text('Buat Produk Baru'));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 400));

        // Create ForSale route should be visible
        expect(find.text('Create ForSale'), findsOneWidget);

        // Tap Cancel → pop without result
        await tester.tap(find.byKey(const ValueKey('cancel-create-listing')));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 400));

        // Back at composer — draft preserved
        expect(find.text('draft before create'), findsOneWidget);

        // Submit count still 0
        expect(submitCount, 0);
      },
    );

    // ── SUCCESS ─────────────────────────────────────────────────────────

    testWidgets(
      'SUCCESS: route returns Listing, listing becomes selected, tap Send fires once',
      (tester) async {
        int submitCount = 0;
        String? sentBody;
        ResourceIdentity? sentResource;

        await tester.pumpWidget(_wrapTest(
          mode: 'success',
          onSubmit: (body, resource) async {
            submitCount++;
            sentBody = body;
            sentResource = resource;
            return true;
          },
        ));
        await tester.pump();

        // Enter draft text
        await tester.enterText(find.byType(TextField), 'check out this listing');
        await tester.pump();

        // Tap attach button to open CommerceResourcePicker
        await tester.tap(find.byIcon(Icons.add_circle_outline));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 400));

        // Picker should be visible
        expect(find.text('Pilih Produk'), findsOneWidget);

        // Tap "Buat Produk Baru" — this closes the picker and pushes the Create
        // ForSale route. Pump multiple frames to let the picker's modal barrier
        // animation fully complete.
        await tester.tap(find.text('Buat Produk Baru'));
        await tester.pump();
        await tester.pump(const Duration(seconds: 2));

        // Create ForSale route should be visible
        expect(find.text('Create ForSale'), findsOneWidget);

        // Tap Create → pop with Listing
        await tester.tap(find.byKey(const ValueKey('success-create-listing')));
        await tester.pump();
        await tester.pump(const Duration(seconds: 2));

        // Submit count remains 0 after navigation return
        expect(submitCount, 0);

        // The selected resource preview should be visible — this proves the
        // Listing result was captured by the production code (no automatic send).
        expect(find.text('Test ForSale new-fps-id'), findsOneWidget,
            reason: 'Listing result must be captured as selected resource');

        // Tap Send
        await tester.tap(find.byKey(const ValueKey('comment-send-button')));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 100));

        // Submit count becomes exactly 1
        expect(submitCount, 1);
        expect(sentBody, 'check out this listing');
        expect(sentResource, isNotNull);
        expect(sentResource!.resourceType, ResourceType.forSale);
        expect(sentResource!.resourceId, 'new-fps-id');
      },
    );
  });
}
