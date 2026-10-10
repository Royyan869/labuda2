import 'package:flutter/material.dart';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/core/config/seller_upgrade_config_entity.dart';
import 'package:hishumi/core/config/seller_upgrade_config_provider.dart';
import 'package:hishumi/domains/commerce/transaction/order/order.dart';
import 'package:hishumi/domains/user/identity/verification/verification.dart';
import 'package:hishumi/domains/commerce/transaction/shipping/domain/domain.dart';
import 'package:hishumi/domains/commerce/transaction/shipping/presentation/providers/providers.dart'
    show shippingNotifierProvider;
import 'package:hishumi/domains/commerce/transaction/shipping/presentation/providers/shipping_notifier.dart';
import 'package:hishumi/domains/commerce/transaction/shipping/presentation/providers/shipping_state.dart';
import 'package:hishumi/domains/user/preference/seller/domain/entities/seller_earnings.dart';
import 'package:hishumi/domains/user/preference/seller/domain/entities/seller_analytics_read.dart';
import 'package:hishumi/domains/user/preference/seller/domain/entities/seller_performance.dart';
import 'package:hishumi/domains/user/preference/seller/domain/entities/seller_subscription.dart';
import 'package:hishumi/domains/user/preference/seller/domain/entities/withdrawal.dart';
import 'package:hishumi/domains/user/preference/seller/domain/repositories/seller_repository.dart';
import 'package:hishumi/domains/user/preference/seller/seller_di.dart';
import 'package:hishumi/domains/user/preference/seller/presentation/screens/seller_dashboard_screen.dart';
import 'package:hishumi/domains/user/preference/seller/presentation/screens/seller_earnings_screen.dart';
import 'package:hishumi/domains/user/preference/seller/presentation/providers/withdraw_notifier.dart';
import 'package:hishumi/domains/user/profile/domain/entities/bank_account_entity.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/bank_account_provider.dart';
import 'package:hishumi/shared/models/wilayah_models.dart';

class _FakeSellerAuthController extends AuthController {
  _FakeSellerAuthController(this._state);

  final AuthState _state;

  @override
  AuthState build() => _state;
}

class _VerifiedSellerVerificationNotifier extends SellerVerificationV2Notifier {
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
  Future<Result<SellerAnalytics>> getAnalytics(String sellerId) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<SellerPerformance>> getPerformance(String sellerId) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<SellerEarnings>> getEarnings(String sellerId) async {
    throw Exception('boom');
  }

  @override
  Future<Result<SellerEarnings>> getEarningsBreakdown({
    required String sellerId,
    required DateTime startDate,
    required DateTime endDate,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<List<WithdrawalRecord>>> getWithdrawalHistory({
    required String sellerId,
    int limit = 20,
    int offset = 0,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<SellerSubscription>> getSubscription(String sellerId) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<WithdrawResult>> requestWithdraw(
    WithdrawRequest request, {
    String? idempotencyKey,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<List<Withdrawal>>> getWithdrawHistory({
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

        // Production balance card titles (Indonesian — this screen is id-first;
        // the old English pins were aligned to the codebase, per doctrine
        // "test mengikuti codebase"). The detailed definitions moved to the
        // canonical AppBar Info surface, so each title now appears once.
        expect(find.text('Saldo Tersedia'), findsOneWidget);
        expect(find.text('Total Penghasilan'), findsOneWidget);
        expect(find.text('Saldo Tertahan'), findsOneWidget);
        expect(find.text('Total Penarikan'), findsOneWidget);

        // Production withdraw button text.
        expect(
          find.widgetWithText(ElevatedButton, 'Tarik Dana'),
          findsOneWidget,
        );

        // Empty withdrawal history (canonical production text).
        expect(find.text('Belum ada riwayat penarikan'), findsOneWidget);

        // Withdraw button is enabled (balance >= minimum).
        final button = tester.widget<ElevatedButton>(
          find.widgetWithText(ElevatedButton, 'Tarik Dana'),
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
          'Tarik Dana',
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

    testWidgets('pending balance card shows when backend returns non-zero', (
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
      // unconditionally, regardless of zero/non-zero pendingRevenue). The
      // definitions card moved to Page Info, so the title appears once.
      expect(find.text('Saldo Tertahan'), findsOneWidget);
    });

    testWidgets('withdrawal history error shows canonical error text', (
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
    });
  });
}
