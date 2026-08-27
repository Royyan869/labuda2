// Stage 1B — Registration username format gate uses the canonical authority.
//
// Requirement 4: "Invalid local username format prevents inappropriate
// submission." Local registration validation may establish empty / invalid
// format / valid format — never a fabricated "available". This proves the
// registration field's validator (CanonicalUsernameValidator) rejects
// non-canonical input the same way the backend does, and that a merely valid
// format is NOT treated as "available" (availability stays backend authority).

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/helpers/canonical_username_validator.dart';

void main() {
  group('CanonicalUsernameValidator format gate (registration)', () {
    test('valid canonical username accepted', () {
      expect(CanonicalUsernameValidator.isValid('alice_01'), isTrue);
      expect(CanonicalUsernameValidator.normalizeAndValidate(' Alice_01 '),
          equals('alice_01'));
    });

    test('uppercase is normalized then accepted', () {
      // Backend normalizes to lowercase before validating — same as canonical.
      expect(CanonicalUsernameValidator.normalizeAndValidate('ALICE01'),
          equals('alice01'));
      expect(CanonicalUsernameValidator.isValid('ALICE01'), isTrue);
    });

    test('dashes and dots are rejected (backend rejects them too)', () {
      expect(CanonicalUsernameValidator.isValid('john-doe'), isFalse);
      expect(CanonicalUsernameValidator.isValid('john.doe'), isFalse);
    });

    test('too short / too long rejected', () {
      expect(CanonicalUsernameValidator.isValid('ab'), isFalse);
      expect(CanonicalUsernameValidator.isValid('a' * 31), isFalse);
    });

    test('reserved-looking name still passes local format (backend reserved '
        'authority)', () {
      // "available" is NOT claimed locally; the name passes format so it can
      // reach the backend, which is the reserved-name authority.
      expect(CanonicalUsernameValidator.isValid('labuda'), isTrue);
    });

    test('local regex never asserts availability', () {
      // A numeric/lowercase name is format-valid but that says nothing about
      // whether the backend will accept it (taken/reserved). The canonical
      // validator has no "available" concept — confirmation.
      expect(CanonicalUsernameValidator.isValid('moderator'), isTrue);
    });
  });
}