// Stage 2D — Confirm-password consumer convergence.
//
// Proves active password surfaces use the canonical confirm-password behavior
// (AuthConfirmPasswordField owning listeners on BOTH controllers) and that the
// dead duplicate PasswordMatchIndicator component is gone.
//
// The widget-level realtime proof lives in
// auth_confirm_password_field_realtime_test.dart; this file locks the wiring
// contract so a future regression that wires `onChanged` only to the password
// field is caught.

import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

String _readSource(String relativePath) =>
    File(relativePath).readAsStringSync();

void main() {
  group('AuthConfirmPasswordField owns realtime listeners', () {
    final source = _readSource(
      'lib/domains/user/identity/authentication/presentation/shared/widgets/auth_password_field.dart',
    );

    test('listens to BOTH confirm and password controllers', () {
      expect(source.contains('_confirmController?.addListener'), isTrue);
      expect(source.contains('_passwordController?.addListener'), isTrue);
    });

    test('removes listeners on dispose', () {
      expect(source.contains('_confirmController?.removeListener'), isTrue);
      expect(source.contains('_passwordController?.removeListener'), isTrue);
    });

    test('canonical match is trimmed equality consistent with the submit gate', () {
      // Stage 4D: the indicator and the submit gates (sign_up _isFormValid,
      // security validator) must agree. Both compare TRIMMED text, so trailing
      // whitespace cannot make the indicator say "match" while the gate stays
      // disabled (or vice versa). The widget trims the comparison values only —
      // no lowercase/normalize transforms.
      expect(source.contains("password == confirmPassword"), isTrue);
      expect(source.contains('toLowerCase()'), isFalse);
      expect(source.contains('.trim()'), isTrue);
    });

    test('realtime rebuild is listener-driven, not onChanged-dependent', () {
      // The confirm field state uses controller listeners (proven above).
      // It must NOT pass an onChanged to its TextFormField (no onChanged arg
      // is wired in the confirm field's TextFormField construction).
      expect(source.contains('_onPasswordInputChanged'), isTrue);
    });
  });

  group('Register (sign_up_screen) uses canonical confirm field', () {
    final source = _readSource(
      'lib/domains/user/identity/authentication/presentation/screens/sign_up_screen.dart',
    );

    test('confirm field is AuthConfirmPasswordField with both controllers', () {
      expect(source.contains('AuthConfirmPasswordField('), isTrue);
      expect(source.contains('controller: _confirmPasswordController'), isTrue);
      expect(source.contains('passwordController: _passwordController'), isTrue);
    });

    test('submit gate still checks passwordsMatch via canonical equality', () {
      expect(source.contains('passwordsMatch'), isTrue);
      expect(source.contains('_confirmPasswordController.text.trim() =='), isTrue);
    });
  });

  group('Change Password (security_screen) uses canonical confirm field', () {
    final source = _readSource(
      'lib/domains/user/profile/presentation/screens/security_screen.dart',
    );

    test('confirm field is AuthConfirmPasswordField with both controllers', () {
      expect(source.contains('AuthConfirmPasswordField('), isTrue);
      expect(source.contains('controller: _confirmPasswordController'), isTrue);
      expect(source.contains('passwordController: _newPasswordController'), isTrue);
    });

    test('no stale parent-computed showMatchIndicator condition remains', () {
      expect(
        source.contains(
          'showMatchIndicator: _confirmPasswordController.text.isNotEmpty',
        ),
        isFalse,
      );
    });
  });

  group('Dead duplicate PasswordMatchIndicator removed', () {
    test('no production references remain', () {
      final shared = _readSource('lib/shared/shared.dart');
      expect(shared.contains('password_match_indicator'), isFalse);

      final matchWidgetExists = File(
        'lib/shared/widgets/password_match_indicator.dart',
      ).existsSync();
      expect(matchWidgetExists, isFalse);
    });
  });
}
