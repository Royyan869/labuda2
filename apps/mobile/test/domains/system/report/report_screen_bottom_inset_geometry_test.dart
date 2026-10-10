// SAFE-AREA-26 — REPORT SCREEN (route): BOTTOM-INSET GEOMETRY CONTRACT.
//
// `ReportScreen` is the ONE `/report` route destination and owns two branches:
//
//   * VALID branch — parses the target and renders the canonical
//     [ReportSubmissionScreen]. That screen's body-level `SafeArea` (the
//     SAFE-AREA-18 authority, locked by
//     `report_submission_bottom_inset_geometry_test.dart`) must remain the ONE
//     inset authority THROUGH this route: the route may not add a second
//     `SafeArea`/clearance around it (duplicate ownership), and it may not
//     drop the wrapper that binds the form to the body boundary. Geometry:
//     through the route the Submit CTA still follows the LIVE system inset
//     0 → 24 → 34 → 48 and never enters the system region.
//
//   * INVALID branch — unparsable target: a single centered, non-scrollable
//     message with no bottom-anchored content. Contract: that message never
//     enters the system navigation region at any inset (0/24/34/48) in the
//     phone-portrait or short-landscape regime, and lays out without
//     exceptions. There is deliberately NO `SafeArea` here: measured, the
//     centered content clears the system region by design geometry alone, so
//     an inset authority would be a workaround without a defect (SAFE-AREA-26
//     root cause G). This test locks that invariant — it fails if the message
//     is ever anchored into the system region.
//
// RENDER-BASED on purpose: every number comes from the actual rendered route
// under injected window metrics — never a source string, never a bare
// `find.byType(SafeArea)` presence check on its own.
//
// VIEWPORT REGIMES: the valid branch runs on the default test view (800×600)
// and short landscape (640×360). It is NOT run at 393 px portrait here:
// `ReportDescriptionField`'s label `Row` overflows under the test box font
// below ~522 px width — a HORIZONTAL, non-safe-area finding reported and left
// out of scope by SAFE-AREA-26 (see the audit report). The invalid branch has
// no such content and runs at 393×852 portrait.
//
// KEYBOARD: the valid branch's keyboard contract (lift exactly once, restore
// to system-inset geometry) is owned and locked by SAFE-AREA-18's keyboard
// test on the identical body tree. The invalid branch has no text input —
// keyboard regime N/A.
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/system/report/presentation/screens/report_screen.dart';

const String _invalidMsg = 'Unable to load report information';
const String _ctaLabel = 'Submit Report';

Widget _app(Widget home) => ProviderScope(
  child: MaterialApp(theme: AppTheme.lightTheme, home: home),
);

Widget _validRoute() => _app(
  const ReportScreen(
    targetType: 'content',
    targetId: 'target-1',
    targetTitle: 'Target title',
  ),
);

Widget _invalidRoute() => _app(const ReportScreen());

/// Window metrics for the TEST VIEW (physical pixels, like a device).
/// Platform semantics: `padding` is whatever `viewInsets` (the keyboard) has
/// NOT consumed of `viewPadding` — an open keyboard zeroes the bottom padding
/// exactly like Android does, so the SafeArea yields to it.
void _setWindowInsets(
  WidgetTester tester, {
  double systemBottom = 0,
  double keyboard = 0,
}) {
  final double dpr = tester.view.devicePixelRatio;
  tester.view.padding = FakeViewPadding(
    bottom: math.max(0.0, systemBottom - keyboard) * dpr,
  );
  tester.view.viewPadding = FakeViewPadding(bottom: systemBottom * dpr);
  tester.view.viewInsets = FakeViewPadding(bottom: keyboard * dpr);
}

/// Puts the route's form at its scroll END — where the Submit CTA rests
/// lowest and therefore bounds "never enters the system region".
Future<void> _scrollToEnd(WidgetTester tester) async {
  final Finder scroll = find.byType(SingleChildScrollView);
  final Finder scrollable = find.descendant(
    of: scroll,
    matching: find.byType(Scrollable),
  );
  for (int i = 0; i < 80; i++) {
    final ScrollPosition position = tester
        .state<ScrollableState>(scrollable.first)
        .position;
    if (position.pixels >= position.maxScrollExtent - 0.5) return;
    await tester.drag(scroll, const Offset(0, -800));
    await tester.pumpAndSettle();
  }
  fail('the report form never reached its scroll end');
}

double _surfaceBottom(WidgetTester tester) =>
    tester.getRect(find.byType(Scaffold)).bottom;

double _ctaBottom(WidgetTester tester) =>
    tester.getBottomRight(find.widgetWithText(FilledButton, _ctaLabel)).dy;

/// Bottom edge of the invalid-branch message INCLUDING its design `p24`
/// padding — the content box that must stay out of the system region.
double _msgBoxBottom(WidgetTester tester) {
  final Finder padding = find.ancestor(
    of: find.textContaining(_invalidMsg),
    matching: find.byType(Padding),
  );
  return tester.getRect(padding.first).bottom;
}

void main() {
  group(
    'VALID route branch — the route may not change the inset authority',
    () {
      testWidgets(
        'exactly ONE SafeArea wraps the body scroll view through the route',
        (tester) async {
          addTearDown(tester.view.reset);
          _setWindowInsets(tester, systemBottom: 34);
          await tester.pumpWidget(_validRoute());
          await tester.pumpAndSettle();

          // Structure check SUPPLEMENTARY to the geometry below: a second
          // SafeArea here would be duplicate bottom-inset ownership, a missing
          // one would leave the form with no body-level authority. The
          // AppBar's framework `SafeArea(bottom: false)` wraps only the app
          // bar, never the scroll view, so it is not counted.
          expect(
            find.ancestor(
              of: find.byType(SingleChildScrollView),
              matching: find.byType(SafeArea),
            ),
            findsOneWidget,
            reason:
                'exactly ONE SafeArea must wrap the body scroll view of '
                'the route — duplicate or missing ownership both fail here',
          );
        },
      );

      testWidgets(
        'inset 0 → 24/34/48: the Submit CTA follows the LIVE inset 1:1 '
        'and never enters the system region (default 800×600 view)',
        (tester) async {
          addTearDown(tester.view.reset);
          _setWindowInsets(tester, systemBottom: 0);
          await tester.pumpWidget(_validRoute());
          await tester.pumpAndSettle();
          await _scrollToEnd(tester);

          final double surface = _surfaceBottom(tester);
          final double cta0 = _ctaBottom(tester);

          // Design trailing spacing at inset 0 (the scroll view's explicit
          // `AppMetrics.p24`), asserted so a fixed bottom clearance cannot
          // hide behind it.
          expect(
            surface - cta0,
            closeTo(AppMetrics.p24, 0.01),
            reason:
                'at inset 0 the gap below the Submit CTA must be the '
                'design spacing p24 — larger would be a fixed clearance',
          );

          for (final double inset in const <double>[24, 34, 48]) {
            _setWindowInsets(tester, systemBottom: inset);
            await tester.pumpAndSettle();
            await _scrollToEnd(tester);
            final double cta = _ctaBottom(tester);

            expect(
              cta,
              closeTo(cta0 - inset, 0.01),
              reason:
                  'the Submit CTA did not follow the live inset $inset '
                  'through the route (got $cta, expected ${cta0 - inset})',
            );
            expect(
              cta,
              lessThanOrEqualTo(surface - inset + 0.01),
              reason:
                  'the Submit CTA entered the system region at inset '
                  '$inset',
            );
          }
        },
      );

      testWidgets(
        'short landscape 640×360: same contract, including the scroll premise',
        (tester) async {
          addTearDown(tester.view.reset);
          tester.view.physicalSize = const Size(640, 360);
          tester.view.devicePixelRatio = 1.0;
          _setWindowInsets(tester, systemBottom: 0);
          await tester.pumpWidget(_validRoute());
          await tester.pumpAndSettle();
          await _scrollToEnd(tester);

          final double surface = _surfaceBottom(tester);
          final double cta0 = _ctaBottom(tester);
          expect(
            surface - cta0,
            closeTo(AppMetrics.p24, 0.01),
            reason:
                'design spacing below the CTA must stay p24 at inset 0 '
                'in the short regime',
          );

          for (final double inset in const <double>[24, 34, 48]) {
            _setWindowInsets(tester, systemBottom: inset);
            await tester.pumpAndSettle();
            await _scrollToEnd(tester);
            final double cta = _ctaBottom(tester);
            expect(
              cta,
              closeTo(cta0 - inset, 0.01),
              reason:
                  'the Submit CTA did not follow the live inset $inset '
                  'in the short regime (got $cta, expected ${cta0 - inset})',
            );
            expect(
              cta,
              lessThanOrEqualTo(surface - inset + 0.01),
              reason:
                  'the Submit CTA entered the system region at inset '
                  '$inset in the short regime',
            );
          }
        },
      );
    },
  );

  group(
    'INVALID route branch — the centered message stays out of the system region',
    () {
      testWidgets('phone portrait 393×852, insets 0/24/34/48', (tester) async {
        addTearDown(tester.view.reset);
        tester.view.physicalSize = const Size(393, 852);
        tester.view.devicePixelRatio = 1.0;
        _setWindowInsets(tester, systemBottom: 0);
        await tester.pumpWidget(_invalidRoute());
        await tester.pumpAndSettle();

        expect(find.textContaining(_invalidMsg), findsOneWidget);

        for (final double inset in const <double>[0, 24, 34, 48]) {
          _setWindowInsets(tester, systemBottom: inset);
          await tester.pumpAndSettle();

          expect(tester.takeException(), isNull);
          expect(
            _msgBoxBottom(tester),
            lessThanOrEqualTo(_surfaceBottom(tester) - inset + 0.01),
            reason:
                'the invalid-parameter message entered the system region '
                'at inset $inset — content would sit behind the system bar',
          );
        }
      });

      testWidgets('short landscape 640×360, insets 0/24/34/48', (tester) async {
        addTearDown(tester.view.reset);
        tester.view.physicalSize = const Size(640, 360);
        tester.view.devicePixelRatio = 1.0;
        _setWindowInsets(tester, systemBottom: 0);
        await tester.pumpWidget(_invalidRoute());
        await tester.pumpAndSettle();

        for (final double inset in const <double>[0, 24, 34, 48]) {
          _setWindowInsets(tester, systemBottom: inset);
          await tester.pumpAndSettle();

          expect(tester.takeException(), isNull);
          expect(
            _msgBoxBottom(tester),
            lessThanOrEqualTo(_surfaceBottom(tester) - inset + 0.01),
            reason:
                'the invalid-parameter message entered the system region '
                'at inset $inset in the short regime',
          );
        }
      });
    },
  );
}
