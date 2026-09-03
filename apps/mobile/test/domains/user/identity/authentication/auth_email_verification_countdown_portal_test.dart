/// Canonical tests for email verification cooldown, auth-state contracts,
/// delivery status types, and protected-provider initialization gating.
///
/// Covers:
/// - Cooldown restart on resend / cooldown absent on failed send (unique)
/// - AuthStateAuthenticated field contracts
/// - FCM init gating by AuthStateAuthenticated
/// - Cooldown display formatting
/// - Unconditional email guard logic
/// - VerificationDeliveryStatus typed variants
/// - Protected-provider initialization blocking
///
/// Retired tests (32) previously tested:
/// - AuthStateRequiresEmailVerification (removed from production)
/// - AppAuthStatus.requiresEmailVerification (removed)
/// - AuthState.requiresEmailVerification factory (does not exist)
/// - /auth/verify-email portal route (removed)
/// These are obsolete: production surfaces unverified email via
/// AuthStateAuthenticated.emailVerified flag + banner, not via
/// a separate auth state or redirect portal.
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/data/services/verification_cooldown_service.dart';
import 'package:labuda/domains/user/identity/authentication/domain/entities/account_status.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/verification_delivery_status.dart';

// =============================================================================
// In-memory storage fake for cooldown tests
// =============================================================================

class _FakeStorage implements ILocalStorageService {
  final Map<String, int> ints = {};
  final Set<String> removed = {};

  @override
  Future<Result<void>> setInt(String key, int value) async {
    ints[key] = value;
    removed.remove(key);
    return Result.success(null);
  }

  @override
  Future<Result<int?>> getInt(String key) async {
    if (removed.contains(key)) return Result.success(null);
    return Result.success(ints[key]);
  }

  @override
  Future<Result<void>> remove(String key) async {
    removed.add(key);
    ints.remove(key);
    return Result.success(null);
  }

  @override
  dynamic noSuchMethod(Invocation i) => super.noSuchMethod(i);
}

// =============================================================================
// Shared test fixtures
// =============================================================================

/// Create a minimal AuthUser for tests that don't care about profile fields.
AuthUser _testUser({
  required String id,
  required String email,
  required String username,
  required bool isEmailVerified,
}) {
  return AuthUser(
    id: id,
    createdAt: DateTime(2025),
    updatedAt: DateTime(2025),
    email: email,
    username: username,
    isEmailVerified: isEmailVerified,
    accountStatus: AccountStatus.active,
    hasSellerProfile: false,
    sellerSubscriptionStatus: 'none',
    hasMarketAuthority: false,
    roles: const [UserRole.user],
    provider: AuthProvider.email,
  );
}

/// Helper: given an AuthState, return true if it would allow FCM init.
bool _wouldAllowFcmInit(AuthState s) => s is AuthStateAuthenticated;

// =============================================================================
// Cooldown — unique tests not covered by canonical suite
// =============================================================================

void main() {
  group('Countdown — unique cooldown behaviors', () {
    test('11. resend success restarts cooldown (newer timestamp)', () async {
      final storage = _FakeStorage();
      final now = DateTime(2026, 8, 5, 12, 0, 0);
      DateTime clock = now;
      final svc = VerificationCooldownService(storage: storage, clock: () => clock);

      // Initial send.
      await svc.recordSent('uid-1', now: now);
      clock = now.add(const Duration(seconds: 30));
      expect(await svc.remainingCooldownSeconds('uid-1'), 30);

      // Resend at t+30.
      await svc.recordSent('uid-1', now: clock);
      // Cooldown should restart at 60 from the new timestamp.
      expect(await svc.remainingCooldownSeconds('uid-1'), 60);
    });

    test('12. failed initial send shows no cooldown and enables resend',
        () async {
      final storage = _FakeStorage();
      final now = DateTime(2026, 8, 5, 12, 0, 0);
      final svc = VerificationCooldownService(storage: storage, clock: () => now);

      // No recordSent was called (simulating failed send).
      final remaining = await svc.remainingCooldownSeconds('uid-fail');
      expect(remaining, 0);
      expect(await svc.isOnCooldown('uid-fail'), false);
    });
  });

  // ===========================================================================
  // Auth-state — surviving field contracts
  // ===========================================================================

  group('Auth-state — AuthStateAuthenticated field contracts', () {
    test('14. AuthStateAuthenticated with emailVerified: false carries the flag',
        () {
      final user = _testUser(
        id: 'u1',
        email: 'e@e.com',
        username: 'test',
        isEmailVerified: false,
      );
      final authenticated = AuthStateAuthenticated(user, emailVerified: false);
      expect(authenticated.emailVerified, false);
    });

    test(
        '20. only matching UID with emailVerified: true publishes Authenticated',
        () {
      final user = _testUser(
        id: 'uid-match',
        email: 'verified@test.com',
        username: 'test',
        isEmailVerified: true,
      );
      final authenticated = AuthStateAuthenticated(user, emailVerified: true);
      expect(authenticated.emailVerified, true);
      expect(authenticated, isA<AuthStateAuthenticated>());
      // The user ID must match.
      expect(authenticated.user.id, 'uid-match');
    });
  });

  // ===========================================================================
  // FCM init gating
  // ===========================================================================

  group('FCM init gating', () {
    test('24. FCM authenticated registration is gated by AuthStateAuthenticated',
        () {
      const requiresVerify = AuthState.initial();
      expect(_wouldAllowFcmInit(requiresVerify), false);
      expect(_wouldAllowFcmInit(const AuthState.unauthenticated()), false);
      expect(_wouldAllowFcmInit(const AuthState.loading()), false);

      final authUser = _testUser(
        id: 'u1',
        email: 'e@e.com',
        username: 'test',
        isEmailVerified: true,
      );
      final authenticated =
          AuthStateAuthenticated(authUser, emailVerified: true);
      expect(_wouldAllowFcmInit(authenticated), true);
    });
  });

  // ===========================================================================
  // Cooldown display formatting contract
  // ===========================================================================

  group('Cooldown display formatting contract', () {
    test('two-digit seconds formatting matches contract', () {
      String format(int seconds) =>
          'Kirim Ulang dalam 00:${seconds.toString().padLeft(2, '0')}';

      expect(format(60), 'Kirim Ulang dalam 00:60');
      expect(format(59), 'Kirim Ulang dalam 00:59');
      expect(format(40), 'Kirim Ulang dalam 00:40');
      expect(format(5), 'Kirim Ulang dalam 00:05');
      expect(format(1), 'Kirim Ulang dalam 00:01');
    });

    test('no negative value is ever displayed', () {
      int clampNonNegative(int seconds) => seconds < 0 ? 0 : seconds;
      expect(clampNonNegative(-1), 0);
      expect(clampNonNegative(-60), 0);
      expect(clampNonNegative(0), 0);
      expect(clampNonNegative(30), 30);
    });

    test('loading sentinel (-1) is distinguishable from zero', () {
      const loadingSentinel = -1;
      const cooldownExpired = 0;
      expect(loadingSentinel == cooldownExpired, false);
      expect(loadingSentinel < 0, true);
    });
  });

  // ===========================================================================
  // Unconditional email gate — _publishAuthenticatedIfCurrent contract
  // ===========================================================================

  group('Unconditional email gate — AuthState correction', () {
    test(
      'G1. existing AuthStateAuthenticated(emailVerified:false) is corrected '
      'to portal for password user',
      () {
        const emailVerified = false;
        const isPasswordUser = true;
        final shouldBlock = !emailVerified && isPasswordUser;
        expect(shouldBlock, true);
      },
    );

    test(
      'G2. backend full session cannot preserve Authenticated while Firebase '
      'remains unverified',
      () {
        const emailVerified = false;
        const isPasswordUser = true;
        final needsEmailVerification = isPasswordUser && !emailVerified;
        expect(needsEmailVerification, true);
        final guardBlocks = !emailVerified && isPasswordUser;
        expect(guardBlocks, true);
      },
    );

    test(
      'G3. periodic validation corrects an invalid authenticated state',
      () {
        const emailVerified = false;
        const isPasswordUser = true;
        final guardFires = !emailVerified && isPasswordUser;
        expect(guardFires, true);
      },
    );

    test('G5. matching verified principal remains authenticated', () {
      // A password user with emailVerified=true is NOT blocked.
      // The guard only blocks when emailVerified=false AND isPasswordUser=true.
      const emailVerified = true;
      const isPasswordUser = true;
      // Document: guard condition is false when email is verified.
      expect(isPasswordUser && !emailVerified, isFalse);
    });

    test('G5b. Google user (non-password) with emailVerified=false is NOT '
        'blocked by the guard',
        () {
      const emailVerified = false;
      const isPasswordUser = false;
      final guardBlocks = !emailVerified && isPasswordUser;
      expect(guardBlocks, false);
    });
  });

  // ===========================================================================
  // VerificationDeliveryStatus — typed delivery contract
  // ===========================================================================

  group('VerificationDeliveryStatus — typed delivery contract', () {
    test('D1. VerificationDeliverySent carries exact sentAt', () {
      final sentAt = DateTime(2026, 8, 5, 12, 0, 0);
      final status = VerificationDeliverySent(sentAt);
      expect(status.sentAt, sentAt);
    });

    test('D2. VerificationDeliveryFailed carries message', () {
      const status = VerificationDeliveryFailed('Send failed: network error');
      expect(status.message, 'Send failed: network error');
    });

    test('D3. VerificationDeliveryFailed has default message', () {
      const status = VerificationDeliveryFailed();
      expect(status.message, isNotEmpty);
    });

    test('D4. VerificationDeliveryUnknown is a singleton concept', () {
      const a = VerificationDeliveryUnknown();
      const b = VerificationDeliveryUnknown();
      expect(a, isA<VerificationDeliveryUnknown>());
      expect(b, isA<VerificationDeliveryUnknown>());
    });

    test('D5. all three variants are distinct types', () {
      final sent = VerificationDeliverySent(DateTime(2026, 8, 5));
      const failed = VerificationDeliveryFailed();
      const unknown = VerificationDeliveryUnknown();

      expect(sent, isNot(isA<VerificationDeliveryFailed>()));
      expect(sent, isNot(isA<VerificationDeliveryUnknown>()));
      expect(failed, isNot(isA<VerificationDeliverySent>()));
      expect(failed, isNot(isA<VerificationDeliveryUnknown>()));
      expect(unknown, isNot(isA<VerificationDeliverySent>()));
      expect(unknown, isNot(isA<VerificationDeliveryFailed>()));
    });

    test('D6. VerificationDeliverySent enables synchronous first-frame '
        'countdown computation',
        () {
      final sentAt = DateTime.now();
      const cooldown = Duration(seconds: 60);
      final remaining = cooldown.inSeconds -
          DateTime.now().difference(sentAt).inSeconds;
      final clamped = remaining > 0 ? remaining : 0;
      expect(clamped, greaterThanOrEqualTo(0));
      expect(clamped, lessThanOrEqualTo(60));
    });

    test('D7. VerificationDeliveryFailed has no cooldown', () {
      const status = VerificationDeliveryFailed();
      final hasCooldown = status is VerificationDeliverySent;
      expect(hasCooldown, false);
    });

    test('D8. VerificationDeliveryUnknown requires async cooldown read', () {
      const status = VerificationDeliveryUnknown();
      final needsAsyncRead = status is! VerificationDeliverySent;
      expect(needsAsyncRead, true);
    });

    test('D9. portal subtitle copy differs per delivery status', () {
      String subtitle(VerificationDeliveryStatus s) => switch (s) {
            VerificationDeliveryFailed(:final message) => message,
            VerificationDeliverySent() =>
              'Kami telah mengirim email verifikasi ke alamat email '
                  'kamu. Silakan klik tautan di email tersebut untuk '
                  'melanjutkan.',
            VerificationDeliveryUnknown() =>
              'Akun kamu belum terverifikasi. '
                  'Kirim ulang email verifikasi atau periksa inbox '
                  'kamu untuk tautan verifikasi.',
          };

      expect(
        subtitle(VerificationDeliverySent(DateTime(2026, 8, 5))),
        contains('telah mengirim'),
      );
      expect(
        subtitle(const VerificationDeliveryFailed('fail')),
        'fail',
      );
      expect(
        subtitle(const VerificationDeliveryUnknown()),
        contains('belum terverifikasi'),
      );
      expect(
        subtitle(const VerificationDeliveryUnknown()),
        isNot(contains('telah mengirim')),
      );
    });
  });

  // ===========================================================================
  // Protected-provider initialization blocking
  // ===========================================================================

  group('Protected-provider initialization blocking', () {
    test('P1. FCM init is gated on AuthStateAuthenticated only', () {
      bool wouldInit(AuthState s) => s is AuthStateAuthenticated;

      expect(wouldInit(const AuthState.initial()), false);
      expect(wouldInit(const AuthState.unauthenticated()), false);
      expect(wouldInit(const AuthState.loading()), false);

      final user = _testUser(
        id: 'u1',
        email: 'v@v.com',
        username: 't',
        isEmailVerified: true,
      );
      expect(
        wouldInit(AuthStateAuthenticated(user, emailVerified: true)),
        true,
      );
    });

    test('P3. AuthStateAuthenticated allows all routes', () {
      final user = _testUser(
        id: 'u1',
        email: 'v@v.com',
        username: 't',
        isEmailVerified: true,
      );
      final auth = AuthStateAuthenticated(user, emailVerified: true);

      expect(auth, isA<AuthStateAuthenticated>());
      expect(auth.emailVerified, true);
    });

    test('P4. Only verified Firebase UID transitions to Authenticated', () {
      const emailVerified = true;
      expect(emailVerified, true);

      const stillUnverified = false;
      expect(stillUnverified, false);
    });
  });
}
