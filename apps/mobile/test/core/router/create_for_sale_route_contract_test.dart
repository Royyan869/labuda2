import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';

AuthUser _activeSeller({
  bool isEmailVerified = true,
  bool? hasSellerProfile = true,
  String? sellerSubscriptionStatus = 'active',
  bool? hasMarketAuthority = true,
}) {
  final now = DateTime(2026, 1, 1);
  return AuthUser(
    id: 'seller-1',
    createdAt: now,
    updatedAt: now,
    email: 'seller@example.com',
    username: 'seller',
    isEmailVerified: isEmailVerified,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
    hasSellerProfile: hasSellerProfile,
    sellerSubscriptionStatus: sellerSubscriptionStatus,
    hasMarketAuthority: hasMarketAuthority,
  );
}

void main() {
  group('/create/for-sale router contract', () {
    test('unauthenticated deep link redirects to /welcome', () {
      final result = handleAuthRedirectForTest(
        const AuthState.unauthenticated(),
        AppAuthStatus.unauthenticated,
        '/create/for-sale',
      );

      expect(result, equals('/welcome'));
    });

    test('initializing deep link fails closed to /splash', () {
      final result = handleAuthRedirectForTest(
        const AuthState.loading(),
        AppAuthStatus.initializing,
        '/create/for-sale',
      );

      expect(result, equals('/splash'));
    });

    test('restricted account follows canonical restricted flow', () {
      final user = _activeSeller();
      final restricted = AuthState.accountRestricted(
        user,
        restrictionType: AccountStatus.suspended,
      );

      final result = handleAuthRedirectForTest(
        restricted,
        AppAuthStatus.accountRestricted,
        '/create/for-sale',
      );

      expect(result, equals(RoutePaths.accountRestricted));
    });

    test('active seller reaches the seller guard for /create/for-sale', () {
      final result = handleSellerRouteGuardForTest(
        _activeSeller(),
        '/create/for-sale',
      );

      expect(result, isNull);
    });
  });
}
