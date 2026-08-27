// Stage 2B — Canonical Labuda password policy.
//
// Proves the canonical policy authority (CanonicalPasswordPolicy):
//
//   MinLength:  8
//   Requires:   at least one uppercase letter [A-Z]
//               at least one lowercase letter [a-z]
//               at least one digit [0-9]
//
// This is the APPLICATION policy. Firebase's weaker min-6 floor is NOT the
// business/security policy; the app must block policy-failing passwords
// before calling Firebase.
//
// It answers POLICY VALIDITY only — strength tiers / entropy / special
// characters are deliberately outside this authority (Stage 2C+).

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/helpers/canonical_password_policy.dart';

void main() {
  group('CanonicalPasswordPolicy validity', () {
    test('valid: "Abcdef12" satisfies the canonical policy', () {
      expect(CanonicalPasswordPolicy.isValid('Abcdef12'), isTrue);
      expect(CanonicalPasswordPolicy.validationMessage('Abcdef12'), isNull);
    });

    test('valid: exactly 8 chars with all required classes', () {
      expect(CanonicalPasswordPolicy.isValid('aB3defgh'), isTrue);
      expect(CanonicalPasswordPolicy.validationMessage('aB3defgh'), isNull);
    });

    test('valid: longer password with all required classes', () {
      expect(
        CanonicalPasswordPolicy.isValid('LongEnoughPassword123'),
        isTrue,
      );
      expect(
        CanonicalPasswordPolicy.validationMessage('LongEnoughPassword123'),
        isNull,
      );
    });

    test('invalid: fewer than 8 characters', () {
      expect(CanonicalPasswordPolicy.isValid('Abc12'), isFalse);
      expect(
        CanonicalPasswordPolicy.validationMessage('Abc12'),
        contains('at least 8'),
      );
    });

    test('invalid: exactly 7 characters is rejected', () {
      expect(CanonicalPasswordPolicy.isValid('Abc1234'), isFalse);
    });

    test('invalid: missing uppercase letter', () {
      expect(CanonicalPasswordPolicy.isValid('abcdef12'), isFalse);
      expect(
        CanonicalPasswordPolicy.validationMessage('abcdef12'),
        contains('uppercase'),
      );
    });

    test('invalid: missing lowercase letter', () {
      expect(CanonicalPasswordPolicy.isValid('ABCDEF12'), isFalse);
      expect(
        CanonicalPasswordPolicy.validationMessage('ABCDEF12'),
        contains('lowercase'),
      );
    });

    test('invalid: missing digit', () {
      expect(CanonicalPasswordPolicy.isValid('Abcdefgh'), isFalse);
      expect(
        CanonicalPasswordPolicy.validationMessage('Abcdefgh'),
        contains('digit'),
      );
    });

    test('invalid: empty string', () {
      expect(CanonicalPasswordPolicy.isValid(''), isFalse);
    });

    test('invalid: null', () {
      expect(CanonicalPasswordPolicy.isValid(null), isFalse);
    });

    test('invalid: whitespace-only is not valid', () {
      expect(CanonicalPasswordPolicy.isValid('        '), isFalse);
    });
  });

  group('CanonicalPasswordPolicy — boundary combinations', () {
    test('8 chars missing digit is invalid', () {
      expect(CanonicalPasswordPolicy.isValid('Abcdefgh'), isFalse);
    });

    test('8 chars with digit but missing case class is invalid', () {
      expect(CanonicalPasswordPolicy.isValid('abcdefg1'), isFalse);
      expect(CanonicalPasswordPolicy.isValid('ABCDEFG1'), isFalse);
    });

    test('multi-digit and multi-case still valid', () {
      expect(CanonicalPasswordPolicy.isValid('Passw0rdWith123'), isTrue);
    });

    test('unicode letters do not satisfy A-Z/a-z classes', () {
      // 'Äbcdéf12' has no ASCII uppercase/lowercase per the canonical regex.
      expect(CanonicalPasswordPolicy.isValid('Äbcdéf12'), isFalse);
    });
  });

  group('CanonicalPasswordPolicy — Firebase distinction', () {
    test('a 6-char Firebase-acceptable password is rejected by the app policy',
        () {
      // Firebase enforces min 6; Labuda policy is min 8 + classes.
      // The app MUST block this before Firebase is ever called.
      expect(CanonicalPasswordPolicy.isValid('Ab1def'), isFalse);
      expect(CanonicalPasswordPolicy.isValid('Abcdef'), isFalse);
    });
  });
}
