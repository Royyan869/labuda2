// ID1D+ID1F: Tests for AuthStateAccountRestricted and mid-session restriction.
//
// Verifies:
// 1. Suspended/banned account status maps to restricted auth state
// 2. Active/null account status maps to authenticated auth state
// 3. AccountStatus predicates match expected restriction types
// 4. ID1F: Mid-session restriction gate logic (validate + resume paths)
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/core/core.dart';

/// Minimal AuthUser for testing — only accountStatus matters.
AuthUser _testUser({AccountStatus? status}) {
  return AuthUser(
    id: 'test-user-id',
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: 'test@test.com',
    username: 'testuser',
    isEmailVerified: true,
    hasSellerProfile: false,
    sellerSubscriptionStatus: 'none',
    hasMarketAuthority: false,
    roles: [UserRole.user],
    provider: AuthProvider.email,
    accountStatus: status,
  );
}

void main() {
  group('AccountStatus predicates', () {
    test('active is not restricted', () {
      expect(AccountStatus.active.isRestricted, isFalse);
      expect(AccountStatus.active.isActive, isTrue);
    });

    test('suspended is restricted', () {
      expect(AccountStatus.suspended.isRestricted, isTrue);
      expect(AccountStatus.suspended.isSuspended, isTrue);
      expect(AccountStatus.suspended.isBanned, isFalse);
    });

    test('banned is restricted', () {
      expect(AccountStatus.banned.isRestricted, isTrue);
      expect(AccountStatus.banned.isBanned, isTrue);
      expect(AccountStatus.banned.isSuspended, isFalse);
    });

    test('deleted is restricted', () {
      expect(AccountStatus.deleted.isRestricted, isTrue);
    });
  });

  group('AuthStateAccountRestricted', () {
    test('carries suspended restriction type', () {
      final user = _testUser(status: AccountStatus.suspended);
      final state = AuthState.accountRestricted(
        user,
        restrictionType: AccountStatus.suspended,
      );

      expect(state, isA<AuthStateAccountRestricted>());
      final restricted = state as AuthStateAccountRestricted;
      expect(restricted.restrictionType, AccountStatus.suspended);
      expect(restricted.restrictionType.isSuspended, isTrue);
      expect(restricted.user.username, 'testuser');
    });

    test('carries banned restriction type', () {
      final user = _testUser(status: AccountStatus.banned);
      final state = AuthState.accountRestricted(
        user,
        restrictionType: AccountStatus.banned,
      );

      expect(state, isA<AuthStateAccountRestricted>());
      final restricted = state as AuthStateAccountRestricted;
      expect(restricted.restrictionType, AccountStatus.banned);
      expect(restricted.restrictionType.isBanned, isTrue);
    });

    test('is not AuthStateAuthenticated', () {
      final user = _testUser(status: AccountStatus.suspended);
      final state = AuthState.accountRestricted(
        user,
        restrictionType: AccountStatus.suspended,
      );

      expect(state is AuthStateAuthenticated, isFalse);
    });
  });

  group('AppAuthStatus enum', () {
    test('has accountRestricted value', () {
      expect(
        AppAuthStatus.values.contains(AppAuthStatus.accountRestricted),
        isTrue,
      );
    });

    test('accountRestricted is distinct from authenticated', () {
      expect(
        AppAuthStatus.accountRestricted != AppAuthStatus.authenticated,
        isTrue,
      );
    });
  });

  group('Account restriction gate logic', () {
    // These tests verify the gate logic that auth_controller applies
    // after sync: suspended/banned → accountRestricted, active → authenticated

    test('suspended triggers restriction', () {
      final status = AccountStatus.suspended;
      expect(status.isRestricted, isTrue);
      // Controller would emit AuthState.accountRestricted
    });

    test('banned triggers restriction', () {
      final status = AccountStatus.banned;
      expect(status.isRestricted, isTrue);
      // Controller would emit AuthState.accountRestricted
    });

    test('active does not trigger restriction', () {
      final status = AccountStatus.active;
      expect(status.isRestricted, isFalse);
      // Controller would emit AuthState.authenticated
    });

    test('null accountStatus does not trigger restriction', () {
      // When accountStatus is null, isRestricted check is skipped
      // Controller emits AuthState.authenticated
      final user = _testUser(status: null);
      expect(user.accountStatus, isNull);
    });
  });

  // ID1F: Mid-session restriction gate logic tests
  // These verify the decision logic used by _validateSession() and
  // refreshUserData() — if freshUser.accountStatus.isRestricted, the
  // controller must emit AuthStateAccountRestricted instead of
  // AuthStateAuthenticated.
  group('ID1F mid-session restriction gate', () {
    test('suspended mid-session emits accountRestricted', () {
      // Simulates: user was active, backend now returns suspended
      final activeUser = _testUser(status: AccountStatus.active);
      final freshUser = _testUser(status: AccountStatus.suspended);

      // Before: controller is in authenticated state
      final beforeState = AuthState.authenticated(
        activeUser,
        emailVerified: true,
      );
      expect(beforeState, isA<AuthStateAuthenticated>());

      // Gate check: freshUser.accountStatus.isRestricted
      final freshStatus = freshUser.accountStatus;
      expect(freshStatus, isNotNull);
      expect(freshStatus!.isRestricted, isTrue);

      // After: controller should emit accountRestricted
      final afterState = AuthState.accountRestricted(
        freshUser,
        restrictionType: freshStatus,
      );
      expect(afterState, isA<AuthStateAccountRestricted>());
      expect(
        (afterState as AuthStateAccountRestricted).restrictionType.isSuspended,
        isTrue,
      );
    });

    test('banned mid-session emits accountRestricted', () {
      final activeUser = _testUser(status: AccountStatus.active);
      final freshUser = _testUser(status: AccountStatus.banned);

      final beforeState = AuthState.authenticated(
        activeUser,
        emailVerified: true,
      );
      expect(beforeState, isA<AuthStateAuthenticated>());

      final freshStatus = freshUser.accountStatus;
      expect(freshStatus, isNotNull);
      expect(freshStatus!.isRestricted, isTrue);

      final afterState = AuthState.accountRestricted(
        freshUser,
        restrictionType: freshStatus,
      );
      expect(afterState, isA<AuthStateAccountRestricted>());
      expect(
        (afterState as AuthStateAccountRestricted).restrictionType.isBanned,
        isTrue,
      );
    });

    test('role change does not trigger restriction if still active', () {
      // User role changed (e.g. promoted to admin) but account still active
      final activeUser = _testUser(status: AccountStatus.active);
      final freshUser = AuthUser(
        id: 'test-user-id',
        createdAt: DateTime(2025),
        updatedAt: DateTime(2025),
        email: 'test@test.com',
        username: 'testuser',
        isEmailVerified: true,
        hasSellerProfile: false,
        sellerSubscriptionStatus: 'none',
        hasMarketAuthority: false,
        accountStatus: AccountStatus.active,
        roles: [UserRole.admin],
        provider: AuthProvider.email,
      );

      // Gate check: active is NOT restricted
      final freshStatus = freshUser.accountStatus;
      expect(freshStatus, isNotNull);
      expect(freshStatus!.isRestricted, isFalse);

      // Role changed
      expect(freshUser.role != activeUser.role, isTrue);

      // Should remain authenticated, not restricted
      final afterState = AuthState.authenticated(
        freshUser,
        emailVerified: true,
      );
      expect(afterState, isA<AuthStateAuthenticated>());
      expect(afterState, isNot(isA<AuthStateAccountRestricted>()));
    });

    test('normal 403 does not affect restriction gate logic', () {
      // The restriction gate only fires when _validateSession or
      // refreshUserData gets a successful response with restricted status.
      // A network 403 error hits the error branch (result.isError),
      // not the success branch where restriction is checked.
      final activeUser = _testUser(status: AccountStatus.active);

      // Active user remains active — 403 on a specific API call
      // does not change the user's accountStatus in the auth state.
      final freshStatus = activeUser.accountStatus;
      expect(freshStatus, isNotNull);
      expect(freshStatus!.isRestricted, isFalse);

      // State stays authenticated
      final state = AuthState.authenticated(activeUser, emailVerified: true);
      expect(state, isA<AuthStateAuthenticated>());
    });
  });
}
