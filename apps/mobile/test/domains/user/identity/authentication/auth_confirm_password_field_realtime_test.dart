// Stage 2D — AuthConfirmPasswordField realtime match behavior.
//
// Proves the canonical confirm-password semantics:
//
//   confirm empty                              → neutral (no indicator)
//   password empty + confirm non-empty         → NOT MATCH
//   both non-empty + equal                     → MATCH
//   both non-empty + different                 → NOT MATCH
//
// And the realtime requirement: the indicator updates when EITHER the
// confirm field or the password field changes — no blur, no submit, no
// Form.validate().

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/shared/widgets/auth_password_field.dart';

/// Harness that drives the widget with real controllers and real typing.
class _Harness {
  _Harness()
    : password = TextEditingController(),
      confirm = TextEditingController();

  final TextEditingController password;
  final TextEditingController confirm;

  Widget build() {
    return MaterialApp(
      home: Scaffold(
        body: StatefulBuilder(
          builder: (context, setState) => Column(
            children: [
              TextField(
                controller: password,
                onChanged: (_) => setState(() {}),
              ),
              TextField(
                controller: confirm,
                onChanged: (_) => setState(() {}),
              ),
              AuthConfirmPasswordField(
                controller: confirm,
                passwordController: password,
                isVisible: false,
                onToggleVisibility: () {},
              ),
            ],
          ),
        ),
      ),
    );
  }
}

void main() {
  group('Canonical match semantics', () {
    testWidgets('both empty → neutral (no indicator)', (tester) async {
      final h = _Harness();
      addTearDown(h.password.dispose);
      addTearDown(h.confirm.dispose);
      await tester.pumpWidget(h.build());

      expect(find.text('Passwords match'), findsNothing);
      expect(find.text('Passwords do not match'), findsNothing);
    });

    testWidgets('password empty + confirm non-empty → NOT MATCH',
        (tester) async {
      final h = _Harness();
      addTearDown(h.password.dispose);
      addTearDown(h.confirm.dispose);
      await tester.pumpWidget(h.build());

      await tester.enterText(find.byType(TextField).at(1), 'Abcdef12');
      await tester.pump();

      expect(find.text('Passwords do not match'), findsOneWidget);
      expect(find.text('Passwords match'), findsNothing);
    });

    testWidgets('both non-empty + equal → MATCH', (tester) async {
      final h = _Harness();
      addTearDown(h.password.dispose);
      addTearDown(h.confirm.dispose);
      await tester.pumpWidget(h.build());

      await tester.enterText(find.byType(TextField).at(0), 'Abcdef12');
      await tester.enterText(find.byType(TextField).at(1), 'Abcdef12');
      await tester.pump();

      expect(find.text('Passwords match'), findsOneWidget);
      expect(find.text('Passwords do not match'), findsNothing);
    });

    testWidgets('both non-empty + different → NOT MATCH', (tester) async {
      final h = _Harness();
      addTearDown(h.password.dispose);
      addTearDown(h.confirm.dispose);
      await tester.pumpWidget(h.build());

      await tester.enterText(find.byType(TextField).at(0), 'Abcdef12');
      await tester.enterText(find.byType(TextField).at(1), 'Abcdef13');
      await tester.pump();

      expect(find.text('Passwords do not match'), findsOneWidget);
      expect(find.text('Passwords match'), findsNothing);
    });
  });

  group('Realtime — confirm field changes (no blur/submit)', () {
    testWidgets('confirm change to different → immediately NOT MATCH, back → MATCH',
        (tester) async {
      final h = _Harness();
      addTearDown(h.password.dispose);
      addTearDown(h.confirm.dispose);
      await tester.pumpWidget(h.build());

      await tester.enterText(find.byType(TextField).at(0), 'Abcdef12');
      await tester.enterText(find.byType(TextField).at(1), 'Abcdef12');
      await tester.pump();
      expect(find.text('Passwords match'), findsOneWidget);

      // Change ONLY the confirm field to a different value.
      await tester.enterText(find.byType(TextField).at(1), 'Abcdef13');
      await tester.pump();
      expect(find.text('Passwords do not match'), findsOneWidget);
      expect(find.text('Passwords match'), findsNothing);

      // Change confirm back → MATCH again.
      await tester.enterText(find.byType(TextField).at(1), 'Abcdef12');
      await tester.pump();
      expect(find.text('Passwords match'), findsOneWidget);
      expect(find.text('Passwords do not match'), findsNothing);
    });
  });

  group('Realtime — password field changes (no blur/submit)', () {
    testWidgets('password change to different → immediately NOT MATCH, back → MATCH',
        (tester) async {
      final h = _Harness();
      addTearDown(h.password.dispose);
      addTearDown(h.confirm.dispose);
      await tester.pumpWidget(h.build());

      await tester.enterText(find.byType(TextField).at(0), 'Abcdef12');
      await tester.enterText(find.byType(TextField).at(1), 'Abcdef12');
      await tester.pump();
      expect(find.text('Passwords match'), findsOneWidget);

      // Change ONLY the password field.
      await tester.enterText(find.byType(TextField).at(0), 'Abcdef13');
      await tester.pump();
      expect(find.text('Passwords do not match'), findsOneWidget);
      expect(find.text('Passwords match'), findsNothing);

      // Change password back → MATCH again.
      await tester.enterText(find.byType(TextField).at(0), 'Abcdef12');
      await tester.pump();
      expect(find.text('Passwords match'), findsOneWidget);
      expect(find.text('Passwords do not match'), findsNothing);
    });
  });

  group('Lifecycle safety', () {
    testWidgets('dispose with active listeners does not throw', (tester) async {
      final h = _Harness();
      await tester.pumpWidget(h.build());

      await tester.enterText(find.byType(TextField).at(0), 'Abcdef12');
      await tester.enterText(find.byType(TextField).at(1), 'Abcdef12');
      await tester.pump();

      // Remove the widget tree while listeners are attached.
      await tester.pumpWidget(const SizedBox.shrink());
      await tester.pump();

      h.password.dispose();
      h.confirm.dispose();
      // No exception thrown = pass.
    });

    testWidgets('rebuilding with new controllers re-attaches listeners',
        (tester) async {
      final h = _Harness();
      addTearDown(h.password.dispose);
      addTearDown(h.confirm.dispose);
      await tester.pumpWidget(h.build());

      // Swap to a second set of controllers via a parent rebuild.
      final password2 = TextEditingController();
      final confirm2 = TextEditingController();
      addTearDown(password2.dispose);
      addTearDown(confirm2.dispose);

      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: StatefulBuilder(
              builder: (context, setState) => Column(
                children: [
                  TextField(controller: password2),
                  TextField(controller: confirm2),
                  AuthConfirmPasswordField(
                    controller: confirm2,
                    passwordController: password2,
                    isVisible: false,
                    onToggleVisibility: () {},
                  ),
                ],
              ),
            ),
          ),
        ),
      );

      // New controllers drive the indicator.
      password2.text = 'Abcdef12';
      confirm2.text = 'Abcdef13';
      await tester.pump();
      expect(find.text('Passwords do not match'), findsOneWidget);

      confirm2.text = 'Abcdef12';
      await tester.pump();
      expect(find.text('Passwords match'), findsOneWidget);
    });
  });
}
