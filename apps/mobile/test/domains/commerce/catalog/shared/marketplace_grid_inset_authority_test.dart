// SAFE-AREA-03 — COMMERCE MARKETPLACE GRID INSET AUTHORITY.
//
// The SAFE-AREA-02 audit proved `CommerceMarketplaceGrid` is content/layout,
// not the system-inset authority: on every live mount its parent (a Scaffold
// bottom bar or a body-level SafeArea) already owns the bottom system inset,
// and the grid's own inset read evaluated to 0.0 everywhere. This suite locks
// the cleanup:
//
//   A — under an injected non-zero system inset the grid keeps exactly its
//       design bottom spacing (16), it never re-adds the inset;
//   B — behind a parent SafeArea the parent consumes the inset and the grid
//       geometry is identical with and without the inset;
//   C — beside a Scaffold bottom bar the bar stays the sole owner: the body
//       ends at the bar, the grid adds nothing, and the last row lands
//       exactly the design padding above the bar and stays reachable;
//   D — inset 0 → design spacing only, no stale inset-sized gap;
//   E — source sweep: the removed parameter and every grid-level inset read
//       stay gone, so the obsolete mechanism cannot quietly return.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_metrics.dart';
import 'package:labuda/domains/commerce/catalog/shared/presentation/widgets/commerce_marketplace_primitives.dart';
import 'package:labuda/generated/app_localizations.dart';
import 'package:labuda/shared/widgets/bottom_action_bar.dart';

/// The injected system bottom inset for the positive proofs (logical px).
const double _inset = 24;

/// The grid's owner-locked design bottom spacing — content rhythm, never
/// system inset (see [CommerceMarketplaceMetrics.gridBottomPadding]).
const double _designBottom = 16;

void _noop() {}

/// Window metrics on the TEST VIEW (same mechanism as the SAFE-AREA-01
/// contract): only the real window sees every layer of the tree.
void _setWindowInsets(WidgetTester tester, {double systemBottom = 0}) {
  final double dpr = tester.view.devicePixelRatio;
  tester.view.padding = FakeViewPadding(bottom: systemBottom * dpr);
  tester.view.viewPadding = FakeViewPadding(bottom: systemBottom * dpr);
  tester.view.viewInsets = FakeViewPadding();
}

Widget _host(Widget child) => MaterialApp(
  localizationsDelegates: AppLocalizations.localizationsDelegates,
  supportedLocales: AppLocalizations.supportedLocales,
  locale: const Locale('id'),
  home: child,
);

CommerceMarketplaceGrid _grid({int itemCount = 6}) => CommerceMarketplaceGrid(
  itemCount: itemCount,
  itemBuilder: (context, index) => SizedBox.expand(
    key: ValueKey<String>('grid-item-$index'),
    child: Text('item $index'),
  ),
);

/// The grid's own sliver padding — the one geometry that would change if the
/// grid ever re-acquired a system-inset read.
Finder _gridSliver() => find.descendant(
  of: find.byType(CustomScrollView),
  matching: find.byType(SliverPadding),
);

EdgeInsets _gridPadding(WidgetTester tester) =>
    tester.widget<SliverPadding>(_gridSliver()).padding as EdgeInsets;

void main() {
  // ==========================================================================
  // TEST A — the grid owns no system inset
  // ==========================================================================
  group('SAFE-AREA-03 (A) — the grid owns no system inset', () {
    testWidgets('injected 24px inset: design bottom stays 16, not 40', (
      tester,
    ) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: _inset);

      // Bare body: no bar, no SafeArea — the ambient inset reaches the grid,
      // which is exactly where it must NOT be consumed.
      await tester.pumpWidget(
        _host(Scaffold(body: CustomScrollView(slivers: [_grid()]))),
      );
      await tester.pumpAndSettle();

      final EdgeInsets padding = _gridPadding(tester);
      expect(
        padding,
        const EdgeInsets.fromLTRB(12, 12, 12, 16),
        reason: 'the grid re-added the system bottom inset ($padding)',
      );
      expect(padding.bottom, _designBottom);
      expect(
        padding.bottom,
        CommerceMarketplaceMetrics.gridBottomPadding,
        reason: 'the design token no longer drives the grid bottom spacing',
      );
      expect(
        padding.bottom,
        isNot(_inset + _designBottom),
        reason: 'system inset + design spacing were merged again',
      );
    });
  });

  // ==========================================================================
  // TEST B — the parent SafeArea owns the inset
  // ==========================================================================
  group('SAFE-AREA-03 (B) — the parent SafeArea owns the inset', () {
    testWidgets('parent consumes the live inset; grid geometry identical', (
      tester,
    ) async {
      addTearDown(tester.view.reset);

      Future<({EdgeInsets padding, Rect viewport, Rect surface, Size cell})>
      pumpAt(double inset) async {
        _setWindowInsets(tester, systemBottom: inset);
        await tester.pumpWidget(
          _host(
            Scaffold(
              body: SafeArea(child: CustomScrollView(slivers: [_grid()])),
            ),
          ),
        );
        await tester.pumpAndSettle();
        return (
          padding: _gridPadding(tester),
          viewport: tester.getRect(find.byType(CustomScrollView)),
          surface: tester.getRect(find.byType(Scaffold)),
          cell: tester.getSize(
            find.byKey(const ValueKey<String>('grid-item-0')),
          ),
        );
      }

      final withInset = await pumpAt(_inset);
      final withoutInset = await pumpAt(0);

      // The grid kept its design spacing in both runs — it never tracked the
      // system inset itself.
      expect(
        withInset.padding,
        const EdgeInsets.fromLTRB(12, 12, 12, 16),
        reason: 'the grid added the inset beside its parent SafeArea',
      );
      expect(withoutInset.padding, withInset.padding);

      // The PARENT tracked the live inset: the scrollable cleared the system
      // navigation area by exactly the inset, and only by the inset.
      expect(
        withInset.surface.bottom - withInset.viewport.bottom,
        closeTo(_inset, 0.01),
        reason: 'the parent SafeArea did not consume the system inset',
      );
      expect(
        withoutInset.surface.bottom - withoutInset.viewport.bottom,
        closeTo(0, 0.01),
        reason: 'a stale inset-sized gap survived the hidden system bar',
      );

      // Grid geometry is byte-identical with and without the inset.
      expect(
        withInset.cell,
        withoutInset.cell,
        reason: 'grid cell geometry changed with the system inset',
      );
    });
  });

  // ==========================================================================
  // TEST C — beside a bottom bar, the bar owns the inset
  // ==========================================================================
  group('SAFE-AREA-03 (C) — beside a bottom bar, the bar owns the inset', () {
    testWidgets('grid adds nothing; last row lands designPadding above bar', (
      tester,
    ) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: _inset);

      await tester.pumpWidget(
        _host(
          Scaffold(
            body: CustomScrollView(slivers: [_grid()]),
            bottomNavigationBar: BottomActionBar(
              primary: BottomBarAction(label: 'Beli', onPressed: _noop),
            ),
          ),
        ),
      );
      await tester.pumpAndSettle();

      // 1. The grid never re-adds the system inset.
      expect(
        _gridPadding(tester),
        const EdgeInsets.fromLTRB(12, 12, 12, 16),
        reason: 'the grid reserved bottom space beside the bottom bar',
      );

      // 2. The bar owns the reservation: the scroll body ends at the bar top,
      //    with no body-side second reservation.
      final Rect body = tester.getRect(find.byType(CustomScrollView));
      final Rect bar = tester.getRect(find.byType(BottomActionBar));
      expect(
        body.bottom,
        closeTo(bar.top, 0.01),
        reason:
            'a second bottom reservation appeared beside the bar '
            '(body ${body.bottom} != bar top ${bar.top})',
      );

      // 3. The last row is reachable and ends exactly the design spacing
      //    above the bar — an inset-sized gap here would fail the closeTo.
      await tester.drag(find.byType(CustomScrollView), const Offset(0, -10000));
      await tester.pumpAndSettle();

      final Finder last = find.byKey(const ValueKey<String>('grid-item-5'));
      expect(last, findsOneWidget, reason: 'the last grid row is unreachable');
      final Rect lastCell = tester.getRect(last);
      expect(
        lastCell.bottom,
        closeTo(bar.top - _designBottom, 0.01),
        reason:
            'the gap under the last row '
            '(${bar.top - lastCell.bottom}) is not the $_designBottom px '
            'design spacing — an inset-sized gap slipped in',
      );
      expect(lastCell.bottom, lessThanOrEqualTo(bar.top));
    });
  });

  // ==========================================================================
  // TEST D — inset zero
  // ==========================================================================
  group('SAFE-AREA-03 (D) — zero inset leaves design spacing only', () {
    testWidgets('inset 0: bottom = 16, viewport reaches the screen edge', (
      tester,
    ) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 0);

      await tester.pumpWidget(
        _host(Scaffold(body: CustomScrollView(slivers: [_grid()]))),
      );
      await tester.pumpAndSettle();

      expect(_gridPadding(tester).bottom, _designBottom);

      final Rect viewport = tester.getRect(find.byType(CustomScrollView));
      final Rect surface = tester.getRect(find.byType(Scaffold));
      expect(
        viewport.bottom,
        closeTo(surface.bottom, 0.01),
        reason: 'stale inset-sized gap survived a hidden system bar',
      );
    });
  });

  // ==========================================================================
  // TEST E — negative source proof
  // ==========================================================================
  group('SAFE-AREA-03 (E) — the obsolete inset authority stays gone', () {
    const String gridSourcePath =
        'lib/domains/commerce/catalog/shared/presentation/widgets/'
        'commerce_marketplace_primitives.dart';

    String readGridSource() =>
        File(gridSourcePath).readAsStringSync().replaceAll('\r\n', '\n');

    test('the grid source reads no system inset', () {
      final String source = readGridSource();

      for (final String banned in const <String>[
        'paddingOf',
        'viewPadding',
        'viewInsets',
        'SafeArea(',
      ]) {
        expect(
          source,
          isNot(contains(banned)),
          reason:
              '`$banned` re-spelled a grid-owned system inset — the parent '
              'screen boundary owns the bottom system inset, never the grid',
        );
      }

      // The removed parameter, spelled as a regex so this very file never
      // matches a repository-wide literal search for it.
      expect(
        source,
        isNot(RegExp(r'includeSafeArea\s*Bottom')),
        reason: 'the removed safe-area parameter came back',
      );

      // The generic ambient read `MediaQuery.of(...).padding` — but not
      // unrelated, legitimate window queries (size, text scaling, …).
      expect(
        source,
        isNot(RegExp(r'MediaQuery\s*\.\s*of\s*\([^)]*\)\s*\.\s*padding')),
        reason: 'an ambient padding read is the same grid-owned inset risk',
      );
    });

    test('the design bottom spacing token remains intact', () {
      expect(CommerceMarketplaceMetrics.gridBottomPadding, 16);
      expect(
        readGridSource(),
        contains('CommerceMarketplaceMetrics.gridBottomPadding'),
        reason:
            '16 px is content/design spacing and must stay token-driven; it '
            'is NOT a stand-in for the system inset',
      );
    });

    test('no source in lib or test references the removed parameter', () {
      final RegExp removed = RegExp(r'includeSafeArea\s*Bottom');
      final List<String> offenders = <String>[];

      for (final String root in const <String>['lib', 'test']) {
        for (final File file
            in Directory(root)
                .listSync(recursive: true)
                .whereType<File>()
                .where((f) => f.path.endsWith('.dart'))) {
          if (removed.hasMatch(file.readAsStringSync())) {
            offenders.add(file.path.replaceAll(r'\', '/'));
          }
        }
      }

      expect(
        offenders,
        isEmpty,
        reason:
            'a caller still depends on the removed parameter:\n'
            '${offenders.join('\n')}',
      );
    });
  });
}
