import 'dart:async';

import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/catalog/auction/domain/domain.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_notifier.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/providers/auction_state.dart';
import 'package:labuda/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show addressRepositoryProvider;
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/domain/repositories/i_address_repository.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/models/wilayah_models.dart';

class _FakeAuthController extends AuthController {
  _FakeAuthController(this._state);

  AuthState _state;

  @override
  AuthState build() => _state;

  void setAuthState(AuthState state) {
    _state = state;
    this.state = state;
  }
}

class _FakeAuctionNotifier extends AuctionNotifier {
  int createCalls = 0;
  bool publicationSucceeds = true;
  String? failureMessage;
  String? failureCode;

  /// When non-null, the notifier returns this completer's future instead of
  /// completing immediately. Used by the duplicate-submit behavioral test.
  Completer<bool>? pendingCreate;

  @override
  AuctionNotifierState build() => const AuctionNotifierState();

  @override
  Future<bool> createAuction({
    required String sellerId,
    String? sellerUsername,
    String? sellerFarmName,
    String? sellerAvatar,
    required String title,
    required String description,
    required List<String> mediaUrls,
    required List<AuctionMediaType> mediaTypes,
    required KoiDetails koiDetails,
    required int openingBid,
    required int bidIncrement,
    int? buyNowPrice,
    required String startMode,
    DateTime? scheduledStartAt,
    required int durationHours,
    String? farmAddressId,
    AuctionLocation? location,
    required List<String> shippingSetupIds,
    String? preparationNote,
  }) async {
    createCalls += 1;
    if (pendingCreate != null) {
      state = state.copyWith(successMessage: null);
      return pendingCreate!.future;
    }
    if (!publicationSucceeds) {
      state = state.copyWith(
        isCreating: false,
        error: failureMessage ?? 'Gagal membuat lelang.',
        errorCode: failureCode,
        clearSuccess: true,
      );
      return false;
    }
    state = state.copyWith(successMessage: 'Lelang berhasil dibuat');
    return true;
  }
}

class _FakeShippingRepository implements ShippingRepository {
  final List<ShippingSetup> _options;

  _FakeShippingRepository()
    : _options = [
        ShippingSetup(
          id: 'ship-1',
          name: 'JNE',
          type: ShippingType.custom,
          coverageAreas: const [],
          createdAt: DateTime.utc(2026, 1, 1),
          updatedAt: DateTime.utc(2026, 1, 1),
        ),
      ];

  @override
  Future<Result<List<ShippingSetup>>> listMyShippingSetups() async =>
      Result.success(_options);

  @override
  Future<Result<List<ShippingSetup>>> listMyActiveShippingSetups() async =>
      Result.success(_options);

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
  Future<Result<void>> toggleActiveStatus(
    String optionId,
    bool isActive,
  ) async => Result.error('not used');

  @override
  Future<Result<void>> setProductShippingSetups(
    String productId,
    List<String> shippingSetupIds,
  ) async => Result.error('not used');

  @override
  Future<Result<List<DeliveryOption>>> checkDeliveryAvailability(
    CheckDeliveryRequest request,
  ) async => Result.error('not used');

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

class _FakeAddressRepository implements IAddressRepository {
  _FakeAddressRepository(this._primarySenderAddress);

  final AddressEntity? _primarySenderAddress;

  @override
  Future<Result<AddressEntity?>> getPrimaryAddress(
    String userId, {
    AddressPurpose? purpose,
  }) async {
    if (purpose == AddressPurpose.sender) {
      return Result.success(_primarySenderAddress);
    }
    return Result.success(null);
  }

  @override
  Future<Result<List<AddressEntity>>> getAddressesByUserId(
    String userId,
  ) async {
    return Result.success(const []);
  }

  @override
  Future<Result<List<AddressEntity>>> getAddressesByPurpose(
    String userId,
    AddressPurpose purpose,
  ) async {
    return Result.success(const []);
  }

  @override
  Future<Result<AddressEntity>> getAddressById(String addressId) async {
    return Result.error('not used');
  }

  @override
  Future<Result<void>> addAddress(AddressEntity address) async {
    return Result.error('not used');
  }

  @override
  Future<Result<void>> updateAddress(AddressEntity address) async {
    return Result.error('not used');
  }

  @override
  Future<Result<void>> deleteAddress(String addressId) async {
    return Result.error('not used');
  }

  @override
  Future<Result<void>> setPrimaryAddress(
    String addressId,
    String userId,
  ) async {
    return Result.error('not used');
  }

  @override
  Stream<Result<List<AddressEntity>>> watchAddresses(String userId) {
    return Stream.value(Result.success(const []));
  }

  @override
  Stream<Result<List<AddressEntity>>> watchAddressesByPurpose(
    String userId,
    AddressPurpose purpose,
  ) {
    return Stream.value(Result.success(const []));
  }

  @override
  Future<Result<int>> countAddresses(
    String userId, {
    AddressPurpose? purpose,
  }) async {
    return Result.success(0);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => null;
}

AuthUser _seller({
  required String id,
  required String username,
  required bool hasSellerProfile,
  required bool hasMarketAuthority,
  // Canonical expiry axis, stated explicitly by tests that assert expiry copy.
  // No capability→expiry coupling: capability-false defaults to 'none' (not
  // active yet), never to a false 'expired' claim (RF-02).
  String? sellerSubscriptionStatus,
  bool isIdVerified = false,
  bool isFarmVerified = false,
  SellerTier sellerTier = SellerTier.sellerBasic,
}) {
  final now = DateTime.utc(2026, 1, 1);
  return AuthUser(
    id: id,
    createdAt: now,
    updatedAt: now,
    email: '$username@example.com',
    username: username,
    avatarUrl: 'https://example.com/$id.png',
    isEmailVerified: true,
    accountStatus: AccountStatus.active,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    hasSellerProfile: hasSellerProfile,
    sellerSubscriptionStatus:
        sellerSubscriptionStatus ?? (hasMarketAuthority ? 'active' : 'none'),
    hasMarketAuthority: hasMarketAuthority,
    sellerTier: sellerTier,
    isIdVerified: isIdVerified,
    isFarmVerified: isFarmVerified,
    lifecycle: ContentLifecycle.active,
  );
}

AddressEntity _completeSenderAddress() {
  return AddressEntity(
    id: 'addr-1',
    userId: 'seller-1',
    purpose: AddressPurpose.sender,
    recipientName: 'Farm Sentosa',
    phone: '08123456789',
    province: Province(id: '33', name: 'Jawa Tengah'),
    city: City(id: '3301', name: 'Kabupaten Demak', provinceId: '33'),
    district: District(id: '330101', name: 'Mranggen', cityId: '3301'),
    village: Village(id: '3301012001', name: 'Rowosari', districtId: '330101'),
    streetAddress: 'Jl. Melati No. 12',
    postalCode: '59511',
    isPrimary: true,
    createdAt: DateTime.utc(2026, 7, 25),
    updatedAt: DateTime.utc(2026, 7, 25),
  );
}

ProviderContainer _container({
  required AuthController authController,
  required AuctionNotifier auctionNotifier,
}) {
  return ProviderContainer(
    overrides: [
      authControllerProvider.overrideWith(() => authController),
      auctionNotifierProvider.overrideWith(() => auctionNotifier),
      addressRepositoryProvider.overrideWithValue(
        _FakeAddressRepository(_completeSenderAddress()),
      ),
      shippingRepositoryProvider.overrideWithValue(_FakeShippingRepository()),
      apiClientProvider.overrideWithValue(ApiClient()),
    ],
  );
}

Widget _wrap({required ProviderContainer container}) {
  return UncontrolledProviderScope(
    container: container,
    child: const MaterialApp(home: CreateAuctionScreen()),
  );
}

Future<void> _scrollFormIntoView(WidgetTester tester, Finder target) async {
  await tester.dragUntilVisible(
    target,
    find.byWidgetPredicate(
      (w) => w is ListView && w.scrollDirection == Axis.vertical,
    ),
    const Offset(0, -300),
  );
  await tester.pumpAndSettle();
}

/// Helper: enter text into a field by finding the EditableText that is a
/// TextFormField whose decoration labelText matches [label].
Future<void> _enterFieldByLabel(
  WidgetTester tester,
  String labelText,
  String value,
) async {
  final labelFinder = find.text(labelText, skipOffstage: false);
  if (labelFinder.evaluate().isEmpty) {
    fail('Could not find TextFormField with label "$labelText"');
  }

  await _scrollFormIntoView(tester, labelFinder);

  final editable = find.descendant(
    of: find.ancestor(
      of: labelFinder,
      matching: find.byType(InputDecorator, skipOffstage: false),
    ),
    matching: find.byType(EditableText, skipOffstage: false),
  );
  await tester.enterText(editable.first, value);
  await tester.pumpAndSettle();
}

void main() {
  group('CreateAuctionScreen authority behavior', () {
    testWidgets('loading hydration fails closed with no operational form', (
      tester,
    ) async {
      final container = _container(
        authController: _FakeAuthController(const AuthState.loading()),
        auctionNotifier: _FakeAuctionNotifier(),
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(_wrap(container: container));

      expect(find.text('Silakan login untuk melanjutkan.'), findsOneWidget);
      expect(find.byType(TextFormField), findsNothing);
      expect(find.text('Informasi Dasar'), findsNothing);
    });

    testWidgets('unauthenticated account fails closed', (tester) async {
      final container = _container(
        authController: _FakeAuthController(const AuthState.unauthenticated()),
        auctionNotifier: _FakeAuctionNotifier(),
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(_wrap(container: container));

      expect(find.text('Silakan login untuk melanjutkan.'), findsOneWidget);
      expect(find.byType(TextFormField), findsNothing);
      expect(find.text('Informasi Dasar'), findsNothing);
    });

    testWidgets('non-seller gets registration gate', (tester) async {
      final container = _container(
        authController: _FakeAuthController(
          AuthState.authenticated(
            _seller(
              id: 'buyer-1',
              username: 'buyer',
              hasSellerProfile: false,
              hasMarketAuthority: false,
            ),
            emailVerified: true,
          ),
        ),
        auctionNotifier: _FakeAuctionNotifier(),
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(_wrap(container: container));

      expect(find.text('Jadi Seller Dulu'), findsOneWidget);
      expect(find.text('Informasi Dasar'), findsNothing);
    });

    testWidgets('expired seller gets renewal gate', (tester) async {
      final container = _container(
        authController: _FakeAuthController(
          AuthState.authenticated(
            _seller(
              id: 'seller-1',
              username: 'seller',
              hasSellerProfile: true,
              hasMarketAuthority: false,
              sellerSubscriptionStatus: 'expired',
            ),
            emailVerified: true,
          ),
        ),
        auctionNotifier: _FakeAuctionNotifier(),
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(_wrap(container: container));

      expect(find.text('Langganan Seller Habis'), findsOneWidget);
      expect(find.text('Perpanjang Langganan'), findsOneWidget);
      expect(find.text('Langganan Belum Aktif'), findsNothing);
      expect(find.text('Informasi Dasar'), findsNothing);
    });

    // The 'none' (not-yet-activated) counterpart of this gate is covered by
    // create_auction_screen_expiry_copy_test.dart, which runs against a minimal
    // harness; this file cannot compile at present (pre-existing media-picker
    // DI drift, unrelated to RF-02).

    testWidgets('restricted account does not expose create mutation', (
      tester,
    ) async {
      final container = _container(
        authController: _FakeAuthController(
          AuthState.accountRestricted(
            _seller(
              id: 'seller-1',
              username: 'seller',
              hasSellerProfile: true,
              hasMarketAuthority: true,
            ),
            restrictionType: AccountStatus.suspended,
          ),
        ),
        auctionNotifier: _FakeAuctionNotifier(),
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(_wrap(container: container));

      expect(find.text('Silakan login untuk melanjutkan.'), findsOneWidget);
      expect(find.byType(TextFormField), findsNothing);
      expect(
        (container.read(auctionNotifierProvider.notifier)
                as _FakeAuctionNotifier)
            .createCalls,
        0,
      );
    });

    testWidgets('active seller sees the operational form', (tester) async {
      final container = _container(
        authController: _FakeAuthController(
          AuthState.authenticated(
            _seller(
              id: 'seller-1',
              username: 'seller',
              hasSellerProfile: true,
              hasMarketAuthority: true,
            ),
            emailVerified: true,
          ),
        ),
        auctionNotifier: _FakeAuctionNotifier(),
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(_wrap(container: container));
      await tester.pumpAndSettle();

      expect(find.text('Informasi Dasar'), findsOneWidget);
      expect(find.byType(TextFormField), findsWidgets);
    });

    testWidgets(
      'active seller switching to buyer B before submit blocks form',
      (tester) async {
        final controller = _FakeAuthController(
          AuthState.authenticated(
            _seller(
              id: 'seller-a',
              username: 'seller-a',
              hasSellerProfile: true,
              hasMarketAuthority: true,
            ),
            emailVerified: true,
          ),
        );
        final notifier = _FakeAuctionNotifier();
        final container = _container(
          authController: controller,
          auctionNotifier: notifier,
        );
        addTearDown(container.dispose);

        await tester.pumpWidget(_wrap(container: container));
        await tester.pumpAndSettle();
        expect(find.text('Informasi Dasar'), findsOneWidget);

        controller.setAuthState(
          AuthState.authenticated(
            _seller(
              id: 'buyer-b',
              username: 'buyer-b',
              hasSellerProfile: false,
              hasMarketAuthority: false,
            ),
            emailVerified: true,
          ),
        );
        await tester.pumpAndSettle();

        expect(find.text('Jadi Seller Dulu'), findsOneWidget);
        expect(find.text('Informasi Dasar'), findsNothing);
        expect(notifier.createCalls, 0);
      },
    );

    testWidgets(
      'active seller switching to expired seller before submit blocks form',
      (tester) async {
        final controller = _FakeAuthController(
          AuthState.authenticated(
            _seller(
              id: 'seller-a',
              username: 'seller-a',
              hasSellerProfile: true,
              hasMarketAuthority: true,
            ),
            emailVerified: true,
          ),
        );
        final notifier = _FakeAuctionNotifier();
        final container = _container(
          authController: controller,
          auctionNotifier: notifier,
        );
        addTearDown(container.dispose);

        await tester.pumpWidget(_wrap(container: container));
        await tester.pumpAndSettle();
        expect(find.text('Informasi Dasar'), findsOneWidget);

        controller.setAuthState(
          AuthState.authenticated(
            _seller(
              id: 'seller-b',
              username: 'seller-b',
              hasSellerProfile: true,
              hasMarketAuthority: false,
              sellerSubscriptionStatus: 'expired',
            ),
            emailVerified: true,
          ),
        );
        await tester.pumpAndSettle();

        expect(find.text('Langganan Seller Habis'), findsOneWidget);
        expect(find.text('Informasi Dasar'), findsNothing);
        expect(notifier.createCalls, 0);
      },
    );

    testWidgets('authority loss while the form is open fails closed', (
      tester,
    ) async {
      final controller = _FakeAuthController(
        AuthState.authenticated(
          _seller(
            id: 'seller-a',
            username: 'seller-a',
            hasSellerProfile: true,
            hasMarketAuthority: true,
          ),
          emailVerified: true,
        ),
      );
      final notifier = _FakeAuctionNotifier();
      final container = _container(
        authController: controller,
        auctionNotifier: notifier,
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(_wrap(container: container));
      await tester.pumpAndSettle();
      await _enterFieldByLabel(tester, 'Judul *', 'Kohaku 50cm');

      controller.setAuthState(
        AuthState.authenticated(
          _seller(
            id: 'seller-a',
            username: 'seller-a',
            hasSellerProfile: true,
            hasMarketAuthority: false,
            sellerSubscriptionStatus: 'expired',
          ),
          emailVerified: true,
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Langganan Seller Habis'), findsOneWidget);
      expect(notifier.createCalls, 0);
    });

    testWidgets('hydration becoming null fails closed', (tester) async {
      final controller = _FakeAuthController(
        AuthState.authenticated(
          _seller(
            id: 'seller-a',
            username: 'seller-a',
            hasSellerProfile: true,
            hasMarketAuthority: true,
          ),
          emailVerified: true,
        ),
      );
      final notifier = _FakeAuctionNotifier();
      final container = _container(
        authController: controller,
        auctionNotifier: notifier,
      );
      addTearDown(container.dispose);

      await tester.pumpWidget(_wrap(container: container));
      await tester.pumpAndSettle();
      expect(find.text('Informasi Dasar'), findsOneWidget);

      controller.setAuthState(const AuthState.unauthenticated());
      await tester.pumpAndSettle();

      expect(find.text('Silakan login untuk melanjutkan.'), findsOneWidget);
      expect(find.byType(TextFormField), findsNothing);
      expect(notifier.createCalls, 0);
    });

    testWidgets('KYC and tier do not change create permission', (tester) async {
      for (final tier in [
        SellerTier.sellerBasic,
        SellerTier.sellerPro,
        SellerTier.sellerElite,
      ]) {
        final container = _container(
          authController: _FakeAuthController(
            AuthState.authenticated(
              _seller(
                id: 'seller-${tier.name}',
                username: 'seller-${tier.name}',
                hasSellerProfile: true,
                hasMarketAuthority: true,
                isIdVerified: false,
                isFarmVerified: false,
                sellerTier: tier,
              ),
              emailVerified: true,
            ),
          ),
          auctionNotifier: _FakeAuctionNotifier(),
        );
        addTearDown(container.dispose);

        await tester.pumpWidget(_wrap(container: container));
        await tester.pumpAndSettle();

        expect(find.text('Informasi Dasar'), findsOneWidget);
        expect(find.byType(TextFormField), findsWidgets);
      }
    });
  });

  group('CreateAuctionScreen discovery contract', () {
    // Sender requirements are no longer expressed as an address-editor route:
    // the screen gates on seller authority routes and collects the canonical
    // shipping setups that the createAuction contract requires
    // (`shippingSetupIds`). The former address-editor discovery assertions were
    // obsolete and are replaced by the current wiring below.
    test('wires seller gating and shipping through canonical routes and selector', () {
      final source = File(
        'lib/domains/commerce/catalog/auction/presentation/screens/create_auction_screen.dart',
      ).readAsStringSync();

      expect(source, contains('RoutePaths.sellerUpgrade'));
      expect(source, contains('RoutePaths.sellerRenewal'));
      expect(source, contains('SellerShippingSetupsSelector('));
      expect(source, contains('shippingSetupIds: _selectedShippingSetupIds'));
    });
  });
}
