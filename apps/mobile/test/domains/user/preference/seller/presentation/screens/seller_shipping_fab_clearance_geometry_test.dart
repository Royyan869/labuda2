// SAFE-AREA-10 — SELLER SHIPPING: LIVE SYSTEM INSET + FAB OVERLAY CLEARANCE.
//
// Locks the converged authorities on SellerShippingScreen:
//
//   * system bottom inset  → the body `SafeArea` (LIVE: the list bottom
//     tracks 0 / 24 / 34 / 48 as the window metrics change; a fixed
//     clearance could not);
//   * FAB position         → Flutter (Scaffold `floatingActionButton`,
//     `endFloat` lifted by `minViewPadding.bottom`) — never asserted here
//     beyond measuring the rendered rect;
//   * FAB overlay clearance→ `AppMetrics.fabClearance`, measured ABOVE the
//     live inset: `contentEnd = systemInset + fabClearance`, so content
//     clears the FAB at every inset instead of being eroded by it.
//
// Geometry is measured on the REAL screen (loaded state, 12 options) with
// injected window metrics — not on a stand-in scaffold.
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_shipping_screen.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/providers/wilayah_provider_simple.dart';

/// Static repository: the list state resolves to [options] without any
/// backend. Mutation paths are never exercised by these geometry tests.
class _StaticRepo implements ShippingRepository {
  _StaticRepo(this._options);

  final List<ShippingSetup> _options;

  @override
  Future<Result<List<ShippingSetup>>> listMyShippingSetups() async =>
      Result.success(_options);

  @override
  Future<Result<List<ShippingSetup>>> listMyActiveShippingSetups() async =>
      Result.success(_options.where((o) => o.isActive).toList());

  @override
  Future<Result<ShippingSetup>> getShippingSetupById(String optionId) async =>
      Result.error('not used');

  @override
  Future<Result<ShippingSetup>> createShippingSetup(
    CreateShippingSetupRequest request,
  ) async => Result.error('not used');

  @override
  Future<Result<ShippingSetup>> updateShippingSetup(
    String optionId,
    UpdateShippingSetupRequest request,
  ) async => Result.error('not used');

  @override
  Future<Result<void>> deleteShippingSetup(String optionId) async =>
      Result.error('not used');

  @override
  Future<Result<void>> toggleActiveStatus(String optionId, bool isActive) async =>
      Result.error('not used');

  @override
  Future<Result<void>> setProductShippingSetups(
    String productId,
    List<String> ids,
  ) async => Result.error('not used');

  @override
  Future<Result<List<DeliveryOption>>> checkDeliveryAvailability(
    CheckDeliveryRequest request,
  ) async => Result.error('not used');
}

/// Failing repository: the list resolves to an error so the screen renders
/// its REAL `_ErrorView` (private — reached only through the screen).
class _ErrorRepo extends _StaticRepo {
  _ErrorRepo() : super(const []);

  @override
  Future<Result<List<ShippingSetup>>> listMyShippingSetups() async =>
      Result.error('load failed');
}

class _FakeAuthController extends AuthController {
  @override
  AuthState build() {
    final now = DateTime.utc(2026, 7, 25);
    final user = AuthUser(
      id: 'seller-1',
      createdAt: now,
      updatedAt: now,
      email: 'seller@example.com',
      username: 'seller',
      isEmailVerified: true,
      accountStatus: AccountStatus.active,
      hasSellerProfile: true,
      sellerSubscriptionStatus: 'active',
      hasMarketAuthority: true,
      roles: const [UserRole.user],
      provider: AuthProvider.email,
      lifecycle: ContentLifecycle.active,
    );
    return AuthState.authenticated(user, emailVerified: true);
  }
}

List<ShippingSetup> _options(int count) => [
  for (var i = 0; i < count; i++)
    ShippingSetup(
      id: 'opt-$i',
      name: 'Opsi $i',
      type: ShippingType.bus,
      coverageAreas: const [],
      isActive: true,
      createdAt: DateTime.utc(2026, 7, 25),
      updatedAt: DateTime.utc(2026, 7, 25),
    ),
];

/// Injects window metrics on the TEST VIEW (same idiom as the SAFE-AREA-01
/// contract test) so every inset below is the REAL, LIVE one.
void _setInsets(WidgetTester tester, {required double bottom}) {
  final double dpr = tester.view.devicePixelRatio;
  tester.view.padding = FakeViewPadding(top: 24 * dpr, bottom: bottom * dpr);
  tester.view.viewPadding = FakeViewPadding(top: 24 * dpr, bottom: bottom * dpr);
  tester.view.viewInsets = const FakeViewPadding();
}

Future<void> _pump(
  WidgetTester tester, {
  required double inset,
  required List<ShippingSetup> options,
  /// Error-state runs inject a failing repository instead of [options].
  ShippingRepository? repository,
  /// Optional surface override (logical px) for the short-viewport case.
  Size? surfaceSize,
}) async {
  addTearDown(tester.view.reset);
  if (surfaceSize != null) {
    tester.view.physicalSize = surfaceSize;
    tester.view.devicePixelRatio = 1.0;
  }
  _setInsets(tester, bottom: inset);

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(_FakeAuthController.new),
        shippingRepositoryProvider.overrideWithValue(
          repository ?? _StaticRepo(options),
        ),
        provincesProvider.overrideWith((ref) async => const []),
      ],
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
        theme: AppTheme.lightTheme,
        home: const SellerShippingScreen(),
      ),
    ),
  );
  // initState schedules the reload in a post-frame callback; settle the
  // load → Loaded transition (static content settles, no looping animation).
  await tester.pumpAndSettle();
}

/// Shared loaded-state invariants at [inset]. Every expectation fails with
/// the inset in the reason text.
///
/// `ScrollView.padding` is an `EdgeInsetsGeometry?`; the screen always
/// builds `EdgeInsets`, so the downcast is safe and keeps call sites terse.
double _bottomPad(ListView list) => (list.padding! as EdgeInsets).bottom;

Future<void> _expectLoadedGeometry(
  WidgetTester tester, {
  required double inset,
}) async {
  final Rect surface = tester.getRect(find.byType(Scaffold));
  final Finder listFinder = find.byType(ListView);
  final ListView list = tester.widget<ListView>(listFinder);
  final Rect listRect = tester.getRect(listFinder);

  // (1) The body SafeArea consumes the LIVE system inset: the list bottom
  // sits exactly `inset` above the screen bottom — for 0, 24, 34 AND 48.
  // A fixed clearance could never track all four values.
  expect(
    listRect.bottom,
    closeTo(surface.bottom - inset, 0.01),
    reason:
        'list bottom must follow the live system inset ($inset), proving the '
        'body SafeArea — not a fixed constant — owns the bottom inset',
  );

  // (2) The FAB overlay clearance is present and is the SAME authority in
  // every state: content end = systemInset + fabClearance.
  expect(list.padding, isNotNull);
  expect(
    _bottomPad(list),
    AppMetrics.fabClearance,
    reason: 'the loaded list must clear the FAB via AppMetrics.fabClearance',
  );

  // (3) Measured content end clears the measured FAB — the clearance is
  // measured ABOVE the inset, so it is not eroded when the inset grows.
  final Rect fab = tester.getRect(find.byType(FloatingActionButton));
  final double contentEnd = listRect.bottom - _bottomPad(list);
  expect(
    contentEnd,
    lessThanOrEqualTo(fab.top),
    reason:
        'content end ($contentEnd) must clear the FAB top (${fab.top}) at '
        'inset $inset',
  );

  // (4) Scrolled to the very end, the last rendered row still clears the
  // FAB (row = Card; the honesty banner is a Container).
  final ScrollableState scrollable = tester.state<ScrollableState>(
    find.descendant(of: listFinder, matching: find.byType(Scrollable)).first,
  );
  scrollable.position.jumpTo(scrollable.position.maxScrollExtent);
  await tester.pump();
  final Rect lastRow = tester.getRect(find.byType(Card).last);
  expect(
    lastRow.bottom,
    lessThanOrEqualTo(fab.top),
    reason: 'the last list row must not tuck under the FAB at inset $inset',
  );
}

/// Shared error-state invariants at [inset] on the DEFAULT test surface:
/// the real `_ErrorView` list clears the FAB exactly like the loaded/empty
/// lists — system inset from the body `SafeArea`, FAB clearance from
/// `AppMetrics.fabClearance`, measured ABOVE that inset.
Future<void> _expectErrorGeometry(
  WidgetTester tester, {
  required double inset,
}) async {
  final Rect surface = tester.getRect(find.byType(Scaffold));
  final Finder listFinder = find.byType(ListView);
  final ListView list = tester.widget<ListView>(listFinder);
  final Rect listRect = tester.getRect(listFinder);
  final Rect fab = tester.getRect(find.byType(FloatingActionButton));

  // (1) SafeArea owns the LIVE system inset — the same proof as loaded/empty.
  expect(
    listRect.bottom,
    closeTo(surface.bottom - inset, 0.01),
    reason:
        'error list bottom must follow the live system inset ($inset), proving '
        'the body SafeArea — not a constant — owns it',
  );

  // (2) The FAB clearance authority is the SAME as the other two states —
  // no second magic bottom value in this screen.
  expect(list.padding, isNotNull);
  expect(
    _bottomPad(list),
    AppMetrics.fabClearance,
    reason: 'the error state must clear the FAB via AppMetrics.fabClearance',
  );

  // (3) Measured content end clears the measured FAB top.
  final double contentEnd = listRect.bottom - _bottomPad(list);
  expect(
    contentEnd,
    lessThanOrEqualTo(fab.top),
    reason:
        'error content end ($contentEnd) must clear the FAB top (${fab.top}) '
        'at inset $inset',
  );

  // (4) Unscrolled, the visible error content (icon → message → retry) is
  // clear of the FAB.
  final Finder retryFinder = find.widgetWithText(ElevatedButton, 'Coba Lagi');
  expect(retryFinder, findsOneWidget);
  expect(
    tester.getRect(retryFinder).bottom,
    lessThanOrEqualTo(fab.top),
    reason: 'the retry button must not sit under the FAB at inset $inset',
  );

  // (5) Scrolled to the very end, the last child still clears the FAB.
  final ScrollableState scrollable = tester.state<ScrollableState>(
    find.descendant(of: listFinder, matching: find.byType(Scrollable)).first,
  );
  scrollable.position.jumpTo(scrollable.position.maxScrollExtent);
  await tester.pump();
  expect(
    tester.getRect(retryFinder).bottom,
    lessThanOrEqualTo(fab.top),
    reason:
        'at maxScrollExtent the retry button must still clear the FAB at '
        'inset $inset',
  );
}

void main() {
  group('SAFE-AREA-11 — error state: live inset + FAB clearance', () {
    Future<void> errorGeometryAt(WidgetTester tester, double inset) async {
      await _pump(
        tester,
        inset: inset,
        options: const [],
        repository: _ErrorRepo(),
      );
      expect(find.text('Gagal memuat opsi pengiriman'), findsOneWidget);
      await _expectErrorGeometry(tester, inset: inset);
    }

    testWidgets('inset 0 — error list clears the FAB', (tester) async {
      await errorGeometryAt(tester, 0);
    });

    testWidgets('inset 24 — error list clears the FAB', (tester) async {
      await errorGeometryAt(tester, 24);
    });

    testWidgets('inset 34 — error list clears the FAB', (tester) async {
      await errorGeometryAt(tester, 34);
    });

    testWidgets('inset 48 — error list clears the FAB', (tester) async {
      await errorGeometryAt(tester, 48);
    });

    testWidgets(
      'short viewport (landscape 640×360) — the error list really scrolls '
      'and the retry button clears the FAB at maxScrollExtent',
      (tester) async {
        await _pump(
          tester,
          inset: 0,
          options: const [],
          repository: _ErrorRepo(),
          surfaceSize: const Size(640, 360),
        );
        expect(find.text('Gagal memuat opsi pengiriman'), findsOneWidget);

        final Rect surface = tester.getRect(find.byType(Scaffold));
        final Finder listFinder = find.byType(ListView);
        final ListView list = tester.widget<ListView>(listFinder);
        final Rect listRect = tester.getRect(listFinder);
        final Rect fab = tester.getRect(find.byType(FloatingActionButton));

        // Two authorities, still separate: SafeArea → inset, fabClearance → FAB.
        expect(listRect.bottom, closeTo(surface.bottom - 0, 0.01));
        expect(list.padding, isNotNull);
        expect(_bottomPad(list), AppMetrics.fabClearance);
        expect(
          listRect.bottom - _bottomPad(list),
          lessThanOrEqualTo(fab.top),
          reason: 'error content end must clear the FAB on a short viewport',
        );

        // This viewport is the configuration the audit flagged: the error
        // content is TALLER than the body, so the scroll path is real.
        final ScrollableState scrollable = tester.state<ScrollableState>(
          find
              .descendant(of: listFinder, matching: find.byType(Scrollable))
              .first,
        );
        expect(
          scrollable.position.maxScrollExtent,
          greaterThan(0),
          reason: 'the short viewport must actually scroll the error content',
        );
        scrollable.position.jumpTo(scrollable.position.maxScrollExtent);
        await tester.pump();

        final Finder retryFinder = find.widgetWithText(
          ElevatedButton,
          'Coba Lagi',
        );
        final double retryBottom = tester.getRect(retryFinder).bottom;
        expect(
          retryBottom,
          lessThanOrEqualTo(fab.top),
          reason:
              'scrolled to the end the retry button ($retryBottom) must clear '
              'the FAB top (${fab.top})',
        );
      },
    );
  });

  group('SAFE-AREA-10 — loaded list: live inset + FAB clearance', () {
    testWidgets('inset 0 — content clears the FAB, no stale gap', (
      tester,
    ) async {
      await _pump(tester, inset: 0, options: _options(12));
      expect(find.text('Opsi 0'), findsOneWidget);
      await _expectLoadedGeometry(tester, inset: 0);
    });

    testWidgets('inset 24 — content clears the FAB', (tester) async {
      await _pump(tester, inset: 24, options: _options(12));
      expect(find.text('Opsi 0'), findsOneWidget);
      await _expectLoadedGeometry(tester, inset: 24);
    });

    testWidgets('inset 34 — content clears the FAB', (tester) async {
      await _pump(tester, inset: 34, options: _options(12));
      expect(find.text('Opsi 0'), findsOneWidget);
      await _expectLoadedGeometry(tester, inset: 34);
    });

    testWidgets('inset 48 — content clears the FAB', (tester) async {
      await _pump(tester, inset: 48, options: _options(12));
      expect(find.text('Opsi 0'), findsOneWidget);
      await _expectLoadedGeometry(tester, inset: 48);
    });
  });

  group('SAFE-AREA-10 — authority proof', () {
    testWidgets(
      'system inset tracks the window metrics (SafeArea), while the FAB '
      'clearance stays a constant measured above it',
      (tester) async {
        await _pump(tester, inset: 0, options: _options(12));

        final Finder listFinder = find.byType(ListView);
        final ListView listBefore = tester.widget<ListView>(listFinder);
        final double bottomAt0 = tester.getRect(listFinder).bottom;
        final Rect surface = tester.getRect(find.byType(Scaffold));

        // System bar appears (48px): only a live authority moves the list.
        _setInsets(tester, bottom: 48);
        await tester.pump();
        await tester.pump();

        final double bottomAt48 = tester.getRect(listFinder).bottom;
        expect(
          bottomAt0 - bottomAt48,
          closeTo(48, 0.01),
          reason:
              'the list bottom must follow the system inset — a fixed '
              'clearance would not move',
        );
        expect(bottomAt48, closeTo(surface.bottom - 48, 0.01));

        // The FAB clearance is untouched by the inset change: one constant,
        // one authority, measured above the live inset.
        final ListView listAfter = tester.widget<ListView>(listFinder);
        expect(_bottomPad(listAfter), AppMetrics.fabClearance);
        expect(_bottomPad(listBefore), AppMetrics.fabClearance);
        expect(find.byType(SafeArea), findsWidgets);
      },
    );

    testWidgets(
      'empty state shares the same geometry authorities as the loaded list',
      (tester) async {
        await _pump(tester, inset: 34, options: const []);
        expect(find.byType(ListView), findsOneWidget);

        final Rect surface = tester.getRect(find.byType(Scaffold));
        final Finder listFinder = find.byType(ListView);
        final ListView list = tester.widget<ListView>(listFinder);
        final Rect listRect = tester.getRect(listFinder);

        // Same two invariants the loaded state proves at inset 34:
        // SafeArea owns the live inset…
        expect(listRect.bottom, closeTo(surface.bottom - 34, 0.01));
        // …and the SAME FAB clearance authority — no second magic value.
        expect(list.padding, isNotNull);
        expect(
          _bottomPad(list),
          AppMetrics.fabClearance,
          reason:
              'the empty state must use the same FAB clearance authority as '
              'the loaded list',
        );
      },
    );
  });
}
