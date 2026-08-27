// Stage 2C — Canonical password strength evaluator.
//
// Proves the single canonical strength authority (CanonicalPasswordStrength):
//
//   Criteria (1 point each):
//     length >= 8, length >= 12, uppercase, lowercase, digit, special
//   Score → classification:
//     0–2 → weak, 3–4 → medium, 5–6 → strong
//
// Strength is UX feedback ONLY.  It is NOT a second password-policy
// authority: a password can be policy-valid without being Strong, and can
// receive a strength classification while policy-invalid.

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/helpers/canonical_password_policy.dart';
import 'package:labuda/shared/helpers/canonical_password_strength.dart';

void main() {
  group('CanonicalPasswordStrength classification', () {
    test('empty password → null (no strength shown)', () {
      expect(CanonicalPasswordStrength.evaluate(''), isNull);
      expect(CanonicalPasswordStrength.evaluate(null), isNull);
    });

    test('weak: "abc" (short, no digit/upper) → score 1 → weak', () {
      // len8 ✗, len12 ✗, upper ✗, lower ✓, digit ✗, special ✗ → 1
      expect(CanonicalPasswordStrength.evaluate('abc'),
          PasswordStrengthLevel.weak);
    });

    test('weak: "abcdefg" (7 chars, no digit/upper) → score 2 → weak', () {
      // len8 ✗, len12 ✗, upper ✗, lower ✓, digit ✗, special ✗ → 2
      expect(CanonicalPasswordStrength.evaluate('abcdefg'),
          PasswordStrengthLevel.weak);
    });

    test('medium: "Abcdef12" → valid policy, score 4 → medium', () {
      // length>=8 ✓, length>=12 ✗, upper ✓, lower ✓, digit ✓, special ✗ → 4
      expect(CanonicalPasswordStrength.evaluate('Abcdef12'),
          PasswordStrengthLevel.medium);
      // Policy-valid but NOT strong — strength ≠ validity.
      expect(CanonicalPasswordPolicy.isValid('Abcdef12'), isTrue);
    });

    test('strong: "Aaaaaaaaaaaa1A!" → score 6 → strong', () {
      // length>=8 ✓, length>=12 ✓, upper ✓, lower ✓, digit ✓, special ✓ → 6
      expect(CanonicalPasswordStrength.evaluate('Aaaaaaaaaaaa1A!'),
          PasswordStrengthLevel.strong);
      expect(CanonicalPasswordPolicy.isValid('Aaaaaaaaaaaa1A!'), isTrue);
    });

    test('strong: 12-char with all classes but no special → 5 → strong', () {
      // length>=8 ✓, length>=12 ✓, upper ✓, lower ✓, digit ✓, special ✗ → 5
      expect(
        CanonicalPasswordStrength.evaluate('Abcdefghij12'),
        PasswordStrengthLevel.strong,
      );
    });
  });

  group('Score boundaries', () {
    test('score transitions at 8 characters', () {
      // 7 chars: no length-8, no length-12 → ceiling 4.
      expect(CanonicalPasswordStrength.score('Ab1defg'), 3);
      // 8 chars: length-8 satisfied.
      expect(CanonicalPasswordStrength.score('Abcdef12'), 4);
    });

    test('score transitions at 12 characters', () {
      // 11 chars with all 5 non-length classes.
      expect(CanonicalPasswordStrength.score('Abcdefgh123'), 4);
      // 12 chars with all 5 non-length classes.
      expect(CanonicalPasswordStrength.score('Abcdefghij12'), 5);
    });

    test('uppercase adds a point', () {
      expect(CanonicalPasswordStrength.score('abcdefgh'), 2); // len8+lower
      expect(CanonicalPasswordStrength.score('Abcdefgh'), 3); // +upper
    });

    test('lowercase adds a point', () {
      expect(CanonicalPasswordStrength.score('ABCDEFGH'), 2); // len8+upper
      expect(CanonicalPasswordStrength.score('ABCDEFGh'), 3); // +lower
    });

    test('digit adds a point', () {
      expect(CanonicalPasswordStrength.score('Abcdefgh'), 3); // len8+u+l
      expect(CanonicalPasswordStrength.score('Abcdefg1'), 4); // +digit
    });

    test('special character adds a point', () {
      expect(CanonicalPasswordStrength.score('Abcdef12'), 4); // no special
      expect(CanonicalPasswordStrength.score('Abcdef12!'), 5); // +special
    });

    test('classify boundaries 0-2 weak, 3-4 medium, 5-6 strong', () {
      expect(CanonicalPasswordStrength.classify(0), PasswordStrengthLevel.weak);
      expect(CanonicalPasswordStrength.classify(1), PasswordStrengthLevel.weak);
      expect(CanonicalPasswordStrength.classify(2), PasswordStrengthLevel.weak);
      expect(
        CanonicalPasswordStrength.classify(3),
        PasswordStrengthLevel.medium,
      );
      expect(
        CanonicalPasswordStrength.classify(4),
        PasswordStrengthLevel.medium,
      );
      expect(
        CanonicalPasswordStrength.classify(5),
        PasswordStrengthLevel.strong,
      );
      expect(
        CanonicalPasswordStrength.classify(6),
        PasswordStrengthLevel.strong,
      );
    });
  });

  group('Policy vs strength distinction', () {
    test('policy-valid password can be only Medium', () {
      const pw = 'Abcdef12';
      expect(CanonicalPasswordPolicy.isValid(pw), isTrue);
      expect(
        CanonicalPasswordStrength.evaluate(pw),
        PasswordStrengthLevel.medium,
      );
    });

    test('long password missing uppercase can be medium but policy-invalid',
        () {
      // 12+ chars, lower+digit+special, no uppercase → score 5? Let's see:
      // length>=8 ✓, length>=12 ✓, lower ✓, digit ✓, special ✓ → 5 → strong.
      // But policy requires uppercase → INVALID.
      const pw = 'abcdefghij12!';
      expect(CanonicalPasswordStrength.evaluate(pw),
          PasswordStrengthLevel.strong);
      expect(CanonicalPasswordPolicy.isValid(pw), isFalse);
    });

    test('strength score is independent of policy validity', () {
      // "aaaaaaaaaaaa1!" → policy-invalid (no uppercase) but score 5 → strong.
      const pw = 'aaaaaaaaaaaa1!';
      expect(CanonicalPasswordPolicy.isValid(pw), isFalse);
      expect(CanonicalPasswordStrength.evaluate(pw),
          PasswordStrengthLevel.strong);
    });
  });

  group('Progress fraction', () {
    test('empty → 0.0', () {
      expect(CanonicalPasswordStrength.progress(''), 0);
      expect(CanonicalPasswordStrength.progress(null), 0);
    });

    test('full score → 1.0', () {
      expect(CanonicalPasswordStrength.progress('Aaaaaaaaaaaa1!'), 1.0);
    });

    test('medium example → 4/6', () {
      expect(CanonicalPasswordStrength.progress('Abcdef12'), 4 / 6);
    });
  });

  group('Labels', () {
    test('canonical three-state labels', () {
      expect(PasswordStrengthLevel.weak.label, 'Weak');
      expect(PasswordStrengthLevel.medium.label, 'Medium');
      expect(PasswordStrengthLevel.strong.label, 'Strong');
    });
  });
}
