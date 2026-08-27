// Stage 2C — Strength consumer convergence.
//
// Proves every active password-strength surface uses the canonical engine
// (CanonicalPasswordStrength via the shared PasswordStrengthIndicator widget)
// and that no screen-local Weak/Medium/Strong algorithm remains.

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _readSource(String relativePath) =>
    File(relativePath).readAsStringSync();

void main() {
  group('Register (sign_up_screen) uses canonical strength widget', () {
    final source = _readSource(
      'lib/domains/user/identity/authentication/presentation/screens/sign_up_screen.dart',
    );

    test('strengthIndicator slot uses PasswordStrengthIndicator widget', () {
      expect(
        source.contains(
          'strengthIndicator: PasswordStrengthIndicator(',
        ),
        isTrue,
      );
    });

    test('no inline strength scoring remains', () {
      expect(source.contains('_buildPasswordStrengthIndicator'), isFalse);
      expect(source.contains('_getStrengthText'), isFalse);
      expect(source.contains('_getStrengthColor'), isFalse);
      expect(source.contains('hasMinLength'), isFalse);
      expect(source.contains("'Fair'"), isFalse);
      expect(source.contains("'Good'"), isFalse);
    });
  });

  group('Change Password (security_screen) uses canonical strength widget', () {
    final source = _readSource(
      'lib/domains/user/profile/presentation/screens/security_screen.dart',
    );

    test('strengthIndicator slot uses PasswordStrengthIndicator widget', () {
      expect(
        source.contains('? PasswordStrengthIndicator('),
        isTrue,
      );
    });

    test('no legacy shared. prefix or onValidationChanged wiring remains', () {
      expect(source.contains('shared.PasswordStrengthIndicator'), isFalse);
      expect(source.contains('onValidationChanged'), isFalse);
    });
  });

  group('No active screen contains its own strength algorithm', () {
    test('no min-6 strength constants in any active screen', () {
      final signUp = _readSource(
        'lib/domains/user/identity/authentication/presentation/screens/sign_up_screen.dart',
      );
      final security = _readSource(
        'lib/domains/user/profile/presentation/screens/security_screen.dart',
      );
      expect(signUp.contains('length >= 6'), isFalse);
      expect(security.contains('length >= 6'), isFalse);
      expect(signUp.contains('Minimal 6'), isFalse);
      expect(security.contains('Minimal 6'), isFalse);
    });

    test('canonical widget file delegates to CanonicalPasswordStrength', () {
      final widget = _readSource(
        'lib/shared/widgets/password_strength_indicator.dart',
      );
      expect(widget.contains('CanonicalPasswordStrength.evaluate'), isTrue);
      expect(widget.contains('CanonicalPasswordStrength.progress'), isTrue);
    });
  });
}
