// BottomActionBar embedded lifted-sheet inset contract.
//
// Default mode rises above the keyboard itself (Scaffold.bottomNavigationBar
// is never lifted by the framework) and owns the system bottom inset through
// its own SafeArea. Embedded mode (`embeddedInLiftedSheet`) is for bars
// inside an `AppBottomSheetBase` sheet whose presenter already owns BOTH
// insets: the bar must not add a second keyboard-height lift, and it must
// not add a second system-bottom reservation either — the base spacer below
// the content region is the ONE system inset owner there (BOTTOMSHEET-03).
// Default mode is unchanged.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/shared/widgets/bottom_action_bar.dart';

Widget _host({
  required bool embedded,
  double keyboard = 300,
  double systemBottom = 0,
}) {
  // Mounted as a real bottom bar: Scaffold strips viewInsets from its *body*
  // MediaQuery (it resizes the body instead), while bottomNavigationBar keeps
  // the live insets — exactly the placement the bar's contract is written for.
  return MaterialApp(
    home: MediaQuery(
      data: MediaQueryData(
        viewInsets: EdgeInsets.only(bottom: keyboard),
        padding: EdgeInsets.only(bottom: systemBottom),
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

    testWidgets('embedded bar applies no second keyboard lift', (tester) async {
      await tester.pumpWidget(_host(embedded: true));
      await tester.pumpAndSettle();
      expect(_liftPadding(tester), EdgeInsets.zero);
      expect(find.text('Simpan'), findsOneWidget);
      // SafeArea stays mounted in both modes; in embedded mode it yields
      // its bottom because the base spacer owns that inset (BOTTOMSHEET-03).
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

  group('BottomActionBar system-bottom inset (BOTTOMSHEET-03)', () {
    Future<double> barHeightAt(
      WidgetTester tester, {
      required bool embedded,
      required double systemBottom,
    }) async {
      await tester.pumpWidget(
        _host(embedded: embedded, keyboard: 0, systemBottom: systemBottom),
      );
      await tester.pumpAndSettle();
      return tester.getSize(find.byType(BottomActionBar)).height;
    }

    testWidgets('non-embedded bar still owns the system inset', (tester) async {
      final double hidden = await barHeightAt(
        tester,
        embedded: false,
        systemBottom: 0,
      );
      final double visible = await barHeightAt(
        tester,
        embedded: false,
        systemBottom: 48,
      );
      expect(
        visible - hidden,
        closeTo(48, 0.01),
        reason: 'the default bar no longer follows the live system inset',
      );
      // Its content still clears the inset: the bar IS the bottom surface.
      final Rect cta = tester.getRect(find.byType(ElevatedButton));
      final Rect surface = tester.getRect(find.byType(Scaffold));
      expect(cta.bottom, lessThanOrEqualTo(surface.bottom - 48));
    });

    testWidgets('embedded bar adds no second system-bottom reservation', (
      tester,
    ) async {
      final double hidden = await barHeightAt(
        tester,
        embedded: true,
        systemBottom: 0,
      );
      final double visible = await barHeightAt(
        tester,
        embedded: true,
        systemBottom: 48,
      );
      expect(
        visible,
        closeTo(hidden, 0.01),
        reason:
            'the embedded bar grew by ${visible - hidden} px with the '
            'system inset — a second reservation beside the base spacer',
      );
    });
  });
}
