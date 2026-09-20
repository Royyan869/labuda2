import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/config/seller_upgrade_config_entity.dart';
import 'package:labuda/core/config/seller_upgrade_config_provider.dart'
    as upgrade_config;
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';
import 'package:labuda/domains/user/identity/authentication/data/auth_providers.dart'
    as auth_data;
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/user_profile_patch.dart';
import 'package:labuda/domains/user/preference/seller/data/dto/seller_dto.dart';
import 'package:labuda/domains/user/preference/seller/data/remote/seller_remote_datasource.dart';
import 'package:labuda/domains/user/preference/seller/data/seller_providers.dart'
    show sellerRemoteDatasourceProvider, sellerRepositoryProvider;
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_subscription.dart';
import 'package:labuda/domains/user/preference/seller/domain/repositories/seller_repository.dart';
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_renewal_screen.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:mockito/mockito.dart';

/// SCOPE 3 — canonical seller renewal contract.
///
/// Seller Renewal is PAYMENT-ONLY for users who are ALREADY sellers:
/// read-only subscription context → payment method → subscription payment
/// → internal Payment WebView → status polling → renewal result.
///
/// These tests are the anti-resurrection lock: renewal must never run seller
/// onboarding, never mutate the seller/store profile, and never require
/// registration terms or registration wizard steps.
class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  AuthState _state;

  @override
  AuthState build() => _state;

  void update(AuthState nextState) {
    _state = nextState;
    state = nextState;
  }

  @override
  Future<void> forceRefreshAuthState() async {}
}

class _FakeAuthRepository implements IAuthRepository {
  int updateProfileCalls = 0;

  @override
  Future<Result<UserProfilePatch>> updateProfile({
    String? photoUrl,
    String? phoneNumber,
    DateTime? phoneVerifiedAt,
    String? username,
    String? bio,
    String? location,
    DateTime? dateOfBirth,
  }) async {
    updateProfileCalls++;
    return Result.success(
      UserProfilePatch(username: username, bio: bio, phoneNumber: phoneNumber),
    );
  }

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeSellerRemoteDatasource extends Mock
    implements SellerRemoteDatasource {
  int onboardingCalls = 0;
  int paymentCalls = 0;
  String paymentUrl = '';
  String? lastPaymentMethodCode;
  int methodsCalls = 0;

  @override
  Future<void> performOnboarding(
    String storeName, {
    String? storeImageUrl,
  }) async {
    onboardingCalls++;
  }

  @override
  Future<SellerSubscriptionPaymentMethodsDto>
  getSubscriptionPaymentMethods() async {
    methodsCalls++;
    return const SellerSubscriptionPaymentMethodsDto(
      principalAmount: 250000,
      currency: 'IDR',
      methods: [
        SellerSubscriptionPaymentMethodDto(
          methodCode: 'bca_va',
          displayName: 'BCA Virtual Account',
          serviceFeeAmount: 6250,
          grossAmount: 256250,
        ),
      ],
    );
  }

  @override
  Future<Map<String, dynamic>> initiateSubscriptionPayment({
    required String paymentMethodCode,
  }) async {
    paymentCalls++;
    lastPaymentMethodCode = paymentMethodCode;
    return <String, dynamic>{'payment_url': paymentUrl};
  }
}

class _FakeSellerRepository implements SellerRepository {
  _FakeSellerRepository({required SellerSubscription initialSubscription})
    : _subscription = initialSubscription;

  int subscriptionCalls = 0;
  SellerSubscription _subscription;

  void updateSubscription(SellerSubscription subscription) {
    _subscription = subscription;
  }

  @override
  Future<RepositoryResult<SellerSubscription>> getSubscription(
    String sellerId,
  ) async {
    subscriptionCalls++;
    return RepositoryResult.success(_subscription);
  }

  @override
  Stream<SellerSubscription?> watchSubscription(String sellerId) {
    throw UnimplementedError();
  }

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

AuthUser _sellerUser({
  required String id,
  required bool hasMarketAuthority,
}) {
  final now = DateTime.utc(2026, 1, 1);
  return AuthUser(
    id: id,
    createdAt: now,
    updatedAt: now,
    email: '$id@example.com',
    username: id,
    bio: 'Bio $id',
    phoneNumber: '+628123456789',
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    // Renewal only exists for users who ALREADY have a seller profile.
    hasSellerProfile: true,
    sellerSubscriptionStatus: hasMarketAuthority ? 'active' : 'expired',
    hasMarketAuthority: hasMarketAuthority,
    sellerTier: SellerTier.sellerElite,
    isIdVerified: false,
    isFarmVerified: false,
    lifecycle: ContentLifecycle.active,
  );
}

SellerSubscription _subscriptionSnapshot({
  required DateTime expiryDate,
  required String paymentId,
}) {
  final now = DateTime.utc(2026, 1, 1);
  return SellerSubscription(
    isActive: true,
    yearlyFee: 250000,
    startDate: now,
    expiryDate: expiryDate,
    status: SubscriptionStatus.active,
    paymentId: paymentId,
    createdAt: now,
    lastRenewalDate: now,
  );
}

/// Real routes: renewal is reached from the signed-in shell (e.g. the seller
/// dashboard CTA) and its payment URL opens the internal Payment WebView.
Widget _wrap(
  _FakeAuthController controller,
  _FakeAuthRepository authRepository,
  _FakeSellerRemoteDatasource sellerRemoteDatasource,
  _FakeSellerRepository sellerRepository,
) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => controller),
      auth_data.authRepositoryProvider.overrideWithValue(authRepository),
      sellerRemoteDatasourceProvider.overrideWithValue(sellerRemoteDatasource),
      sellerRepositoryProvider.overrideWithValue(sellerRepository),
      upgrade_config.sellerUpgradeConfigProvider.overrideWith(
        (ref) async => const SellerUpgradeConfigEntity(
          yearlyFee: 250000,
          durationDays: 365,
          isEnabled: true,
          renewalReminderDays: 30,
        ),
      ),
    ],
    child: MaterialApp.router(
      routerConfig: GoRouter(
        initialLocation: '/home',
        routes: [
          GoRoute(
            path: '/home',
            builder: (context, state) => Scaffold(
              body: Center(
                child: ElevatedButton(
                  onPressed: () => context.push(RoutePaths.sellerRenewal),
                  child: const Text('Open renewal'),
                ),
              ),
            ),
          ),
          GoRoute(
            path: RoutePaths.sellerRenewal,
            builder: (context, state) => const SellerRenewalScreen(),
          ),
          GoRoute(
            path: RoutePaths.paymentWebview,
            builder: (context, state) =>
                const Scaffold(body: Text('Payment WebView')),
          ),
        ],
      ),
    ),
  );
}

Future<void> _openRenewal(WidgetTester tester) async {
  await tester.pumpAndSettle();
  await tester.tap(find.text('Open renewal'));
  await tester.pumpAndSettle();
}

Future<void> _selectPaymentMethod(WidgetTester tester) async {
  await tester.tap(find.text('Pilih metode pembayaran'));
  await tester.pumpAndSettle();
  await tester.tap(find.text('BCA Virtual Account'));
  await tester.pumpAndSettle();
}

/// The renewal payment flow awaits the internal Payment WebView (`context.push`
/// completes when that route is dismissed); returning from it starts payment
/// confirmation polling.
Future<void> _returnFromPaymentWebView(WidgetTester tester) async {
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 400));
  expect(find.text('Payment WebView'), findsOneWidget);

  GoRouter.of(tester.element(find.text('Payment WebView'))).pop();
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 400));

  expect(find.text('Processing payment'), findsOneWidget);
}

List<MethodCall> _mockUrlLauncher(TestWidgetsFlutterBinding binding) {
  const channel = MethodChannel('plugins.flutter.io/url_launcher');
  final calls = <MethodCall>[];
  binding.defaultBinaryMessenger.setMockMethodCallHandler(channel, (
    call,
  ) async {
    calls.add(call);
    return true;
  });
  return calls;
}

void main() {
  group('SellerRenewalScreen — payment-only lifecycle', () {
    testWidgets(
      'reads subscription context only: no editable seller/store fields',
      (tester) async {
        final user = _sellerUser(id: 'expired-seller', hasMarketAuthority: false);
        final controller = _FakeAuthController(
          AuthState.authenticated(user, emailVerified: true),
        );
        final authRepository = _FakeAuthRepository();
        final sellerRemoteDatasource = _FakeSellerRemoteDatasource();
        final sellerRepository = _FakeSellerRepository(
          initialSubscription: _subscriptionSnapshot(
            expiryDate: DateTime.utc(2026, 6, 30),
            paymentId: 'payment-old',
          ),
        );

        await tester.pumpWidget(
          _wrap(
            controller,
            authRepository,
            sellerRemoteDatasource,
            sellerRepository,
          ),
        );
        await _openRenewal(tester);

        // Read-only context.
        expect(find.text('Renewal mode'), findsOneWidget);
        expect(find.text('Seller Payment Summary'), findsOneWidget);

        // No registration surface whatsoever: no account/store fields and no
        // registration terms consent.
        expect(find.byType(TextField), findsNothing);
        expect(find.byType(TextFormField), findsNothing);
        expect(find.byType(Checkbox), findsNothing);
        expect(find.text('Lanjut Lengkapi Data'), findsNothing);
        expect(find.text('Lengkapi Akun Seller Baru'), findsNothing);
        expect(find.text('Paket & Syarat Seller'), findsNothing);
      },
    );

    testWidgets(
      'renewal payment never calls onboarding and never mutates the seller profile',
      (tester) async {
        final binding = TestWidgetsFlutterBinding.ensureInitialized();
        final urlLauncherCalls = _mockUrlLauncher(binding);

        final user = _sellerUser(id: 'expired-seller', hasMarketAuthority: false);
        final controller = _FakeAuthController(
          AuthState.authenticated(user, emailVerified: true),
        );
        final authRepository = _FakeAuthRepository();
        final sellerRemoteDatasource = _FakeSellerRemoteDatasource()
          ..paymentUrl = 'https://example.com/pay';
        final sellerRepository = _FakeSellerRepository(
          initialSubscription: _subscriptionSnapshot(
            expiryDate: DateTime.utc(2026, 6, 30),
            paymentId: 'payment-old',
          ),
        );

        await tester.pumpWidget(
          _wrap(
            controller,
            authRepository,
            sellerRemoteDatasource,
            sellerRepository,
          ),
        );
        await _openRenewal(tester);

        await _selectPaymentMethod(tester);
        await tester.tap(find.text('Bayar & Perpanjang'));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 10));

        // Payment uses the shared payment infrastructure (POST initiate with the
        // chosen method code only) — never POST /seller/onboarding, never
        // updateProfile.
        expect(sellerRemoteDatasource.onboardingCalls, 0);
        expect(authRepository.updateProfileCalls, 0);
        expect(sellerRemoteDatasource.paymentCalls, 1);
        expect(sellerRemoteDatasource.lastPaymentMethodCode, 'bca_va');
        // Payment URLs are presented exclusively inside Labuda's internal WebView.
        expect(urlLauncherCalls, isEmpty);

        await _returnFromPaymentWebView(tester);
      },
    );

    testWidgets(
      'renewal result is confirmed by subscription expiry extension',
      (tester) async {
        final binding = TestWidgetsFlutterBinding.ensureInitialized();
        final urlLauncherCalls = _mockUrlLauncher(binding);

        final expiredUser = _sellerUser(
          id: 'expired-seller',
          hasMarketAuthority: false,
        );
        final activeUser = _sellerUser(
          id: 'expired-seller',
          hasMarketAuthority: true,
        );
        final controller = _FakeAuthController(
          AuthState.authenticated(expiredUser, emailVerified: true),
        );
        final authRepository = _FakeAuthRepository();
        final sellerRemoteDatasource = _FakeSellerRemoteDatasource()
          ..paymentUrl = 'https://example.com/pay';
        final sellerRepository = _FakeSellerRepository(
          initialSubscription: _subscriptionSnapshot(
            expiryDate: DateTime.utc(2026, 6, 30),
            paymentId: 'payment-old',
          ),
        );

        await tester.pumpWidget(
          _wrap(
            controller,
            authRepository,
            sellerRemoteDatasource,
            sellerRepository,
          ),
        );
        await _openRenewal(tester);

        await _selectPaymentMethod(tester);
        await tester.tap(find.text('Bayar & Perpanjang'));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 10));

        expect(urlLauncherCalls, isEmpty);
        await _returnFromPaymentWebView(tester);

        // Not confirmed yet: subscription expiry unchanged.
        controller.update(
          AuthState.authenticated(activeUser, emailVerified: true),
        );
        await tester.pump(const Duration(seconds: 3));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 50));

        expect(find.text('Processing payment'), findsOneWidget);
        expect(
          find.text('Perpanjangan seller berhasil diproses'),
          findsNothing,
        );

        // Backend confirmed the renewal: subscription expiry moved forward.
        sellerRepository.updateSubscription(
          _subscriptionSnapshot(
            expiryDate: DateTime.utc(2027, 6, 30),
            paymentId: 'payment-new',
          ),
        );
        await tester.pump(const Duration(seconds: 3));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 600));

        expect(find.text('Processing payment'), findsNothing);
        expect(
          find.text('Perpanjangan seller berhasil diproses'),
          findsOneWidget,
        );
        expect(sellerRemoteDatasource.onboardingCalls, 0);
        expect(authRepository.updateProfileCalls, 0);
      },
    );

    testWidgets('active seller can early renew without recreating identity', (
      tester,
    ) async {
      final user = _sellerUser(id: 'active-seller', hasMarketAuthority: true);
      final controller = _FakeAuthController(
        AuthState.authenticated(user, emailVerified: true),
      );
      final authRepository = _FakeAuthRepository();
      final sellerRemoteDatasource = _FakeSellerRemoteDatasource()
        ..paymentUrl = 'https://example.com/pay';
      final sellerRepository = _FakeSellerRepository(
        initialSubscription: _subscriptionSnapshot(
          expiryDate: DateTime.utc(2026, 12, 31),
          paymentId: 'payment-active',
        ),
      );

      await tester.pumpWidget(
        _wrap(
          controller,
          authRepository,
          sellerRemoteDatasource,
          sellerRepository,
        ),
      );
      await _openRenewal(tester);

      expect(find.text('Early renewal mode'), findsOneWidget);

      await _selectPaymentMethod(tester);
      await tester.tap(find.text('Bayar & Perpanjang'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 10));

      expect(sellerRemoteDatasource.onboardingCalls, 0);
      expect(authRepository.updateProfileCalls, 0);
      expect(sellerRemoteDatasource.paymentCalls, 1);
    });
  });
}
