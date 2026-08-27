// STAGE 4B-1: canonical email validator behavior.
//
// Owner/ChatGPT-locked rule: deterministic format check only — accept normal
// valid user emails, reject clearly invalid input, no network/domain
// verification, no full RFC implementation.
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/helpers/canonical_email_validator.dart';

void main() {
  group('CanonicalEmailValidator.isValid', () {
    test('accepts normal valid user emails', () {
      const valid = [
        'user@example.com',
        'first.last@example.co.id',
        'user+tag@example.com',
        'User.Name@Example.com',
        'a@b.co',
        'name@sub.domain.org',
      ];
      for (final email in valid) {
        expect(CanonicalEmailValidator.isValid(email), isTrue, reason: email);
      }
    });

    test('rejects clearly invalid input', () {
      const invalid = [
        'plainaddress',
        'no-at-sign.com',
        'user@nodot',
        '@missing-local.com',
        'user@.com',
        'user@domain.',
        'user name@example.com', // space in local part
        'user@exa mple.com', // space in domain
        'user@@example.com',
      ];
      for (final email in invalid) {
        expect(CanonicalEmailValidator.isValid(email), isFalse, reason: email);
      }
    });

    test('rejects empty and whitespace-only input', () {
      expect(CanonicalEmailValidator.isValid(null), isFalse);
      expect(CanonicalEmailValidator.isValid(''), isFalse);
      expect(CanonicalEmailValidator.isValid('   '), isFalse);
    });

    test('trims surrounding whitespace before validating', () {
      expect(CanonicalEmailValidator.isValid('  user@example.com  '), isTrue);
    });
  });

  group('CanonicalEmailValidator.validationMessage', () {
    test('returns null for valid email', () {
      expect(
        CanonicalEmailValidator.validationMessage('user@example.com'),
        isNull,
      );
    });

    test('returns required message for empty input', () {
      expect(
        CanonicalEmailValidator.validationMessage(''),
        'Email is required',
      );
      expect(CanonicalEmailValidator.validationMessage(null), isNotNull);
    });

    test('returns invalid-format message for malformed input', () {
      expect(
        CanonicalEmailValidator.validationMessage('plainaddress'),
        'Invalid email format',
      );
    });
  });
}
