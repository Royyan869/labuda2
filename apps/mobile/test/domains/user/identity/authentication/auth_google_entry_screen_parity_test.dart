// AUTH-H1 — Google entry-point SCREEN parity regression.
//
// The controller-level convergence (identical state machine for the Login
// and Sign-up Google buttons) is proven in
// auth_google_entry_convergence_test.dart. These widget tests lock the
// screen-level contract for the SAME account across entry points
// (Owner decision: an unprovable "registration successful" claim is bad UX
// and must never appear):
//
//   W1a  Sign-up Google → RequiresProfileCompletion (new account, profile
//        still incomplete): NO registration-success claim — the router
//        takes the user to Complete Profile.
//   W1b  Sign-up Google → Authenticated (EXISTING account with a complete
//        profile — this is a LOGIN, not a registration): NO
//        registration-success claim, identical to the Login entry point.
//   W2   Sign-in screen surfaces AuthStateError inline — Google failures
//        from the Login entry (cancelled, invalid credential, account-exists
//        guidance) must be visible, matching the Sign-up screen.

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart' hide NotificationEntity;
import 'package:hishumi/domains/user/identity/authentication/domain/entities/account_status.dart';

/// Fake controller: seeds an initial state via build() (no dependency
/// graph) and plays a scripted terminal state when the Google button is
/// pressed — the screen logic under test is the post-await state handling.
class _ScriptedGoogleAuthController extends AuthController {
  _ScriptedGoogleAuthController({required this.initial, required this.next});
  final AuthState initial;
  final AuthState next;

  /// Proves the Google button actually drove the scripted outcome.
  bool playedNext = false;

  @override
  AuthState build() => initial;

  @override
  Future<void> signUpWithGoogle() async {
    playedNext = true;
    state = next;
  }

  @override
  Future<void> signInWithGoogle() async {
    playedNext = true;
    state = next;
  }
}

AuthUser _user() => AuthUser(
      id: 'backend-1',
      createdAt: DateTime.utc(2026, 1, 1),
      updatedAt: DateTime.utc(2026, 1, 1),
      email: 'g@example.com',
      username: 'google-user',
      isEmailVerified: true,
      accountStatus: AccountStatus.active,
      roles: const [UserRole.user],
      provider: AuthProvider.google,
    );

void main() {
  testWidgets(
    'W1a — Sign-up Google landing on RequiresProfileCompletion shows NO '
    'fake success snackbar (Complete Profile is not a completed registration)',
    (tester) async {
      final controller = _ScriptedGoogleAuthController(
        initial: const AuthState.unauthenticated(),
        next: const AuthState.requiresProfileCompletion(
          userId: 'fb-g1',
          email: 'g@example.com',
        ),
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authControllerProvider.overrideWith(() => controller),
          ],
          child: const MaterialApp(home: SignUpScreen()),
        ),
      );
      await tester.pump();

      await tester.ensureVisible(find.text('Sign up with Google'));
      await tester.pump();
      await tester.tap(find.text('Sign up with Google'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));

      expect(controller.playedNext, isTrue,
          reason: 'the tap must actually drive the Google flow');
      expect(
        find.text('Berhasil mendaftar dengan Google!'),
        findsNothing,
        reason: 'an incomplete profile must never celebrate — the router '
            'takes the user to the Complete Profile surface',
      );
    },
  );

  testWidgets(
    'W1b — Sign-up Google landing on Authenticated (existing account) shows '
    'NO registration-success claim (it is a login, not a signup)',
    (tester) async {
      final controller = _ScriptedGoogleAuthController(
        initial: const AuthState.unauthenticated(),
        next: AuthState.authenticated(_user(), emailVerified: true),
      );
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authControllerProvider.overrideWith(() => controller),
          ],
          child: const MaterialApp(home: SignUpScreen()),
        ),
      );
      await tester.pump();

      await tester.ensureVisible(find.text('Sign up with Google'));
      await tester.pump();
      await tester.tap(find.text('Sign up with Google'));
      await tester.pump();
      await tester.pump(const Duration(milliseconds: 400));

      expect(controller.playedNext, isTrue,
          reason: 'the tap must actually drive the Google flow');
      expect(
        find.text('Berhasil mendaftar dengan Google!'),
        findsNothing,
        reason: 'Authenticated from this entry may be an existing account '
            'logging in — a "registration successful" claim cannot be '
            'proven and must never be shown (entry-point parity: the Login '
            'screen shows nothing)',
      );
    },
  );

  testWidgets(
    'W2 — Sign-in screen surfaces AuthStateError inline (Google failure '
    'visibility parity with the Sign-up entry point)',
    (tester) async {
      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            authControllerProvider.overrideWith(
              () => _ScriptedGoogleAuthController(
                initial: AuthState.error(
                  'Akun ini terdaftar dengan metode masuk berbeda. Masuk '
                  'dengan metode semula, lalu tautkan Google setelah '
                  'verifikasi email.',
                ),
                next: const AuthState.unauthenticated(),
              ),
            ),
          ],
          child: const MaterialApp(home: SignInScreen()),
        ),
      );
      await tester.pump();

      expect(
        find.textContaining('metode masuk berbeda'),
        findsOneWidget,
        reason: 'the Login entry must render the same AuthStateError the '
            'Sign-up entry already renders inline',
      );
    },
  );
}
