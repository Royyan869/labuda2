// SAFE-AREA-09A — ORDER DETAIL BOTTOM BAR SLOT AUTHORITY.
//
// Locks the structural correction: `Scaffold.bottomNavigationBar` presence
// must EXACTLY represent whether a bottom action surface exists.
//
//   * bar present (party + loaded) → the slot is non-null, the body ends
//     exactly at the bar, and `BottomActionBar` owns the live system inset
//     (SAFE-AREA-09: exactly ONE reservation, by the bar).
//   * no bar (order/refunds loading, or viewer not a party) → the slot is
//     `null`, Scaffold does NOT strip the body's bottom MediaQuery padding,
//     and the existing body `SafeArea` consumes the live system inset —
//     including left/right cutout padding (SAFE-AREA-09A).
//
// Geometry is measured against injected window metrics (0 / 24 / 48 and a
// landscape-style left cutout) on the real OrderDetailScreen — not on a
// stand-in scaffold.
import 'package:flutter/material.dart' hide Action;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/src/auth/app_role.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';
import 'package:hishumi/domains/commerce/transaction/order/domain/entities/order.dart'
    show Action;
import 'package:hishumi/domains/commerce/transaction/order/order.dart'
    hide Action;
import 'package:hishumi/domains/social/rating/rating.dart';
import 'package:hishumi/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:hishumi/domains/user/identity/authentication/authentication.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';
import 'package:hishumi/shared/widgets/bottom_action_bar.dart';

// The id must be ≥8 chars: `OrderInfoCard` renders
// `order.orderNumber ?? order.id.substring(0, 8)`.
const String _orderId = 'order-0001';
const String _buyerId = 'buyer-1';
const String _sellerId = 'seller-1';
const double _surfaceHeight = 600;

int _scopeGeneration = 0;

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._userId);

  final String _userId;

  @override
  AuthState build() {
    final now = DateTime.parse('2026-07-01T00:00:00.000Z');
    final user = AuthUser(
      id: _userId,
      createdAt: now,
      updatedAt: now,
      email: 'user@example.com',
      username: 'user1',
      isEmailVerified: true,
      accountStatus: AccountStatus.active,
      hasSellerProfile: false,
      sellerSubscriptionStatus: 'none',
      hasMarketAuthority: false,
      roles: const [UserRole.user],
      provider: AuthProvider.email,
      lifecycle: ContentLifecycle.active,
    );
    return AuthState.authenticated(user, emailVerified: true);
  }
}

Order _order() {
  return Order(
    id: _orderId,
    buyerId: _buyerId,
    sellerId: _sellerId,
    items: const [],
    status: OrderStatus.pending,
    paymentMethodCode: 'bank_transfer',
    paymentStatus: PaymentStatus.pending,
    shippingInfo: const ShippingInfo(
      recipientName: 'Buyer',
      phone: '08123456789',
      address: 'Some address',
      method: ShippingMethod.bus,
      shippingCost: 10000,
    ),
    pricing: const OrderPricing(
      subtotal: 100000,
      shippingCost: 10000,
      commissionAmount: 0,
      totalBeforeCoinsAmount: 110000,
      totalPayableAmount: 110000,
    ),
    createdAt: DateTime.utc(2026, 7, 1),
    source: OrderSource.forSale,
    decision: const DecisionContract(
      state: 'pending',
      primaryAction: Action(
        type: 'pay',
        labelKey: 'action.payment_continue',
        enabled: true,
        endpoint: '/api/v1/payments',
        method: 'POST',
        requiresIdempotency: true,
        financial: false,
      ),
    ),
  );
}

/// Injects window metrics on the TEST VIEW (same idiom as the SAFE-AREA-01
/// contract test) so every inset below is the REAL, LIVE one.
void _setWindowInsets(
  WidgetTester tester, {
  double systemBottom = 0,
  double systemTop = 0,
  double systemLeft = 0,
  double systemRight = 0,
}) {
  final double dpr = tester.view.devicePixelRatio;
  tester.view.padding = FakeViewPadding(
    top: systemTop * dpr,
    bottom: systemBottom * dpr,
    left: systemLeft * dpr,
    right: systemRight * dpr,
  );
  tester.view.viewPadding = FakeViewPadding(
    top: systemTop * dpr,
    bottom: systemBottom * dpr,
    left: systemLeft * dpr,
    right: systemRight * dpr,
  );
  tester.view.viewInsets = const FakeViewPadding();
}

/// Pumps the REAL OrderDetailScreen.
///
/// * [order] == null → the order stream never emits (loading state).
/// * [authUserId] != order buyer/seller → the not-a-party no-bar state.
Future<void> _pumpOrderDetail(
  WidgetTester tester, {
  required String authUserId,
  required double systemBottom,
  Order? order,
  double systemTop = 0,
  double systemLeft = 0,
  double systemRight = 0,
}) async {
  addTearDown(tester.view.reset);
  _setWindowInsets(
    tester,
    systemBottom: systemBottom,
    systemTop: systemTop,
    systemLeft: systemLeft,
    systemRight: systemRight,
  );

  final Order? resolved = order;
  await tester.pumpWidget(
    ProviderScope(
      // A fresh key per pump guarantees a fresh container, so re-pumping
      // with different overrides (the authority-switch test) cannot observe
      // the previous pump's auth/provider state.
      key: ValueKey('order-detail-scope-${_scopeGeneration++}'),
      overrides: [
        authControllerProvider.overrideWith(
          () => _FakeAuthController(authUserId),
        ),
        if (resolved != null) ...[
          watchOrderProvider(resolved.id)
              .overrideWith((ref) => Stream.value(resolved)),
          refundsByOrderProvider(resolved.id)
              .overrideWith((ref) => Stream.value(<RefundRequest>[])),
        ] else
          // Never emits and never errors: the provider stays in loading
          // forever without leaving pending timers behind.
          watchOrderProvider(_orderId)
              .overrideWith((ref) => const Stream<Order>.empty()),
        hasUserRatedOrderProvider(
          orderId: _orderId,
          buyerId: _buyerId,
          sellerId: _sellerId,
        ).overrideWith((ref) async => false),
      ],
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
        theme: AppTheme.lightTheme,
        home: OrderDetailScreen(orderId: _orderId),
      ),
    ),
  );
  // One frame to build, one to settle the StreamProvider emission — avoids
  // pumpAndSettle, which would spin forever on the loading spinner state.
  await tester.pump();
  await tester.pump();
}

Scaffold _scaffold(WidgetTester tester) =>
    tester.widget<Scaffold>(find.byType(Scaffold));

/// The bottom-most button inside the bar (the stacked layout ends with the
/// support-link footer, below the CTA): its bottom edge is the bar content's
/// lowest visible edge and must sit exactly `p12 + inset` above the screen
/// bottom.
Rect _bottomMostButtonRect(WidgetTester tester) {
  final Iterable<Rect> rects = find
      .descendant(
        of: find.byType(BottomActionBar),
        matching: find.byWidgetPredicate((w) => w is ButtonStyleButton),
      )
      .evaluate()
      .map(
        (element) =>
            tester.getRect(find.byElementPredicate((e) => identical(e, element))),
      );
  return rects.reduce((a, b) => a.bottom >= b.bottom ? a : b);
}

void main() {
  group('SAFE-AREA-09A — slot presence mirrors the actual surface', () {
    testWidgets('party + loaded data: slot non-null, BottomActionBar exists',
        (tester) async {
      await _pumpOrderDetail(
        tester,
        authUserId: _buyerId,
        systemBottom: 48,
        order: _order(),
      );

      expect(_scaffold(tester).bottomNavigationBar, isNotNull);
      expect(find.byType(BottomActionBar), findsOneWidget);
      expect(find.byType(ListView), findsOneWidget);
    });

    testWidgets('not a party + loaded data: slot is null, no bar', (
      tester,
    ) async {
      await _pumpOrderDetail(
        tester,
        authUserId: 'intruder-1',
        systemBottom: 24,
        order: _order(),
      );

      expect(_scaffold(tester).bottomNavigationBar, isNull);
      expect(find.byType(BottomActionBar), findsNothing);
      // Body content is active in this state — the whole point of 09A.
      expect(find.byType(ListView), findsOneWidget);
    });

    testWidgets('loading: slot is null, no fake zero-height bar', (
      tester,
    ) async {
      await _pumpOrderDetail(
        tester,
        authUserId: _buyerId,
        systemBottom: 48,
        order: null,
      );

      expect(_scaffold(tester).bottomNavigationBar, isNull);
      expect(find.byType(BottomActionBar), findsNothing);
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      // No bottom UI was invented for the loading state.
      expect(find.byType(ListView), findsNothing);
    });
  });

  group('SAFE-AREA-09A — geometry per state', () {
    testWidgets('party, inset 48: body ends at the bar; bar owns the inset '
        'exactly once', (tester) async {
      await _pumpOrderDetail(
        tester,
        authUserId: _buyerId,
        systemBottom: 48,
        order: _order(),
      );

      final Rect surface = tester.getRect(find.byType(Scaffold));
      final Rect bar = tester.getRect(find.byType(BottomActionBar));
      final Rect body = tester.getRect(find.byType(ListView));
      expect(surface.height, _surfaceHeight);

      // No body-side second reservation: body bottom == bar top.
      expect(
        body.bottom,
        closeTo(bar.top, 0.01),
        reason: 'the body carried a SECOND bottom reservation beside the bar',
      );
      // The bar reaches the screen bottom and clears the live inset once:
      // chrome p12 below the CTA, then the system inset (48).
      expect(bar.bottom, closeTo(surface.bottom, 0.01));
      final Rect cta = _bottomMostButtonRect(tester);
      expect(
        cta.bottom,
        closeTo(surface.bottom - 48 - AppMetrics.p12, 0.01),
        reason: 'the bar did not consume the 48px inset exactly once',
      );
    });

    testWidgets('no-bar, inset 24: body SafeArea consumes the live inset', (
      tester,
    ) async {
      await _pumpOrderDetail(
        tester,
        authUserId: 'intruder-1',
        systemBottom: 24,
        order: _order(),
      );

      final Rect surface = tester.getRect(find.byType(Scaffold));
      final Rect body = tester.getRect(find.byType(ListView));

      expect(_scaffold(tester).bottomNavigationBar, isNull);
      expect(
        body.bottom,
        closeTo(surface.bottom - 24, 0.01),
        reason:
            'with the slot null, the body SafeArea must clear the 24px inset',
      );
    });

    testWidgets('no-bar, inset 48: body SafeArea follows the changed inset', (
      tester,
    ) async {
      await _pumpOrderDetail(
        tester,
        authUserId: 'intruder-1',
        systemBottom: 48,
        order: _order(),
      );

      final Rect surface = tester.getRect(find.byType(Scaffold));
      final Rect body = tester.getRect(find.byType(ListView));

      expect(_scaffold(tester).bottomNavigationBar, isNull);
      expect(
        body.bottom,
        closeTo(surface.bottom - 48, 0.01),
        reason:
            'with the slot null, the body SafeArea must clear the 48px inset',
      );
    });

    testWidgets('no-bar, inset 0: no stale or fixed bottom clearance', (
      tester,
    ) async {
      await _pumpOrderDetail(
        tester,
        authUserId: 'intruder-1',
        systemBottom: 0,
        order: _order(),
      );

      final Rect surface = tester.getRect(find.byType(Scaffold));
      final Rect body = tester.getRect(find.byType(ListView));

      expect(_scaffold(tester).bottomNavigationBar, isNull);
      expect(
        body.bottom,
        closeTo(surface.bottom, 0.01),
        reason: 'a stale inset-sized gap survived the hidden system bar',
      );
    });

    testWidgets(
        'landscape-style left cutout: SafeArea still protects the side edge',
        (tester) async {
      await _pumpOrderDetail(
        tester,
        authUserId: 'intruder-1',
        systemBottom: 24,
        systemLeft: 40,
        order: _order(),
      );

      final Rect surface = tester.getRect(find.byType(Scaffold));
      final Rect body = tester.getRect(find.byType(ListView));

      expect(body.left, closeTo(surface.left + 40, 0.01));
      expect(body.bottom, closeTo(surface.bottom - 24, 0.01));
    });

    testWidgets('authority switch: bar state and no-bar state never overlap',
        (tester) async {
      // State 1 — party: bar is the ONLY bottom authority.
      await _pumpOrderDetail(
        tester,
        authUserId: _buyerId,
        systemBottom: 48,
        order: _order(),
      );
      expect(_scaffold(tester).bottomNavigationBar, isNotNull);
      expect(find.byType(BottomActionBar), findsOneWidget);
      final Rect partyBody = tester.getRect(find.byType(ListView));
      final Rect partyBar = tester.getRect(find.byType(BottomActionBar));
      expect(partyBody.bottom, closeTo(partyBar.top, 0.01));

      // State 2 — no bar: the slot goes null and the body SafeArea becomes
      // the sole authority. There is no frame with two active authorities.
      await _pumpOrderDetail(
        tester,
        authUserId: 'intruder-1',
        systemBottom: 48,
        order: _order(),
      );
      expect(_scaffold(tester).bottomNavigationBar, isNull);
      expect(find.byType(BottomActionBar), findsNothing);
      final Rect noBarBody = tester.getRect(find.byType(ListView));
      final Rect surface = tester.getRect(find.byType(Scaffold));
      expect(noBarBody.bottom, closeTo(surface.bottom - 48, 0.01));
    });
  });
}
