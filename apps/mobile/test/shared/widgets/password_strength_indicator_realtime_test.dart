// Stage 2C — Realtime strength widget proof.
//
// Proves the canonical PasswordStrengthIndicator updates while the password
// text changes, WITHOUT blur / submit / Form validation:
//
//   1. Empty password → no strength label rendered (neutral).
//   2. Typing a weak password → "Weak" appears immediately.
//   3. Typing a strong password → "Strong" appears immediately.
//
// The widget derives its state purely from the current password value, so a
// parent that rebuilds on every keystroke (controller listener / onChanged)
// shows live feedback.  This is exactly how SignUp (password controller
// listener) and SecurityScreen (onChanged setState) drive it.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/widgets/password_strength_indicator.dart';

void main() {
  testWidgets('empty password renders no strength label', (tester) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: Scaffold(
          body: PasswordStrengthIndicator(password: '', isDark: false),
        ),
      ),
    );

    expect(find.text('Weak'), findsNothing);
    expect(find.text('Medium'), findsNothing);
    expect(find.text('Strong'), findsNothing);
  });

  testWidgets('strength label updates live while typing (no blur)',
      (tester) async {
    final controller = TextEditingController();
    addTearDown(controller.dispose);

    late StateSetter setState;
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: StatefulBuilder(
            builder: (context, setStateInner) {
              setState = setStateInner;
              return Column(
                children: [
                  TextField(controller: controller),
                  PasswordStrengthIndicator(
                    password: controller.text,
                    isDark: false,
                  ),
                ],
              );
            },
          ),
        ),
      ),
    );

    // Start empty → no label.
    expect(find.text('Weak'), findsNothing);

    // Type a weak password → parent rebuilds via onChanged → "Weak" shows.
    controller.text = 'abc';
    setState(() {});
    await tester.pump();
    expect(find.text('Weak'), findsOneWidget);

    // Type a strong password → label updates to "Strong" immediately.
    controller.text = 'Aaaaaaaaaaaa1!';
    setState(() {});
    await tester.pump();
    expect(find.text('Strong'), findsOneWidget);
    expect(find.text('Weak'), findsNothing);

    // Type a medium password → label updates to "Medium".
    controller.text = 'Abcdef12';
    setState(() {});
    await tester.pump();
    expect(find.text('Medium'), findsOneWidget);
    expect(find.text('Strong'), findsNothing);

    // Clear → back to neutral (no label).
    controller.text = '';
    setState(() {});
    await tester.pump();
    expect(find.text('Weak'), findsNothing);
    expect(find.text('Medium'), findsNothing);
    expect(find.text('Strong'), findsNothing);
  });

  testWidgets('real TextField typing updates strength without submit',
      (tester) async {
    final controller = TextEditingController();
    addTearDown(controller.dispose);

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: StatefulBuilder(
            builder: (context, setState) => Column(
              children: [
                TextField(
                  controller: controller,
                  onChanged: (_) => setState(() {}),
                ),
                PasswordStrengthIndicator(
                  password: controller.text,
                  isDark: false,
                ),
              ],
            ),
          ),
        ),
      ),
    );

    // Type via the real text field (no blur, no submit).
    await tester.enterText(find.byType(TextField), 'Aaaaaaaaaaaa1!');
    await tester.pump();

    expect(find.text('Strong'), findsOneWidget);

    await tester.enterText(find.byType(TextField), 'Abcdef12');
    await tester.pump();

    expect(find.text('Medium'), findsOneWidget);
    expect(find.text('Strong'), findsNothing);
  });
}
