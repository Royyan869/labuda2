/// Verification Cooldown Service
///
/// Canonical owner of verification email delivery timestamp persistence,
/// cooldown eligibility, and remaining-duration computation.
///
/// Scoped to Firebase UID — account switch isolates cooldown state.
/// Injected clock supports deterministic testing without mocking DateTime.
///
/// This lives in the auth DATA layer (matching the barrel convention that
/// data/ is PRIVATE — accessed via Riverpod provider, not direct import).
library;

import 'package:labuda/core/core.dart';

/// Canonical cooldown duration for verification email resend.
const verificationCooldownDuration = Duration(seconds: 60);

/// Service that owns verification email cooldown persistence and eligibility.
class VerificationCooldownService {
  final ILocalStorageService _storage;
  final DateTime Function() _clock;

  VerificationCooldownService({
    required ILocalStorageService storage,
    DateTime Function()? clock,
  }) : _storage = storage,
       _clock = clock ?? (() => DateTime.now());

  /// Persist a successful-send timestamp for [uid].
  /// Scoped to the Firebase UID so switching accounts isolates cooldown.
  Future<void> recordSent(String uid, {DateTime? now}) async {
    final key = '${StorageKeys.lastVerificationEmailSentAt}_$uid';
    await _storage.setInt(key, (now ?? _clock()).millisecondsSinceEpoch);
  }

  /// Read the remaining cooldown seconds for [uid].
  /// Returns 0 when the key is absent, expired, or corrupt.
  Future<int> remainingCooldownSeconds(String uid, {DateTime? now}) async {
    final key = '${StorageKeys.lastVerificationEmailSentAt}_$uid';
    final result = await _storage.getInt(key);
    if (!result.isSuccess || result.data == null) return 0;

    final sentAt = DateTime.fromMillisecondsSinceEpoch(result.data!);
    final elapsed = (now ?? _clock()).difference(sentAt).inSeconds;
    final remaining = verificationCooldownDuration.inSeconds - elapsed;
    return remaining > 0 ? remaining : 0;
  }

  /// Whether [uid] is currently within the cooldown window.
  Future<bool> isOnCooldown(String uid) async {
    return await remainingCooldownSeconds(uid) > 0;
  }

  /// Clear the persisted timestamp for [uid].
  /// Called on: successful verification, sign-out, account deletion.
  Future<void> clearCooldown(String uid) async {
    final key = '${StorageKeys.lastVerificationEmailSentAt}_$uid';
    await _storage.remove(key);
  }
}
