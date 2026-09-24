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
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_upgrade_wizard_screen.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show addressRepositoryProvider;
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart'
    show FarmInfo, ProfileEntity, ProfileStats, UserVerificationInfo;
import 'package:labuda/domains/user/profile/domain/repositories/i_address_repository.dart';
import 'package:labuda/domains/user/profile/presentation/providers/profile_stream_provider.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:mockito/mockito.dart';

/// SCOPE 3 — canonical lifecycle split contract.
///
/// The seller upgrade wizard is the REGISTRATION lifecycle and serves
/// never-sellers only. An existing seller who opens it (`hasSellerProfile ==
/// true`) is failed closed by the `existingSeller` gate and forwarded to the
/// payment-only renewal lifecycle at `/seller/renewal`.
///
/// Renewal never runs registration onboarding, registration terms, or wizard
/// steps: its canonical contract is asserted in `seller_renewal_screen_test.dart`.
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

class _FakeAddressRepository implements IAddressRepository {
  _FakeAddressRepository(this._addresses);

  final List<AddressEntity> _addresses;

  @override
  Future<Result<List<AddressEntity>>> getAddressesByUserId(
    String userId,
  ) async {
    return Result.success(_addresses);
  }

  @override
  Future<Result<List<AddressEntity>>> getAddressesByPurpose(
    String userId,
    AddressPurpose purpose,
  ) async {
    return Result.success(
      _addresses.where((address) => address.purpose == purpose).toList(),
    );
  }

  @override
  Future<Result<AddressEntity>> getAddressById(String addressId) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<AddressEntity?>> getPrimaryAddress(
    String userId, {
    AddressPurpose? purpose,
  }) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<void>> addAddress(AddressEntity address) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<void>> updateAddress(AddressEntity address) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<void>> deleteAddress(String addressId) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<void>> setPrimaryAddress(
    String addressId,
    String userId,
  ) async {
    throw UnimplementedError();
  }

  @override
  Stream<Result<List<AddressEntity>>> watchAddresses(String userId) {
    throw UnimplementedError();
  }

  @override
  Stream<Result<List<AddressEntity>>> watchAddressesByPurpose(
    String userId,
    AddressPurpose purpose,
  ) {
    throw UnimplementedError();
  }

  @override
  Future<Result<int>> countAddresses(String userId, {AddressPurpose? purpose}) {
    throw UnimplementedError();
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

  @override
  Future<void> performOnboarding(
    String storeName, {
    String? storeImageUrl,
  }) async {
    onboardingCalls++;
  }

  // PMF-02: the payment step loads the canonical methods, each already carrying
  // the backend-calculated fee and gross, then sends the chosen code.
  @override
  Future<SellerSubscriptionPaymentMethodsDto>
  getSubscriptionPaymentMethods() async {
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
  required bool hasSellerProfile,
  required bool hasMarketAuthority,
  required String username,
  required String bio,
  required String phoneNumber,
}) {
  final now = DateTime.utc(2026, 1, 1);
  return AuthUser(
    id: id,
    createdAt: now,
    updatedAt: now,
    email: '$id@example.com',
    username: username,
    bio: bio,
    phoneNumber: phoneNumber,
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    hasSellerProfile: hasSellerProfile,
    sellerSubscriptionStatus: hasMarketAuthority ? 'active' : 'expired',
    hasMarketAuthority: hasMarketAuthority,
    sellerTier: SellerTier.sellerElite,
    isIdVerified: false,
    isFarmVerified: false,
    lifecycle: ContentLifecycle.active,
  );
}

ProfileEntity _profileFor(AuthUser user, {required String farmName}) {
  return ProfileEntity(
    id: 'profile-${user.id}',
    userId: user.id,
    joinedAt: DateTime.utc(2026, 1, 1),
    stats: const ProfileStats(followersCount: 0, followingCount: 0),
    verification: UserVerificationInfo.fromAuthUser(user),
    contactInfo: null,
    farmInfo: FarmInfo(farmName: farmName),
  );
}

AddressEntity _senderAddressFor(String userId) {
  return AddressEntity.fromJson({
    'user_id': userId,
    'purpose': 'sender',
    'recipient_name': 'Test Seller',
    'phone': '+62123456789',
    'province': {'id': 'province-1', 'name': 'Jawa Barat'},
    'city': {'id': 'city-1', 'name': 'Bandung', 'province_id': 'province-1'},
    'district': {'id': 'district-1', 'name': 'Coblong', 'city_id': 'city-1'},
    'village': {'id': 'village-1', 'name': 'Dago', 'district_id': 'district-1'},
    'street_address': 'Jl. Test No. 1',
    'postal_code': '40135',
    'is_primary': true,
    'created_at': '2026-01-01T00:00:00.000Z',
    'updated_at': '2026-01-01T00:00:00.000Z',
  }, 'address-1');
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

/// Routed harness: the canonical lifecycle split is expressed with real routes,
/// so the wizard's renewal hand-off (`/seller/renewal`) and its payment WebView
/// push (`/payment-webview`) resolve exactly as they do in the app.
Widget _wrap(
  _FakeAuthController controller,
  _FakeAuthRepository authRepository,
  _FakeAddressRepository addressRepository,
  _FakeSellerRemoteDatasource sellerRemoteDatasource, {
  SellerRepository? sellerRepository,
  required AuthUser authUser,
  required String farmName,
}) {
  final profile = _profileFor(authUser, farmName: farmName);
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(() => controller),
      auth_data.authRepositoryProvider.overrideWithValue(authRepository),
      addressRepositoryProvider.overrideWithValue(addressRepository),
      sellerRemoteDatasourceProvider.overrideWithValue(sellerRemoteDatasource),
      sellerRepositoryProvider.overrideWithValue(
        sellerRepository ??
            _FakeSellerRepository(
              initialSubscription: _subscriptionSnapshot(
                expiryDate: DateTime.utc(2026, 12, 31),
                paymentId: 'payment-baseline',
              ),
            ),
      ),
      upgrade_config.sellerUpgradeConfigProvider.overrideWith(
        (ref) async => const SellerUpgradeConfigEntity(
          yearlyFee: 250000,
          durationDays: 365,
          isEnabled: true,
          renewalReminderDays: 30,
        ),
      ),
      profileStreamProvider(
        authUser.id,
      ).overrideWith((ref) => Stream.value(profile)),
    ],
    child: MaterialApp.router(
      routerConfig: GoRouter(
        initialLocation: RoutePaths.sellerUpgrade,
        routes: [
          GoRoute(
            path: RoutePaths.sellerUpgrade,
            builder: (context, state) => const SellerUpgradeWizardScreen(),
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

Future<void> _pumpRegistrationFlow(WidgetTester tester) async {
  await tester.tap(find.text('Lanjut Lengkapi Data'));
  await tester.pumpAndSettle();

  await tester.tap(find.text('Lanjut'));
  await tester.pumpAndSettle();

  await tester.tap(find.text('Lanjut'));
  await tester.pumpAndSettle();

  final checkbox = find.byType(Checkbox);
  await tester.ensureVisible(checkbox);
  await tester.tap(checkbox);
  await tester.pumpAndSettle();

  final paymentButton = find.text('Lanjut');
  await tester.ensureVisible(paymentButton);
  await tester.tap(paymentButton);
  await tester.pumpAndSettle();

  await _selectSubscriptionPaymentMethod(tester);
}

// PMF-02: the payment step requires an explicit method choice before submit.
Future<void> _selectSubscriptionPaymentMethod(WidgetTester tester) async {
  final methodSelector = find.text('Pilih metode pembayaran');
  await tester.ensureVisible(methodSelector);
  await tester.pumpAndSettle();
  await tester.tap(methodSelector);
  await tester.pumpAndSettle();
  await tester.tap(find.text('BCA Virtual Account'));
  await tester.pumpAndSettle();
}

/// The payment flow awaits the internal Payment WebView (`context.push`
/// completes when that route is dismissed); returning from it starts payment
/// confirmation polling.
Future<void> _returnFromPaymentWebView(WidgetTester tester) async {
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 400));
  expect(find.text('Payment WebView'), findsOneWidget);

  GoRouter.of(tester.element(find.text('Payment WebView'))).pop();
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 400));

  // Batch 2 parity with SellerRenewalScreen: Indonesian copy + always-
  // available manual re-entry button.
  expect(find.text('Memproses pembayaran'), findsOneWidget);
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

AuthUser _neverSeller() => _sellerUser(
  id: 'never-seller',
  hasSellerProfile: false,
  hasMarketAuthority: false,
  username: 'never-seller',
  bio: 'Bio registration',
  phoneNumber: '+6211111111',
);

AuthUser _expiredSeller() => _sellerUser(
  id: 'expired-seller',
  hasSellerProfile: true,
  hasMarketAuthority: false,
  username: 'expired-seller',
  bio: 'Bio renewal',
  phoneNumber: '+6222222222',
);

void main() {
  group('SellerUpgradeWizardScreen — registration lifecycle', () {
    testWidgets('loading state fails closed', (tester) async {
      final controller = _FakeAuthController(const AuthState.loading());
      final authRepository = _FakeAuthRepository();
      final addressRepository = _FakeAddressRepository(<AddressEntity>[]);
      final sellerRemoteDatasource = _FakeSellerRemoteDatasource();
      final authUser = _neverSeller();

      await tester.pumpWidget(
        _wrap(
          controller,
          authRepository,
          addressRepository,
          sellerRemoteDatasource,
          authUser: authUser,
          farmName: 'Loading Farm',
        ),
      );

      await tester.pumpAndSettle();

      expect(find.text('Seller account is loading'), findsOneWidget);
      expect(find.text('Registration mode'), findsNothing);
      expect(find.text('Sudah Menjadi Seller'), findsNothing);
    });

    testWidgets('unauthenticated state fails closed', (tester) async {
      final controller = _FakeAuthController(const AuthState.unauthenticated());
      final authRepository = _FakeAuthRepository();
      final addressRepository = _FakeAddressRepository(<AddressEntity>[]);
      final sellerRemoteDatasource = _FakeSellerRemoteDatasource();
      final authUser = _neverSeller();

      await tester.pumpWidget(
        _wrap(
          controller,
          authRepository,
          addressRepository,
          sellerRemoteDatasource,
          authUser: authUser,
          farmName: 'Anon Farm',
        ),
      );

      await tester.pumpAndSettle();

      expect(find.text('Login diperlukan'), findsOneWidget);
      expect(find.text('Registration mode'), findsNothing);
      expect(find.text('Sudah Menjadi Seller'), findsNothing);
    });

    testWidgets(
      'never-seller enters registration mode and invokes onboarding',
      (tester) async {
        final binding = TestWidgetsFlutterBinding.ensureInitialized();
        final urlLauncherCalls = _mockUrlLauncher(binding);

        final user = _neverSeller();
        final controller = _FakeAuthController(
          AuthState.authenticated(user, emailVerified: true),
        );
        final authRepository = _FakeAuthRepository();
        final addressRepository = _FakeAddressRepository([
          _senderAddressFor(user.id),
        ]);
        final sellerRemoteDatasource = _FakeSellerRemoteDatasource()
          ..paymentUrl = 'https://example.com/pay';

        await tester.pumpWidget(
          _wrap(
            controller,
            authRepository,
            addressRepository,
            sellerRemoteDatasource,
            authUser: user,
            farmName: 'Koi Baru',
          ),
        );

        await tester.pumpAndSettle();

        expect(find.text('Registration mode'), findsOneWidget);
        expect(find.text('Daftar Seller'), findsOneWidget);
        // Registration-only surface: no renewal lifecycle copy anywhere.
        expect(find.text('Perpanjang Seller'), findsNothing);
        expect(find.text('Renewal mode'), findsNothing);
        expect(find.text('Early renewal mode'), findsNothing);

        await _pumpRegistrationFlow(tester);

        await tester.tap(find.text('Bayar Sekarang'));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 10));

        expect(authRepository.updateProfileCalls, 1);
        expect(sellerRemoteDatasource.onboardingCalls, 1);
        expect(sellerRemoteDatasource.paymentCalls, 1);
        expect(sellerRemoteDatasource.lastPaymentMethodCode, 'bca_va');
        // Payment URLs are presented exclusively inside Labuda's internal WebView.
        expect(urlLauncherCalls, isEmpty);

        await _returnFromPaymentWebView(tester);
      },
    );
  });

  group('SellerUpgradeWizardScreen — renewal separation (no wizard renewal)', () {
    testWidgets(
      'existing seller fails closed: renewal is not a wizard mode',
      (tester) async {
        final user = _expiredSeller();
        final controller = _FakeAuthController(
          AuthState.authenticated(user, emailVerified: true),
        );
        final authRepository = _FakeAuthRepository();
        final addressRepository = _FakeAddressRepository([
          _senderAddressFor(user.id),
        ]);
        final sellerRemoteDatasource = _FakeSellerRemoteDatasource();

        await tester.pumpWidget(
          _wrap(
            controller,
            authRepository,
            addressRepository,
            sellerRemoteDatasource,
            authUser: user,
            farmName: 'Koi Renewal',
          ),
        );

        await tester.pumpAndSettle();

        // The wizard is not a renewal surface: it gates instead of running any
        // registration step for an existing seller.
        expect(find.text('Sudah Menjadi Seller'), findsOneWidget);
        expect(find.text('Registration mode'), findsNothing);
        expect(find.text('Renewal mode'), findsNothing);
        expect(find.text('Early renewal mode'), findsNothing);
        expect(find.text('Lanjut Lengkapi Data'), findsNothing);
        expect(find.text('Bayar Sekarang'), findsNothing);

        expect(authRepository.updateProfileCalls, 0);
        expect(sellerRemoteDatasource.onboardingCalls, 0);
        expect(sellerRemoteDatasource.paymentCalls, 0);
      },
    );

    testWidgets(
      'existing seller gate forwards to the payment-only renewal screen',
      (tester) async {
        final user = _expiredSeller();
        final controller = _FakeAuthController(
          AuthState.authenticated(user, emailVerified: true),
        );
        final authRepository = _FakeAuthRepository();
        final addressRepository = _FakeAddressRepository([
          _senderAddressFor(user.id),
        ]);
        final sellerRemoteDatasource = _FakeSellerRemoteDatasource()
          ..paymentUrl = 'https://example.com/pay';

        await tester.pumpWidget(
          _wrap(
            controller,
            authRepository,
            addressRepository,
            sellerRemoteDatasource,
            authUser: user,
            farmName: 'Koi Renewal',
          ),
        );

        await tester.pumpAndSettle();

        await tester.tap(find.text('Buka Perpanjang Seller'));
        await tester.pumpAndSettle();

        // Canonical renewal screen: read-only context + payment method only.
        expect(find.text('Bayar & Perpanjang'), findsOneWidget);
        expect(find.text('Mode perpanjang'), findsOneWidget);
        expect(find.text('Pembayaran'), findsNothing);
        expect(find.text('Lanjut Lengkapi Data'), findsNothing);
        expect(sellerRemoteDatasource.onboardingCalls, 0);
        expect(sellerRemoteDatasource.paymentCalls, 0);
      },
    );

    testWidgets(
      'principal switch recomputes registration versus existing-seller gate',
      (tester) async {
        final registrationUser = _neverSeller();
        final existingSeller = _expiredSeller();
        final controller = _FakeAuthController(
          AuthState.authenticated(registrationUser, emailVerified: true),
        );
        final authRepository = _FakeAuthRepository();
        final addressRepository = _FakeAddressRepository([
          _senderAddressFor(registrationUser.id),
          _senderAddressFor(existingSeller.id),
        ]);
        final sellerRemoteDatasource = _FakeSellerRemoteDatasource();

        await tester.pumpWidget(
          ProviderScope(
            overrides: [
              authControllerProvider.overrideWith(() => controller),
              auth_data.authRepositoryProvider.overrideWithValue(
                authRepository,
              ),
              addressRepositoryProvider.overrideWithValue(addressRepository),
              sellerRemoteDatasourceProvider.overrideWithValue(
                sellerRemoteDatasource,
              ),
              sellerRepositoryProvider.overrideWithValue(
                _FakeSellerRepository(
                  initialSubscription: _subscriptionSnapshot(
                    expiryDate: DateTime.utc(2026, 12, 31),
                    paymentId: 'payment-baseline',
                  ),
                ),
              ),
              upgrade_config.sellerUpgradeConfigProvider.overrideWith(
                (ref) async => const SellerUpgradeConfigEntity(
                  yearlyFee: 250000,
                  durationDays: 365,
                  isEnabled: true,
                  renewalReminderDays: 30,
                ),
              ),
              profileStreamProvider(registrationUser.id).overrideWith(
                (ref) => Stream.value(
                  _profileFor(registrationUser, farmName: 'Farm A'),
                ),
              ),
              profileStreamProvider(existingSeller.id).overrideWith(
                (ref) => Stream.value(
                  _profileFor(existingSeller, farmName: 'Farm B'),
                ),
              ),
            ],
            child: MaterialApp.router(
              routerConfig: GoRouter(
                initialLocation: RoutePaths.sellerUpgrade,
                routes: [
                  GoRoute(
                    path: RoutePaths.sellerUpgrade,
                    builder: (context, state) =>
                        const SellerUpgradeWizardScreen(),
                  ),
                  GoRoute(
                    path: RoutePaths.sellerRenewal,
                    builder: (context, state) => const SellerRenewalScreen(),
                  ),
                ],
              ),
            ),
          ),
        );

        await tester.pumpAndSettle();

        expect(find.text('Registration mode'), findsOneWidget);
        expect(find.text('Sudah Menjadi Seller'), findsNothing);

        controller.update(
          AuthState.authenticated(existingSeller, emailVerified: true),
        );
        await tester.pumpAndSettle();

        expect(find.text('Registration mode'), findsNothing);
        expect(find.text('Sudah Menjadi Seller'), findsOneWidget);
      },
    );
  });
}
