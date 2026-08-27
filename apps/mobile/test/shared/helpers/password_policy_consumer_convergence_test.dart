// Stage 2B — Password policy consumer convergence.
//
// Proves that production password-policy consumers delegate to the canonical
// Labuda policy (CanonicalPasswordPolicy) instead of enforcing a divergent
// local rule:
//
//   1. ValidationService.validatePassword delegates to the canonical policy.
//   2. Register (sign_up_screen) password validator + submit gate use the
//      canonical policy.
//   3. Change Password (security_screen) new-password validator uses the
//      canonical policy.
//   4. Login does NOT apply the registration password policy (existing
//      credentials predate it; Firebase is the acceptance authority).

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/services/validation_service.dart';

String _readSource(String relativePath) =>
    File(relativePath).readAsStringSync();

void main() {
  group('ValidationService.validatePassword delegates to canonical policy', () {
    const service = ValidationService();

    test('valid password passes', () async {
      final result = await service.validatePassword('Abcdef12');
      expect(result.isSuccess, isTrue);
    });

    test('short password rejected (canonical message)', () async {
      final result = await service.validatePassword('Abc12');
      expect(result.isError, isTrue);
      expect(result.error, contains('at least 8'));
    });

    test('missing digit rejected', () async {
      final result = await service.validatePassword('Abcdefgh');
      expect(result.isError, isTrue);
      expect(result.error, contains('digit'));
    });

    test('missing uppercase rejected', () async {
      final result = await service.validatePassword('abcdef12');
      expect(result.isError, isTrue);
      expect(result.error, contains('uppercase'));
    });

    test('missing lowercase rejected', () async {
      final result = await service.validatePassword('ABCDEF12');
      expect(result.isError, isTrue);
      expect(result.error, contains('lowercase'));
    });

    test('6-char password rejected (Firebase min-6 is not Labuda policy)',
        () async {
      final result = await service.validatePassword('Ab1def');
      expect(result.isError, isTrue);
    });
  });

  group('Register (sign_up_screen) uses the canonical policy', () {
    final source = _readSource(
      'lib/domains/user/identity/authentication/presentation/screens/sign_up_screen.dart',
    );

    test('password field validator delegates to CanonicalPasswordPolicy', () {
      expect(
        source.contains(
          'return CanonicalPasswordPolicy.validationMessage(value);',
        ),
        isTrue,
      );
    });

    test('submit gate uses CanonicalPasswordPolicy.isValid', () {
      expect(
        source.contains(
          'final hasPassword = CanonicalPasswordPolicy.isValid(',
        ),
        isTrue,
      );
    });

    test('no divergent inline min-8-only validator remains', () {
      // The old validator only checked length; it must be gone.
      expect(source.contains("'Must be at least 8 characters'"), isFalse);
      expect(source.contains('value.length < 8'), isFalse);
    });
  });

  group('Change Password (security_screen) uses the canonical policy', () {
    final source = _readSource(
      'lib/domains/user/profile/presentation/screens/security_screen.dart',
    );

    test('new-password validator delegates to CanonicalPasswordPolicy', () {
      expect(
        source.contains(
          'return CanonicalPasswordPolicy.validationMessage(value);',
        ),
        isTrue,
      );
    });

    test('no divergent inline min-8-only validator remains', () {
      expect(source.contains('value.length < 8'), isFalse);
    });
  });

  group('Login (sign_in_screen) does not apply registration policy', () {
    final source = _readSource(
      'lib/domains/user/identity/authentication/presentation/screens/sign_in_screen.dart',
    );

    test('submit gate requires only a non-empty password', () {
      expect(
        source.contains(
          "final hasPassword = _passwordController.text.trim().isNotEmpty;",
        ),
        isTrue,
      );
    });

    test('no min-8 gate on login', () {
      expect(source.contains('length >= 8'), isFalse);
      expect(source.contains('hasMinPasswordLength'), isFalse);
    });
  });
}
