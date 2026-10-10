// BOTTOMSHEET-04 — FIXED / NON-SCROLLING CONTENT FIT (geometry proof).
//
// The two canonical body behaviors of the `AppBottomSheetBase` sheet,
// measured on the ACTUALLY RENDERED sheet under injected window metrics:
//
//   N — NATURAL-FIT body: content-sized `Column(mainAxisSize: min)` (the
//       shape of every form/action/selection sheet). The base's outer scroll
//       is the silent overflow carrier: extent 0 at rest, positive only when
//       content genuinely exceeds the content region — nothing else sizes,
//       clips or clears anything.
//   F — FINITE-SLOT body: an interactive inner viewport (ListView.builder
//       results / TabBarView) cannot exist in the base's unbounded scroll,
//       so the body asks the SHEET AUTHORITY for a bounded share — the Tag
//       People sheet renders `contentAllocationOf × 0.75` (measured here on
//       the REAL `UserSearchBottomSheet`). The share of the live content
//       ALLOCATION (sheet chrome and system spacer already spent) keeps the
//       slot strictly inside the content region at every inset; the outer
//       scroll still carries nothing, and the sheet adapts live to keyboard
//       and system insets through the base.
//
// Locked facts (this file is the ratchet): the slot is EXACTLY the
// allocation share (never a ceiling fraction, never a raw-screen fraction,
// never a body-side inset computation), the outer scroll extent is 0 in
// both behaviors at rest, the content clip is the ONE base lift/spacer, and
// nothing clips.
//
// View: 560×800 logical (1680×2400 @ dpr 3); `systemTop: 24` injected;
// `padding` follows platform semantics (an open keyboard consumes
// `viewPadding`, exactly like Android).
import 'dart:math' as math;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/api/api_client.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';
import 'package:hishumi/features/search/search/search.dart';
import 'package:hishumi/shared/widgets/app_bottom_sheet_base.dart';
import 'package:hishumi/shared/widgets/user_search_bottom_sheet.dart';

const double _systemTop = 24;
const double _keyboard = 300;

/// The Tag People sheet, opened exactly as production opens it.
Future<List<String>?> _openTagPeople(BuildContext context) =>
    UserSearchBottomSheet.show(context: context);

/// Window metrics on the TEST VIEW (physical pixels, like a device).
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

/// Hermetic host: the tag-people sheet resolves its search service from a
/// locally-constructed value — no HTTP, no session.
Widget _host(void Function(BuildContext context) openSheet) => ProviderScope(
  overrides: [
    searchApiServiceProvider.overrideWithValue(SearchApiService(ApiClient())),
  ],
  child: MaterialApp(
    theme: AppTheme.lightTheme,
    home: Scaffold(
      body: Center(
        child: Builder(
          builder: (BuildContext context) => ElevatedButton(
            onPressed: () => openSheet(context),
            child: const Text('open'),
          ),
        ),
      ),
    ),
  ),
);

Future<void> _open(
  WidgetTester tester,
  void Function(BuildContext context) sheet,
) async {
  await tester.pumpWidget(_host(sheet));
  await tester.pumpAndSettle();
  await tester.tap(find.text('open'));
  await tester.pumpAndSettle();
  expect(find.byType(BottomSheet), findsOneWidget);
}

double _surfaceBottom(WidgetTester tester) =>
    tester.getRect(find.byType(Scaffold)).bottom;

/// The BASE's outer content scroll — its viewport bottom IS the content
/// clip boundary: surface − the ONE keyboard lift or the ONE system spacer.
Rect _outerScroll(WidgetTester tester) => tester.getRect(
  find
      .descendant(
        of: find.byType(BottomSheet),
        matching: find.byType(SingleChildScrollView),
      )
      .first,
);

double _outerExtent(WidgetTester tester) =>
    _outerPosition(tester).maxScrollExtent;

double _outerPixels(WidgetTester tester) => _outerPosition(tester).pixels;

ScrollPosition _outerPosition(WidgetTester tester) {
  final Finder outer = find.descendant(
    of: find.byType(BottomSheet),
    matching: find.byType(SingleChildScrollView),
  );
  return tester
      .state<ScrollableState>(
        find.descendant(of: outer, matching: find.byType(Scrollable)).first,
      )
      .position;
}

void main() {
  group(
    'BOTTOMSHEET-04 (N) — natural-fit body: content-sized, outer scroll silent',
    () {
      Future<void> measureNatural(
        WidgetTester tester, {
        required double systemBottom,
        required double keyboard,
        required String tag,
      }) async {
        _setWindowInsets(
          tester,
          systemBottom: systemBottom,
          keyboard: keyboard,
        );
        await _open(
          tester,
          (context) => AppBottomSheetBase.show<void>(
            context: context,
            content: const Column(
              mainAxisSize: MainAxisSize.min,
              children: [SizedBox(height: 120), Text('natural body')],
            ),
          ),
        );

        final double surface = _surfaceBottom(tester);
        final double clip = _outerScroll(tester).bottom;
        final double wantClip =
            surface - (keyboard > 0 ? keyboard : systemBottom);
        debugPrint(
          '[FIT04/$tag] surface=$surface sys=$systemBottom kb=$keyboard '
          'clip=$clip outerMax=${_outerExtent(tester)}',
        );

        // The ONE base lift/spacer positions the content clip.
        expect(clip, closeTo(wantClip, 0.01), reason: 'content clip moved');
        // At rest the outer scroll carries NOTHING: the sheet is exactly as
        // tall as its content needs, never more.
        expect(
          _outerExtent(tester),
          closeTo(0, 0.01),
          reason:
              'a natural-fit body left the outer scroll something to scroll',
        );
        // The sheet bottom reaches the screen bottom (the spacer lives BELOW
        // the clip and belongs to the base alone).
        expect(
          tester.getRect(find.byType(BottomSheet)).bottom,
          closeTo(surface, 0.01),
        );
      }

      for (final double inset in const <double>[0, 24, 34, 48]) {
        testWidgets('kb OFF, system $inset', (tester) async {
          addTearDown(tester.view.reset);
          tester.view.physicalSize = const Size(1680, 2400);
          tester.view.devicePixelRatio = 3.0;
          await measureNatural(
            tester,
            systemBottom: inset,
            keyboard: 0,
            tag: 'n-off-$inset',
          );
        });
      }

      for (final double inset in const <double>[0, 34, 48]) {
        testWidgets('kb 300, system $inset', (tester) async {
          addTearDown(tester.view.reset);
          tester.view.physicalSize = const Size(1680, 2400);
          tester.view.devicePixelRatio = 3.0;
          await measureNatural(
            tester,
            systemBottom: inset,
            keyboard: _keyboard,
            tag: 'n-on-$inset',
          );
        });
      }

      testWidgets('long content: the outer scroll is the overflow carrier', (
        tester,
      ) async {
        addTearDown(tester.view.reset);
        tester.view.physicalSize = const Size(1680, 2400);
        tester.view.devicePixelRatio = 3.0;
        _setWindowInsets(tester, systemBottom: 0, keyboard: 0);
        await _open(
          tester,
          (context) => AppBottomSheetBase.show<void>(
            context: context,
            content: Column(
              mainAxisSize: MainAxisSize.min,
              children: List<Widget>.generate(
                60,
                (int i) => const SizedBox(height: 40, child: Text('row')),
              ),
            ),
          ),
        );

        // 60×40 px rows overflow the region → the base scroll shows it.
        expect(_outerExtent(tester), greaterThan(0));
        await tester.drag(
          find
              .descendant(
                of: find.byType(BottomSheet),
                matching: find.byType(SingleChildScrollView),
              )
              .first,
          const Offset(0, -2000),
        );
        await tester.pumpAndSettle();
        expect(
          _outerPixels(tester),
          greaterThan(0),
          reason: 'the outer scroll did not carry the overflow',
        );
      });
    },
  );

  group(
    'BOTTOMSHEET-04 (F) — finite-slot body: the authority share, exactly',
    () {
      Future<void> measureTagPeople(
        WidgetTester tester, {
        required double systemBottom,
        required double keyboard,
        required String tag,
      }) async {
        _setWindowInsets(
          tester,
          systemBottom: systemBottom,
          keyboard: keyboard,
        );
        await _open(tester, _openTagPeople);

        final double surface = _surfaceBottom(tester);
        final Rect outer = _outerScroll(tester);
        final Rect slot = tester.getRect(
          find.descendant(
            of: find.byType(BottomSheet),
            matching: find.byType(UserSearchBottomSheet),
          ),
        );
        // The sheet authority's ceiling INSIDE the modal route:
        // the route has already absorbed the system TOP padding, so what
        // the base (and the body) sees is window − keyboard.
        final double available = 800.0 - keyboard;
        final double ceiling = available * 0.9;
        final Rect sheet = tester.getRect(find.byType(BottomSheet));
        final double headerSpend = outer.top - sheet.top;
        final double spacer = keyboard > 0 ? 0.0 : systemBottom;
        final double allocation = ceiling - headerSpend - spacer;
        final double deadBand = outer.bottom - slot.bottom;

        debugPrint(
          '[FIT04/$tag] surface=$surface sys=$systemBottom kb=$keyboard '
          'available=$available '
          'sheet=[${sheet.top.toStringAsFixed(1)},'
          '${sheet.bottom.toStringAsFixed(1)}] h=${sheet.height} '
          'outer=[${outer.top.toStringAsFixed(1)},'
          '${outer.bottom.toStringAsFixed(1)}] h=${outer.height} '
          'headerSpend=${(outer.top - sheet.top).toStringAsFixed(1)} '
          'outerMax=${_outerExtent(tester)} '
          'slot=[${slot.top.toStringAsFixed(1)},'
          '${slot.bottom.toStringAsFixed(1)}] h=${slot.height} '
          'deadBand=$deadBand',
        );

        // (1) The slot is EXACTLY the canonical share of the live content
        // ALLOCATION — sheet chrome and system spacer already spent, so it
        // fits the region at every inset. (A ceiling-denominated share
        // overflowed by `78 + N − 0.15×available` px — BOTTOMSHEET-04.)
        expect(
          slot.height,
          closeTo(allocation * 0.75, 0.01),
          reason:
              'the tag-people slot is ${slot.height}px, not the '
              '${allocation * 0.75}px allocation share — the body sized '
              'itself from something other than the sheet authority',
        );

        // (2) The slot is allocation-safe at every inset: strictly inside
        // the content region, so the base outer scroll has nothing to do.
        expect(
          _outerExtent(tester),
          closeTo(0, 0.01),
          reason: 'the finite slot outgrew the content region',
        );
        expect(slot.top, greaterThanOrEqualTo(outer.top - 0.01));
        expect(slot.bottom, lessThanOrEqualTo(outer.bottom + 0.01));

        // (3) The clip is the ONE base lift/spacer; the sheet reaches the
        // screen bottom.
        final double wantClip =
            surface - (keyboard > 0 ? keyboard : systemBottom);
        expect(outer.bottom, closeTo(wantClip, 0.01));
        expect(
          tester.getRect(find.byType(BottomSheet)).bottom,
          closeTo(surface, 0.01),
        );
      }

      for (final double inset in const <double>[0, 24, 34, 48]) {
        testWidgets('kb OFF, system $inset', (tester) async {
          addTearDown(tester.view.reset);
          tester.view.physicalSize = const Size(1680, 2400);
          tester.view.devicePixelRatio = 3.0;
          await measureTagPeople(
            tester,
            systemBottom: inset,
            keyboard: 0,
            tag: 'f-off-$inset',
          );
        });
      }

      for (final double inset in const <double>[0, 34, 48]) {
        testWidgets('kb 300, system $inset', (tester) async {
          addTearDown(tester.view.reset);
          tester.view.physicalSize = const Size(1680, 2400);
          tester.view.devicePixelRatio = 3.0;
          await measureTagPeople(
            tester,
            systemBottom: inset,
            keyboard: _keyboard,
            tag: 'f-on-$inset',
          );
        });
      }

      testWidgets('the slot follows the live inset both directions', (
        tester,
      ) async {
        addTearDown(tester.view.reset);
        tester.view.physicalSize = const Size(1680, 2400);
        tester.view.devicePixelRatio = 3.0;
        _setWindowInsets(tester, systemBottom: 0, keyboard: 0);
        await _open(tester, _openTagPeople);

        final Rect bare = tester.getRect(find.byType(UserSearchBottomSheet));

        // System inset appears → the ONE base spacer grows, the live
        // allocation shrinks by exactly that spacer, and the share follows:
        // the slot gives up 0.75 × N.
        _setWindowInsets(tester, systemBottom: 48, keyboard: 0);
        await tester.pumpAndSettle();
        final Rect inset = tester.getRect(find.byType(UserSearchBottomSheet));
        final Finder slotBoxes = find.descendant(
          of: find.byType(UserSearchBottomSheet),
          matching: find.byType(SizedBox),
        );
        final double? builtSlot = slotBoxes.evaluate().isEmpty
            ? null
            : (slotBoxes.evaluate().first.widget as SizedBox).height;
        debugPrint(
          '[FIT04/live-flip] bare=${bare.height} inset=${inset.height} '
          'builtSlot=$builtSlot '
          'outer=${_outerScroll(tester)} '
          'sheet=${tester.getRect(find.byType(BottomSheet))} '
          'outerMax=${_outerExtent(tester)}',
        );
        expect(
          bare.height - inset.height,
          closeTo(48 * 0.75, 0.01),
          reason:
              'the slot did not follow the live system inset via the '
              'allocation',
        );

        // …but the content clip moved with the ONE base spacer.
        expect(
          _outerScroll(tester).bottom,
          closeTo(800.0 - 48, 0.01),
          reason: 'the base spacer did not follow the live system inset',
        );

        // Keyboard opens → the slot follows the authority live.
        _setWindowInsets(tester, systemBottom: 48, keyboard: _keyboard);
        await tester.pumpAndSettle();
        final Rect lifted = tester.getRect(find.byType(UserSearchBottomSheet));
        final Rect liftedOuter = _outerScroll(tester);
        final Rect liftedSheet = tester.getRect(find.byType(BottomSheet));
        final double liftedAllocation =
            (800.0 - _keyboard) * 0.9 - (liftedOuter.top - liftedSheet.top);
        expect(
          lifted.height,
          closeTo(liftedAllocation * 0.75, 0.01),
          reason: 'the slot did not shrink with the keyboard via the authority',
        );
        expect(
          _outerScroll(tester).bottom,
          closeTo(800.0 - _keyboard, 0.01),
          reason: 'the base keyboard lift moved',
        );
      });
    },
  );
}
