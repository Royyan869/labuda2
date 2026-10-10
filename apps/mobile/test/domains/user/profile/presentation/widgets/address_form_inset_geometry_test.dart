// BOTTOMSHEET-02 / BOTTOMSHEET-02-FIT-GAP — ADDRESS FORM INSET + FIT
// AUTHORITY (geometry proof).
//
// The AddressFormDialog body rides the canonical `AppBottomSheetBase` sheet
// (hosts: address list, checkout, seller wizard). Ownership this file pins:
//
//   * AppBottomSheetBase — the ONE keyboard lift (`Padding` over
//     `viewInsets.bottom`), the ONE system-bottom reservation
//     (`SizedBox(MediaQuery.paddingOf.bottom)` — measured here as the outer
//     scroll viewport bottom), and the sheet ceiling;
//   * BottomActionBar(embeddedInLiftedSheet: true) — no second keyboard
//     rise AND no second system-bottom reservation (BOTTOMSHEET-03): the
//     CTA rests at the form bottom minus the bar's design padding at EVERY
//     inset; the base spacer below the content region is the ONE
//     system-bottom spend;
//   * the AddressForm content — NO keyboard reservation of its own: the
//     ListView tail is pure design spacing (`p24`), identical with and
//     without a keyboard. A resurrected `+ viewInsets.bottom` tail reads
//     `p24 + keyboard` (324 px at keyboard 300) here.
//
//   A — keyboard closed (system inset 0 / 24 / 34 / 48): the outer scroll
//       viewport bottom = surface − inset (the base spacer, exactly once,
//       live 1:1); the scroll tail stays design-sized;
//   B — keyboard 300: the base lifts the content clip exactly once; the
//       embedded bar does not rise again; the scroll tail does NOT grow by
//       the keyboard; the last field stays reachable above the keyboard;
//   C — independence both directions: flipping the system inset while the
//       keyboard is open moves nothing; closing restores the system
//       geometry with no stale band;
//   D — source proof: the address form carries no inset arithmetic;
//   E — allocation / ceiling convergence (BOTTOMSHEET-02-FIT-GAP): the
//       form fills the sheet's live content allocation exactly, the base
//       outer scroll carries NO dead band (the old 72+N px overflow), and
//       the CTA rests fully above the content clip in every scenario.
//
// MEASUREMENT MODEL (the fit gap is RESOLVED — assertions are
// overflow-SENSITIVE): the base owns the ONE sheet ceiling
// (`availableHeight * 0.9`) and publishes the live content allocation
// (`AppBottomSheetBase.contentAllocationOf`, read from the column layout
// minus the content wrap); the form fills exactly that allocation and
// claims no ceiling of its own, so the 72+N px band and the sub-clip CTA
// cannot exist — asserted in the FIT group as outer extent 0, form =
// ceiling − handle − spacer − wrap, and CTA clearance = wrap + p12 +
// spacer. The embedded bar adds NO system-bottom reservation of its own
// (BOTTOMSHEET-03, Owner decision): its height is inset-independent, and
// the only space below the CTA is the design gap (p12 + p24) plus the base
// spacer — exactly ONE N, never two.
//
// RENDER-BASED on purpose: every number comes from the actual rendered sheet
// under injected window metrics — never a source string alone.
//
// View: 560×800 logical (1680×2400 @ dpr 3); `systemTop: 24` is injected.
// `padding` follows platform semantics (`viewPadding − viewInsets`, clamped):
// an open keyboard zeroes the bottom padding like Android does.
import 'dart:io';
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/user/profile/presentation/widgets/address_form_dialog.dart';
import 'package:hishumi/generated/app_localizations.dart';
import 'package:hishumi/shared/models/wilayah_models.dart';
import 'package:hishumi/shared/providers/authenticated_account_provider.dart';
import 'package:hishumi/shared/providers/wilayah_provider_simple.dart';
import 'package:hishumi/shared/widgets/app_bottom_sheet_base.dart';
import 'package:hishumi/shared/widgets/bottom_action_bar.dart';

/// Exact CTA label (addressToEdit == null → "Add" mode).
const String _ctaLabel = 'Save Address';

const double _systemTop = 24;
const double _keyboard = 300;

double _p24() => AppMetrics.p24;
double _p12() => AppMetrics.p12;

/// Window metrics on the TEST VIEW (physical pixels, like a device).
///
/// Platform semantics: `padding` is whatever `viewInsets` (the keyboard) has
/// NOT consumed of `viewPadding` — an open keyboard zeroes the bottom padding
/// exactly like Android does, so every SafeArea below the top layer yields.
void _setWindowInsets(
  WidgetTester tester, {
  double systemBottom = 0,
  double keyboard = 0,
}) {
  final double dpr = tester.view.devicePixelRatio;
  tester.view.padding = FakeViewPadding(
    top: _systemTop * dpr,
    bottom: math.max(0.0, systemBottom - keyboard) * dpr,
  );
  tester.view.viewPadding = FakeViewPadding(
    top: _systemTop * dpr,
    bottom: systemBottom * dpr,
  );
  tester.view.viewInsets = FakeViewPadding(bottom: keyboard * dpr);
}

Widget _host() => ProviderScope(
  // Hermetic: no signed-in session (autofill early-returns) and no geography
  // HTTP (the province dropdown renders an empty list instantly).
  overrides: [
    authenticatedUserProvider.overrideWithValue(null),
    provincesProvider.overrideWith((ref) async => const <Province>[]),
  ],
  child: MaterialApp(
    theme: AppTheme.lightTheme,
    localizationsDelegates: AppLocalizations.localizationsDelegates,
    supportedLocales: AppLocalizations.supportedLocales,
    locale: const Locale('id'),
    home: Scaffold(
      body: Center(
        child: Builder(
          builder: (BuildContext context) => ElevatedButton(
            onPressed: () => AppBottomSheetBase.show<void>(
              context: context,
              content: const AddressFormDialog(),
            ),
            child: const Text('open'),
          ),
        ),
      ),
    ),
  ),
);

Future<void> _openSheet(WidgetTester tester) async {
  await tester.pumpWidget(_host());
  await tester.pumpAndSettle();
  await tester.tap(find.text('open'));
  await tester.pumpAndSettle();
  expect(find.byType(AddressFormDialog), findsOneWidget);
  expect(find.byType(BottomSheet), findsOneWidget);
}

/// Puts the form ListView at its scroll END — the state where the last field
/// rests highest and therefore bounds "no second keyboard reservation".
/// Drags like a user (re-reading maxExtent every step: a sliver's extent
/// starts as a lazy estimate and grows as children inflate). Re-run after
/// every metrics change: the extent moves when the sheet resizes.
Future<void> _scrollFormToEnd(WidgetTester tester) async {
  final Finder list = _formList();
  expect(list, findsOneWidget, reason: 'form ListView not found');
  final Finder scrollable = find.descendant(
    of: list,
    matching: find.byType(Scrollable),
  );
  for (int i = 0; i < 80; i++) {
    final ScrollPosition position = tester
        .state<ScrollableState>(scrollable.first)
        .position;
    if (position.pixels >= position.maxScrollExtent - 0.5) return;
    await tester.drag(list, const Offset(0, -600));
    await tester.pumpAndSettle();
  }
  final ScrollPosition stuck = tester
      .state<ScrollableState>(scrollable.first)
      .position;
  fail(
    'the form never reached its scroll end '
    '(pixels ${stuck.pixels} vs max ${stuck.maxScrollExtent})',
  );
}

Finder _formList() => find.byWidgetPredicate(
  (w) => w is ListView && w.scrollDirection == Axis.vertical,
);

double _maxExtent(WidgetTester tester) {
  final Finder list = _formList();
  return tester
      .state<ScrollableState>(
        find.descendant(of: list, matching: find.byType(Scrollable)).first,
      )
      .position
      .maxScrollExtent;
}

/// Bottom of the Scaffold surface (the screen bottom under test).
double _surfaceBottom(WidgetTester tester) =>
    tester.getRect(find.byType(Scaffold)).bottom;

/// Top edge of the sheet's rendered surface (the modal Material).
double _sheetTop(WidgetTester tester) =>
    tester.getRect(find.byType(BottomSheet)).top;

/// Bottom edge of the sheet's rendered surface (the modal Material).
double _sheetBottom(WidgetTester tester) =>
    tester.getRect(find.byType(BottomSheet)).bottom;

double _formTop(WidgetTester tester) =>
    tester.getRect(find.byType(AddressFormDialog)).top;

double _formBottom(WidgetTester tester) =>
    tester.getRect(find.byType(AddressFormDialog)).bottom;

/// The BASE's outer content scroll — its viewport bottom IS the content
/// clip boundary: surface − system spacer − keyboard lift. This rect is the
/// overflow-insensitive carrier of both inset authorities.
Rect _outerScroll(WidgetTester tester) {
  final Finder outer = find.byType(SingleChildScrollView);
  return tester.getRect(outer.first);
}

double _outerPixels(WidgetTester tester) {
  final Finder outer = find.byType(SingleChildScrollView);
  if (outer.evaluate().isEmpty) return 0.0;
  return tester
      .state<ScrollableState>(
        find.descendant(of: outer, matching: find.byType(Scrollable)).first,
      )
      .position
      .pixels;
}

double _outerExtent(WidgetTester tester) {
  final Finder outer = find.byType(SingleChildScrollView);
  if (outer.evaluate().isEmpty) return 0.0;
  return tester
      .state<ScrollableState>(
        find.descendant(of: outer, matching: find.byType(Scrollable)).first,
      )
      .position
      .maxScrollExtent;
}

double _ctaTop(WidgetTester tester) =>
    tester.getTopLeft(find.widgetWithText(ElevatedButton, _ctaLabel)).dy;

double _ctaBottom(WidgetTester tester) =>
    tester.getBottomRight(find.widgetWithText(ElevatedButton, _ctaLabel)).dy;

/// The sheet body ceiling exactly as the form itself computes it — read
/// from the live context inside the sheet (base authority, applied by the
/// AddressForm root).
double _availableHeightInSheet(WidgetTester tester) =>
    AppBottomSheetBase.availableHeight(
      tester.element(find.byType(AddressFormDialog)),
    );

/// The AddressForm header icon — a drag landing here is claimed by the
/// BASE's outer scroll (it sits outside the form ListView), which is the
/// only gesture path that moves the outer scroll at all.
Finder _headerIcon() => find.byIcon(Icons.add_location);

Rect _viewport(WidgetTester tester) => tester.getRect(_formList());

Rect _barRect(WidgetTester tester) =>
    tester.getRect(find.byType(BottomActionBar));

/// The postal-code field is the LAST form field (last vertical child of the
/// ListView). Built only when the sliver reaches it — which is exactly the
/// reachability question under a resurrected keyboard tail. `skipOffstage:
/// false` on purpose: at scroll end a resurrected tail parks the last field
/// ABOVE the viewport, where the default finder would report "absent"
/// instead of the measurable distance that proves the defect.
Finder _lastField() => find.byType(TextFormField, skipOffstage: false).last;

void _dump(
  String tag, {
  required double surface,
  required double systemBottom,
  required double keyboard,
  double? sheetTop,
  double? sheetBottom,
  double? formTop,
  double? formBottom,
  double? ctaBottom,
  Rect? bar,
  Rect? viewport,
  Rect? outer,
  double? maxExtent,
  double? outerExtent,
  double? lastTop,
  double? lastBottom,
}) {
  final double? outerBottom = outer?.bottom;
  // Canonical clip: the keyboard consumes the bottom padding when open,
  // so the system inset only applies while the keyboard is closed.
  final double wantClip = surface - (keyboard > 0 ? keyboard : systemBottom);
  final String tail = (viewport != null && lastBottom != null)
      ? (viewport.bottom - lastBottom).toStringAsFixed(1)
      : 'n/a';
  debugPrint(
    '[B02/$tag] surface=$surface sys=$systemBottom kb=$keyboard '
    'sheet=[${sheetTop?.toStringAsFixed(1)},${sheetBottom?.toStringAsFixed(1)}] '
    'form=[${formTop?.toStringAsFixed(1)},${formBottom?.toStringAsFixed(1)}] '
    'outerClip=${outerBottom?.toStringAsFixed(1) ?? 'n/a'} '
    '(want ${wantClip.toStringAsFixed(1)}) '
    'outerMax=${outerExtent?.toStringAsFixed(1)} '
    'vp=${viewport == null ? 'n/a' : '[${viewport.top.toStringAsFixed(1)},${viewport.bottom.toStringAsFixed(1)}]'} '
    'innerMax=${maxExtent?.toStringAsFixed(1)} '
    'bar=${bar == null ? 'n/a' : '[${bar.top.toStringAsFixed(1)},${bar.bottom.toStringAsFixed(1)}] h=${bar.height.toStringAsFixed(1)}'} '
    'ctaBottom=${ctaBottom?.toStringAsFixed(1)} '
    'lastTop=${lastTop?.toStringAsFixed(1) ?? 'ABSENT'} '
    'lastBottom=${lastBottom?.toStringAsFixed(1) ?? 'ABSENT'} '
    'tailGap=$tail kbTop=${(surface - keyboard).toStringAsFixed(1)}',
  );
}

void main() {
  group('BOTTOMSHEET-02 (A) — keyboard closed: one system-inset consumer', () {
    for (final double inset in const <double>[0, 24, 34, 48]) {
      testWidgets('system inset $inset: content clip = surface − inset', (
        tester,
      ) async {
        addTearDown(tester.view.reset);
        tester.view.physicalSize = const Size(1680, 2400);
        tester.view.devicePixelRatio = 3.0;
        _setWindowInsets(tester, systemBottom: inset, keyboard: 0);
        await _openSheet(tester);

        final double surface = _surfaceBottom(tester);

        // Contract premise: the form exceeds one viewport, so the scroll-end
        // geometry below is the regime under test (loud re-audit if not).
        await _scrollFormToEnd(tester);
        expect(
          _maxExtent(tester),
          greaterThan(0),
          reason:
              'the address form no longer exceeds one viewport — re-audit '
              'the scroll-end geometry contract',
        );

        final Rect outer = _outerScroll(tester);
        final Rect viewport = _viewport(tester);
        final Finder lastField = _lastField();
        final bool built = lastField.evaluate().isNotEmpty;
        final double? lastTop = built ? tester.getTopLeft(lastField).dy : null;
        final double? lastBottom = built
            ? tester.getBottomRight(lastField).dy
            : null;

        _dump(
          'A$inset',
          surface: surface,
          systemBottom: inset,
          keyboard: 0,
          sheetTop: _sheetTop(tester),
          sheetBottom: _sheetBottom(tester),
          formTop: _formTop(tester),
          formBottom: _formBottom(tester),
          ctaBottom: _ctaBottom(tester),
          bar: _barRect(tester),
          viewport: viewport,
          outer: outer,
          maxExtent: _maxExtent(tester),
          outerExtent: _outerExtent(tester),
          lastTop: lastTop,
          lastBottom: lastBottom,
        );

        // ONE system-inset consumer: the base's spacer below the content —
        // the content clip boundary sits exactly surface − inset. A second
        // consumer at THIS layer would move the clip a second time; content-
        // side reservations cannot move it at all (they only grow the tail).
        expect(
          outer.bottom,
          closeTo(surface - inset, 0.01),
          reason:
              'content clip ${outer.bottom} != surface $surface − inset '
              '$inset — the sheet must consume the system bottom inset '
              'exactly once, live',
        );

        // The sheet surface reaches the screen bottom.
        expect(
          _sheetBottom(tester),
          closeTo(surface, 0.01),
          reason: 'the sheet surface does not rest on the screen bottom',
        );

        // Scroll tail with the keyboard closed: design spacing only (the
        // defect under test only exists while the keyboard is open).
        expect(
          built,
          isTrue,
          reason: 'the last field was never built at scroll end',
        );
        final double tail = viewport.bottom - lastBottom!;
        expect(
          tail,
          closeTo(_p24(), 0.01),
          reason:
              'the scroll tail under the last field is $tail px, expected '
              'p24 — a reservation landed in the tail',
        );
        expect(
          lastTop,
          greaterThanOrEqualTo(viewport.top - 0.01),
          reason: 'the last field is outside the viewport at scroll end',
        );
      });
    }

    testWidgets('a changed system inset moves the content clip 1:1', (
      tester,
    ) async {
      addTearDown(tester.view.reset);
      tester.view.physicalSize = const Size(1680, 2400);
      tester.view.devicePixelRatio = 3.0;

      final Map<double, double> clipByInset = <double, double>{};
      for (final double inset in const <double>[24, 48]) {
        _setWindowInsets(tester, systemBottom: inset, keyboard: 0);
        if (clipByInset.isEmpty) {
          await _openSheet(tester);
        } else {
          await tester.pumpAndSettle();
        }
        clipByInset[inset] = _outerScroll(tester).bottom;
      }

      expect(
        clipByInset[24]! - clipByInset[48]!,
        closeTo(24, 0.01),
        reason:
            'the content clip moved ${clipByInset[24]! - clipByInset[48]!} '
            'px for a 24 px inset change — the system inset must be '
            'consumed exactly once, live 1:1',
      );
    });
  });

  group('BOTTOMSHEET-02 (B) — keyboard 300: one lift, one tail', () {
    testWidgets('base lifts the content clip exactly once; the bar yields', (
      tester,
    ) async {
      addTearDown(tester.view.reset);
      tester.view.physicalSize = const Size(1680, 2400);
      tester.view.devicePixelRatio = 3.0;
      _setWindowInsets(tester, systemBottom: 48, keyboard: 0);
      await _openSheet(tester);
      final double surface = _surfaceBottom(tester);

      _setWindowInsets(tester, systemBottom: 48, keyboard: _keyboard);
      await tester.pumpAndSettle();

      final Rect outer = _outerScroll(tester);
      final double formBottom = _formBottom(tester);
      final double ctaBottom = _ctaBottom(tester);
      final Rect bar = _barRect(tester);

      _dump(
        'B-lift',
        surface: surface,
        systemBottom: 48,
        keyboard: _keyboard,
        sheetTop: _sheetTop(tester),
        sheetBottom: _sheetBottom(tester),
        formTop: _formTop(tester),
        formBottom: formBottom,
        ctaBottom: ctaBottom,
        bar: bar,
        viewport: _viewport(tester),
        outer: outer,
        maxExtent: _maxExtent(tester),
        outerExtent: _outerExtent(tester),
      );

      // The surface still reaches the screen bottom (the sheet extends
      // behind the keyboard); the CONTENT clip sits at the keyboard top:
      // lift = viewInsets exactly ONCE (the system spacer yields — platform
      // padding is 0 under the keyboard). No lift would clip at the screen
      // bottom; a second lift would clip a keyboard-height higher.
      expect(
        _sheetBottom(tester),
        closeTo(surface, 0.01),
        reason:
            'the sheet surface left the screen bottom when the '
            'keyboard opened',
      );
      expect(
        outer.bottom,
        closeTo(surface - _keyboard, 0.01),
        reason:
            'content clip ${outer.bottom} != surface $surface − keyboard '
            '$_keyboard — expected exactly ONE keyboard lift (the base '
            'Padding). No lift would read ${surface - 48.0}; a second lift '
            'would read ${surface - 2 * _keyboard}.',
      );

      // The embedded CTA rides the lifted sheet: the bar does NOT rise a
      // second time (its own viewInsets lift is off), its SafeArea yields
      // to the keyboard, and the CTA rests at form bottom − design p12.
      // A non-embedded bar would sit a keyboard-height above that.
      expect(
        ctaBottom,
        closeTo(formBottom - _p12(), 0.01),
        reason:
            'the CTA is ${formBottom - ctaBottom} px above the form bottom '
            '(expected p12 ${_p12()}) — the bar reserved an inset of its '
            'own inside the lifted sheet',
      );
    });

    testWidgets('the scroll tail does NOT grow by the keyboard height', (
      tester,
    ) async {
      addTearDown(tester.view.reset);
      tester.view.physicalSize = const Size(1680, 2400);
      tester.view.devicePixelRatio = 3.0;
      _setWindowInsets(tester, systemBottom: 48, keyboard: _keyboard);
      await _openSheet(tester);

      final double surface = _surfaceBottom(tester);
      await _scrollFormToEnd(tester);

      final Rect viewport = _viewport(tester);
      final Rect outer = _outerScroll(tester);
      final Finder lastField = _lastField();
      final bool built = lastField.evaluate().isNotEmpty;
      final double? lastTop = built ? tester.getTopLeft(lastField).dy : null;
      final double? lastBottom = built
          ? tester.getBottomRight(lastField).dy
          : null;

      _dump(
        'B-tail',
        surface: surface,
        systemBottom: 48,
        keyboard: _keyboard,
        sheetTop: _sheetTop(tester),
        sheetBottom: _sheetBottom(tester),
        formTop: _formTop(tester),
        formBottom: _formBottom(tester),
        ctaBottom: _ctaBottom(tester),
        bar: _barRect(tester),
        viewport: viewport,
        outer: outer,
        maxExtent: _maxExtent(tester),
        outerExtent: _outerExtent(tester),
        lastTop: lastTop,
        lastBottom: lastBottom,
      );

      // Premise: the tail question only exists while the form scrolls.
      expect(
        _maxExtent(tester),
        greaterThan(0),
        reason: 'the address form no longer exceeds one viewport',
      );

      expect(
        built,
        isTrue,
        reason:
            'the last field was never built even at max scroll — the tail '
            'is larger than viewport+cacheExtent, i.e. an unreachable '
            'keyboard-sized reservation',
      );

      // ONE keyboard authority: with the sheet already lifted by the base,
      // the ListView tail is design spacing only. A resurrected
      // `+ viewInsets.bottom` reads p24 + 300 = 324 px here.
      final double tail = viewport.bottom - lastBottom!;
      expect(
        tail,
        closeTo(_p24(), 0.01),
        reason:
            'the scroll tail under the last field is $tail px with the '
            'keyboard open (expected p24 ${_p24()}) — a SECOND keyboard '
            'reservation inside a sheet the base already lifted',
      );

      // Reachability: the last field enters the viewport at scroll end and
      // stays above the content clip (keyboard top).
      expect(
        lastTop!,
        greaterThanOrEqualTo(viewport.top - 0.01),
        reason:
            'the last field is above the viewport at scroll end (top '
            '$lastTop, viewport ${viewport.top}) — unreachable content '
            'caused by an inflated tail',
      );
      expect(
        lastBottom,
        lessThanOrEqualTo(outer.bottom + 0.01),
        reason: 'the last field sits below the keyboard-top content clip',
      );
    });

    testWidgets(
      'independence: system inset flips under an open keyboard change '
      'nothing; closing restores system geometry with no stale band',
      (tester) async {
        addTearDown(tester.view.reset);
        tester.view.physicalSize = const Size(1680, 2400);
        tester.view.devicePixelRatio = 3.0;
        _setWindowInsets(tester, systemBottom: 48, keyboard: _keyboard);
        await _openSheet(tester);
        final double surface = _surfaceBottom(tester);

        // Keyboard open — flip the system inset 48 → 0: the keyboard
        // consumes the bottom padding, so NOTHING may move.
        final double clipUnderKeyboard = _outerScroll(tester).bottom;
        expect(
          clipUnderKeyboard,
          closeTo(surface - _keyboard, 0.01),
          reason: 'control measurement under the keyboard drifted',
        );
        _setWindowInsets(tester, systemBottom: 0, keyboard: _keyboard);
        await tester.pumpAndSettle();
        expect(
          _outerScroll(tester).bottom,
          closeTo(clipUnderKeyboard, 0.01),
          reason:
              'the content clip moved when the (keyboard-covered) system '
              'inset changed — keyboard and system insets are not separated',
        );
        _dump(
          'C-kb',
          surface: surface,
          systemBottom: 0,
          keyboard: _keyboard,
          sheetTop: _sheetTop(tester),
          sheetBottom: _sheetBottom(tester),
          formTop: _formTop(tester),
          formBottom: _formBottom(tester),
          ctaBottom: _ctaBottom(tester),
          bar: _barRect(tester),
          viewport: _viewport(tester),
          outer: _outerScroll(tester),
        );

        // Keyboard closes (system inset 48 live again): the clip returns to
        // the system-inset geometry — no stale keyboard band — and the
        // scroll tail is design spacing again.
        _setWindowInsets(tester, systemBottom: 48, keyboard: 0);
        await tester.pumpAndSettle();
        await _scrollFormToEnd(tester);

        final Rect outerClosed = _outerScroll(tester);
        final Rect viewport = _viewport(tester);
        final Finder lastField = _lastField();
        final bool built = lastField.evaluate().isNotEmpty;
        final double? lastBottom = built
            ? tester.getBottomRight(lastField).dy
            : null;

        _dump(
          'C-closed',
          surface: surface,
          systemBottom: 48,
          keyboard: 0,
          sheetTop: _sheetTop(tester),
          sheetBottom: _sheetBottom(tester),
          formTop: _formTop(tester),
          formBottom: _formBottom(tester),
          ctaBottom: _ctaBottom(tester),
          bar: _barRect(tester),
          viewport: viewport,
          outer: outerClosed,
          maxExtent: _maxExtent(tester),
          outerExtent: _outerExtent(tester),
          lastBottom: lastBottom,
        );

        expect(
          outerClosed.bottom,
          closeTo(surface - 48, 0.01),
          reason:
              'after the keyboard closed the content clip is '
              '${outerClosed.bottom} — expected the live system-inset '
              'geometry; a stale keyboard-sized band survives',
        );
        expect(
          built,
          isTrue,
          reason: 'the last field is not built at scroll end',
        );
        final double tail = viewport.bottom - lastBottom!;
        expect(
          tail,
          closeTo(_p24(), 0.01),
          reason: 'the closed-state scroll tail is $tail px (expected p24)',
        );
      },
    );
  });

  group('BOTTOMSHEET-02-FIT-GAP — allocation / ceiling convergence', () {
    /// One FIT scenario: open the sheet under the given window metrics and
    /// measure every allocation boundary. No scroll is performed first —
    /// this is the AT-REST state a user first sees.
    Future<void> fitMeasure(
      WidgetTester tester, {
      required double systemBottom,
      required double keyboard,
      required String tag,
    }) async {
      _setWindowInsets(tester, systemBottom: systemBottom, keyboard: keyboard);
      await _openSheet(tester);

      final double surface = _surfaceBottom(tester);
      final double available = _availableHeightInSheet(tester);
      final double ceiling = available * 0.9;
      final Rect sheet = tester.getRect(find.byType(BottomSheet));
      final Rect outer = _outerScroll(tester);
      final Rect viewport = _viewport(tester);
      final Rect bar = _barRect(tester);
      final double formTop = _formTop(tester);
      final double formBottom = _formBottom(tester);
      final double outerMax = _outerExtent(tester);
      final double innerMax = _maxExtent(tester);
      final double ctaTop = _ctaTop(tester);
      final double ctaBottom = _ctaBottom(tester);
      final double handleH = outer.top - sheet.top;

      debugPrint(
        '[FIT/$tag] surface=$surface sys=$systemBottom kb=$keyboard '
        'sheet=[${sheet.top.toStringAsFixed(1)},${sheet.bottom.toStringAsFixed(1)}] '
        'available=${available.toStringAsFixed(1)} '
        'ceiling=${ceiling.toStringAsFixed(1)} '
        'handleH=${handleH.toStringAsFixed(1)} '
        'outerVP=[${outer.top.toStringAsFixed(1)},${outer.bottom.toStringAsFixed(1)}] '
        'h=${outer.height.toStringAsFixed(1)} '
        'outerMax=${outerMax.toStringAsFixed(1)} '
        'form=[${formTop.toStringAsFixed(1)},${formBottom.toStringAsFixed(1)}] '
        'formH=${(formBottom - formTop).toStringAsFixed(1)} '
        'innerVP=[${viewport.top.toStringAsFixed(1)},${viewport.bottom.toStringAsFixed(1)}] '
        'innerMax=${innerMax.toStringAsFixed(1)} '
        'bar=[${bar.top.toStringAsFixed(1)},${bar.bottom.toStringAsFixed(1)}] '
        'barH=${bar.height.toStringAsFixed(1)} '
        'cta=[${ctaTop.toStringAsFixed(1)},${ctaBottom.toStringAsFixed(1)}] '
        'ctaBelowClip=${(ctaBottom - outer.bottom).toStringAsFixed(1)}',
      );

      // (1) The form fills EXACTLY the sheet's content allocation: ceiling
      // minus handle, system spacer and the p24 content wrap — never the
      // ceiling itself (that duplicate was the 72+N px fit gap).
      final double spacer = keyboard > 0 ? 0.0 : systemBottom;
      final double allocation = ceiling - handleH - spacer - 2 * _p24();
      expect(
        formBottom - formTop,
        closeTo(allocation, 0.01),
        reason:
            'the AddressForm claims ${formBottom - formTop} px, the sheet '
            'allocates $allocation px — a body-side ceiling is back',
      );

      // (2) The content clip is the base's viewport bottom: surface minus
      // the ONE keyboard lift or the ONE system spacer.
      final double wantClip =
          surface - (keyboard > 0 ? keyboard : systemBottom);
      expect(
        outer.bottom,
        closeTo(wantClip, 0.01),
        reason: 'content clip ${outer.bottom} != $wantClip',
      );

      // (3) Zero dead band: content (form + p24 wrap) exactly fills the
      // region, so the base's outer scroll has NOTHING left to scroll —
      // the 72+N px artifact band cannot exist.
      expect(
        outerMax,
        closeTo(0, 0.01),
        reason:
            'outer extent $outerMax px — the body does not fill its '
            'allocation exactly (a second ceiling is over-claiming)',
      );
      expect(
        outer.bottom - formBottom,
        closeTo(_p24(), 0.01),
        reason:
            'the form bottom sits ${outer.bottom - formBottom} px above the '
            'clip (expected the p24 content wrap)',
      );

      // (4) At rest the CTA is fully ABOVE the content clip — reachable
      // with no gesture: p24 wrap below the bar + the bar's p12. The base
      // spacer lives BELOW the clip and the embedded bar reserves no inset
      // of its own (BOTTOMSHEET-03), so this clearance never grows with N.
      final double margin = outer.bottom - ctaBottom;
      expect(
        margin,
        closeTo(_p24() + _p12(), 0.01),
        reason:
            'CTA clearance below the clip is $margin px (expected '
            '${_p24() + _p12()}) — the CTA is clipped again or a second '
            'system-bottom reservation grew the gap',
      );

      // (5) The system-bottom gap below the CTA is spent EXACTLY ONCE:
      // the bar design padding (p12) + the content wrap (p24) + the base
      // spacer (N). A bar-side SafeArea in embedded mode would add a
      // second N here (BOTTOMSHEET-03 ratchet).
      if (keyboard == 0) {
        final double systemGap = surface - ctaBottom;
        expect(
          systemGap,
          closeTo(_p24() + _p12() + spacer, 0.01),
          reason:
              'gap below the CTA to the screen bottom is $systemGap px '
              '(expected ${_p24() + _p12() + spacer}) — a second '
              'system-bottom reservation is back',
        );
      }
    }

    for (final double inset in const <double>[0, 24, 34, 48]) {
      testWidgets('kb OFF, system $inset: ceiling claimed vs allocated', (
        tester,
      ) async {
        addTearDown(tester.view.reset);
        tester.view.physicalSize = const Size(1680, 2400);
        tester.view.devicePixelRatio = 3.0;
        await fitMeasure(
          tester,
          systemBottom: inset,
          keyboard: 0,
          tag: 'off-$inset',
        );

        // Per-inset canonical numbers: the body fills ceiling − handle −
        // spacer − wrap (648−N at surface 800) and the CTA clears the clip
        // by wrap + p12 (36 px, inset-independent) — both asserted exactly
        // in fitMeasure; nothing below the clip may survive at rest.
        expect(
          _outerScroll(tester).bottom - _ctaBottom(tester),
          greaterThan(0),
          reason: 'the CTA is below the content clip at rest',
        );
      });
    }

    for (final double inset in const <double>[0, 34, 48]) {
      testWidgets('kb 300, system $inset: ceiling claimed vs allocated', (
        tester,
      ) async {
        addTearDown(tester.view.reset);
        tester.view.physicalSize = const Size(1680, 2400);
        tester.view.devicePixelRatio = 3.0;
        await fitMeasure(
          tester,
          systemBottom: inset,
          keyboard: _keyboard,
          tag: 'on-$inset',
        );

        // Under an open keyboard the platform bottom padding is consumed
        // (the harness follows platform semantics), so the allocation is
        // system-inset-independent: outerMax 0 and the SAME clearance for
        // system 0 / 34 / 48 — keyboard inset and system inset never mix.
        // (The reported `outerMax 106 @ kb300/system34` did NOT reproduce
        // from the current filesystem: under a keyboard it was 72 flat
        // pre-fix, and is 0 now.)
        expect(
          _outerExtent(tester),
          closeTo(0, 0.01),
          reason:
              'kb-ON outer extent != 0 for system $inset — either the '
              'allocation changed or a second inset consumer mixed in',
        );
        expect(
          _outerScroll(tester).bottom - _ctaBottom(tester),
          closeTo(_p24() + _p12(), 0.01),
          reason: 'kb-ON CTA clearance below the clip changed for $inset',
        );
      });
    }

    testWidgets('the CTA is above the content clip at rest and survives inner '
        'scrolling; the base outer scroll has no band left to reveal', (
      tester,
    ) async {
      addTearDown(tester.view.reset);
      tester.view.physicalSize = const Size(1680, 2400);
      tester.view.devicePixelRatio = 3.0;
      const double inset = 34;
      _setWindowInsets(tester, systemBottom: inset, keyboard: 0);
      await _openSheet(tester);

      final double clip = _outerScroll(tester).bottom;

      // The 72+N px artifact band is gone: content (form + wrap) fills
      // the region exactly, so there is nothing for the outer scroll to
      // scroll — no reveal gesture exists or is needed.
      expect(
        _outerExtent(tester),
        closeTo(0, 0.01),
        reason: 'the base outer scroll still carries an overflow band',
      );

      // At rest — before ANY gesture — the CTA clears the content clip.
      expect(
        _ctaBottom(tester),
        lessThan(clip),
        reason: 'the CTA is below the content clip at rest',
      );

      // Scrolling the FORM (the gesture users actually make) never moves
      // the CTA out of view and never stirs the outer scroll.
      await tester.drag(_formList(), const Offset(0, -400));
      await tester.pumpAndSettle();
      expect(
        _outerPixels(tester),
        closeTo(0, 0.01),
        reason: 'an inner-list drag moved the outer scroll',
      );
      expect(
        _ctaBottom(tester),
        lessThan(_outerScroll(tester).bottom),
        reason: 'the CTA fell below the clip while the form scrolled',
      );

      // A header-area drag has no band to consume and cannot push the
      // CTA out of view.
      await tester.drag(_headerIcon(), const Offset(0, -160));
      await tester.pumpAndSettle();
      expect(
        _outerPixels(tester),
        closeTo(0, 0.01),
        reason: 'the outer scroll left 0 px despite having no extent',
      );
      expect(
        _ctaBottom(tester),
        lessThan(_outerScroll(tester).bottom),
        reason: 'the CTA fell below the clip after a header drag',
      );

      // And the last field is still reachable inside the form.
      await _scrollFormToEnd(tester);
      final Finder lastField = _lastField();
      expect(
        lastField,
        findsOneWidget,
        reason: 'the last field was never built at form scroll end',
      );
      expect(
        tester.getBottomRight(lastField).dy,
        lessThanOrEqualTo(_viewport(tester).bottom + 0.01),
        reason: 'the last field sits below the form viewport',
      );
    });
  });

  group('BOTTOMSHEET-03 — embedded bar: the base owns the system bottom, '
      'exactly once', () {
    testWidgets(
      'the embedded bar height is inset-independent and the gap below '
      'the CTA spends exactly one N',
      (tester) async {
        addTearDown(tester.view.reset);
        tester.view.physicalSize = const Size(1680, 2400);
        tester.view.devicePixelRatio = 3.0;
        _setWindowInsets(tester, systemBottom: 0, keyboard: 0);
        await _openSheet(tester);

        final double bareBarH = _barRect(tester).height;
        final double bareGap = _surfaceBottom(tester) - _ctaBottom(tester);

        // Flip the LIVE system inset: the base spacer must follow it 1:1
        // while the bar itself may NOT grow by the inset — that growth was
        // the pre-fix double reservation (bar strip + spacer = 2N below
        // the CTA, 132 px at N=48).
        _setWindowInsets(tester, systemBottom: 48, keyboard: 0);
        await tester.pumpAndSettle();

        final double barH = _barRect(tester).height;
        final double gap = _surfaceBottom(tester) - _ctaBottom(tester);

        expect(
          barH,
          closeTo(bareBarH, 0.01),
          reason:
              'the embedded bar grew by ${barH - bareBarH} px with the '
              'system inset — it reserved an inset the base spacer owns',
        );
        expect(
          gap - bareGap,
          closeTo(48, 0.01),
          reason:
              'the gap below the CTA moved by ${gap - bareGap} px '
              '(expected exactly the 48 px base spacer) — the inset was '
              'spent twice or not at all',
        );
        expect(
          gap,
          closeTo(_p12() + _p24() + 48, 0.01),
          reason: 'the system-bottom gap below the CTA is $gap px',
        );
        expect(
          _ctaBottom(tester),
          lessThan(_outerScroll(tester).bottom),
          reason: 'the CTA fell below the content clip after the flip',
        );
      },
    );
  });

  group('BOTTOMSHEET-02 (D) — source proof', () {
    test('the address form carries no inset arithmetic', () {
      // Code only — prose may name a purged API when explaining why it is
      // gone (same rule as the bottom-sheet authority contract).
      final String source =
          File(
                'lib/domains/user/profile/presentation/widgets/'
                'address_form_dialog.dart',
              )
              .readAsStringSync()
              .replaceAll('\r\n', '\n')
              .replaceAll(RegExp(r'/\*[\s\S]*?\*/'), '')
              .replaceAll(RegExp(r'//[^\n]*'), '');

      for (final String banned in const <String>[
        'viewInsets',
        'viewPadding',
        'MediaQuery.of',
        'bottomBarClearance',
        'fabClearance',
      ]) {
        expect(
          source,
          isNot(contains(banned)),
          reason:
              '`$banned` re-introduces a body-owned inset calculation — the '
              'AppBottomSheetBase sheet owns keyboard + system bottom inset',
        );
      }

      // The CTA keeps its canonical embedded contract (no second keyboard
      // rise) and still renders through the canonical bar.
      expect(source, contains('embeddedInLiftedSheet: true'));
      expect(source, contains('BottomActionBar('));
    });
  });
}
