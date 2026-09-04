/// Signup outcome binding tests — canonical SyncOutcome coverage.
///
/// SyncRequiresEmailVerification has been removed (Phase 6.2). Email
/// verification is NOT a SyncOutcome/AuthState — it is a property of
/// AuthStateAuthenticated (emailVerified) surfaced via banner/inline gate.
/// This file now proves only the canonical remaining variants.
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/sync_outcome.dart';

void main() {
  group('SyncOutcome — canonical variant coverage', () {
    test('SyncAuthenticated is a SyncOutcome', () {
      final user = AuthUser(
        id: 'u1',
        createdAt: DateTime(2025),
        updatedAt: DateTime(2025),
        email: 'e@e.com',
        username: 'test',
        isEmailVerified: true,
        accountStatus: AccountStatus.active,
        hasSellerProfile: false,
        sellerSubscriptionStatus: 'none',
        hasMarketAuthority: false,
        roles: const [UserRole.user],
        provider: AuthProvider.email,
      );
      final outcome = SyncAuthenticated(userId: user.id, email: user.email, emailVerified: true);
      expect(outcome, isA<SyncOutcome>());
      expect(outcome, isA<SyncAuthenticated>());
    });

    test('SyncRequiresProfileCompletion is a SyncOutcome', () {
      const outcome = SyncRequiresProfileCompletion(
        userId: 'u1',
        email: 'e@e.com',
      );
      expect(outcome, isA<SyncOutcome>());
    });

    test('SyncFailed is a SyncOutcome', () {
      const outcome = SyncFailed(error: 'test error');
      expect(outcome, isA<SyncOutcome>());
    });

    test('SyncAuthenticated != SyncRequiresProfileCompletion', () {
      final authOutcome = SyncAuthenticated(userId: 'u1', email: 'e@e.com', emailVerified: true);
      const profileOutcome = SyncRequiresProfileCompletion(
        userId: 'u1',
        email: 'e@e.com',
      );
      expect(authOutcome, isNot(profileOutcome));
    });
  });
}
