import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart'
    show addressRepositoryProvider;
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/domain/repositories/i_address_repository.dart';
import 'package:labuda/domains/user/profile/presentation/screens/address_list_screen.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

class _FakeAuthController extends AuthController {
  @override
  AuthState build() {
    final now = DateTime.utc(2026, 7, 26);
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
      provider: ShonaAuthProvider.email,
      lifecycle: ContentLifecycle.active,
    );
    return AuthState.authenticated(user, emailVerified: true);
  }
}

class _EmptyAddressRepository implements IAddressRepository {
  @override
  Future<Result<AddressEntity?>> getPrimaryAddress(
    String userId, {
    AddressPurpose? purpose,
  }) async {
    return Result.success(null);
  }

  @override
  Future<Result<List<AddressEntity>>> getAddressesByUserId(String userId) async {
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

Widget _wrap(Widget child) {
  return ProviderScope(
    overrides: [
      authControllerProvider.overrideWith(_FakeAuthController.new),
      addressRepositoryProvider.overrideWithValue(_EmptyAddressRepository()),
    ],
    child: MaterialApp(home: child),
  );
}

void main() {
  group('AddressListScreen initial tab', () {
    testWidgets('defaults to the shipping tab for the general route', (
      tester,
    ) async {
      await tester.pumpWidget(_wrap(const AddressListScreen()));
      await tester.pumpAndSettle();

      expect(find.text('No Shipping Address Yet'), findsOneWidget);
      expect(find.text('Add Shipping Address'), findsOneWidget);
      expect(find.text('No Sender Address Yet'), findsNothing);
      expect(find.text('Add Sender Address'), findsNothing);
    });

    testWidgets('seeds the sender tab when requested', (tester) async {
      await tester.pumpWidget(
        _wrap(
          const AddressListScreen(initialTab: AddressInitialTab.sender),
        ),
      );
      await tester.pumpAndSettle();

      expect(find.text('No Sender Address Yet'), findsOneWidget);
      expect(find.text('Add Sender Address'), findsOneWidget);
      expect(find.text('No Shipping Address Yet'), findsNothing);
      expect(find.text('Add Shipping Address'), findsNothing);
    });
  });

  group('Address route helpers', () {
    test('invalid or missing route arguments fall back safely', () {
      expect(parseAddressInitialTab(null), AddressInitialTab.shipping);
      expect(parseAddressInitialTab(''), AddressInitialTab.shipping);
      expect(parseAddressInitialTab('bogus'), AddressInitialTab.shipping);
      expect(
        RoutePaths.addressesWithInitialTab(AddressInitialTab.shipping),
        RoutePaths.addresses,
      );
      expect(
        RoutePaths.addressesWithInitialTab(AddressInitialTab.sender),
        '/profile/addresses?initialTab=sender',
      );
    });
  });
}
