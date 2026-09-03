import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:labuda/domains/user/profile/domain/entities/address_entity.dart';
import 'package:labuda/domains/user/profile/presentation/providers/profile_about_provider.dart'
    show resolveProfileLocation;
import 'package:labuda/shared/models/wilayah_models.dart';

AuthUser _testUser({
  required String id,
  required String username,
  required bool hasSellerProfile,
  bool? hasMarketAuthority,
}) {
  return AuthUser(
    id: id,
    createdAt: DateTime(2025, 1, 1),
    updatedAt: DateTime(2025, 1, 1),
    email: '$username@test.com',
    username: username,
    isEmailVerified: true,
    sellerTier: SellerTier.sellerBasic,
    hasSellerProfile: hasSellerProfile,
    hasMarketAuthority: hasMarketAuthority,
    sellerSubscriptionStatus: hasMarketAuthority == true ? 'active' : 'none',
    accountStatus: AccountStatus.active,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
  );
}

AddressEntity _address({
  required String id,
  required AddressPurpose purpose,
  required bool isPrimary,
  required String cityName,
  required String provinceName,
}) {
  return AddressEntity(
    id: id,
    userId: 'user-1',
    purpose: purpose,
    recipientName: 'Recipient',
    phone: '08123456789',
    province: Province(id: 'prov-1', name: provinceName),
    city: City(id: 'city-1', name: cityName, provinceId: 'prov-1'),
    district: District(id: 'dist-1', name: 'District', cityId: 'city-1'),
    village: Village(id: 'vill-1', name: 'Village', districtId: 'dist-1'),
    streetAddress: 'Street',
    postalCode: '12345',
    isPrimary: isPrimary,
    createdAt: DateTime(2025, 1, 1),
    updatedAt: DateTime(2025, 1, 1),
  );
}

void main() {
  group('resolveProfileLocation', () {
    test('canonical location wins for viewed seller profiles', () {
      final seller = _testUser(
        id: 'seller-1',
        username: 'sellerone',
        hasSellerProfile: true,
        hasMarketAuthority: true,
      );
      final addresses = [
        _address(
          id: 'sender-1',
          purpose: AddressPurpose.sender,
          isPrimary: true,
          cityName: 'Bogor',
          provinceName: 'West Java',
        ),
      ];

      final location = resolveProfileLocation(
        canonicalLocation: '  Depok, West Java  ',
        addresses: addresses,
        user: seller,
        isOwnProfile: false,
      );

      expect(location, 'Depok, West Java');
    });

    test('viewed seller stays hidden when canonical location is absent', () {
      final seller = _testUser(
        id: 'seller-2',
        username: 'sellertwo',
        hasSellerProfile: true,
        hasMarketAuthority: true,
      );
      final addresses = [
        _address(
          id: 'sender-1',
          purpose: AddressPurpose.sender,
          isPrimary: true,
          cityName: 'Bogor',
          provinceName: 'West Java',
        ),
      ];

      final location = resolveProfileLocation(
        canonicalLocation: null,
        addresses: addresses,
        user: seller,
        isOwnProfile: false,
      );

      expect(location, isNull);
    });

    test('own profile uses canonical location before address fallback', () {
      final buyer = _testUser(
        id: 'buyer-1',
        username: 'buyerone',
        hasSellerProfile: false,
        hasMarketAuthority: false,
      );
      final addresses = [
        _address(
          id: 'shipping-1',
          purpose: AddressPurpose.shipping,
          isPrimary: true,
          cityName: 'Depok',
          provinceName: 'West Java',
        ),
        _address(
          id: 'sender-1',
          purpose: AddressPurpose.sender,
          isPrimary: true,
          cityName: 'Bogor',
          provinceName: 'West Java',
        ),
      ];

      final location = resolveProfileLocation(
        canonicalLocation: 'Bandung, West Java',
        addresses: addresses,
        user: buyer,
        isOwnProfile: true,
      );

      expect(location, 'Bandung, West Java');
    });

    test(
      'own non-seller profile falls back to shipping address when canonical is absent',
      () {
        final buyer = _testUser(
          id: 'buyer-2',
          username: 'buyertwo',
          hasSellerProfile: false,
          hasMarketAuthority: false,
        );
        final addresses = [
          _address(
            id: 'shipping-1',
            purpose: AddressPurpose.shipping,
            isPrimary: true,
            cityName: 'Depok',
            provinceName: 'West Java',
          ),
        ];

        final location = resolveProfileLocation(
          canonicalLocation: null,
          addresses: addresses,
          user: buyer,
          isOwnProfile: true,
        );

        expect(location, 'Depok, West Java');
      },
    );

    test(
      'own seller profile falls back to sender address when canonical is absent',
      () {
        final seller = _testUser(
          id: 'seller-3',
          username: 'sellerthree',
          hasSellerProfile: true,
          hasMarketAuthority: true,
        );
        final addresses = [
          _address(
            id: 'sender-1',
            purpose: AddressPurpose.sender,
            isPrimary: true,
            cityName: 'Bogor',
            provinceName: 'West Java',
          ),
        ];

        final location = resolveProfileLocation(
          canonicalLocation: null,
          addresses: addresses,
          user: seller,
          isOwnProfile: true,
        );

        expect(location, 'Bogor, West Java');
      },
    );

    test('non-seller profile stays hidden to other viewers', () {
      final buyer = _testUser(
        id: 'buyer-3',
        username: 'buyerthree',
        hasSellerProfile: false,
        hasMarketAuthority: false,
      );
      final addresses = [
        _address(
          id: 'shipping-1',
          purpose: AddressPurpose.shipping,
          isPrimary: true,
          cityName: 'Depok',
          provinceName: 'West Java',
        ),
      ];

      final location = resolveProfileLocation(
        canonicalLocation: null,
        addresses: addresses,
        user: buyer,
        isOwnProfile: false,
      );

      expect(location, isNull);
    });
  });
}
