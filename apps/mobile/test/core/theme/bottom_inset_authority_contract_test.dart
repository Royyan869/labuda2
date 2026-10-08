// SAFE-AREA-01 — SCREEN & BOTTOM SHEET BOTTOM-INSET AUTHORITY.
//
// Locks the Owner invariant:
//   * the layer that owns the bottom surface owns the bottom system inset, and
//     it owns it EXACTLY ONCE (no body-side second reservation beside a bar);
//   * the inset consumed is the REAL, LIVE one, so a system navigation bar
//     appearing, changing or disappearing moves the layout with it;
//   * a disappearing inset leaves no stale gap behind (no 80/96/100 stand-in);
//   * a bottom sheet follows the same rule for the system inset AND the
//     keyboard inset.
//
// Two halves, deliberately:
//   POSITIVE — widget tests that inject real window metrics and measure the
//              rendered geometry (proves the mechanism, not the prose);
//   NEGATIVE — a source sweep that fails the moment one of the purged fixed
//              clearances is re-spelled beside a bottom bar. A contract that
//              only proves the canonical path exists cannot stop the old one
//              from quietly coming back.
import 'dart:io';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:labuda/shared/widgets/app_bottom_sheet.dart';
import 'package:labuda/shared/widgets/bottom_action_bar.dart';

const ValueKey<String> _bodyKey = ValueKey<String>('safe-area-body');
const ValueKey<String> _sheetContentKey = ValueKey<String>(
  'safe-area-sheet-body',
);

const double _surfaceHeight = 600;

void _noop() {}

/// The window metrics a device reports, set on the TEST VIEW rather than
/// injected as a widget: the sheet is presented by a modal route above the
/// page, so only the real window sees every layer.
void _setWindowInsets(
  WidgetTester tester, {
  double systemBottom = 0,
  double systemTop = 0,
  double keyboard = 0,
}) {
  final double dpr = tester.view.devicePixelRatio;
  tester.view.padding = FakeViewPadding(
    top: systemTop * dpr,
    bottom: systemBottom * dpr,
  );
  tester.view.viewPadding = FakeViewPadding(
    top: systemTop * dpr,
    bottom: systemBottom * dpr,
  );
  tester.view.viewInsets = FakeViewPadding(bottom: keyboard * dpr);
}

Widget _host({required Widget child}) =>
    MaterialApp(theme: AppTheme.lightTheme, home: child);

Widget _barBackedScaffold() => Scaffold(
  body: const SizedBox.expand(key: _bodyKey),
  bottomNavigationBar: BottomActionBar(
    primary: BottomBarAction(label: 'Kirim', onPressed: _noop),
  ),
);

Iterable<File> _libSources() => Directory('lib')
    .listSync(recursive: true)
    .whereType<File>()
    .where((file) => file.path.endsWith('.dart'))
    .where((file) => !file.path.replaceAll(r'\', '/').contains('/generated/'));

String _read(String path) => File(path).readAsStringSync();

void main() {
  // ==========================================================================
  // POSITIVE PROOF — the bar owns the bottom reservation, exactly once
  // ==========================================================================
  group('SAFE-AREA-01 — the bottom action bar owns the bottom reservation', () {
    testWidgets('the body ends exactly at the bar; the bar owns the inset', (
      tester,
    ) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 48);

      await tester.pumpWidget(_host(child: _barBackedScaffold()));
      await tester.pumpAndSettle();

      final Rect surface = tester.getRect(find.byType(Scaffold));
      final Rect body = tester.getRect(find.byKey(_bodyKey));
      final Rect bar = tester.getRect(find.byType(BottomActionBar));

      // `Scaffold.bottomNavigationBar` already reserves the bar's region; a
      // body-side clearance would show up here as a gap.
      expect(
        body.bottom,
        closeTo(bar.top, 0.01),
        reason:
            'the body carried a SECOND bottom reservation beside the bar '
            '(body bottom ${body.bottom} != bar top ${bar.top})',
      );
      expect(surface.height, _surfaceHeight);

      // The bar IS the bottom surface: it reaches the screen bottom...
      expect(bar.bottom, closeTo(surface.bottom, 0.01));
      // ...and its own content clears the live system inset, exactly once.
      final Rect cta = tester.getRect(find.byType(ElevatedButton));
      expect(cta.bottom, lessThanOrEqualTo(surface.bottom - 48));
    });

    testWidgets('the bar follows the live inset: visible → changed → hidden', (
      tester,
    ) async {
      addTearDown(tester.view.reset);

      Future<double> barHeightAt(double systemBottom) async {
        _setWindowInsets(tester, systemBottom: systemBottom);
        await tester.pumpWidget(_host(child: _barBackedScaffold()));
        await tester.pumpAndSettle();
        return tester.getSize(find.byType(BottomActionBar)).height;
      }

      final double visible = await barHeightAt(48);
      final double hidden = await barHeightAt(0);
      final double changed = await barHeightAt(24);

      expect(
        visible - hidden,
        closeTo(48, 0.01),
        reason: 'the bar did not shrink when the system bar hid',
      );
      expect(
        changed - hidden,
        closeTo(24, 0.01),
        reason: 'the bar did not follow a changed inset',
      );
    });

    testWidgets('a hidden system bar leaves no stale gap in the bar', (
      tester,
    ) async {
      addTearDown(tester.view.reset);
      _setWindowInsets(tester, systemBottom: 0);

      await tester.pumpWidget(_host(child: _barBackedScaffold()));
      await tester.pumpAndSettle();

      final Rect body = tester.getRect(find.byKey(_bodyKey));
      final Rect bar = tester.getRect(find.byType(BottomActionBar));
      final Rect cta = tester.getRect(find.byType(ElevatedButton));

      // No inset → no reserved bottom space anywhere.
      expect(body.bottom, closeTo(bar.top, 0.01));
      expect(bar.bottom, closeTo(_surfaceHeight, 0.01));
      // The only space below the CTA is the bar's OWN design padding — a
      // leftover inset-sized gap would break this.
      expect(cta.bottom, closeTo(bar.bottom - AppMetrics.p12, 0.01));
    });
  });

  // ==========================================================================
  // POSITIVE PROOF — the canonical sheet consumes the live inset
  // ==========================================================================
  group('SAFE-AREA-01 — the canonical bottom sheet consumes the live inset', () {
    Future<void> showSheet(WidgetTester tester) async {
      await tester.pumpWidget(
        _host(
          child: Scaffold(
            body: Builder(
              builder: (BuildContext context) => Center(
                child: ElevatedButton(
                  onPressed: () => AppBottomSheetBase.show<void>(
                    context: context,
                    title: 'Tag People',
                    // No local padding: the geometry under test must be the
                    // sheet's own inset handling, not a content wrapper.
                    padding: EdgeInsets.zero,
                    content: const SizedBox(
                      height: 40,
                      child: SizedBox.expand(key: _sheetContentKey),
                    ),
                  ),
                  child: const Text('open'),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.tap(find.text('open'));
      await tester.pumpAndSettle();
    }

    testWidgets('system inset: consumed once, and gone when it is gone', (
      tester,
    ) async {
      addTearDown(tester.view.reset);

      _setWindowInsets(tester, systemBottom: 48, systemTop: 24);
      await showSheet(tester);
      final double withInset = tester
          .getRect(find.byKey(_sheetContentKey))
          .bottom;

      // Sheet content clears the system navigation area...
      expect(
        withInset,
        closeTo(_surfaceHeight - 48, 0.01),
        reason: 'sheet content did not clear the system bottom inset',
      );

      // ...and when the system bar disappears the space disappears with it.
      _setWindowInsets(tester, systemBottom: 0, systemTop: 24);
      await tester.pumpAndSettle();
      final double withoutInset = tester
          .getRect(find.byKey(_sheetContentKey))
          .bottom;

      expect(
        withoutInset,
        closeTo(_surfaceHeight, 0.01),
        reason: 'a stale system-inset-sized gap survived the hidden bar',
      );
      expect(withoutInset - withInset, closeTo(48, 0.01));
    });

    testWidgets('keyboard inset: content stays above it, no stale gap after', (
      tester,
    ) async {
      addTearDown(tester.view.reset);

      _setWindowInsets(tester, systemBottom: 48, systemTop: 24, keyboard: 300);
      await showSheet(tester);
      final double withKeyboard = tester
          .getRect(find.byKey(_sheetContentKey))
          .bottom;

      // Content (and the search field above it) stays above the keyboard.
      expect(
        withKeyboard,
        lessThanOrEqualTo(_surfaceHeight - 300),
        reason: 'sheet content was pushed under the keyboard',
      );

      // Keyboard down → the sheet returns to the system-inset position, with
      // no keyboard-sized gap left behind.
      _setWindowInsets(tester, systemBottom: 48, systemTop: 24, keyboard: 0);
      await tester.pumpAndSettle();
      final double withoutKeyboard = tester
          .getRect(find.byKey(_sheetContentKey))
          .bottom;

      expect(
        withoutKeyboard,
        closeTo(_surfaceHeight - 48, 0.01),
        reason: 'a stale keyboard-sized gap survived the closed keyboard',
      );
    });

    testWidgets(
      'availableHeight is the ONE ceiling: keyboard and top removed',
      (tester) async {
        addTearDown(tester.view.reset);
        _setWindowInsets(
          tester,
          systemBottom: 48,
          systemTop: 24,
          keyboard: 300,
        );

        late double measured;
        await tester.pumpWidget(
          _host(
            child: Builder(
              builder: (BuildContext context) {
                measured = AppBottomSheetBase.availableHeight(context);
                return const SizedBox.shrink();
              },
            ),
          ),
        );

        expect(measured, closeTo(_surfaceHeight - 300 - 24, 0.01));
        expect(math.max(0, measured), measured);
      },
    );
  });

  // ==========================================================================
  // NEGATIVE PROOF — the purged fixed clearances cannot come back
  // ==========================================================================
  group('SAFE-AREA-01 — obsolete fixed bottom clearances stay purged', () {
    test('no bar-backed screen reserves a fixed bottom clearance', () {
      final RegExp fixedClearance = RegExp(
        r'SizedBox\(\s*height:\s*(?:96|100)\b',
      );
      final List<String> offenders = <String>[];

      for (final File file in _libSources()) {
        final String path = file.path.replaceAll(r'\', '/');
        final String source = file.readAsStringSync();
        if (!source.contains('bottomNavigationBar:')) continue;
        if (fixedClearance.hasMatch(source)) {
          offenders.add('$path: fixed bottom clearance beside a bottom bar');
        }
        if (source.contains('AppMetrics.bottomBarClearance')) {
          offenders.add(
            '$path: bottomBarClearance beside a bottom bar — the bar already '
            'reserves that region',
          );
        }
      }

      expect(offenders, isEmpty, reason: offenders.join('\n'));
    });

    test('the five purged clearances and the duplicate token stay purged', () {
      const Map<String, List<String>> locks = <String, List<String>>{
        'lib/features/home/presentation/screens/home_screen.dart': <String>[
          'SizedBox(height: 100',
        ],
        'lib/domains/social/content/presentation/screens/'
            'content_detail_screen.dart': <String>[
          'SizedBox(height: 100',
        ],
        'lib/domains/commerce/transaction/checkout/presentation/screens/'
            'checkout_screen_impl.dart': <String>[
          'SizedBox(height: 100',
        ],
        'lib/domains/commerce/pricing/discount/presentation/screens/'
            'edit_discount_screen.dart': <String>[
          'SizedBox(height: 80',
        ],
        'lib/domains/commerce/pricing/discount/presentation/screens/'
            'create_discount_screen.dart': <String>[
          'SizedBox(height: 80',
        ],
        'lib/domains/commerce/transaction/shipping/presentation/widgets/'
            'shipping_option_setup_screen.dart': <String>[
          'bottomBarClearance',
        ],
      };

      final List<String> offenders = <String>[];
      for (final MapEntry<String, List<String>> entry in locks.entries) {
        final File file = File(entry.key);
        if (!file.existsSync()) {
          offenders.add('${entry.key} does not exist');
          continue;
        }
        final String source = file.readAsStringSync();
        for (final String banned in entry.value) {
          if (source.contains(banned)) {
            offenders.add('${entry.key} regained `$banned`');
          }
        }
      }

      expect(
        offenders,
        isEmpty,
        reason:
            'a purged bottom clearance was re-spelled. The layer that owns the '
            'bottom surface owns the inset; a body-side constant cannot follow '
            'a system UI change:\n${offenders.join('\n')}',
      );
    });

    test('the clearance comment idiom is gone', () {
      final List<String> offenders = <String>[];
      for (final File file in _libSources()) {
        final String source = file.readAsStringSync();
        if (source.contains('Space for bottom bar') ||
            source.contains('Space for bottom button')) {
          offenders.add(file.path.replaceAll(r'\', '/'));
        }
      }
      expect(offenders, isEmpty, reason: offenders.join('\n'));
    });

    test('a bar-less screen body consumes the inset with SafeArea', () {
      final String source = _read(
        'lib/domains/social/content/presentation/screens/'
        'content_detail_screen.dart',
      );
      expect(
        source,
        contains('body: SafeArea('),
        reason:
            'the screen has no bottom bar, so its BODY is the layer that owns '
            'the system bottom inset',
      );
      expect(source, isNot(contains('SizedBox(height: 100')));
    });

    test('the sheet base asks the live inset, not the raw view padding', () {
      final String source = _read(
        'lib/shared/widgets/app_bottom_sheet_base.dart',
      );
      expect(source, contains('MediaQuery.paddingOf(sheetContext).bottom'));
      expect(
        source,
        isNot(contains('viewPaddingOf(sheetContext).bottom')),
        reason:
            'viewPadding survives the keyboard, which would leave an '
            'inset-sized gap above it',
      );
    });

    test('the tag-people sheet takes its height from the sheet authority', () {
      final String source = _read(
        'lib/shared/widgets/user_search_bottom_sheet.dart',
      );
      expect(source, contains('AppBottomSheetBase.availableHeight(context)'));
      expect(
        source,
        isNot(contains('mediaQuery.size.height')),
        reason:
            'a fraction of the RAW screen height goes stale the moment the '
            'keyboard opens',
      );
    });
  });
}
