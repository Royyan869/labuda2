// C1B3 — Reserved-name availability contract (canonical form).
//
// Verifies the end-to-end contract after removing the mobile reserved list
// AND the advisory availability pre-check service:
//  1. Backend-reserved usernames (e.g. "labuda") pass LOCAL format validation
//     — mobile must NOT reject them from any local authority.
//  2. Formerly-mobile-only reserved names (e.g. "moderator") also pass — the
//     backend alone decides reserved-ness, at the transactional moment
//     (/auth/firebase/exchange or /auth/complete-profile).
//  3. Invalid format (e.g. "john-doe") is rejected locally via the canonical
//     validator — the only local username authority (format-only).
//
// Availability is NEVER claimed locally anywhere in the app: the backend's
// structured rejections (USERNAME_TAKEN / USERNAME_RESERVED) surface INLINE
// via registrationUsernameErrorMessage / ProfileCompletionOutcome on the same
// screen that collected the username.

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/helpers/canonical_username_validator.dart';

void main() {
  group('C1B3 Reserved-name contract (backend = single authority)', () {
    test('backend-reserved "labuda" passes local canonical format validation', () {
      // The local validator is FORMAT-ONLY: it must let backend-reserved
      // names through so the backend alone can reject them at submit time.
      expect(CanonicalUsernameValidator.isValid('labuda'), isTrue);
    });

    test('former mobile-only "moderator" passes local format validation', () {
      // "moderator" was in the old mobile reserved list but not in the
      // backend's. Local validation must not resurrect that second authority.
      expect(CanonicalUsernameValidator.isValid('moderator'), isTrue);
    });

    test('"john-doe" is rejected locally by the canonical validator', () {
      expect(CanonicalUsernameValidator.isValid('john-doe'), isFalse);
    });

    test('normalize lowercases and strips "@" — one canonical shape', () {
      expect(CanonicalUsernameValidator.normalize('@Labuda_01'), 'labuda_01');
    });

    test('empty / whitespace-only input normalizes to null (invalid)', () {
      expect(CanonicalUsernameValidator.normalize('   '), isNull);
      expect(CanonicalUsernameValidator.isValid('   '), isFalse);
    });
  });
}
