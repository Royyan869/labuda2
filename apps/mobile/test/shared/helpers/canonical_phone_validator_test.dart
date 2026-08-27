// STAGE 4B-1: canonical Indonesian phone validator behavior.
//
// Owner/ChatGPT-locked rule:
// - Accepted prefixes: +62, 62, 0
// - 9-12 digits AFTER the applicable prefix
// - FORMAT ONLY — no E.164 normalization, no verification, no network.
//
// Prefix accounting:
//   +62  -> 3-char prefix, 9-12 digits follow
//   62   -> 2-char prefix, 9-12 digits follow
//   0    -> 1-char prefix, 9-12 digits follow
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/helpers/canonical_phone_validator.dart';

void main() {
  group('CanonicalPhoneValidator.isValid — prefix variants', () {
    test('accepts +62 prefix with 9-12 digits', () {
      expect(CanonicalPhoneValidator.isValid('+62812345678'), isTrue); // 9
      expect(CanonicalPhoneValidator.isValid('+628123456789'), isTrue); // 10
      expect(CanonicalPhoneValidator.isValid('+6281234567890'), isTrue); // 11
      expect(CanonicalPhoneValidator.isValid('+62812345678901'), isTrue); // 12
    });

    test('accepts 62 prefix with 9-12 digits', () {
      expect(CanonicalPhoneValidator.isValid('62812345678'), isTrue);
      expect(CanonicalPhoneValidator.isValid('62812345678901'), isTrue);
    });

    test('accepts 0 prefix with 9-12 digits', () {
      expect(CanonicalPhoneValidator.isValid('0812345678'), isTrue); // 9
      expect(CanonicalPhoneValidator.isValid('081234567890'), isTrue); // 11
      expect(CanonicalPhoneValidator.isValid('0812345678901'), isTrue); // 12
    });
  });

  group('CanonicalPhoneValidator.isValid — digit boundaries', () {
    test('rejects fewer than 9 digits after prefix', () {
      expect(CanonicalPhoneValidator.isValid('081234567'), isFalse); // 8
      expect(CanonicalPhoneValidator.isValid('+6281234567'), isFalse); // 8
      expect(CanonicalPhoneValidator.isValid('6281234567'), isFalse); // 8
    });

    test('rejects more than 12 digits after prefix', () {
      expect(CanonicalPhoneValidator.isValid('08123456789012'), isFalse); // 13
      expect(CanonicalPhoneValidator.isValid('+628123456789012'), isFalse);
    });

    test('rejects missing prefix', () {
      expect(CanonicalPhoneValidator.isValid('8123456789'), isFalse);
      expect(CanonicalPhoneValidator.isValid('12345678901'), isFalse);
    });

    test('rejects non-digit characters in the number body', () {
      expect(CanonicalPhoneValidator.isValid('0812345678a'), isFalse);
      expect(CanonicalPhoneValidator.isValid('+62812345678x'), isFalse);
    });
  });

  group('CanonicalPhoneValidator.isValid — formatting tolerance', () {
    test('accepts spaces/hyphens between digits', () {
      expect(CanonicalPhoneValidator.isValid('0812-3456-7890'), isTrue);
      expect(CanonicalPhoneValidator.isValid('0812 3456 7890'), isTrue);
    });

    test('trims surrounding whitespace', () {
      expect(CanonicalPhoneValidator.isValid(' 081234567890 '), isTrue);
    });
  });

  group('CanonicalPhoneValidator.isValid — empty/other', () {
    test('rejects null, empty, whitespace-only', () {
      expect(CanonicalPhoneValidator.isValid(null), isFalse);
      expect(CanonicalPhoneValidator.isValid(''), isFalse);
      expect(CanonicalPhoneValidator.isValid('   '), isFalse);
    });

    test('rejects foreign/non-Indonesian prefixes', () {
      expect(CanonicalPhoneValidator.isValid('+6512345678'), isFalse);
      expect(CanonicalPhoneValidator.isValid('+1 234 567 8901'), isFalse);
    });
  });

  group('CanonicalPhoneValidator.validationMessage', () {
    test('returns null for valid number', () {
      expect(
        CanonicalPhoneValidator.validationMessage('081234567890'),
        isNull,
      );
    });

    test('returns required message for empty input', () {
      expect(
        CanonicalPhoneValidator.validationMessage(''),
        'Phone number is required',
      );
    });

    test('returns invalid-format message for malformed input', () {
      expect(
        CanonicalPhoneValidator.validationMessage('0812345'),
        'Invalid phone number format',
      );
    });
  });
}
