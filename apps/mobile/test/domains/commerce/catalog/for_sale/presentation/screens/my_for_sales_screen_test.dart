import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

import 'package:hishumi/core/common/result.dart';
import 'package:hishumi/core/providers/core_providers.dart';
import 'package:hishumi/core/src/auth/app_role.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/domain/domain.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/presentation/providers/for_sale_providers.dart';
import 'package:hishumi/domains/commerce/catalog/for_sale/presentation/screens/my_for_sales_screen.dart';
import 'package:hishumi/domains/user/identity/authentication/authentication.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';
import 'package:hishumi/shared/services/logger_service.dart';
import 'package:hishumi/shared/widgets/empty_state.dart';
import 'package:hishumi/shared/widgets/loading_indicator.dart';
import 'package:hishumi/shared/widgets/page_error_state.dart';

const _uid = 'seller-1';

class _FakeAuthController extends AuthController {
  @override
  AuthState build() =>
      AuthState.authenticated(_user(_uid), emailVerified: true);
}

AuthUser _user(String id) => AuthUser(
  id: id,
  createdAt: DateTime.utc(2026, 1, 1),
  updatedAt: DateTime.utc(2026, 1, 1),
  email: '$id@example.com',
  username: id,
  isEmailVerified: true,
  accountStatus: AccountStatus.active,
  hasSellerProfile: true,
  sellerSubscriptionStatus: 'active',
  hasMarketAuthority: true,
  roles: const [UserRole.user],
  provider: AuthProvider.email,
  lifecycle: ContentLifecycle.active,
);

ForSale _forSale(String id, ForSaleStatus status) => ForSale(
  forSaleId: id,
  title: 'Koi $id',
  description: 'desc',
  price: 100000,
  stock: 1,
  sellerId: _uid,
  status: status,
  createdAt: DateTime.utc(2026, 1, 1),
  updatedAt: DateTime.utc(2026, 1, 1),
);

List<ForSale> _allStates() => [
  _forSale('active-1', ForSaleStatus.active),
  _forSale('sold-1', ForSaleStatus.sold),
  _forSale('withdrawn-1', ForSaleStatus.withdrawn),
];

class _ObservedFetch {
  const _ObservedFetch({
    required this.sellerId,
    required this.page,
    required this.pageSize,
    required this.includeWithdrawn,
  });

  final String sellerId;
  final int page;
  final int pageSize;
  final bool includeWithdrawn;
}

/// Scripted repository: the screen runs the REAL controller + the REAL
/// canonical [sellerForSalesProvider], so every fetch (initial, refresh,
/// retry, post-delete invalidation) is observable with its exact params.
class _ScriptedForSaleRepository implements ForSaleRepository {
  _ScriptedForSaleRepository({required this.onFetchSeller});

  Future<Result<List<ForSale>>> Function(int call) onFetchSeller;
  Future<Result<void>> Function(String forSaleId)? onDelete;

  int fetchCalls = 0;
  final List<_ObservedFetch> observedFetches = [];
  final List<String> deletedIds = [];

  @override
  Future<Result<List<ForSale>>> getSellerForSales(
    String sellerId, {
    int page = 1,
    int pageSize = 20,
    bool includeWithdrawn = false,
  }) {
    fetchCalls++;
    observedFetches.add(
      _ObservedFetch(
        sellerId: sellerId,
        page: page,
        pageSize: pageSize,
        includeWithdrawn: includeWithdrawn,
      ),
    );
    return onFetchSeller(fetchCalls);
  }

  @override
  Future<Result<void>> deleteForSale(String forSaleId) {
    deletedIds.add(forSaleId);
    final handler = onDelete;
    if (handler != null) return handler(forSaleId);
    return Future.value(Result.success(null));
  }

  @override
  Future<Result<List<ForSale>>> getForSales(GetForSalesParams params) =>
      throw UnimplementedError();

  @override
  Future<Result<ForSale?>> getForSaleById(String forSaleId) =>
      throw UnimplementedError();

  @override
  Future<Result<ForSale>> createForSale(CreateForSaleRequest request) =>
      throw UnimplementedError();

  @override
  Future<Result<ForSale>> updateForSale(
    String forSaleId,
    UpdateForSaleRequest request,
  ) => throw UnimplementedError();
}

void main() {
  Future<void> pumpScreen(
    WidgetTester tester,
    _ScriptedForSaleRepository repo, {
    bool settle = true,
  }) async {
    await tester.pumpWidget(
      ProviderScope(
        // Retry is disabled so a failed provider does not schedule timers.
        retry: (retryCount, error) => null,
        overrides: [
          authControllerProvider.overrideWith(_FakeAuthController.new),
          loggerServiceProvider.overrideWithValue(LoggerService.instance),
          forSaleRepositoryProvider.overrideWithValue(repo),
        ],
        child: MaterialApp(
          home: const MyForSalesScreen(),
          localizationsDelegates: AppLocalizations.localizationsDelegates,
          supportedLocales: AppLocalizations.supportedLocales,
          locale: const Locale('id'),
        ),
      ),
    );
    if (settle) await tester.pumpAndSettle();
  }

  _ScriptedForSaleRepository repoOf(List<ForSale> data) =>
      _ScriptedForSaleRepository(
        onFetchSeller: (_) => Future.value(Result.success(data)),
      );

  Finder tabByLabel(String label) =>
      find.descendant(of: find.byType(TabBar), matching: find.text(label));

  Future<void> selectTab(WidgetTester tester, String label) async {
    await tester.tap(tabByLabel(label));
    await tester.pumpAndSettle();
  }

  group('MyForSalesScreen — canonical status tabs', () {
    testWidgets('renders every canonical filter tab', (tester) async {
      await pumpScreen(tester, repoOf(_allStates()));

      expect(tabByLabel('Semua Status'), findsOneWidget);
      expect(tabByLabel('Active'), findsOneWidget);
      expect(tabByLabel('Sold'), findsOneWidget);
      expect(tabByLabel('Withdrawn'), findsOneWidget);
    });

    testWidgets('default selected filter is Active', (tester) async {
      await pumpScreen(tester, repoOf(_allStates()));

      expect(find.text('Koi active-1'), findsOneWidget);
      expect(find.text('Koi sold-1'), findsNothing);
      expect(find.text('Koi withdrawn-1'), findsNothing);
    });

    testWidgets('Semua Status shows all owner states', (tester) async {
      await pumpScreen(tester, repoOf(_allStates()));
      await selectTab(tester, 'Semua Status');

      expect(find.text('Koi active-1'), findsOneWidget);
      expect(find.text('Koi sold-1'), findsOneWidget);
      expect(find.text('Koi withdrawn-1'), findsOneWidget);
    });

    testWidgets('Active shows only active', (tester) async {
      await pumpScreen(tester, repoOf(_allStates()));
      await selectTab(tester, 'Sold');
      await selectTab(tester, 'Active');

      expect(find.text('Koi active-1'), findsOneWidget);
      expect(find.text('Koi sold-1'), findsNothing);
      expect(find.text('Koi withdrawn-1'), findsNothing);
    });

    testWidgets('Sold shows only sold', (tester) async {
      await pumpScreen(tester, repoOf(_allStates()));
      await selectTab(tester, 'Sold');

      expect(find.text('Koi active-1'), findsNothing);
      expect(find.text('Koi sold-1'), findsOneWidget);
      expect(find.text('Koi withdrawn-1'), findsNothing);
    });

    testWidgets('Withdrawn shows only withdrawn', (tester) async {
      await pumpScreen(tester, repoOf(_allStates()));
      await selectTab(tester, 'Withdrawn');

      expect(find.text('Koi active-1'), findsNothing);
      expect(find.text('Koi sold-1'), findsNothing);
      expect(find.text('Koi withdrawn-1'), findsOneWidget);
    });

    testWidgets('switching tabs does not refetch the collection', (
      tester,
    ) async {
      final repo = repoOf(_allStates());
      await pumpScreen(tester, repo);
      expect(repo.fetchCalls, 1);

      await selectTab(tester, 'Sold');
      await selectTab(tester, 'Withdrawn');
      await selectTab(tester, 'Semua Status');

      expect(
        repo.fetchCalls,
        1,
        reason: 'provider is keyed by SellerForSalesParams, not the filter',
      );
    });

    testWidgets('empty state is preserved for an empty tab', (tester) async {
      await pumpScreen(
        tester,
        repoOf([_forSale('active-1', ForSaleStatus.active)]),
      );
      await selectTab(tester, 'Sold');

      // The account DOES own a listing, so an empty "Sold" tab is a
      // filter-empty state (distinct copy + reset), never collection copy.
      expect(find.text('Tidak Ada Hasil'), findsOneWidget);
      expect(find.text('Belum Ada For Sale'), findsNothing);
      expect(find.widgetWithText(FilledButton, 'Atur Ulang'), findsOneWidget);
    });
  });

  group('MyForSalesScreen — tab swipe synchronization', () {
    // The [TabBar] and [TabBarView] share one controller, so its index is the
    // authoritative selected-tab observable for these assertions.
    TabController controllerOf(WidgetTester tester) =>
        tester.widget<TabBar>(find.byType(TabBar)).controller!;

    // Drag past half the page width so the PageView snaps exactly one page.
    Future<void> swipe(WidgetTester tester, {required bool forward}) async {
      final width = tester.getSize(find.byType(TabBarView)).width;
      final dx = width * 0.75 * (forward ? -1 : 1);
      await tester.drag(find.byType(TabBarView), Offset(dx, 0));
      await tester.pumpAndSettle();
    }

    testWidgets('swipe changes indicator and content, and back again', (
      tester,
    ) async {
      await pumpScreen(tester, repoOf(_allStates()));

      // Default is Active (index 1).
      expect(controllerOf(tester).index, 1);
      expect(find.text('Koi active-1'), findsOneWidget);
      expect(find.text('Koi sold-1'), findsNothing);

      // Swipe forward → Sold (index 2).
      await swipe(tester, forward: true);
      expect(controllerOf(tester).index, 2);
      expect(find.text('Koi sold-1'), findsOneWidget);
      expect(find.text('Koi active-1'), findsNothing);

      // Swipe back → Active (index 1).
      await swipe(tester, forward: false);
      expect(controllerOf(tester).index, 1);
      expect(find.text('Koi active-1'), findsOneWidget);
      expect(find.text('Koi sold-1'), findsNothing);
    });

    testWidgets('tap then swipe then tap keeps indicator and content in sync', (
      tester,
    ) async {
      await pumpScreen(tester, repoOf(_allStates()));

      // Tap → Semua Status (index 0).
      await selectTab(tester, 'Semua Status');
      expect(controllerOf(tester).index, 0);
      expect(find.text('Koi withdrawn-1'), findsOneWidget);

      // Swipe → Active (index 1).
      await swipe(tester, forward: true);
      expect(controllerOf(tester).index, 1);
      expect(find.text('Koi active-1'), findsOneWidget);
      expect(find.text('Koi withdrawn-1'), findsNothing);

      // Tap → Withdrawn (index 3).
      await selectTab(tester, 'Withdrawn');
      expect(controllerOf(tester).index, 3);
      expect(find.text('Koi withdrawn-1'), findsOneWidget);
      expect(find.text('Koi active-1'), findsNothing);
    });

    testWidgets('swipe does not refetch the owner collection', (tester) async {
      final repo = repoOf(_allStates());
      await pumpScreen(tester, repo);
      expect(repo.fetchCalls, 1);

      await swipe(tester, forward: true);
      await swipe(tester, forward: true);
      await swipe(tester, forward: false);

      expect(
        repo.fetchCalls,
        1,
        reason: 'the watched provider key is filter-independent',
      );
    });

    testWidgets('reset after a swiped filter restores Semua Status content', (
      tester,
    ) async {
      await pumpScreen(
        tester,
        repoOf([_forSale('active-1', ForSaleStatus.active)]),
      );

      // Active (1) → Sold (2), which has no rows → filter-empty state.
      await swipe(tester, forward: true);
      expect(controllerOf(tester).index, 2);
      expect(find.text('Tidak Ada Hasil'), findsOneWidget);

      await tester.tap(find.widgetWithText(FilledButton, 'Atur Ulang'));
      await tester.pumpAndSettle();

      expect(controllerOf(tester).index, 0);
      expect(find.text('Tidak Ada Hasil'), findsNothing);
      expect(find.text('Koi active-1'), findsOneWidget);
    });
  });

  group('MyForSalesScreen — initial state', () {
    testWidgets('first request shows LoadingIndicator, never empty/error', (
      tester,
    ) async {
      final gate = Completer<Result<List<ForSale>>>();
      final repo = _ScriptedForSaleRepository(
        onFetchSeller: (_) => gate.future,
      );
      await pumpScreen(tester, repo, settle: false);
      await tester.pump();
      await tester.pump();

      expect(find.byType(LoadingIndicator), findsOneWidget);
      expect(find.byType(EmptyState), findsNothing);
      expect(find.byType(PageErrorState), findsNothing);

      gate.complete(Result.success(_allStates()));
      await tester.pumpAndSettle();
      expect(find.text('Koi active-1'), findsOneWidget);
    });

    testWidgets('successful zero-result shows EmptyState', (tester) async {
      await pumpScreen(tester, repoOf(const <ForSale>[]));

      expect(find.byType(EmptyState), findsOneWidget);
      expect(find.text('Belum Ada For Sale'), findsOneWidget);
      expect(find.byType(PageErrorState), findsNothing);
      expect(find.byType(LoadingIndicator), findsNothing);
    });

    testWidgets('non-empty result shows the collection', (tester) async {
      await pumpScreen(tester, repoOf(_allStates()));
      await selectTab(tester, 'Semua Status');

      expect(find.text('Koi active-1'), findsOneWidget);
      expect(find.text('Koi sold-1'), findsOneWidget);
      expect(find.text('Koi withdrawn-1'), findsOneWidget);
    });
  });

  group('MyForSalesScreen — initial failure', () {
    testWidgets('failure with no data shows PageErrorState, retry reloads', (
      tester,
    ) async {
      final repo = _ScriptedForSaleRepository(
        onFetchSeller: (call) => call == 1
            ? Future.value(Result.error('HTTP 500: boom-initial sql: no rows'))
            : Future.value(Result.success(_allStates())),
      );
      await pumpScreen(tester, repo);

      // CANONICAL error surface: safe localized copy only — the raw
      // backend text must never reach the screen.
      expect(find.byType(PageErrorState), findsOneWidget);
      expect(find.text('Terjadi Kesalahan'), findsOneWidget);
      expect(
        find.text('Data belum bisa dimuat. Silakan coba lagi.'),
        findsOneWidget,
      );
      expect(find.text('Coba Lagi'), findsOneWidget);
      expect(find.textContaining('boom-initial'), findsNothing);
      expect(find.textContaining('HTTP 500'), findsNothing);
      expect(find.textContaining('sql:'), findsNothing);
      expect(find.byType(EmptyState), findsNothing);

      // Retry executes the actual initial-load operation.
      expect(repo.fetchCalls, 1);
      await tester.tap(find.widgetWithText(ElevatedButton, 'Coba Lagi'));
      await tester.pumpAndSettle();

      expect(repo.fetchCalls, 2);
      expect(find.byType(PageErrorState), findsNothing);
      expect(find.text('Koi active-1'), findsOneWidget);
    });
  });

  group('MyForSalesScreen — refresh', () {
    testWidgets('existing rows stay visible with refresh indicator', (
      tester,
    ) async {
      final gate = Completer<Result<List<ForSale>>>();
      final repo = _ScriptedForSaleRepository(
        onFetchSeller: (call) => call == 1
            ? Future.value(
                Result.success([_forSale('a', ForSaleStatus.active)]),
              )
            : gate.future,
      );
      await pumpScreen(tester, repo);
      expect(find.text('Koi a'), findsOneWidget);

      await tester.fling(
        find.byType(CustomScrollView),
        const Offset(0, 300),
        1000,
      );
      // Allow the RefreshIndicator to fire onRefresh and the refetch to
      // start (bounded: the gate stays open, so never settle here).
      for (var i = 0; i < 50 && repo.fetchCalls < 2; i++) {
        await tester.pump(const Duration(milliseconds: 50));
      }
      expect(repo.fetchCalls, 2);

      // Refresh must not clear the list into full loading.
      expect(find.text('Koi a'), findsOneWidget);
      expect(find.byType(LinearProgressIndicator), findsOneWidget);
      expect(find.byType(LoadingIndicator), findsNothing);
      expect(find.byType(PageErrorState), findsNothing);

      gate.complete(Result.success([_forSale('b', ForSaleStatus.active)]));
      await tester.pumpAndSettle();

      expect(find.text('Koi b'), findsOneWidget);
      expect(find.text('Koi a'), findsNothing);
      expect(find.byType(LinearProgressIndicator), findsNothing);
    });

    testWidgets(
      'refresh failure keeps rows with inline banner, retry recovers',
      (tester) async {
        final repo = _ScriptedForSaleRepository(
          onFetchSeller: (call) {
            if (call == 1) {
              return Future.value(
                Result.success([_forSale('a', ForSaleStatus.active)]),
              );
            }
            if (call == 2) {
              return Future.value(
                Result.error('HTTP 500: boom-refresh sql: timeout'),
              );
            }
            return Future.value(
              Result.success([_forSale('b', ForSaleStatus.active)]),
            );
          },
        );
        await pumpScreen(tester, repo);
        expect(find.text('Koi a'), findsOneWidget);

        await tester.fling(
          find.byType(CustomScrollView),
          const Offset(0, 300),
          1000,
        );
        await tester.pumpAndSettle();

        // Valid data is preserved; failure renders inline, never full-page.
        expect(find.text('Koi a'), findsOneWidget);
        expect(find.byType(PageErrorState), findsNothing);
        expect(
          find.text('Data belum bisa dimuat. Silakan coba lagi.'),
          findsOneWidget,
        );
        expect(find.widgetWithText(TextButton, 'Coba Lagi'), findsOneWidget);
        expect(find.textContaining('boom-refresh'), findsNothing);
        expect(find.textContaining('HTTP 500'), findsNothing);
        expect(find.textContaining('sql:'), findsNothing);

        // Retry executes refresh; success replaces stale data and clears banner.
        await tester.tap(find.widgetWithText(TextButton, 'Coba Lagi'));
        await tester.pumpAndSettle();

        expect(repo.fetchCalls, 3);
        expect(find.text('Koi b'), findsOneWidget);
        expect(find.text('Koi a'), findsNothing);
        expect(
          find.text('Data belum bisa dimuat. Silakan coba lagi.'),
          findsNothing,
        );
      },
    );
  });

  group('MyForSalesScreen — delete / invalidation', () {
    testWidgets('delete reloads through the canonical key with fresh data', (
      tester,
    ) async {
      final repo = _ScriptedForSaleRepository(
        onFetchSeller: (call) => call == 1
            ? Future.value(Result.success(_allStates()))
            : Future.value(
                Result.success([_forSale('sold-1', ForSaleStatus.sold)]),
              ),
      );
      await pumpScreen(tester, repo);
      expect(find.text('Koi active-1'), findsOneWidget);

      await tester.tap(find.byType(PopupMenuButton<String>).first);
      await tester.pumpAndSettle();
      await tester.tap(find.text('Hapus'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Ya, Hapus'));
      await tester.pumpAndSettle();

      expect(repo.deletedIds, ['active-1']);
      expect(find.text('For Sale berhasil dihapus'), findsOneWidget);

      // The invalidation rebuilt the watched provider: one reload, same key.
      expect(repo.fetchCalls, 2);
      final invalidation = repo.observedFetches[1];
      expect(invalidation.sellerId, _uid);
      expect(invalidation.page, 1);
      expect(invalidation.pageSize, 50);
      expect(invalidation.includeWithdrawn, isTrue);
      for (final observed in repo.observedFetches) {
        expect(
          observed.pageSize,
          50,
          reason: 'no invalidation path may use a mismatched page size',
        );
      }

      // Resulting collection is fresh: the deleted row is gone.
      expect(find.text('Koi active-1'), findsNothing);
    });

    testWidgets('failed delete keeps the collection and reports inline', (
      tester,
    ) async {
      final repo = _ScriptedForSaleRepository(
        onFetchSeller: (_) => Future.value(Result.success(_allStates())),
      )..onDelete = (_) => Future.value(Result.error('boom-delete'));
      await pumpScreen(tester, repo);

      await tester.tap(find.byType(PopupMenuButton<String>).first);
      await tester.pumpAndSettle();
      await tester.tap(find.text('Hapus'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Ya, Hapus'));
      await tester.pumpAndSettle();

      expect(repo.deletedIds, ['active-1']);
      expect(find.text('Gagal menghapus For Sale. Coba lagi.'), findsOneWidget);
      // No reload was triggered by the failed delete.
      expect(repo.fetchCalls, 1);
      expect(find.text('Koi active-1'), findsOneWidget);
    });
  });

  group('MyForSalesScreen — canonical status badge labels', () {
    Finder badge(String label) => find.descendant(
      of: find.byType(CustomScrollView),
      matching: find.text(label),
    );

    testWidgets('active badge uses the canonical label authority', (
      tester,
    ) async {
      await pumpScreen(tester, repoOf([_forSale('a', ForSaleStatus.active)]));

      expect(badge(ForSaleStatus.active.displayName), findsOneWidget);
      expect(find.text('Aktif'), findsNothing);
    });

    testWidgets('sold badge uses the canonical label authority', (
      tester,
    ) async {
      await pumpScreen(tester, repoOf([_forSale('s', ForSaleStatus.sold)]));
      await selectTab(tester, 'Sold');

      expect(badge(ForSaleStatus.sold.displayName), findsOneWidget);
      expect(find.text('Terjual'), findsNothing);
    });

    testWidgets('withdrawn badge uses the canonical label authority', (
      tester,
    ) async {
      await pumpScreen(
        tester,
        repoOf([_forSale('w', ForSaleStatus.withdrawn)]),
      );
      await selectTab(tester, 'Withdrawn');

      expect(badge(ForSaleStatus.withdrawn.displayName), findsOneWidget);
      expect(find.text('Ditarik'), findsNothing);
    });

    test('the screen has no local status-to-label mapping', () {
      final source = File(
        'lib/domains/commerce/catalog/for_sale/presentation/screens/my_for_sales_screen.dart',
      ).readAsStringSync();

      // One label authority: the canonical getter.
      expect(source.contains('status.displayName'), isTrue);
      // No duplicate hardcoded labels remain.
      expect(source.contains("'Aktif'"), isFalse);
      expect(source.contains("'Ditarik'"), isFalse);
      expect(source.contains("'Terjual'"), isFalse);
    });
  });

  group('MyForSalesScreen — negative proof (static contract)', () {
    String source() => File(
      'lib/domains/commerce/catalog/for_sale/presentation/screens/my_for_sales_screen.dart',
    ).readAsStringSync().replaceAll('\r\n', '\n');

    test('canonical renderers own every page state', () {
      final src = source();
      expect(src.contains('LoadingIndicator('), isTrue);
      expect(src.contains('PageErrorState('), isTrue);
      expect(src.contains('EmptyState('), isTrue);
    });

    test('no raw first-load spinner remains', () {
      expect(
        source(),
        isNot(contains('Center(child: CircularProgressIndicator())')),
      );
    });

    test('no raw technical error reaches the widget tree', () {
      final src = source();
      expect(src.contains('error.toString()'), isFalse);
      expect(src.contains('Text(error'), isFalse);
      expect(src.contains('Text(message'), isFalse);
    });

    test('single params authority and single reload path', () {
      final src = source();
      // One construction site: watch, refresh, retry, and delete all derive
      // the key from it, so pageSize can never mismatch again.
      expect('SellerForSalesParams('.allMatches(src).length, 1);
      expect(src.contains('ref.refresh(sellerForSalesProvider'), isTrue);
      // The obsolete invalidate-and-clear path is gone.
      expect(src.contains('ref.invalidate(sellerForSalesProvider'), isFalse);
    });

    test('no second state authority in scope', () {
      final src = source();
      expect(src.contains('NotifierProvider'), isFalse);
      expect(src.contains('_ErrorState'), isFalse);
    });
  });
}
