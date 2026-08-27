// PASS 2C: MainBottomNavigation basic routing smoke tests.
//
// MainBottomNavigation is a dumb, StatelessWidget — it renders items and
// forwards the tapped item's raw display index to `onTap`. All routing
// decisions (switch tab vs push Orders/Settings vs open the Create sheet)
// live in MainScreen's onTap callback, not in this widget. These tests only
// lock the widget's own contract: which items it renders, and which index
// it reports for each tap — they intentionally make no seller/admin
// assumptions, since MainBottomNavigation itself has none (that lives
// entirely in MainScreen._showCreateContentModal's use of sellerState).
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/features/home/presentation/models/main_tab.dart';
import 'package:labuda/features/home/presentation/widgets/main_bottom_navigation.dart';

List<MainTab> _twoTabs() => [
  const MainTab(
    label: 'Home',
    icon: Icons.home_outlined,
    selectedIcon: Icons.home,
    page: SizedBox.shrink(),
  ),
  const MainTab(
    label: 'Explore',
    icon: Icons.explore_outlined,
    selectedIcon: Icons.explore,
    page: SizedBox.shrink(),
  ),
];

Widget _wrap({
  required List<MainTab> tabs,
  int currentIndex = 0,
  bool showMultiFAB = false,
  required ValueChanged<int> onTap,
}) {
  return MaterialApp(
    home: Scaffold(
      bottomNavigationBar: MainBottomNavigation(
        currentIndex: currentIndex,
        showMultiFAB: showMultiFAB,
        tabs: tabs,
        onTap: onTap,
      ),
    ),
  );
}

void main() {
  group('MainBottomNavigation — renders expected core items', () {
    testWidgets('renders Home, Explore, Create, Orders, Settings', (
      tester,
    ) async {
      await tester.pumpWidget(_wrap(tabs: _twoTabs(), onTap: (_) {}));

      expect(find.text('Home'), findsOneWidget);
      expect(find.text('Explore'), findsOneWidget);
      expect(find.text('Create'), findsOneWidget);
      expect(find.text('Orders'), findsOneWidget);
      expect(find.text('Settings'), findsOneWidget);
    });

    testWidgets(
      'renders exactly 5 items for a 2-tab registry (no more, no fewer)',
      (tester) async {
        await tester.pumpWidget(_wrap(tabs: _twoTabs(), onTap: (_) {}));

        final bar = tester.widget<BottomNavigationBar>(
          find.byType(BottomNavigationBar),
        );
        expect(bar.items.length, 5);
      },
    );

    testWidgets(
      'shows close icon instead of add icon for Create when showMultiFAB is true',
      (tester) async {
        await tester.pumpWidget(
          _wrap(tabs: _twoTabs(), showMultiFAB: true, onTap: (_) {}),
        );

        expect(find.byIcon(Icons.close), findsWidgets);
        expect(find.byIcon(Icons.add), findsNothing);
      },
    );
  });

  group('MainBottomNavigation — tap forwards the correct raw index', () {
    testWidgets('tapping Home reports index 0 (selected-tab callback)', (
      tester,
    ) async {
      int? tapped;
      await tester.pumpWidget(
        _wrap(tabs: _twoTabs(), onTap: (i) => tapped = i),
      );

      await tester.tap(find.text('Home'));
      await tester.pump();

      expect(tapped, 0);
    });

    testWidgets('tapping Explore reports index 1 (selected-tab callback)', (
      tester,
    ) async {
      int? tapped;
      await tester.pumpWidget(
        _wrap(tabs: _twoTabs(), onTap: (i) => tapped = i),
      );

      await tester.tap(find.text('Explore'));
      await tester.pump();

      expect(tapped, 1);
    });

    testWidgets('tapping Create reports index 2 (create action slot)', (
      tester,
    ) async {
      int? tapped;
      await tester.pumpWidget(
        _wrap(tabs: _twoTabs(), onTap: (i) => tapped = i),
      );

      await tester.tap(find.text('Create'));
      await tester.pump();

      expect(tapped, 2);
    });

    testWidgets('tapping Orders reports index 3 (navigation callback)', (
      tester,
    ) async {
      int? tapped;
      await tester.pumpWidget(
        _wrap(tabs: _twoTabs(), onTap: (i) => tapped = i),
      );

      await tester.tap(find.text('Orders'));
      await tester.pump();

      expect(tapped, 3);
    });

    testWidgets('tapping Settings reports index 4 (navigation callback)', (
      tester,
    ) async {
      int? tapped;
      await tester.pumpWidget(
        _wrap(tabs: _twoTabs(), onTap: (i) => tapped = i),
      );

      await tester.tap(find.text('Settings'));
      await tester.pump();

      expect(tapped, 4);
    });
  });

  group('MainBottomNavigation — no seller/admin assumptions', () {
    testWidgets(
      'renders identically regardless of any seller/admin concept — the '
      'widget takes no role/capability parameter at all',
      (tester) async {
        // This is a structural guard: MainBottomNavigation's constructor
        // only accepts currentIndex/showMultiFAB/tabs/onTap. If a
        // role/capability parameter were ever added here (instead of in
        // MainScreen, which is the correct owner of that decision), this
        // test would need updating to pass it — its absence today is the
        // invariant being locked.
        await tester.pumpWidget(_wrap(tabs: _twoTabs(), onTap: (_) {}));

        expect(find.byType(MainBottomNavigation), findsOneWidget);
        expect(find.text('Home'), findsOneWidget);
        expect(find.text('Explore'), findsOneWidget);
      },
    );
  });
}
