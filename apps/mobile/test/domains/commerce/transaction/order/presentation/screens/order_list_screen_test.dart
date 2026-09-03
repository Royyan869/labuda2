import 'package:flutter/material.dart' hide Action;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/src/auth/app_role.dart';
import 'package:labuda/domains/commerce/transaction/order/order.dart'
    hide Action;
import 'package:labuda/domains/commerce/transaction/order/domain/entities/order.dart'
    show Action;
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

/// Locks the current V1 invariant: the order list is detail-navigation only.
/// No backend-driven or local quick-action button (pay/ship/confirm) may
/// ever appear on a list card, for either role or any status - see the
/// Order Decision Display Hints audit (2026-07-03), finding D.7.
const _currentUserId = 'user-list-1';

class _FakeAuthController extends AuthController {
  @override
  AuthState build() {
    final now = DateTime.parse('2026-07-01T00:00:00.000Z');
    final user = AuthUser(
      id: _currentUserId,
      createdAt: now,
      updatedAt: now,
      email: 'buyer@example.com',
      username: 'buyer1',
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

OrderItem _item() {
  return const OrderItem(
    id: 'item-1',
    productId: 'product-1',
    listingName: 'Koi Kohaku',
    listingImage: 'https://example.com/koi.jpg',
    price: 100000,
    quantity: 1,
  );
}

Order _order({required String id, required OrderStatus status}) {
  return Order(
    id: id,
    buyerId: _currentUserId,
    sellerId: 'seller-1',
    items: [_item()],
    status: status,
    paymentMethod: PaymentMethodType.bankTransfer,
    paymentStatus: PaymentStatus.pending,
    shippingInfo: const ShippingInfo(
      recipientName: 'Buyer',
      phone: '08123456789',
      address: 'Some address',
      method: ShippingMethod.courier,
      shippingCost: 10000,
    ),
    pricing: const OrderPricing(
      subtotal: 100000,
      shippingCost: 10000,
      discount: 0,
      total: 110000,
    ),
    createdAt: DateTime.utc(2026, 6, 1),
    source: OrderSource.forSale,
    // A DecisionContract with a live "pay" primary action is attached to
    // prove the invariant even when the backend *does* send an actionable
    // decision - the list must still never render it.
    decision: status == OrderStatus.pending
        ? const DecisionContract(
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
          )
        : null,
  );
}

/// The 4 statuses required by the audit: pending, paid, shipped, and one
/// terminal status (completed).
List<Order> _ordersAcrossStatuses() => [
  _order(id: 'order-pending', status: OrderStatus.pending),
  _order(id: 'order-paid', status: OrderStatus.paid),
  _order(id: 'order-shipped', status: OrderStatus.shipped),
  _order(id: 'order-completed', status: OrderStatus.completed),
];

/// Enlarges the test viewport so all 4 stacked cards fit without needing to
/// scroll the ListView - keeps the assertions below simple presence checks
/// instead of scroll-dependent finders.
void _useTallViewport(WidgetTester tester) {
  tester.view.physicalSize = const Size(1080, 4000);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
}

Future<void> _pumpOrderList(
  WidgetTester tester, {
  required bool isSeller,
}) async {
  _useTallViewport(tester);
  final orders = _ordersAcrossStatuses();

  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        authControllerProvider.overrideWith(_FakeAuthController.new),
        if (isSeller)
          watchSellerOrdersProvider(
            sellerId: _currentUserId,
            status: null,
          ).overrideWith((ref) => Stream.value(orders))
        else
          watchBuyerOrdersProvider(
            buyerId: _currentUserId,
            status: null,
          ).overrideWith((ref) => Stream.value(orders)),
      ],
      child: MaterialApp(home: OrderListScreen(isSeller: isSeller)),
    ),
  );
  // Settle the initial StreamProvider emission without driving a full
  // Navigator transition (kept intentionally shallow - this is a smoke
  // test of the list, not of OrderDetailScreen's own dependencies).
  await tester.pump();
  await tester.pump();
}

/// The full set of quick-action labels that must never appear on the list,
/// covering both the payment CTA states (Phase 2B-1) and seller shipment
/// actions.
const _quickActionLabels = [
  'Bayar Sekarang',
  'Lanjutkan Pembayaran',
  'Cek Status Pembayaran',
  'Bayar Ulang',
  'Kirim Pesanan',
  'Terima Barang',
];

void main() {
  group('OrderListScreen - detail-only invariant (buyer)', () {
    testWidgets(
      'renders order summary/status for pending/paid/shipped/completed',
      (tester) async {
        await _pumpOrderList(tester, isSeller: false);

        expect(find.text('Koi Kohaku'), findsNWidgets(4));
        expect(find.text('View Details'), findsNWidgets(4));
      },
    );

    testWidgets('no quick action button appears for any status', (
      tester,
    ) async {
      await _pumpOrderList(tester, isSeller: false);

      for (final label in _quickActionLabels) {
        expect(
          find.text(label),
          findsNothing,
          reason: '"$label" must not appear on the order list',
        );
      }
    });

    testWidgets('tapping the card triggers navigation to detail', (
      tester,
    ) async {
      await _pumpOrderList(tester, isSeller: false);

      final observer = _RecordingNavigatorObserver();
      // Re-pump with an observer attached to detect the push without
      // requiring OrderDetailScreen's own provider graph to be mocked.
      final orders = _ordersAcrossStatuses();
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authControllerProvider.overrideWith(_FakeAuthController.new),
            watchBuyerOrdersProvider(
              buyerId: _currentUserId,
              status: null,
            ).overrideWith((ref) => Stream.value(orders)),
          ],
          child: MaterialApp(
            navigatorObservers: [observer],
            home: const OrderListScreen(isSeller: false),
          ),
        ),
      );
      await tester.pump();
      await tester.pump();

      await tester.tap(find.text('View Details').first);
      await tester.pump();
      // The destination (OrderDetailScreen) may itself error without its
      // own provider graph mocked - that is out of scope for this list-only
      // smoke test, so any such exception is acknowledged and discarded.
      tester.takeException();

      expect(observer.pushCount, greaterThan(0));
    });
  });

  group('OrderListScreen - detail-only invariant (seller)', () {
    testWidgets(
      'renders order summary/status for pending/paid/shipped/completed',
      (tester) async {
        await _pumpOrderList(tester, isSeller: true);

        expect(find.text('Koi Kohaku'), findsNWidgets(4));
        expect(find.text('View Details'), findsNWidgets(4));
      },
    );

    testWidgets('no quick action button appears for any status', (
      tester,
    ) async {
      await _pumpOrderList(tester, isSeller: true);

      for (final label in _quickActionLabels) {
        expect(
          find.text(label),
          findsNothing,
          reason: '"$label" must not appear on the seller order list',
        );
      }
    });
  });
}

class _RecordingNavigatorObserver extends NavigatorObserver {
  int pushCount = 0;

  @override
  void didPush(Route route, Route? previousRoute) {
    pushCount++;
  }
}
