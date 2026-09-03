import 'package:flutter/material.dart';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/config/seller_upgrade_config_entity.dart';
import 'package:labuda/core/config/seller_upgrade_config_provider.dart';
import 'package:labuda/domains/commerce/transaction/order/order.dart';
import 'package:labuda/domains/user/identity/verification/verification.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart'
    show shippingNotifierProvider;
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/shipping_notifier.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/shipping_state.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_earnings.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_activity.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_analytics.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_dashboard.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_subscription.dart';
import 'package:labuda/domains/user/preference/seller/domain/entities/withdrawal.dart';
import 'package:labuda/domains/user/preference/seller/domain/repositories/seller_repository.dart';
import 'package:labuda/domains/user/preference/seller/seller_di.dart';
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_dashboard_screen.dart';
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_earnings_screen.dart';
import 'package:labuda/domains/user/preference/seller/presentation/providers/withdraw_notifier.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/address_list_provider.dart';
import 'package:labuda/domains/user/profile/domain/entities/bank_account_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/bank_account_provider.dart';
import 'package:labuda/shared/models/wilayah_models.dart';

class _FakeSellerAuthController extends AuthController {
  _FakeSellerAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _VerifiedSellerVerificationNotifier
    extends SellerVerificationV2Notifier {
  @override
  SellerVerificationV2State build() => const SellerVerificationV2State(
    isVerified: true,
    status: SellerVerificationStatus.approved,
  );

  @override
  Future<void> loadStatus() async {}
}

class _UnverifiedSellerVerificationNotifier
    extends SellerVerificationV2Notifier {
  @override
  SellerVerificationV2State build() => const SellerVerificationV2State(
    isVerified: false,
    status: SellerVerificationStatus.notSubmitted,
  );

  @override
  Future<void> loadStatus() async {}
}

class _ReadyShippingNotifier extends ShippingNotifier {
  @override
  ShippingSetupsListState build() =>
      ShippingSetupsListLoaded([_activeShippingSetup()]);

  @override
  Future<void> loadActiveShippingSetups() async {}
}

class _FailingSellerRepository implements SellerRepository {
  @override
  Future<RepositoryResult<SellerDashboardStats>> getDashboardStats(
    String sellerId,
  ) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<SellerAnalytics>> getAnalytics({
    required String sellerId,
    required AnalyticsPeriod period,
    required DateTime startDate,
    required DateTime endDate,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<SellerPerformance>> getPerformance(
    String sellerId,
  ) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<List<SalesDataPoint>>> getSalesTrendData({
    required String sellerId,
    int days = 30,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<SellerEarnings>> getEarnings(String sellerId) async {
    throw Exception('boom');
  }

  @override
  Future<RepositoryResult<SellerEarnings>> getEarningsBreakdown({
    required String sellerId,
    required DateTime startDate,
    required DateTime endDate,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<List<WithdrawalRecord>>> getWithdrawalHistory({
    required String sellerId,
    int limit = 20,
    int offset = 0,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<List<RecentActivityItem>>> getRecentActivity(
    String sellerId, {
    int limit = 10,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<List<RecentActivityItem>>> getActivityHistory(
    ActivityHistoryParams params, {
    int limit = 100,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<SellerSubscription>> getSubscription(
    String sellerId,
  ) async {
    throw UnimplementedError();
  }

  @override
  Stream<SellerSubscription?> watchSubscription(String sellerId) {
    return const Stream<SellerSubscription?>.empty();
  }

  @override
  Future<RepositoryResult<WithdrawResult>> requestWithdraw(
    WithdrawRequest request, {
    String? idempotencyKey,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<RepositoryResult<List<Withdrawal>>> getWithdrawHistory({
    int limit = 100,
    int offset = 0,
  }) async {
    throw UnimplementedError();
  }
}

AuthUser _sellerUser() {
  final now = DateTime.utc(2026, 8, 1);
  return AuthUser(
    id: 'seller-001',
    createdAt: now,
    updatedAt: now,
    email: 'seller@example.com',
    username: 'seller01',
    isEmailVerified: true,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    hasSellerProfile: true,
    sellerSubscriptionStatus: 'active',
    hasMarketAuthority: true,
  );
}

SellerEarnings _earnings({
  double availableBalance = 125000,
  double totalRevenue = 890000,
  double totalWithdrawn = 420000,
  double pendingRevenue = 0,
  double withdrawalFeeAmount = 5000,
  double? grossPayable = 130000,
  double? activeDisputeFreeze = 5000,
}) {
  return SellerEarnings(
    sellerId: _sellerUser().id,
    totalRevenue: totalRevenue,
    pendingRevenue: pendingRevenue,
    totalPlatformFees: 0,
    availableBalance: availableBalance,
    withdrawalFeeAmount: withdrawalFeeAmount,
    totalWithdrawn: totalWithdrawn,
    totalWithdrawals: 2,
    totalCompletedOrders: 0,
    calculatedAt: DateTime.utc(2026, 8, 1),
    grossPayable: grossPayable,
    activeDisputeFreeze: activeDisputeFreeze,
  );
}

BankAccountEntity _defaultBankAccount() {
  final now = DateTime.utc(2026, 8, 1);
  return BankAccountEntity(
    id: 'bank-001',
    bankName: 'BCA',
    bankCode: 'BCA',
    accountNumber: '1234567890',
    accountHolderName: 'Seller One',
    isDefault: true,
    status: BankAccountStatus.active,
    createdAt: now,
    updatedAt: now,
  );
}

AddressEntity _senderAddress() {
  final now = DateTime.utc(2026, 8, 1);
  return AddressEntity(
    id: 'sender-address-001',
    userId: _sellerUser().id,
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

ShippingSetup _activeShippingSetup() {
  final now = DateTime.utc(2026, 8, 1);
  return ShippingSetup(
    id: 'shipping-001',
    name: 'Bus Kencana',
    type: ShippingType.bus,
    coverageAreas: const [],
    isActive: true,
    createdAt: now,
    updatedAt: now,
  );
}

SellerSubscription _activeSubscription() {
  final now = DateTime.utc(2026, 8, 1);
  return SellerSubscription(
    isActive: true,
    yearlyFee: 70000,
    startDate: now.subtract(const Duration(days: 30)),
    expiryDate: now.add(const Duration(days: 60)),
    status: SubscriptionStatus.active,
    paymentId: 'payment-001',
    createdAt: now.subtract(const Duration(days: 30)),
  );
}

dynamic _queueSafeOverrides() {
  return [
    shippingNotifierProvider.overrideWith(() => _ReadyShippingNotifier()),
    watchSellerOrdersProvider(
      sellerId: _sellerUser().id,
      status: OrderStatus.pending,
    ).overrideWith((ref) => Stream.value(const [])),
    watchSellerOrdersProvider(
      sellerId: _sellerUser().id,
      status: OrderStatus.paid,
    ).overrideWith((ref) => Stream.value(const [])),
    primaryAddressProvider(
      _sellerUser().id,
    ).overrideWith((ref) async => Result.success(_senderAddress())),
    sellerSubscriptionFutureProvider(
      _sellerUser().id,
    ).overrideWith((ref) async => _activeSubscription()),
    sellerUpgradeConfigProvider.overrideWith(
      (ref) async => const SellerUpgradeConfigEntity(
        yearlyFee: 70000,
        durationDays: 365,
        isEnabled: true,
        renewalReminderDays: 7,
      ),
    ),
  ];
}

GoRouter _router({String initialLocation = RoutePaths.sellerDashboard}) {
  return GoRouter(
    initialLocation: initialLocation,
    routes: [
      GoRoute(
        path: RoutePaths.sellerDashboard,
        builder: (context, state) => const SellerDashboardScreen(),
      ),
      GoRoute(
        path: RoutePaths.sellerEarnings,
        builder: (context, state) => const SellerEarningsScreen(),
      ),
      GoRoute(
        path: RoutePaths.sellerBankAccounts,
        builder: (context, state) =>
            const Scaffold(body: Text('Bank Accounts')),
      ),
    ],
  );
}

Widget _buildApp({
  required dynamic overrides,
  String initialLocation = RoutePaths.sellerDashboard,
}) {
  return ProviderScope(
    overrides: [..._queueSafeOverrides(), ...overrides],
    child: MaterialApp.router(
      routerConfig: _router(initialLocation: initialLocation),
      theme: ThemeData(useMaterial3: true),
    ),
  );
}

void main() {
  group('Seller dashboard earnings exposure', () {
    testWidgets('available balance error stays isolated from dashboard', (
      tester,
    ) async {
      await tester.pumpWidget(
        _buildApp(
          overrides: [
            authControllerProvider.overrideWith(
              () => _FakeSellerAuthController(
                AuthState.authenticated(_sellerUser(), emailVerified: true),
              ),
            ),
            sellerVerificationV2NotifierProvider.overrideWith(
              _VerifiedSellerVerificationNotifier.new,
            ),
            sellerRepositoryProvider.overrideWith(
              (ref) => _FailingSellerRepository(),
            ),
          ],
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Dashboard Penjual'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });

    testWidgets(
      'earnings screen shows balance hierarchy and empty withdrawal history',
      (tester) async {
        await tester.pumpWidget(
          _buildApp(
            overrides: [
              authControllerProvider.overrideWith(
                () => _FakeSellerAuthController(
                  AuthState.authenticated(_sellerUser(), emailVerified: true),
                ),
              ),
              sellerVerificationV2NotifierProvider.overrideWith(
                _VerifiedSellerVerificationNotifier.new,
              ),
              sellerEarningsProvider.overrideWith(
                (ref, sellerId) async => _earnings(pendingRevenue: 0),
              ),
              bankAccountsStreamProvider(_sellerUser().id).overrideWith(
                (ref) => Stream.value(Result.success([_defaultBankAccount()])),
              ),
              withdrawalHistoryProvider.overrideWith((ref) async => const []),
            ],
            initialLocation: RoutePaths.sellerEarnings,
          ),
        );
        await tester.pumpAndSettle();

        // Production balance card titles (English, canonical).
        // 'Available Balance' and 'Pending Balance' also appear in the
        // 'About Earnings' info section, so use findsWidgets.
        expect(find.text('Available Balance'), findsWidgets);
        expect(find.text('Total Earned'), findsOneWidget);
        expect(find.text('Pending Balance'), findsWidgets);
        expect(find.text('Total Withdrawn'), findsOneWidget);

        // Production withdraw button text.
        expect(
          find.widgetWithText(ElevatedButton, 'Withdraw Funds'),
          findsOneWidget,
        );

        // Empty withdrawal history (canonical production text).
        expect(find.text('Belum ada riwayat penarikan'), findsOneWidget);

        // Withdraw button is enabled (balance >= minimum).
        final button = tester.widget<ElevatedButton>(
          find.widgetWithText(ElevatedButton, 'Withdraw Funds'),
        );
        expect(button.onPressed, isNotNull);
      },
    );

    testWidgets(
      'unverified seller triggers verification dialog on withdraw tap',
      (tester) async {
        await tester.pumpWidget(
          _buildApp(
            overrides: [
              authControllerProvider.overrideWith(
                () => _FakeSellerAuthController(
                  AuthState.authenticated(_sellerUser(), emailVerified: true),
                ),
              ),
              sellerVerificationV2NotifierProvider.overrideWith(
                _UnverifiedSellerVerificationNotifier.new,
              ),
              sellerEarningsProvider.overrideWith(
                (ref, sellerId) async => _earnings(),
              ),
              bankAccountsStreamProvider(_sellerUser().id).overrideWith(
                (ref) => Stream.value(Result.success([_defaultBankAccount()])),
              ),
              withdrawalHistoryProvider.overrideWith((ref) async => const []),
            ],
            initialLocation: RoutePaths.sellerEarnings,
          ),
        );
        await tester.pumpAndSettle();

        // Tap the withdraw button — production gates verification inside
        // the WithdrawDialog, not at the button level.
        final withdrawButton = find.widgetWithText(
          ElevatedButton,
          'Withdraw Funds',
        );
        await tester.scrollUntilVisible(
          withdrawButton,
          200,
          scrollable: find.byType(Scrollable),
        );
        await tester.tap(withdrawButton);
        await tester.pumpAndSettle();

        // WithdrawDialog._buildVerificationRequiredDialog renders these
        // when sellerVerificationV2NotifierProvider.isVerified == false.
        expect(find.text('Verifikasi Diperlukan'), findsOneWidget);
        expect(
          find.text(
            'Anda perlu melakukan verifikasi identitas sebelum dapat '
            'menarik dana.',
          ),
          findsOneWidget,
        );
        expect(
          find.widgetWithText(ElevatedButton, 'Verifikasi Sekarang'),
          findsOneWidget,
        );
      },
    );

    testWidgets(
      'pending balance card shows when backend returns non-zero',
      (tester) async {
        await tester.pumpWidget(
          _buildApp(
            overrides: [
              authControllerProvider.overrideWith(
                () => _FakeSellerAuthController(
                  AuthState.authenticated(_sellerUser(), emailVerified: true),
                ),
              ),
              sellerVerificationV2NotifierProvider.overrideWith(
                _VerifiedSellerVerificationNotifier.new,
              ),
              sellerEarningsProvider.overrideWith(
                (ref, sellerId) async => _earnings(pendingRevenue: 65000),
              ),
              bankAccountsStreamProvider(_sellerUser().id).overrideWith(
                (ref) => Stream.value(Result.success([_defaultBankAccount()])),
              ),
              withdrawalHistoryProvider.overrideWith((ref) async => const []),
            ],
            initialLocation: RoutePaths.sellerEarnings,
          ),
        );
        await tester.pumpAndSettle();

        // Pending Balance card is rendered (production uses this exact title
        // unconditionally, regardless of zero/non-zero pendingRevenue).
        // The text also appears in the 'About Earnings' info section.
        expect(find.text('Pending Balance'), findsWidgets);
      },
    );

    testWidgets(
      'withdrawal history error shows canonical error text',
      (tester) async {
        await tester.pumpWidget(
          _buildApp(
            overrides: [
              authControllerProvider.overrideWith(
                () => _FakeSellerAuthController(
                  AuthState.authenticated(_sellerUser(), emailVerified: true),
                ),
              ),
              sellerVerificationV2NotifierProvider.overrideWith(
                _VerifiedSellerVerificationNotifier.new,
              ),
              sellerEarningsProvider.overrideWith(
                (ref, sellerId) async => _earnings(),
              ),
              bankAccountsStreamProvider(_sellerUser().id).overrideWith(
                (ref) => Stream.value(Result.success([_defaultBankAccount()])),
              ),
              withdrawalHistoryProvider.overrideWith((ref) async {
                throw Exception('history boom');
              }),
            ],
            initialLocation: RoutePaths.sellerEarnings,
          ),
        );
        await tester.pumpAndSettle();

        // Production error handler renders this exact text (no key, no retry
        // button — this is the canonical error state).
        expect(find.text('Gagal memuat riwayat penarikan'), findsOneWidget);
      },
    );
  });
}
