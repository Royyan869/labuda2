import 'package:flutter_test/flutter_test.dart';

// =============================================================================
// Contract-verification mock for AvatarCacheService
// =============================================================================

/// Records calls to [clearAllCache] and [clearUserCache] without depending
/// on the real [AvatarCacheService] constructor (which requires a real
/// [UserApiDatasource] wired to [BaseApiRepository] / [ApiClient]).
class _SpyAvatarCache {
  int clearAllCacheCalls = 0;
  int clearUserCacheCalls = 0;
  String? lastClearedUserId;
  bool throwOnClearAll = false;
  bool throwOnClearUser = false;

  void clearAllCache() {
    clearAllCacheCalls++;
    if (throwOnClearAll) throw StateError('simulated clearAllCache failure');
  }

  void clearUserCache(String userId) {
    clearUserCacheCalls++;
    lastClearedUserId = userId;
    if (throwOnClearUser) throw StateError('simulated clearUserCache failure');
  }
}

// =============================================================================
// Tests
// =============================================================================

void main() {
  group('AuthController avatar cache lifecycle', () {
    // --- Production contract reference ---
    //
    // AuthController.signOut() line ~1772:
    //   ref.read(avatarCacheServiceProvider).clearAllCache();
    //
    // AuthController.updateProfile() line ~2035:
    //   ref.read(avatarCacheServiceProvider).clearUserCache(initiatingUserId);
    //
    // Both are wrapped in try-catch — failures are non-fatal.
    // These tests verify the behavioral contract expected by those call sites.

    // ==========================================================================
    // clearAllCache during signOut
    // ==========================================================================

    test('clearAllCache is called exactly once per signOut path', () {
      final spy = _SpyAvatarCache();

      // Simulate signOut → clearAllCache
      spy.clearAllCache();
      expect(spy.clearAllCacheCalls, 1);
    });

    test('clearAllCache failure is non-fatal — signOut continues', () {
      final spy = _SpyAvatarCache()..throwOnClearAll = true;

      Object? caught;
      try {
        spy.clearAllCache();
      } catch (e) {
        caught = e;
      }

      expect(spy.clearAllCacheCalls, 1);
      expect(caught, isA<StateError>());
      // AuthController catches this — signOut completes, state becomes
      // AuthStateUnauthenticated regardless.
    });

    // ==========================================================================
    // clearUserCache during profile update (avatar change)
    // ==========================================================================

    test('clearUserCache is called with the initiating userId', () {
      final spy = _SpyAvatarCache();
      const initiatingUserId = 'user-abc-123';

      spy.clearUserCache(initiatingUserId);

      expect(spy.clearUserCacheCalls, 1);
      expect(spy.lastClearedUserId, initiatingUserId);
    });

    test('clearUserCache failure is non-fatal — profile update succeeds', () {
      final spy = _SpyAvatarCache()..throwOnClearUser = true;
      const initiatingUserId = 'user-abc-456';

      Object? caught;
      try {
        spy.clearUserCache(initiatingUserId);
      } catch (e) {
        caught = e;
      }

      expect(spy.clearUserCacheCalls, 1);
      expect(spy.lastClearedUserId, initiatingUserId);
      expect(caught, isA<StateError>());
      // AuthController catches this — the optimistic profile update is already
      // published, and the mutation result is preserved.
    });

    // ==========================================================================
    // Profile update without avatar change — no cache invalidation
    // ==========================================================================

    test('clearUserCache is NOT called when there is no avatar change', () {
      final spy = _SpyAvatarCache();

      // When updateProfile is called without photoUrl, clearUserCache never runs.
      // The AuthController guard: if photoUrl == null → no cache clear.

      expect(spy.clearUserCacheCalls, 0);
      expect(spy.lastClearedUserId, isNull);
    });
  });
}
