import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/config/seller_upgrade_config_entity.dart';
import 'package:labuda/core/config/seller_upgrade_config_provider.dart';
import 'package:labuda/domains/commerce/transaction/order/order.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart'
    show shippingNotifierProvider;
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/shipping_notifier.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/shipping_state.dart';
import 'package:labuda/domains/user/identity/verification/verification.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_earnings.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_subscription.dart';
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_dashboard_screen.dart';
import 'package:labuda/domains/user/preference/seller/seller_di.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/address_list_provider.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/models/wilayah_models.dart';
import 'package:labuda/shared/providers/authenticated_account_provider.dart';

const _sellerId = 'seller-queue-001';

class _StaticAuthController extends AuthController {
  _StaticAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _StaticSellerVerificationNotifier extends SellerVerificationV2Notifier {
  _StaticSellerVerificationNotifier(this._state);

  final SellerVerificationV2State _state;

  @override
  SellerVerificationV2State build() => _state;

  @override
  Future<void> loadStatus() async {}
}

class _StaticShippingNotifier extends ShippingNotifier {
  _StaticShippingNotifier(this._state);

  final ShippingOptionsListState _state;

  @override
  ShippingOptionsListState build() => _state;

  @override
  Future<void> loadActiveShippingOptions() async {}
}

AuthUser _sellerUser({
  required bool hasMarketAuthority,
  required String sellerSubscriptionStatus,
}) {
  final now = DateTime.now();
  return AuthUser(
    id: _sellerId,
    createdAt: now,
    updatedAt: now,
    email: 'seller@example.com',
    username: 'seller-queue',
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: ShonaAuthProvider.email,
    hasSellerProfile: true,
    sellerSubscriptionStatus: sellerSubscriptionStatus,
    hasMarketAuthority: hasMarketAuthority,
  );
}

OrderItem _orderItem({required String id, required String label}) {
  return OrderItem(
    id: id,
    productId: 'product-$id',
    listingName: label,
    listingImage: 'https://example.com/$id.jpg',
    price: 100000,
  );
}

Order _order({
  required String id,
  required OrderStatus status,
}) {
  final now = DateTime.now();
  return Order(
    id: id,
    buyerId: 'buyer-001',
    sellerId: _sellerId,
    items: [_orderItem(id: id, label: 'Order $id')],
    status: status,
    paymentMethod: PaymentMethodType.bankTransfer,
    paymentStatus: PaymentStatus.pending,
    shippingInfo: const ShippingInfo(
      recipientName: 'Buyer',
      phone: '08123456789',
      address: 'Jl. Contoh 1',
      method: ShippingMethod.courier,
      shippingCost: 10000,
    ),
    pricing: const OrderPricing(
      subtotal: 100000,
      shippingCost: 10000,
      discount: 0,
      total: 110000,
    ),
    createdAt: now,
    source: OrderSource.fixedPriceSale,
  );
}

AddressEntity _senderAddress() {
  final now = DateTime.now();
  return AddressEntity(
    id: 'address-sender-1',
    userId: _sellerId,
    purpose: AddressPurpose.sender,
    recipientName: 'Farm Sentosa',
    phone: '08123456789',
    province: const Province(id: '33', name: 'Jawa Tengah'),
    city: const City(id: '3301', name: 'Kabupaten Demak', provinceId: '33'),
    district: const District(id: '330101', name: 'Mranggen', cityId: '3301'),
    village: const Village(
      id: '3301012001',
      name: 'Rowosari',
      districtId: '330101',
    ),
    streetAddress: 'Jl. Melati No. 12',
    postalCode: '59511',
    isPrimary: true,
    createdAt: now,
    updatedAt: now,
  );
}

ShippingOption _activeShippingOption() {
  final now = DateTime.now();
  return ShippingOption(
    id: 'shipping-1',
    name: 'Bus Kencana',
    type: ShippingType.bus,
    coverageAreas: const [],
    isActive: true,
    createdAt: now,
    updatedAt: now,
  );
}

SellerSubscription _subscription({
  required Duration expiresIn,
}) {
  final now = DateTime.now();
  return SellerSubscription(
    isActive: true,
    yearlyFee: 70000,
    startDate: now.subtract(const Duration(days: 30)),
    expiryDate: now.add(expiresIn),
    status: SubscriptionStatus.active,
    paymentId: 'pay-001',
    createdAt: now.subtract(const Duration(days: 30)),
  );
}

SellerUpgradeConfigEntity _upgradeConfig({
  int renewalReminderDays = 7,
}) {
  return SellerUpgradeConfigEntity(
    yearlyFee: 70000,
    durationDays: 365,
    isEnabled: true,
    renewalReminderDays: renewalReminderDays,
  );
}

SellerEarnings _earnings() {
  final now = DateTime.now();
  return SellerEarnings(
    sellerId: _sellerId,
    totalRevenue: 890000,
    pendingRevenue: 0,
    totalPlatformFees: 0,
    availableBalance: 125000,
    withdrawalFeeAmount: 5000,
    totalWithdrawn: 420000,
    totalWithdrawals: 2,
    totalCompletedOrders: 0,
    calculatedAt: now,
    grossPayable: 130000,
    activeDisputeFreeze: 5000,
  );
}

dynamic _dashboardOverrides({
  required AuthUser user,
  required List<Order> pendingOrders,
  required List<Order> paidOrders,
  required SellerVerificationV2State verificationState,
  required Result<AddressEntity?> senderAddressResult,
  required ShippingOptionsListState shippingState,
  required SellerSubscription subscription,
  required SellerUpgradeConfigEntity upgradeConfig,
}) {
  return [
    authControllerProvider.overrideWith(
      () => _StaticAuthController(
        AuthState.authenticated(user, emailVerified: true),
      ),
    ),
    authenticatedUserProvider.overrideWith((ref) => user),
    sellerVerificationV2NotifierProvider.overrideWith(
      () => _StaticSellerVerificationNotifier(verificationState),
    ),
    shippingNotifierProvider.overrideWith(
      () => _StaticShippingNotifier(shippingState),
    ),
    sellerEarningsProvider(_sellerId).overrideWith(
      (ref) async => _earnings(),
    ),
    watchSellerOrdersProvider(
      sellerId: _sellerId,
      status: OrderStatus.pending,
    ).overrideWith((ref) => Stream.value(pendingOrders)),
    watchSellerOrdersProvider(
      sellerId: _sellerId,
      status: OrderStatus.paid,
    ).overrideWith((ref) => Stream.value(paidOrders)),
    primarySenderAddressProvider(_sellerId).overrideWith(
      (ref) async => senderAddressResult,
    ),
    sellerSubscriptionFutureProvider(_sellerId).overrideWith(
      (ref) async => subscription,
    ),
    sellerUpgradeConfigProvider.overrideWith(
      (ref) async => upgradeConfig,
    ),
  ];
}

GoRouter _router() {
  return GoRouter(
    initialLocation: RoutePaths.sellerDashboard,
    routes: [
      GoRoute(
        path: RoutePaths.sellerDashboard,
        builder: (context, state) => const SellerDashboardScreen(),
      ),
      GoRoute(
        path: '/seller/orders',
        builder: (context, state) => Scaffold(
          body: Center(child: Text(state.uri.toString())),
        ),
      ),
      GoRoute(
        path: RoutePaths.addresses,
        builder: (context, state) => Scaffold(
          body: Center(child: Text(state.uri.toString())),
        ),
      ),
      GoRoute(
        path: RoutePaths.sellerShipping,
        builder: (context, state) => Scaffold(
          body: Center(child: Text(state.uri.toString())),
        ),
      ),
      GoRoute(
        path: RoutePaths.sellerListings,
        builder: (context, state) => Scaffold(
          body: Center(child: Text(state.uri.toString())),
        ),
      ),
      GoRoute(
        path: RoutePaths.sellerAuctions,
        builder: (context, state) => Scaffold(
          body: Center(child: Text(state.uri.toString())),
        ),
      ),
      GoRoute(
        path: RoutePaths.sellerVerification,
        builder: (context, state) => Scaffold(
          body: Center(child: Text(state.uri.toString())),
        ),
      ),
      GoRoute(
        path: RoutePaths.sellerUpgrade,
        builder: (context, state) => Scaffold(
          body: Center(child: Text(state.uri.toString())),
        ),
      ),
    ],
  );
}

Widget _buildApp(dynamic overrides) {
  return ProviderScope(
    overrides: overrides,
    child: MaterialApp.router(
      routerConfig: _router(),
      theme: ThemeData(useMaterial3: true),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: const Locale('id'),
    ),
  );
}

Future<void> _pumpDashboard(
  WidgetTester tester, {
  required dynamic overrides,
}) async {
  await tester.pumpWidget(_buildApp(overrides));
  await tester.pumpAndSettle();
}

Future<void> _showActionQueue(
  WidgetTester tester, {
  required Finder anchor,
}) async {
  await tester.dragUntilVisible(
    anchor,
    find.byType(Scrollable),
    const Offset(0, -300),
  );
  await tester.pumpAndSettle();
}

Future<void> _tapAndExpectRoute(
  WidgetTester tester, {
  required dynamic overrides,
  required Finder anchor,
  required Key key,
  required String expectedLocation,
}) async {
  await _pumpDashboard(tester, overrides: overrides);
  await _showActionQueue(tester, anchor: anchor);
  await tester.tap(find.byKey(key));
  await tester.pumpAndSettle();

  expect(find.text(expectedLocation), findsOneWidget);
}

Future<void> _tapDashboardQuickActionAndExpectRoute(
  WidgetTester tester, {
  required dynamic overrides,
  required Finder target,
  required String expectedLocation,
}) async {
  await _pumpDashboard(tester, overrides: overrides);
  await tester.dragUntilVisible(
    target,
    find.byType(Scrollable),
    const Offset(0, -300),
  );
  await tester.pumpAndSettle();
  await tester.tap(target);
  await tester.pumpAndSettle();

  expect(find.text(expectedLocation), findsOneWidget);
}

void main() {
  group('SellerDashboardScreen operational action queue', () {
    testWidgets('renders every actionable seller task in one queue', (
      tester,
    ) async {
      await _pumpDashboard(
        tester,
        overrides: _dashboardOverrides(
          user: _sellerUser(
            hasMarketAuthority: false,
            sellerSubscriptionStatus: 'expired',
          ),
          pendingOrders: [_order(id: 'pending-1', status: OrderStatus.pending)],
          paidOrders: [_order(id: 'paid-1', status: OrderStatus.paid)],
          verificationState: const SellerVerificationV2State(
            status: SellerVerificationStatus.needsResubmission,
          ),
          senderAddressResult: Result.success(null),
          shippingState: const ShippingOptionsListLoaded([]),
          subscription: _subscription(expiresIn: const Duration(days: -1)),
          upgradeConfig: _upgradeConfig(),
        ),
      );

      await _showActionQueue(
        tester,
        anchor: find.byKey(const Key('seller-action-queue-pending-orders')),
      );

      expect(find.byKey(const Key('seller-action-queue-pending-orders')), findsOneWidget);
      expect(find.byKey(const Key('seller-action-queue-paid-orders')), findsOneWidget);
      expect(find.byKey(const Key('seller-action-queue-verification')), findsOneWidget);
      expect(find.byKey(const Key('seller-action-queue-sender-address')), findsOneWidget);
      expect(find.byKey(const Key('seller-action-queue-shipping-option')), findsOneWidget);
      expect(find.byKey(const Key('seller-action-queue-subscription-expired')), findsOneWidget);
      expect(find.text('Antrian Tindakan Operasional'), findsOneWidget);
    });

    testWidgets('routes the order queue CTA to /seller/orders', (tester) async {
      await _pumpDashboard(
        tester,
        overrides: _dashboardOverrides(
          user: _sellerUser(
            hasMarketAuthority: false,
            sellerSubscriptionStatus: 'expired',
          ),
          pendingOrders: [_order(id: 'pending-1', status: OrderStatus.pending)],
          paidOrders: [_order(id: 'paid-1', status: OrderStatus.paid)],
          verificationState: const SellerVerificationV2State(
            status: SellerVerificationStatus.needsResubmission,
          ),
          senderAddressResult: Result.success(null),
          shippingState: const ShippingOptionsListLoaded([]),
          subscription: _subscription(expiresIn: const Duration(days: -1)),
          upgradeConfig: _upgradeConfig(),
        ),
      );

      await _tapAndExpectRoute(
        tester,
        overrides: _dashboardOverrides(
          user: _sellerUser(
            hasMarketAuthority: false,
            sellerSubscriptionStatus: 'expired',
          ),
          pendingOrders: [_order(id: 'pending-1', status: OrderStatus.pending)],
          paidOrders: [_order(id: 'paid-1', status: OrderStatus.paid)],
          verificationState: const SellerVerificationV2State(
            status: SellerVerificationStatus.needsResubmission,
          ),
          senderAddressResult: Result.success(null),
          shippingState: const ShippingOptionsListLoaded([]),
          subscription: _subscription(expiresIn: const Duration(days: -1)),
          upgradeConfig: _upgradeConfig(),
        ),
        anchor: find.byKey(const Key('seller-action-queue-pending-orders')),
        key: const Key('seller-action-queue-pending-orders'),
        expectedLocation: '/seller/orders',
      );
    });

    testWidgets('routes the non-order queue CTAs to canonical pages', (
      tester,
    ) async {
      await _pumpDashboard(
        tester,
        overrides: _dashboardOverrides(
          user: _sellerUser(
            hasMarketAuthority: false,
            sellerSubscriptionStatus: 'expired',
          ),
          pendingOrders: [_order(id: 'pending-1', status: OrderStatus.pending)],
          paidOrders: [_order(id: 'paid-1', status: OrderStatus.paid)],
          verificationState: const SellerVerificationV2State(
            status: SellerVerificationStatus.needsResubmission,
          ),
          senderAddressResult: Result.success(null),
          shippingState: const ShippingOptionsListLoaded([]),
          subscription: _subscription(expiresIn: const Duration(days: -1)),
          upgradeConfig: _upgradeConfig(),
        ),
      );

      await _tapAndExpectRoute(
        tester,
        overrides: _dashboardOverrides(
          user: _sellerUser(
            hasMarketAuthority: false,
            sellerSubscriptionStatus: 'expired',
          ),
          pendingOrders: [_order(id: 'pending-1', status: OrderStatus.pending)],
          paidOrders: [_order(id: 'paid-1', status: OrderStatus.paid)],
          verificationState: const SellerVerificationV2State(
            status: SellerVerificationStatus.needsResubmission,
          ),
          senderAddressResult: Result.success(null),
          shippingState: const ShippingOptionsListLoaded([]),
          subscription: _subscription(expiresIn: const Duration(days: -1)),
          upgradeConfig: _upgradeConfig(),
        ),
        anchor: find.byKey(const Key('seller-action-queue-sender-address')),
        key: const Key('seller-action-queue-sender-address'),
        expectedLocation: '/profile/addresses?initialTab=sender',
      );
      await _tapAndExpectRoute(
        tester,
        overrides: _dashboardOverrides(
          user: _sellerUser(
            hasMarketAuthority: false,
            sellerSubscriptionStatus: 'expired',
          ),
          pendingOrders: [_order(id: 'pending-1', status: OrderStatus.pending)],
          paidOrders: [_order(id: 'paid-1', status: OrderStatus.paid)],
          verificationState: const SellerVerificationV2State(
            status: SellerVerificationStatus.needsResubmission,
          ),
          senderAddressResult: Result.success(null),
          shippingState: const ShippingOptionsListLoaded([]),
          subscription: _subscription(expiresIn: const Duration(days: -1)),
          upgradeConfig: _upgradeConfig(),
        ),
        anchor: find.byKey(const Key('seller-action-queue-shipping-option')),
        key: const Key('seller-action-queue-shipping-option'),
        expectedLocation: '/seller/shipping',
      );
      await _tapAndExpectRoute(
        tester,
        overrides: _dashboardOverrides(
          user: _sellerUser(
            hasMarketAuthority: false,
            sellerSubscriptionStatus: 'expired',
          ),
          pendingOrders: [_order(id: 'pending-1', status: OrderStatus.pending)],
          paidOrders: [_order(id: 'paid-1', status: OrderStatus.paid)],
          verificationState: const SellerVerificationV2State(
            status: SellerVerificationStatus.needsResubmission,
          ),
          senderAddressResult: Result.success(null),
          shippingState: const ShippingOptionsListLoaded([]),
          subscription: _subscription(expiresIn: const Duration(days: -1)),
          upgradeConfig: _upgradeConfig(),
        ),
        anchor: find.byKey(const Key('seller-action-queue-verification')),
        key: const Key('seller-action-queue-verification'),
        expectedLocation: '/verification/seller',
      );
      await _tapAndExpectRoute(
        tester,
        overrides: _dashboardOverrides(
          user: _sellerUser(
            hasMarketAuthority: false,
            sellerSubscriptionStatus: 'expired',
          ),
          pendingOrders: [_order(id: 'pending-1', status: OrderStatus.pending)],
          paidOrders: [_order(id: 'paid-1', status: OrderStatus.paid)],
          verificationState: const SellerVerificationV2State(
            status: SellerVerificationStatus.needsResubmission,
          ),
          senderAddressResult: Result.success(null),
          shippingState: const ShippingOptionsListLoaded([]),
          subscription: _subscription(expiresIn: const Duration(days: -1)),
          upgradeConfig: _upgradeConfig(),
        ),
        anchor: find.byKey(const Key('seller-action-queue-subscription-expired')),
        key: const Key('seller-action-queue-subscription-expired'),
        expectedLocation: '/seller/upgrade',
      );
    });

    testWidgets('routes Listing Saya and Lelang Saya to canonical seller inventory pages', (
      tester,
    ) async {
      final overrides = _dashboardOverrides(
        user: _sellerUser(
          hasMarketAuthority: false,
          sellerSubscriptionStatus: 'expired',
        ),
        pendingOrders: [_order(id: 'pending-1', status: OrderStatus.pending)],
        paidOrders: [_order(id: 'paid-1', status: OrderStatus.paid)],
        verificationState: const SellerVerificationV2State(
          status: SellerVerificationStatus.needsResubmission,
        ),
        senderAddressResult: Result.success(null),
        shippingState: const ShippingOptionsListLoaded([]),
        subscription: _subscription(expiresIn: const Duration(days: -1)),
        upgradeConfig: _upgradeConfig(),
      );

      await _tapDashboardQuickActionAndExpectRoute(
        tester,
        overrides: overrides,
        target: find.text('Listing Saya'),
        expectedLocation: '/seller/listings',
      );

      await _tapDashboardQuickActionAndExpectRoute(
        tester,
        overrides: overrides,
        target: find.byKey(const Key('seller-quick-action-auctions')),
        expectedLocation: '/seller/auctions',
      );
    });

    testWidgets('shows the expiring soon warning inside the reminder window', (
      tester,
    ) async {
      await _pumpDashboard(
        tester,
        overrides: _dashboardOverrides(
          user: _sellerUser(
            hasMarketAuthority: true,
            sellerSubscriptionStatus: 'active',
          ),
          pendingOrders: const [],
          paidOrders: const [],
          verificationState: const SellerVerificationV2State(
            status: SellerVerificationStatus.approved,
          ),
          senderAddressResult: Result.success(_senderAddress()),
          shippingState: ShippingOptionsListLoaded([_activeShippingOption()]),
          subscription: _subscription(expiresIn: const Duration(days: 5)),
          upgradeConfig: _upgradeConfig(renewalReminderDays: 7),
        ),
      );

      await _showActionQueue(
        tester,
        anchor: find.text('Subscription Segera Berakhir'),
      );

      expect(find.byKey(const Key('seller-action-queue-subscription-expiring')), findsOneWidget);
      expect(find.text('Subscription Segera Berakhir'), findsOneWidget);
      expect(find.textContaining('Berakhir dalam'), findsOneWidget);
    });

    testWidgets('shows the ready state when no action remains', (tester) async {
      await _pumpDashboard(
        tester,
        overrides: _dashboardOverrides(
          user: _sellerUser(
            hasMarketAuthority: true,
            sellerSubscriptionStatus: 'active',
          ),
          pendingOrders: const [],
          paidOrders: const [],
          verificationState: const SellerVerificationV2State(
            status: SellerVerificationStatus.approved,
          ),
          senderAddressResult: Result.success(_senderAddress()),
          shippingState: ShippingOptionsListLoaded([_activeShippingOption()]),
          subscription: _subscription(expiresIn: const Duration(days: 60)),
          upgradeConfig: _upgradeConfig(),
        ),
      );

      await _showActionQueue(tester, anchor: find.text('Operasional toko siap'));

      expect(find.text('Operasional toko siap'), findsOneWidget);
      expect(find.text('Tidak ada tindakan yang menunggu saat ini.'), findsOneWidget);
      expect(find.byKey(const Key('seller-action-queue-pending-orders')), findsNothing);
      expect(find.byKey(const Key('seller-action-queue-paid-orders')), findsNothing);
      expect(find.byKey(const Key('seller-action-queue-verification')), findsNothing);
      expect(find.byKey(const Key('seller-action-queue-sender-address')), findsNothing);
      expect(find.byKey(const Key('seller-action-queue-shipping-option')), findsNothing);
      expect(find.byKey(const Key('seller-action-queue-subscription-expiring')), findsNothing);
      expect(find.byKey(const Key('seller-action-queue-subscription-expired')), findsNothing);
    });
  });
}
