import 'package:firebase_auth/firebase_auth.dart';
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/email_verification_controller.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/providers/email_verification_state.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/widgets/email_verify_bottom_sheet.dart';

class _FakeFirebaseAuth extends Fake implements FirebaseAuth {
  @override
  Stream<User?> authStateChanges() => const Stream<User?>.empty();

  @override
  User? get currentUser => null;
}

class _FakeEmailVerificationController extends EmailVerificationController {
  _FakeEmailVerificationController() : super(firebaseAuth: _FakeFirebaseAuth());

  int refreshCallCount = 0;
  int sendCallCount = 0;

  @override
  EmailVerificationState build() => const EmailVerificationState.unverified();

  @override
  String? get currentEmail => 'verified@test.com';

  @override
  Future<void> refreshEmailVerificationStatus() async {
    refreshCallCount++;
    state = const EmailVerificationState.verified();
  }

  @override
  Future<bool> sendVerificationEmail() async {
    sendCallCount++;
    state = const EmailVerificationState.unverified();
    return true;
  }
}

Widget _wrap(_FakeEmailVerificationController controller) {
  return ProviderScope(
    overrides: [
      emailVerificationControllerProvider.overrideWith(() => controller),
    ],
    child: MaterialApp(
      home: Builder(
        builder: (context) {
          return Scaffold(
            body: Center(
              child: ElevatedButton(
                onPressed: () => EmailVerifyBottomSheet.show(context),
                child: const Text('Open'),
              ),
            ),
          );
        },
      ),
    ),
  );
}

void main() {
  testWidgets(
    'Saya Sudah Verifikasi calls refreshEmailVerificationStatus and closes the sheet',
    (tester) async {
      final controller = _FakeEmailVerificationController();

      await tester.pumpWidget(_wrap(controller));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Open'));
      await tester.pumpAndSettle();

      expect(find.text('Saya Sudah Verifikasi'), findsOneWidget);

      await tester.tap(find.text('Saya Sudah Verifikasi'));
      await tester.pumpAndSettle();

      expect(controller.refreshCallCount, 1);
      expect(find.text('Saya Sudah Verifikasi'), findsNothing);
    },
  );

  testWidgets(
    'Kirim Ulang calls sendVerificationEmail on the verification controller',
    (tester) async {
      final controller = _FakeEmailVerificationController();

      await tester.pumpWidget(_wrap(controller));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Open'));
      await tester.pumpAndSettle();

      await tester.tap(find.text('Kirim Ulang Email Verifikasi'));
      await tester.pumpAndSettle();

      expect(controller.sendCallCount, 1);
    },
  );
}
