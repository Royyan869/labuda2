import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/seller_tier.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

AuthUser _seller({
  required bool hasSellerProfile,
  required bool hasMarketAuthority,
}) {
  final now = DateTime.utc(2026, 1, 1);
  return AuthUser(
    id: 'seller-1',
    createdAt: now,
    updatedAt: now,
    email: 'seller@example.com',
    username: 'seller',
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

void main() {
  group('/create/auction router contract', () {
    test('active seller reaches the seller guard', () {
      final result = handleSellerRouteGuardForTest(
        _seller(hasSellerProfile: true, hasMarketAuthority: true),
        RoutePaths.createAuction,
      );

      expect(result, isNull);
    });

    test('expired seller is redirected to /seller/upgrade', () {
      final result = handleSellerRouteGuardForTest(
        _seller(hasSellerProfile: true, hasMarketAuthority: false),
        RoutePaths.createAuction,
      );

      expect(result, equals(RoutePaths.sellerUpgrade));
    });

    test('user without seller profile is redirected to /seller/upgrade', () {
      final result = handleSellerRouteGuardForTest(
        _seller(hasSellerProfile: false, hasMarketAuthority: false),
        RoutePaths.createAuction,
      );

      expect(result, equals(RoutePaths.sellerUpgrade));
    });

    test('null user is redirected to /seller/upgrade', () {
      final result = handleSellerRouteGuardForTest(
        null,
        RoutePaths.createAuction,
      );

      expect(result, equals(RoutePaths.sellerUpgrade));
    });
  });
}
