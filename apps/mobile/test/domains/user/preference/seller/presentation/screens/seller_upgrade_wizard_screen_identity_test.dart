import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/config/seller_upgrade_config_entity.dart';
import 'package:labuda/core/config/seller_upgrade_config_provider.dart'
    as upgrade_config;
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/data/auth_providers.dart'
    as auth_data;
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/user_profile_patch.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/repositories/repository_result.dart';
import 'package:labuda/domains/user/preference/seller/data/remote/seller_remote_datasource.dart';
import 'package:labuda/domains/user/preference/seller/data/seller_providers.dart'
    show sellerRemoteDatasourceProvider, sellerRepositoryProvider;
import 'package:labuda/domains/user/preference/seller/domain/entities/seller_subscription.dart';
import 'package:labuda/domains/user/preference/seller/domain/repositories/seller_repository.dart';
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_upgrade_wizard_screen.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show addressRepositoryProvider;
import 'package:labuda/domains/user/profile/presentation/providers/profile_stream_provider.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/domain/entities/profile_entity.dart'
    show FarmInfo, ProfileEntity, ProfileStats, UserVerificationInfo;
import 'package:labuda/domains/user/profile/domain/repositories/i_address_repository.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:mockito/mockito.dart';

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

  @override
  Future<void> performOnboarding(String storeName) async {
    onboardingCalls++;
  }

  @override
  Future<Map<String, dynamic>> initiateSubscriptionPayment() async {
    paymentCalls++;
    return <String, dynamic>{'payment_url': paymentUrl};
  }
}

class _FakeSellerRepository implements SellerRepository {
  _FakeSellerRepository({
    required SellerSubscription initialSubscription,
  }) : _subscription = initialSubscription;

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
    provider: ShonaAuthProvider.email,
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
      if (sellerRepository != null)
        sellerRepositoryProvider.overrideWithValue(sellerRepository),
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
    child: const MaterialApp(home: SellerUpgradeWizardScreen()),
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
  group('SellerUpgradeWizardScreen identity split', () {
    testWidgets('loading state fails closed', (tester) async {
      final controller = _FakeAuthController(const AuthState.loading());
      final authRepository = _FakeAuthRepository();
      final addressRepository = _FakeAddressRepository(<AddressEntity>[]);
      final sellerRemoteDatasource = _FakeSellerRemoteDatasource();
      final authUser = _sellerUser(
        id: 'loading-user',
        hasSellerProfile: false,
        hasMarketAuthority: false,
        username: 'loading-user',
        bio: 'Loading bio',
        phoneNumber: '+620000000001',
      );

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
      expect(find.text('Renewal mode'), findsNothing);
    });

    testWidgets('unauthenticated state fails closed', (tester) async {
      final controller = _FakeAuthController(const AuthState.unauthenticated());
      final authRepository = _FakeAuthRepository();
      final addressRepository = _FakeAddressRepository(<AddressEntity>[]);
      final sellerRemoteDatasource = _FakeSellerRemoteDatasource();
      final authUser = _sellerUser(
        id: 'anon-user',
        hasSellerProfile: false,
        hasMarketAuthority: false,
        username: 'anon-user',
        bio: 'Anon bio',
        phoneNumber: '+620000000002',
      );

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
      expect(find.text('Renewal mode'), findsNothing);
    });

    testWidgets(
      'never-seller enters registration mode and invokes onboarding',
      (tester) async {
        final controller = _FakeAuthController(
          AuthState.authenticated(
            _sellerUser(
              id: 'never-seller',
              hasSellerProfile: false,
              hasMarketAuthority: false,
              username: 'never-seller',
              bio: 'Bio registration',
              phoneNumber: '+6211111111',
            ),
            emailVerified: true,
          ),
        );
        final authRepository = _FakeAuthRepository();
        final addressRepository = _FakeAddressRepository([
          _senderAddressFor('never-seller'),
        ]);
        final sellerRemoteDatasource = _FakeSellerRemoteDatasource();

        await tester.pumpWidget(
          _wrap(
            controller,
            authRepository,
            addressRepository,
            sellerRemoteDatasource,
            authUser: _sellerUser(
              id: 'never-seller',
              hasSellerProfile: false,
              hasMarketAuthority: false,
              username: 'never-seller',
              bio: 'Bio registration',
              phoneNumber: '+6211111111',
            ),
            farmName: 'Koi Baru',
          ),
        );

        await tester.pumpAndSettle();

        expect(find.text('Registration mode'), findsOneWidget);
        expect(find.text('Daftar Seller'), findsOneWidget);

        await _pumpRegistrationFlow(tester);

        await tester.tap(find.text('Bayar Sekarang'));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 10));

        expect(authRepository.updateProfileCalls, 1);
        expect(sellerRemoteDatasource.onboardingCalls, 1);
        expect(sellerRemoteDatasource.paymentCalls, 1);
      },
    );

    testWidgets('expired seller enters renewal mode and skips onboarding', (
      tester,
    ) async {
      final user = _sellerUser(
        id: 'expired-seller',
        hasSellerProfile: true,
        hasMarketAuthority: false,
        username: 'expired-seller',
        bio: 'Bio renewal',
        phoneNumber: '+6222222222',
      );
      final controller = _FakeAuthController(
        AuthState.authenticated(user, emailVerified: true),
      );
      final authRepository = _FakeAuthRepository();
      final addressRepository = _FakeAddressRepository([
        _senderAddressFor('expired-seller'),
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

      expect(find.text('Renewal mode'), findsOneWidget);
      expect(find.text('Perpanjang Seller'), findsOneWidget);

      await _pumpRegistrationFlow(tester);

      await tester.tap(find.text('Bayar Sekarang'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 10));

      expect(authRepository.updateProfileCalls, 1);
      expect(sellerRemoteDatasource.onboardingCalls, 0);
      expect(sellerRemoteDatasource.paymentCalls, 1);
    });

    testWidgets('active seller can early renew without recreating profile', (
      tester,
    ) async {
      final binding = TestWidgetsFlutterBinding.ensureInitialized();
      final urlLauncherCalls = _mockUrlLauncher(binding);

      final user = _sellerUser(
        id: 'active-seller',
        hasSellerProfile: true,
        hasMarketAuthority: true,
        username: 'active-seller',
        bio: 'Bio early renewal',
        phoneNumber: '+6233333333',
      );
      final controller = _FakeAuthController(
        AuthState.authenticated(user, emailVerified: true),
      );
      final authRepository = _FakeAuthRepository();
      final addressRepository = _FakeAddressRepository([
        _senderAddressFor('active-seller'),
      ]);
      final sellerRemoteDatasource = _FakeSellerRemoteDatasource()
        ..paymentUrl = 'https://example.com/pay';
      final sellerRepository = _FakeSellerRepository(
        initialSubscription: _subscriptionSnapshot(
          expiryDate: DateTime.utc(2026, 12, 31),
          paymentId: 'payment-old',
        ),
      );

      await tester.pumpWidget(
        _wrap(
          controller,
          authRepository,
          addressRepository,
          sellerRemoteDatasource,
          sellerRepository: sellerRepository,
          authUser: user,
          farmName: 'Koi Aktif',
        ),
      );

      await tester.pumpAndSettle();

      expect(find.text('Early renewal mode'), findsOneWidget);

      await _pumpRegistrationFlow(tester);

      await tester.tap(find.text('Bayar Sekarang'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 10));

      expect(sellerRemoteDatasource.onboardingCalls, 0);
      expect(sellerRemoteDatasource.paymentCalls, 1);
      expect(urlLauncherCalls, isNotEmpty);

      await tester.pump(const Duration(seconds: 3));
      await tester.pump();

      expect(find.text('Processing payment'), findsOneWidget);
      expect(
        find.text('Selamat! Perpanjangan seller berhasil diproses'),
        findsNothing,
      );
      expect(sellerRepository.subscriptionCalls, 2);

      sellerRepository.updateSubscription(
        _subscriptionSnapshot(
          expiryDate: DateTime.utc(2027, 12, 31),
          paymentId: 'payment-new',
        ),
      );

      await tester.pump(const Duration(seconds: 3));
      await tester.pumpAndSettle();

      expect(find.text('Processing payment'), findsNothing);
      expect(find.text('Early renewal mode'), findsNothing);
      expect(find.text('Perpanjang Seller'), findsNothing);
      expect(sellerRepository.subscriptionCalls, 3);
    });

    testWidgets(
      'payment polling aborts when the initiating principal switches before confirmation',
      (tester) async {
        final binding = TestWidgetsFlutterBinding.ensureInitialized();
        final urlLauncherCalls = _mockUrlLauncher(binding);

        final initiatingUser = _sellerUser(
          id: 'payment-a',
          hasSellerProfile: true,
          hasMarketAuthority: false,
          username: 'payment-a',
          bio: 'Payment A bio',
          phoneNumber: '+6266666666',
        );
        final switchedUser = _sellerUser(
          id: 'payment-b',
          hasSellerProfile: true,
          hasMarketAuthority: true,
          username: 'payment-b',
          bio: 'Payment B bio',
          phoneNumber: '+6277777777',
        );
        final controller = _FakeAuthController(
          AuthState.authenticated(initiatingUser, emailVerified: true),
        );
        final authRepository = _FakeAuthRepository();
        final addressRepository = _FakeAddressRepository([
          _senderAddressFor('payment-a'),
          _senderAddressFor('payment-b'),
        ]);
        final sellerRemoteDatasource = _FakeSellerRemoteDatasource()
          ..paymentUrl = 'https://example.com/pay';

        await tester.pumpWidget(
          _wrap(
            controller,
            authRepository,
            addressRepository,
            sellerRemoteDatasource,
            authUser: initiatingUser,
            farmName: 'Payment A Farm',
          ),
        );

        await tester.pumpAndSettle();

        await _pumpRegistrationFlow(tester);

        await tester.tap(find.text('Bayar Sekarang'));
        await tester.pump();
        await tester.pump(const Duration(milliseconds: 100));

        expect(find.text('Processing payment'), findsOneWidget);
        expect(urlLauncherCalls, isNotEmpty);

        controller.update(
          AuthState.authenticated(switchedUser, emailVerified: true),
        );
        await tester.pump(const Duration(seconds: 3));
        await tester.pumpAndSettle();

        expect(find.text('Processing payment'), findsNothing);
        expect(
          find.text('Selamat! Perpanjangan seller berhasil diproses'),
          findsNothing,
        );
        expect(find.text('Perpanjang Seller'), findsOneWidget);
        expect(find.text('Lanjut Lengkapi Data'), findsOneWidget);
      },
    );

    testWidgets(
      'principal switch recomputes registration versus renewal mode',
      (tester) async {
        final registrationUser = _sellerUser(
          id: 'principal-a',
          hasSellerProfile: false,
          hasMarketAuthority: false,
          username: 'principal-a',
          bio: 'Principal A bio',
          phoneNumber: '+6244444444',
        );
        final renewalUser = _sellerUser(
          id: 'principal-b',
          hasSellerProfile: true,
          hasMarketAuthority: false,
          username: 'principal-b',
          bio: 'Principal B bio',
          phoneNumber: '+6255555555',
        );
        final controller = _FakeAuthController(
          AuthState.authenticated(registrationUser, emailVerified: true),
        );
        final authRepository = _FakeAuthRepository();
        final addressRepository = _FakeAddressRepository([
          _senderAddressFor('principal-a'),
          _senderAddressFor('principal-b'),
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
              profileStreamProvider(renewalUser.id).overrideWith(
                (ref) =>
                    Stream.value(_profileFor(renewalUser, farmName: 'Farm B')),
              ),
            ],
            child: const MaterialApp(home: SellerUpgradeWizardScreen()),
          ),
        );

        await tester.pumpAndSettle();

        expect(find.text('Registration mode'), findsOneWidget);
        expect(find.text('Renewal mode'), findsNothing);

        controller.update(
          AuthState.authenticated(renewalUser, emailVerified: true),
        );
        await tester.pump();

        expect(find.text('Registration mode'), findsNothing);
        expect(find.text('Renewal mode'), findsOneWidget);
        expect(find.text('Seller identity: Berakhir'), findsOneWidget);
      },
    );
  });
}
