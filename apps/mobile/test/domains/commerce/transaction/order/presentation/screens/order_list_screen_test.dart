import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart' hide Action;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/common/result.dart';
import 'package:labuda/core/src/auth/app_role.dart';
import 'package:labuda/domains/commerce/transaction/order/order.dart'
    hide Action;
import 'package:labuda/domains/commerce/transaction/order/domain/entities/order.dart'
    show Action;
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/widgets/empty_state.dart';
import 'package:labuda/shared/widgets/loading_indicator.dart';
import 'package:labuda/shared/widgets/page_error_state.dart';

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

OrderItem _item({String forSaleName = 'Koi Kohaku'}) {
  return OrderItem(
    id: 'item-1',
    productId: 'product-1',
    forSaleName: forSaleName,
    forSaleImage: 'https://example.com/koi.jpg',
    price: 100000,
    quantity: 1,
  );
}

Order _order({
  required String id,
  required OrderStatus status,
  String itemName = 'Koi Kohaku',
}) {
  return Order(
    id: id,
    buyerId: _currentUserId,
    sellerId: 'seller-1',
    items: [_item(forSaleName: itemName)],
    status: status,
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
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
        home: OrderListScreen(isSeller: isSeller),
      ),
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
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            locale: const Locale('id'),
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

  group('OrderListScreen - seller empty state carries no create For Sale', () {
    Future<void> pumpEmpty(
      WidgetTester tester, {
      required bool isSeller,
    }) async {
      _useTallViewport(tester);
      const statuses = <OrderStatus?>[
        null,
        OrderStatus.pending,
        OrderStatus.paid,
        OrderStatus.shipped,
        OrderStatus.completed,
      ];

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authControllerProvider.overrideWith(_FakeAuthController.new),
            for (final status in statuses)
              if (isSeller)
                watchSellerOrdersProvider(
                  sellerId: _currentUserId,
                  status: status,
                ).overrideWith((ref) => Stream.value(const <Order>[]))
              else
                watchBuyerOrdersProvider(
                  buyerId: _currentUserId,
                  status: status,
                ).overrideWith((ref) => Stream.value(const <Order>[])),
          ],
          child: MaterialApp(
            localizationsDelegates: AppLocalizations.localizationsDelegates,
            supportedLocales: AppLocalizations.supportedLocales,
            locale: const Locale('id'),
            home: OrderListScreen(isSeller: isSeller),
          ),
        ),
      );
      await tester.pump();
      await tester.pump();
    }

    testWidgets('renders and offers no create For Sale action', (tester) async {
      await pumpEmpty(tester, isSeller: true);

      expect(find.text('Belum Ada Pesanan Masuk'), findsOneWidget);
      expect(
        find.text('Pesanan dari pembeli akan muncul di sini'),
        findsOneWidget,
      );
      expect(find.text('Tambah ForSale'), findsNothing);
      expect(find.byType(FilledButton), findsNothing);
    });

    testWidgets('keeps the legitimate buyer marketplace action', (
      tester,
    ) async {
      await pumpEmpty(tester, isSeller: false);

      expect(find.text('Belum Ada Pesanan'), findsOneWidget);
      expect(find.text('Jelajahi Marketplace'), findsOneWidget);
    });

    test('the screen source carries no create-for-sale entry point', () {
      final source = File(
        'lib/domains/commerce/transaction/order/presentation/screens/order_list_screen.dart',
      ).readAsStringSync();

      expect(source.contains('Tambah ForSale'), isFalse);
      expect(source.contains('RoutePaths.createForSale'), isFalse);
      expect(source.contains('_handleEmptyStateAction'), isFalse);
    });
  });

  group('OrderListScreen — canonical producer and initial state', () {
    testWidgets('first request shows LoadingIndicator, never empty/error', (
      tester,
    ) async {
      final gate = Completer<List<Order>>();
      final repository = _ScriptedOrderRepository(
        onWatchBuyer: (status, call) => status == null
            ? gate.future.asStream()
            : Stream.value(const <Order>[]),
      );
      await _pumpWithRepository(tester, repository: repository, settle: false);
      await tester.pump();
      await tester.pump();

      expect(find.byType(LoadingIndicator), findsOneWidget);
      expect(find.byType(EmptyState), findsNothing);
      expect(find.byType(PageErrorState), findsNothing);

      gate.complete([_order(id: 'order-aa01', status: OrderStatus.pending)]);
      await tester.pumpAndSettle();
      expect(find.text('Koi Kohaku'), findsOneWidget);
    });

    testWidgets('successful zero-result shows EmptyState', (tester) async {
      final repository = _ScriptedOrderRepository();
      await _pumpWithRepository(tester, repository: repository);

      expect(find.byType(EmptyState), findsOneWidget);
      expect(find.text('Belum Ada Pesanan'), findsOneWidget);
      expect(find.text('Jelajahi Marketplace'), findsOneWidget);
      expect(find.byType(PageErrorState), findsNothing);
      expect(find.byType(LoadingIndicator), findsNothing);
    });

    testWidgets('seller mode loads through the seller stream key', (
      tester,
    ) async {
      final repository = _ScriptedOrderRepository(
        onWatchSeller: (status, call) => Stream.value(
          status == null
              ? [_order(id: 'order-ss03', status: OrderStatus.paid)]
              : const <Order>[],
        ),
      );
      await _pumpWithRepository(tester, repository: repository, isSeller: true);

      expect(repository.watchSellerCalls['all'], 1);
      expect(find.text('Koi Kohaku'), findsOneWidget);
      expect(find.text('Incoming Orders'), findsOneWidget);
    });

    testWidgets('tabs share one stream authority without a pager', (
      tester,
    ) async {
      final repository = _ScriptedOrderRepository(
        onWatchBuyer: (status, call) => Stream.value(
          status == null
              ? [_order(id: 'order-aa01', status: OrderStatus.pending)]
              : const <Order>[],
        ),
      );
      await _pumpWithRepository(tester, repository: repository);

      // The visible All tab loaded through the canonical stream; every tab
      // subscribes its own status key exactly once.
      expect(repository.buyerCalls(null), 1);
      expect(find.text('Koi Kohaku'), findsOneWidget);
    });
  });

  group('OrderListScreen — initial failure and retry', () {
    testWidgets('failure with no data shows PageErrorState, retry reloads', (
      tester,
    ) async {
      final repository = _ScriptedOrderRepository(
        onWatchBuyer: (status, call) {
          if (status != null) return Stream.value(const <Order>[]);
          return call == 1
              ? Stream<List<Order>>.error(Exception('boom-initial'))
              : Stream.value([
                  _order(id: 'order-aa01', status: OrderStatus.pending),
                ]);
        },
      );
      await _pumpWithRepository(tester, repository: repository);

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
      expect(find.byType(EmptyState), findsNothing);

      // Retry resubscribes the canonical stream.
      expect(repository.buyerCalls(null), 1);
      await tester.tap(find.widgetWithText(ElevatedButton, 'Coba Lagi'));
      await tester.pumpAndSettle();

      expect(repository.buyerCalls(null), 2);
      expect(find.byType(PageErrorState), findsNothing);
      expect(find.text('Koi Kohaku'), findsOneWidget);
    });
  });

  group('OrderListScreen — existing data survives later failure', () {
    testWidgets('stream error after data keeps rows with inline banner', (
      tester,
    ) async {
      final repository = _ScriptedOrderRepository(
        onWatchBuyer: (status, call) {
          if (status != null) return Stream.value(const <Order>[]);
          return call == 1
              ? _dataThenError([
                  _order(id: 'order-aa01', status: OrderStatus.pending),
                ], Exception('boom-transient'))
              : Stream.value([
                  _order(
                    id: 'order-bb02',
                    status: OrderStatus.paid,
                    itemName: 'Koi Sanke',
                  ),
                ]);
        },
      );
      await _pumpWithRepository(tester, repository: repository);
      expect(find.text('Koi Kohaku'), findsOneWidget);

      // A later stream failure must stay distinguishable from empty: rows
      // persist with an inline banner, never a full-page error or a silent
      // slide into EmptyState.
      expect(find.byType(PageErrorState), findsNothing);
      expect(find.byType(EmptyState), findsNothing);
      expect(
        find.text('Data belum bisa dimuat. Silakan coba lagi.'),
        findsOneWidget,
      );
      expect(find.widgetWithText(TextButton, 'Coba Lagi'), findsOneWidget);
      expect(find.textContaining('boom-transient'), findsNothing);

      // Banner retry resubscribes and recovers with fresh data.
      await tester.tap(find.widgetWithText(TextButton, 'Coba Lagi'));
      await tester.pumpAndSettle();

      expect(repository.buyerCalls(null), 2);
      expect(find.text('Koi Sanke'), findsOneWidget);
      expect(
        find.text('Data belum bisa dimuat. Silakan coba lagi.'),
        findsNothing,
      );
    });
  });

  group('OrderListScreen — pull-to-refresh resubscribes', () {
    testWidgets('existing orders stay visible with refresh indicator', (
      tester,
    ) async {
      final gate = Completer<List<Order>>();
      final repository = _ScriptedOrderRepository(
        onWatchBuyer: (status, call) {
          if (status != null) return Stream.value(const <Order>[]);
          return call == 1
              ? Stream.value([
                  _order(id: 'order-aa01', status: OrderStatus.pending),
                ])
              : gate.future.asStream();
        },
      );
      await _pumpWithRepository(
        tester,
        repository: repository,
        tallViewport: false,
      );
      expect(find.text('Koi Kohaku'), findsOneWidget);

      // Fling from the visible row so the gesture lands in the active
      // tab's scrollable (five tab scrollables share the tree).
      await tester.fling(find.text('Koi Kohaku'), const Offset(0, 400), 1000);
      // Allow the RefreshIndicator to fire onRefresh and the resubscribe to
      // start (bounded: the gate stays open, so never settle here).
      for (var i = 0; i < 50 && repository.buyerCalls(null) < 2; i++) {
        await tester.pump(const Duration(milliseconds: 50));
      }
      expect(repository.buyerCalls(null), 2);

      // Refresh must not clear the list into full loading.
      expect(find.text('Koi Kohaku'), findsOneWidget);
      expect(find.byType(LinearProgressIndicator), findsOneWidget);
      expect(find.byType(LoadingIndicator), findsNothing);
      expect(find.byType(PageErrorState), findsNothing);

      gate.complete([
        _order(
          id: 'order-bb02',
          status: OrderStatus.paid,
          itemName: 'Koi Sanke',
        ),
      ]);
      await tester.pumpAndSettle();

      expect(find.text('Koi Sanke'), findsOneWidget);
      expect(find.byType(LinearProgressIndicator), findsNothing);
    });

    testWidgets('refresh failure keeps rows with inline banner', (
      tester,
    ) async {
      final repository = _ScriptedOrderRepository(
        onWatchBuyer: (status, call) {
          if (status != null) return Stream.value(const <Order>[]);
          if (call == 1) {
            return Stream.value([
              _order(id: 'order-aa01', status: OrderStatus.pending),
            ]);
          }
          if (call == 2) {
            return Stream<List<Order>>.error(Exception('boom-refresh'));
          }
          return Stream.value([
            _order(
              id: 'order-bb02',
              status: OrderStatus.paid,
              itemName: 'Koi Sanke',
            ),
          ]);
        },
      );
      await _pumpWithRepository(
        tester,
        repository: repository,
        tallViewport: false,
      );
      expect(find.text('Koi Kohaku'), findsOneWidget);

      await tester.fling(find.text('Koi Kohaku'), const Offset(0, 400), 1000);
      await tester.pumpAndSettle();

      expect(find.text('Koi Kohaku'), findsOneWidget);
      expect(find.byType(PageErrorState), findsNothing);
      expect(
        find.text('Data belum bisa dimuat. Silakan coba lagi.'),
        findsOneWidget,
      );
      expect(find.widgetWithText(TextButton, 'Coba Lagi'), findsOneWidget);
      expect(find.textContaining('boom-refresh'), findsNothing);

      await tester.tap(find.widgetWithText(TextButton, 'Coba Lagi'));
      await tester.pumpAndSettle();

      expect(repository.buyerCalls(null), 3);
      expect(find.text('Koi Sanke'), findsOneWidget);
      expect(
        find.text('Data belum bisa dimuat. Silakan coba lagi.'),
        findsNothing,
      );
    });
  });

  group('OrderListScreen — negative proof (static contract)', () {
    String screenSource() => File(
      'lib/domains/commerce/transaction/order/presentation/screens/order_list_screen.dart',
    ).readAsStringSync().replaceAll('\r\n', '\n');

    test('canonical renderers own every page state', () {
      final src = screenSource();
      expect(src.contains('LoadingIndicator('), isTrue);
      expect(src.contains('PageErrorState('), isTrue);
      expect(src.contains('EmptyState('), isTrue);
    });

    test('no raw spinner or custom static error remains', () {
      final src = screenSource();
      expect(src.contains('CircularProgressIndicator('), isFalse);
      expect(src.contains("const Text('Data belum bisa dimuat.')"), isFalse);
    });

    test('no raw technical error reaches the widget tree', () {
      final src = screenSource();
      expect(src.contains('error.toString()'), isFalse);
      expect(src.contains('Text(error'), isFalse);
      expect(src.contains('state.error!'), isFalse);
    });

    test('single reload path: resubscribe, never a second authority', () {
      final src = screenSource();
      expect('ref.refresh('.allMatches(src).length, 1);
      expect(src.contains('orderListPagerProvider'), isFalse);
      expect(src.contains('OrderListPager'), isFalse);
      expect(src.contains('NotifierProvider'), isFalse);
    });

    test('obsolete pager and its exclusive plumbing are gone', () {
      expect(
        File(
          'lib/domains/commerce/transaction/order/presentation/providers/order_list_controller.dart',
        ).existsSync(),
        isFalse,
      );
      expect(
        File(
          'lib/domains/commerce/transaction/order/domain/entities/order_page_result.dart',
        ).existsSync(),
        isFalse,
      );
      final repository = File(
        'lib/domains/commerce/transaction/order/domain/repositories/order_repository.dart',
      ).readAsStringSync();
      expect(repository.contains('getBuyerOrdersPage'), isFalse);
      expect(repository.contains('getSellerOrdersPage'), isFalse);
      expect(repository.contains('OrderPageResult'), isFalse);
      final domain = File(
        'lib/domains/commerce/transaction/order/domain/domain.dart',
      ).readAsStringSync();
      expect(domain.contains('order_page_result'), isFalse);
      final impl = File(
        'lib/domains/commerce/transaction/order/data/order_repository_impl.dart',
      ).readAsStringSync();
      expect(impl.contains('getBuyerOrdersPage'), isFalse);
      expect(impl.contains('getSellerOrdersPage'), isFalse);
      expect(impl.contains('OrderPageResult'), isFalse);
    });
  });
}

/// Scripted repository: the screen runs the REAL watch stream providers,
/// so every subscription (initial per-tab watch, resubscribe retry,
/// pull-to-refresh) is observable per status key at the canonical
/// producer boundary.
class _ScriptedOrderRepository implements OrderRepository {
  _ScriptedOrderRepository({this.onWatchBuyer, this.onWatchSeller});

  Stream<List<Order>> Function(OrderStatus? status, int call)? onWatchBuyer;
  Stream<List<Order>> Function(OrderStatus? status, int call)? onWatchSeller;

  final Map<String, int> watchBuyerCalls = {};
  final Map<String, int> watchSellerCalls = {};

  static String _key(OrderStatus? status) => status?.name ?? 'all';

  int buyerCalls(OrderStatus? status) => watchBuyerCalls[_key(status)] ?? 0;

  @override
  Stream<List<Order>> watchBuyerOrders(WatchOrdersParams params) {
    final key = _key(params.status);
    final call = (watchBuyerCalls[key] ?? 0) + 1;
    watchBuyerCalls[key] = call;
    final handler = onWatchBuyer;
    if (handler != null) return handler(params.status, call);
    return Stream.value(const <Order>[]);
  }

  @override
  Stream<List<Order>> watchSellerOrders(WatchOrdersParams params) {
    final key = _key(params.status);
    final call = (watchSellerCalls[key] ?? 0) + 1;
    watchSellerCalls[key] = call;
    final handler = onWatchSeller;
    if (handler != null) return handler(params.status, call);
    return Stream.value(const <Order>[]);
  }

  @override
  Future<Result<PreviewOrderResult>> previewOrder(PreviewOrderParams params) =>
      throw UnimplementedError();

  @override
  Future<Result<Order>> getOrderById(String orderId) =>
      throw UnimplementedError();

  @override
  Future<Result<List<Order>>> getBuyerOrders(GetOrdersParams params) =>
      throw UnimplementedError();

  @override
  Future<Result<List<Order>>> getSellerOrders(GetOrdersParams params) =>
      throw UnimplementedError();

  @override
  Future<Result<Order>> cancelOrder(String orderId, CancelOrderParams params) =>
      throw UnimplementedError();

  @override
  Future<Result<Order>> markAsShipped(MarkAsShippedParams params) =>
      throw UnimplementedError();

  @override
  Future<Result<Order>> markAsDelivered(String orderId) =>
      throw UnimplementedError();

  @override
  Future<Result<void>> extendOrderConfirmation(String orderId) =>
      throw UnimplementedError();

  @override
  Stream<Order> watchOrder(String orderId) => throw UnimplementedError();

  @override
  Stream<List<Order>> watchSellerNewOrders(String sellerId) =>
      throw UnimplementedError();
}

Future<void> _pumpWithRepository(
  WidgetTester tester, {
  required _ScriptedOrderRepository repository,
  bool isSeller = false,
  bool settle = true,
  bool tallViewport = true,
}) async {
  if (tallViewport) _useTallViewport(tester);
  await tester.pumpWidget(
    ProviderScope(
      // Retry is disabled so a failed stream does not schedule timers.
      retry: (retryCount, error) => null,
      overrides: [
        authControllerProvider.overrideWith(_FakeAuthController.new),
        orderRepositoryProvider.overrideWithValue(repository),
      ],
      child: MaterialApp(
        localizationsDelegates: AppLocalizations.localizationsDelegates,
        supportedLocales: AppLocalizations.supportedLocales,
        locale: const Locale('id'),
        home: OrderListScreen(isSeller: isSeller),
      ),
    ),
  );
  if (settle) await tester.pumpAndSettle();
}

/// Single-shot stream emitting [data] and then failing. Models a live
/// stream whose later emission fails after valid data arrived. The
/// stream closes after the error, so no open subscription can outlive
/// the test.
Stream<List<Order>> _dataThenError(List<Order> data, Object error) async* {
  yield data;
  throw error;
}

class _RecordingNavigatorObserver extends NavigatorObserver {
  int pushCount = 0;

  @override
  void didPush(Route route, Route? previousRoute) {
    pushCount++;
  }
}
