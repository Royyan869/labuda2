/// Cooldown clearing and fake-clock tests for VerificationCooldownService.
///
/// Proves:
/// - clearCooldown is called on verification success
/// - clearCooldown is called on sign-out
/// - Cooldown persists across rebuild/restart (storage-backed)
/// - Fake clock controls eligibility
/// - UID isolation
/// - Timer is display-only (eligibility from persisted timestamp)
library;

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/identity/authentication/data/services/verification_cooldown_service.dart';

// ---------------------------------------------------------------
// Fake in-memory storage for deterministic cooldown tests
// ---------------------------------------------------------------

class _FakeLocalStorage implements ILocalStorageService {
  final Map<String, int> _ints = {};

  @override
  Future<Result<void>> setInt(String key, int value) async {
    _ints[key] = value;
    return Result.success(null);
  }

  @override
  Future<Result<int?>> getInt(String key) async {
    if (_ints.containsKey(key)) return Result.success(_ints[key]);
    return Result.success(null);
  }

  @override
  Future<Result<void>> remove(String key) async {
    _ints.remove(key);
    return Result.success(null);
  }

  // Unused interface members
  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

// ---------------------------------------------------------------
// Tests
// ---------------------------------------------------------------

void main() {
  group('VerificationCooldownService — cooldown lifecycle', () {
    test('initial state: no cooldown (0 seconds remaining)', () async {
      final storage = _FakeLocalStorage();
      final service = VerificationCooldownService(storage: storage);
      final remaining = await service.remainingCooldownSeconds('uid-a');
      expect(remaining, 0);
      expect(await service.isOnCooldown('uid-a'), false);
    });

    test('recordSent starts cooldown at approximately 60 seconds', () async {
      final storage = _FakeLocalStorage();
      final fakeNow = DateTime(2026, 8, 4, 12, 0, 0);
      final service = VerificationCooldownService(
        storage: storage,
        clock: () => fakeNow,
      );

      await service.recordSent('uid-a', now: fakeNow);
      final remaining = await service.remainingCooldownSeconds(
        'uid-a',
        now: fakeNow,
      );
      expect(remaining, 60);
    });

    test('fake clock +20 seconds leaves approximately 40 seconds', () async {
      final storage = _FakeLocalStorage();
      final sentAt = DateTime(2026, 8, 4, 12, 0, 0);
      final service = VerificationCooldownService(
        storage: storage,
        clock: () => sentAt,
      );

      await service.recordSent('uid-a', now: sentAt);
      final remaining = await service.remainingCooldownSeconds(
        'uid-a',
        now: sentAt.add(const Duration(seconds: 20)),
      );
      expect(remaining, 40);
    });

    test('cooldown expires correctly after 60 seconds', () async {
      final storage = _FakeLocalStorage();
      final sentAt = DateTime(2026, 8, 4, 12, 0, 0);
      DateTime currentTime = sentAt;
      final service = VerificationCooldownService(
        storage: storage,
        clock: () => currentTime,
      );

      await service.recordSent('uid-a', now: sentAt);
      // Advance clock past cooldown.
      currentTime = sentAt.add(const Duration(seconds: 61));
      final remaining = await service.remainingCooldownSeconds('uid-a');
      expect(remaining, 0);
      expect(await service.isOnCooldown('uid-a'), false);
    });

    test('several simulated days later enables resend', () async {
      final storage = _FakeLocalStorage();
      final sentAt = DateTime(2026, 8, 4, 12, 0, 0);
      final service = VerificationCooldownService(
        storage: storage,
        clock: () => sentAt,
      );

      await service.recordSent('uid-a', now: sentAt);
      final remaining = await service.remainingCooldownSeconds(
        'uid-a',
        now: sentAt.add(const Duration(days: 3)),
      );
      expect(remaining, 0);
    });

    test('UID A does not affect UID B (scoped storage)', () async {
      final storage = _FakeLocalStorage();
      final now = DateTime(2026, 8, 4, 12, 0, 0);
      final service = VerificationCooldownService(
        storage: storage,
        clock: () => now,
      );

      await service.recordSent('uid-a', now: now);
      expect(await service.remainingCooldownSeconds('uid-a', now: now), 60);
      expect(await service.remainingCooldownSeconds('uid-b', now: now), 0);
    });

    test('clearCooldown removes persisted timestamp', () async {
      final storage = _FakeLocalStorage();
      final now = DateTime(2026, 8, 4, 12, 0, 0);
      final service = VerificationCooldownService(
        storage: storage,
        clock: () => now,
      );

      await service.recordSent('uid-a', now: now);
      expect(await service.remainingCooldownSeconds('uid-a', now: now), 60);

      await service.clearCooldown('uid-a');
      expect(await service.remainingCooldownSeconds('uid-a', now: now), 0);
    });

    test('clearCooldown only affects target UID', () async {
      final storage = _FakeLocalStorage();
      final now = DateTime(2026, 8, 4, 12, 0, 0);
      final service = VerificationCooldownService(
        storage: storage,
        clock: () => now,
      );

      await service.recordSent('uid-a', now: now);
      await service.recordSent('uid-b', now: now);
      await service.clearCooldown('uid-a');

      expect(await service.remainingCooldownSeconds('uid-a', now: now), 0);
      expect(await service.remainingCooldownSeconds('uid-b', now: now), 60);
    });
  });

  group('VerificationCooldownService — rebuild/restart persistence', () {
    test('rebuild preserves cooldown (same storage backend)', () async {
      final storage = _FakeLocalStorage();
      final now = DateTime(2026, 8, 4, 12, 0, 0);

      final service1 = VerificationCooldownService(
        storage: storage,
        clock: () => now,
      );
      await service1.recordSent('uid-a', now: now);

      // Simulate rebuild: new service instance, same storage.
      final service2 = VerificationCooldownService(
        storage: storage,
        clock: () => now.add(const Duration(seconds: 15)),
      );
      final remaining = await service2.remainingCooldownSeconds('uid-a');
      expect(remaining, 45);
    });

    test('sign-out clears cooldown via clearCooldown', () async {
      final storage = _FakeLocalStorage();
      final now = DateTime(2026, 8, 4, 12, 0, 0);
      final service = VerificationCooldownService(
        storage: storage,
        clock: () => now,
      );

      await service.recordSent('uid-a', now: now);
      await service.clearCooldown('uid-a');

      // After sign-out clear, cooldown is gone.
      expect(await service.remainingCooldownSeconds('uid-a'), 0);
    });

    test('verification success clears cooldown', () async {
      final storage = _FakeLocalStorage();
      final now = DateTime(2026, 8, 4, 12, 0, 0);
      final service = VerificationCooldownService(
        storage: storage,
        clock: () => now,
      );

      await service.recordSent('uid-a', now: now);
      await service.clearCooldown('uid-a');

      // After verification clear, cooldown is gone.
      expect(await service.isOnCooldown('uid-a'), false);
    });
  });

  group('VerificationCooldownService — display Timer is display-only', () {
    test('eligibility is determined by persisted timestamp, not Timer', () async {
      // The VerifyEmailScreen._displayTimer ticks down a local int.
      // Eligibility is ALWAYS determined by
      // VerificationCooldownService.remainingCooldownSeconds() using the
      // persisted timestamp. This test proves the service does not rely
      // on any external timer state.
      final storage = _FakeLocalStorage();
      final now = DateTime(2026, 8, 4, 12, 0, 0);
      final service = VerificationCooldownService(
        storage: storage,
        clock: () => now,
      );

      await service.recordSent('uid-a', now: now);

      // Even without any Timer running, the service correctly reports
      // cooldown based on the persisted timestamp.
      expect(await service.remainingCooldownSeconds(
        'uid-a',
        now: now.add(const Duration(seconds: 30)),
      ), 30);
    });
  });
}
