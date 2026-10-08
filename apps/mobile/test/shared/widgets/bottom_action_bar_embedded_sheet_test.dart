// BottomActionBar embedded lifted-sheet keyboard contract.
//
// Default mode rises above the keyboard itself (Scaffold.bottomNavigationBar
// is never lifted by the framework). Embedded mode (`embeddedInLiftedSheet`)
// is for bars inside an `AppBottomSheetBase` sheet whose presenter already
// owns that movement: the bar must not add a second keyboard-height lift,
// while Safe Area handling stays intact in both modes.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/shared/widgets/bottom_action_bar.dart';

Widget _host({required bool embedded, double keyboard = 300}) {
  // Mounted as a real bottom bar: Scaffold strips viewInsets from its *body*
  // MediaQuery (it resizes the body instead), while bottomNavigationBar keeps
  // the live insets — exactly the placement the bar's contract is written for.
  return MaterialApp(
    home: MediaQuery(
      data: MediaQueryData(
        viewInsets: EdgeInsets.only(bottom: keyboard),
      ),
      child: Scaffold(
        bottomNavigationBar: BottomActionBar(
          embeddedInLiftedSheet: embedded,
          primary: BottomBarAction(label: 'Simpan', onPressed: () {}),
        ),
      ),
    ),
  );
}

/// The keyboard-lift Padding is the one whose direct child is the bar's
/// action Column; SafeArea's own Padding and button internals have
/// different children and must not match.
EdgeInsets _liftPadding(WidgetTester tester) {
  EdgeInsets? found;
  final paddings = tester.elementList(
    find.descendant(
      of: find.byType(BottomActionBar),
      matching: find.byType(Padding),
    ),
  );
  for (final element in paddings) {
    element.visitChildElements((child) {
      if (child.widget is Column) {
        found = (element.widget as Padding).padding as EdgeInsets;
      }
    });
  }
  return found!;
}

void main() {
  group('BottomActionBar keyboard lift', () {
    testWidgets('default bar rises by its own keyboard inset', (tester) async {
      await tester.pumpWidget(_host(embedded: false));
      await tester.pumpAndSettle();
      expect(_liftPadding(tester), const EdgeInsets.only(bottom: 300));
      expect(find.text('Simpan'), findsOneWidget);
      expect(
        find.descendant(
          of: find.byType(BottomActionBar),
          matching: find.byType(SafeArea),
        ),
        findsOneWidget,
      );
    });

    testWidgets('embedded bar applies no second keyboard lift', (
      tester,
    ) async {
      await tester.pumpWidget(_host(embedded: true));
      await tester.pumpAndSettle();
      expect(_liftPadding(tester), EdgeInsets.zero);
      expect(find.text('Simpan'), findsOneWidget);
      // System bottom Safe Area handling is unchanged in embedded mode.
      expect(
        find.descendant(
          of: find.byType(BottomActionBar),
          matching: find.byType(SafeArea),
        ),
        findsOneWidget,
      );
    });

    testWidgets('both modes render identically with keyboard closed', (
      tester,
    ) async {
      await tester.pumpWidget(_host(embedded: false, keyboard: 0));
      await tester.pumpAndSettle();
      expect(_liftPadding(tester), EdgeInsets.zero);

      await tester.pumpWidget(_host(embedded: true, keyboard: 0));
      await tester.pumpAndSettle();
      expect(_liftPadding(tester), EdgeInsets.zero);
    });
  });
}
