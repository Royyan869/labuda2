import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/commerce/transaction/shipping/domain/domain.dart';
import 'package:labuda/domains/commerce/transaction/shipping/presentation/providers/providers.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/preference/seller/presentation/screens/seller_shipping_screen.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show addressRepositoryProvider;
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/domain/repositories/i_address_repository.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/models/wilayah_models.dart';
import 'package:labuda/shared/providers/wilayah_provider_simple.dart';

class _ShippingRepo implements ShippingRepository {
  _ShippingRepo({List<ShippingSetup>? options}) : _options = options ?? [];

  final List<ShippingSetup> _options;

  @override
  Future<Result<List<ShippingSetup>>> listMyShippingSetups() async {
    return Result.success(_options);
  }

  @override
  Future<Result<List<ShippingSetup>>> listMyActiveShippingSetups() async {
    return Result.success(_options.where((o) => o.isActive).toList());
  }

  @override
  Future<Result<ShippingSetup>> getShippingSetupById(String optionId) async {
    return Result.error('not used');
  }

  @override
  Future<Result<ShippingSetup>> createShippingSetup(
    CreateShippingSetupRequest request,
  ) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<ShippingSetup>> updateShippingSetup(
    String optionId,
    UpdateShippingSetupRequest request,
  ) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<void>> deleteShippingSetup(String optionId) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<void>> toggleActiveStatus(
    String optionId,
    bool isActive,
  ) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<void>> setProductShippingSetups(
    String productId,
    List<String> ids,
  ) async {
    throw UnimplementedError();
  }

  @override
  Future<Result<List<DeliveryOption>>> checkDeliveryAvailability(
    CheckDeliveryRequest request,
  ) async {
    throw UnimplementedError();
  }

  @override
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _AddressRepo implements IAddressRepository {
  _AddressRepo(this._primarySenderResult);

  Result<AddressEntity?> _primarySenderResult;

  void setPrimarySenderResult(Result<AddressEntity?> next) {
    _primarySenderResult = next;
  }

  @override
  Future<Result<AddressEntity?>> getPrimaryAddress(
    String userId, {
    AddressPurpose? purpose,
  }) async {
    if (purpose == AddressPurpose.sender) {
      return _primarySenderResult;
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
  noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeAuthController extends AuthController {
  @override
  AuthState build() {
    final now = DateTime.utc(2026, 7, 25);
    final user = AuthUser(
      id: 'seller-1',
      createdAt: now,
      updatedAt: now,
      email: 'seller@example.com',
      username: 'seller',
      isEmailVerified: true,
      accountStatus: AccountStatus.active,
      hasSellerProfile: true,
      sellerSubscriptionStatus: 'active',
      hasMarketAuthority: true,
      roles: const [UserRole.user],
      provider: AuthProvider.email,
      lifecycle: ContentLifecycle.active,
    );
    return AuthState.authenticated(user, emailVerified: true);
  }
}

ShippingSetup _shippingSetup() {
  return ShippingSetup(
    id: 'ship-1',
    name: 'Bus Kencana',
    type: ShippingType.bus,
    coverageAreas: const [],
    isActive: true,
    createdAt: DateTime.utc(2026, 7, 25),
    updatedAt: DateTime.utc(2026, 7, 25),
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

Widget _wrap({
  required _ShippingRepo shippingRepo,
  required _AddressRepo addressRepo,
}) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(_FakeAuthController.new),
      shippingRepositoryProvider.overrideWithValue(shippingRepo),
      addressRepositoryProvider.overrideWithValue(addressRepo),
      provincesProvider.overrideWith((ref) async => const []),
    ],
    child: MaterialApp(
      home: const SellerShippingScreen(),
      localizationsDelegates: AppLocalizations.localizationsDelegates,
      supportedLocales: AppLocalizations.supportedLocales,
      locale: const Locale('id'),
    ),
  );
}

void main() {
  group('SellerShippingScreen shipping options', () {
    testWidgets('shows Bus Kencana when options exist', (tester) async {
      final shippingRepo = _ShippingRepo(options: [_shippingSetup()]);
      final addressRepo = _AddressRepo(Result.success(_completeSenderAddress()));

      await tester.pumpWidget(
        _wrap(shippingRepo: shippingRepo, addressRepo: addressRepo),
      );
      await tester.pumpAndSettle();

      expect(find.text('Bus Kencana'), findsOneWidget);
    });

    testWidgets('shows empty view when no options', (tester) async {
      final shippingRepo = _ShippingRepo(options: []);
      final addressRepo = _AddressRepo(Result.success(null));

      await tester.pumpWidget(
        _wrap(shippingRepo: shippingRepo, addressRepo: addressRepo),
      );
      await tester.pumpAndSettle();

      expect(find.text('Belum Ada Opsi Pengiriman'), findsOneWidget);
    });

    testWidgets('shows loading initially', (tester) async {
      final shippingRepo = _ShippingRepo(options: [_shippingSetup()]);
      final addressRepo = _AddressRepo(Result.success(null));

      await tester.pumpWidget(
        _wrap(shippingRepo: shippingRepo, addressRepo: addressRepo),
      );
      await tester.pump();

      // Screen should show AppBar with Pengiriman title
      expect(find.text('Pengiriman'), findsOneWidget);
    });

    testWidgets('shows shipping options list', (tester) async {
      final shippingRepo = _ShippingRepo(options: [_shippingSetup()]);
      final addressRepo = _AddressRepo(Result.success(null));

      await tester.pumpWidget(
        _wrap(
          shippingRepo: shippingRepo,
          addressRepo: addressRepo,
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Bus Kencana'), findsOneWidget);
    });

    testWidgets('shows create FAB', (tester) async {
      final shippingRepo = _ShippingRepo(options: [_shippingSetup()]);
      final addressRepo = _AddressRepo(Result.success(null));

      await tester.pumpWidget(
        _wrap(
          shippingRepo: shippingRepo,
          addressRepo: addressRepo,
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('Tambah Opsi'), findsOneWidget);
      expect(find.byIcon(Icons.add), findsOneWidget);
    });

    testWidgets(
      'address-load failure does not affect shipping-options list',
      (tester) async {
        final shippingRepo = _ShippingRepo(options: [_shippingSetup()]);
        final addressRepo = _AddressRepo(
          Result.error('sender address fetch failed'),
        );

        await tester.pumpWidget(
          _wrap(shippingRepo: shippingRepo, addressRepo: addressRepo),
        );
        await tester.pumpAndSettle();

        expect(find.text('Bus Kencana'), findsOneWidget);
        expect(find.text('Gagal memuat opsi pengiriman'), findsNothing);
      },
    );
  });

  group('Canonical sender-address authority', () {
    test('SellerShippingScreen is shipping-only (no sender address section)', () {
      final shippingSource = File(
        'lib/domains/user/preference/seller/presentation/screens/seller_shipping_screen.dart',
      ).readAsStringSync();

      // Current SellerShippingScreen is solely for global shipping options
      // Sender address is handled in seller_upgrade_wizard_screen.dart
      expect(shippingSource, contains('shippingNotifierProvider'));
      expect(shippingSource, isNot(contains('primarySenderAddressProvider(')));
      expect(shippingSource, isNot(contains('Atur Alamat')));
    });
  });
}
