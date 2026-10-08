// BOUNDED TAB-CELL VERTICAL COMPOSITION — CANONICAL AUTHORITY CONTRACT.
//
// The Profile Reviews device symptom ("BOTTOM OVERFLOWED BY 30 PIXELS") is not
// a screen bug: it is the visible edge of a missing composition rule. This
// suite locks the rule that was independently proven from Flutter framework
// source (`packages/flutter/lib/src/rendering/sliver_fill.dart`):
//
//   hasScrollBody: true  → RenderSliverFillRemainingWithScrollable
//       extent = remainingPaintExtent
//       child.layout(minExtent = extent, maxExtent = extent)   // TIGHT clamp
//       geometry.scrollExtent = viewportMainAxisExtent         // fixed viewport
//
//   hasScrollBody: false → RenderSliverFillRemaining
//       extent = max(viewportMainAxisExtent - precedingScrollExtent,
//                    child.getMaxIntrinsicHeight(crossAxisExtent))
//       child.layout(minExtent = extent, maxExtent = extent)   // defer to intrinsic
//       geometry.scrollExtent = extent                          // grows, outer scrolls
//
// THEREFORE, inside a bounded tab cell whose body is ONE CustomScrollView /
// ListView, a NON-SCROLLABLE state (EmptyState, LoadingIndicator,
// PageErrorState, Center>Column, …) MUST use `hasScrollBody: false`. Under the
// default `true` it is force-clamped to the leftover viewport and overflows
// whenever its intrinsic height is taller. `true` is legitimate ONLY when the
// child itself owns a scroll position — the framework's own NestedScrollView
// body wrapper is the reference case.
//
// The contract is documented at:
//   lib/shared/widgets/docs/bounded_tab_cell_vertical_composition.md
//
// Three halves, deliberately (matching the SAFE-AREA / TAB-BAR authority suites):
//   POSITIVE — widget tests that prove the framework distinction in rendered
//              geometry (no device, no fragile crash assertion);
//   STRUCTURAL — a source sweep that fails the moment a non-scrollable state
//              is spelled with implicit/default `hasScrollBody: true`, plus a
//              per-file lock on every canonical non-scrollable consumer;
//   TEETH — planted-violation probes so the detector cannot pass by not looking,
//           and the legitimate scrollable-child exception stays legal.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:labuda/shared/widgets/empty_state.dart';

/// The canonical non-scrollable semantic-state renderers (and the generic
/// centering/column boxes used in their place). A `SliverFillRemaining` whose
/// child matches one of these must state `hasScrollBody: false`.
const List<String> _nonScrollableMarkers = <String>[
  'EmptyState(',
  'LoadingIndicator',
  'PageErrorState(',
  'CircularProgressIndicator(',
  'ReviewsEmptyState(',
  'Center(',
  'Column(',
];

/// A child that owns its own scroll position — the ONLY legitimate use of
/// `hasScrollBody: true`.
const List<String> _scrollableMarkers = <String>[
  'CustomScrollView(',
  'ListView',
  'GridView',
  'SingleChildScrollView(',
  'TabBarView(',
  'PageView(',
  'NestedScrollView(',
  'Scrollable(',
];

/// The canonical contract document this suite enforces.
const String _contractDocPath =
    'lib/shared/widgets/docs/bounded_tab_cell_vertical_composition.md';

/// Files whose every `SliverFillRemaining` wraps a non-scrollable state, so
/// every one of their call sites must state `hasScrollBody: false` explicitly.
/// The profile pair are the two consumers that regressed to the default; they
/// are locked here so the regression cannot silently return.
const List<String> _canonicalNonScrollableStateFiles = <String>[
  'lib/domains/commerce/catalog/shared/presentation/widgets/'
      'commerce_marketplace_primitives.dart',
  'lib/domains/commerce/transaction/order/presentation/screens/'
      'order_list_screen.dart',
  'lib/domains/user/profile/presentation/screens/address_list_screen.dart',
  'lib/domains/commerce/catalog/for_sale/presentation/screens/'
      'my_for_sales_screen.dart',
  'lib/domains/commerce/catalog/auction/presentation/screens/'
      'seller_auctions_screen.dart',
  'lib/domains/system/support/presentation/screens/'
      'support_tickets_list_screen.dart',
  'lib/domains/system/report/presentation/screens/my_reports_screen.dart',
  'lib/domains/social/follow/presentation/screens/follow_list_screen.dart',
  'lib/domains/user/profile/presentation/widgets/profile_feed_tab.dart',
  'lib/domains/user/profile/presentation/widgets/profile_reviews_tab.dart',
];

// ============================================================================
// Source parser — balanced-paren top-level argument extraction
// (the same shape the TAB-BAR authority gate uses, so a nested child argument
// is read whole and never confused with the enclosing ctor).
// ============================================================================

final RegExp _sliverRe = RegExp(r'\bSliverFillRemaining\s*\(');
final RegExp _idRe = RegExp(r'[A-Za-z_][A-Za-z0-9_]*');

bool _isWs(int c) => c == 0x20 || c == 0x09 || c == 0x0A || c == 0x0D;

int _skipTrivia(String s, int i) {
  while (i < s.length) {
    final c = s.codeUnitAt(i);
    if (_isWs(c)) {
      i++;
      continue;
    }
    if (s.startsWith('//', i)) {
      final nl = s.indexOf('\n', i);
      if (nl == -1) return s.length;
      i = nl + 1;
      continue;
    }
    break;
  }
  return i;
}

bool _atString(String s, int i) =>
    s[i] == "'" ||
    s[i] == '"' ||
    (s[i] == 'r' && i + 1 < s.length && (s[i + 1] == "'" || s[i + 1] == '"'));

int _skipString(String s, int i) {
  final raw = s[i] == 'r';
  if (raw) i++;
  final quote = s[i];
  i++;
  while (i < s.length) {
    if (!raw && s[i] == r'\') {
      i += 2;
      continue;
    }
    if (s[i] == quote) return i + 1;
    i++;
  }
  return i;
}

/// i points at the opening `(`.
int _findClose(String s, int open) {
  var i = open + 1;
  var depth = 1;
  while (i < s.length) {
    if (_atString(s, i)) {
      i = _skipString(s, i);
      continue;
    }
    if (s.startsWith('//', i)) {
      final nl = s.indexOf('\n', i);
      if (nl == -1) return -1;
      i = nl;
      continue;
    }
    if (s[i] == '(') {
      depth++;
    } else if (s[i] == ')') {
      depth--;
      if (depth == 0) return i;
    }
    i++;
  }
  return -1;
}

/// Scan one argument value; returns the index just past its terminating comma.
int _skipValue(String s, int i) {
  var depth = 0;
  while (i < s.length) {
    if (_atString(s, i)) {
      i = _skipString(s, i);
      continue;
    }
    if (s.startsWith('//', i)) {
      final nl = s.indexOf('\n', i);
      if (nl == -1) return s.length;
      i = nl;
      continue;
    }
    final c = s[i];
    if (c == '(' || c == '[' || c == '{') {
      depth++;
      i++;
      continue;
    }
    if (c == ')' || c == ']' || c == '}') {
      if (depth == 0) return i;
      depth--;
      i++;
      continue;
    }
    if (c == ',' && depth == 0) return i + 1;
    i++;
  }
  return s.length;
}

/// Top-level named args of one constructor: name -> raw value text. Returns
/// null for a shape this gate cannot vouch for (treated as a violation, never
/// waved through).
Map<String, String>? _argMap(String s) {
  final out = <String, String>{};
  var i = 0;
  while (i < s.length) {
    i = _skipTrivia(s, i);
    if (i >= s.length) break;
    final idm = _idRe.matchAsPrefix(s, i);
    if (idm == null) return null;
    final k = _skipTrivia(s, idm.end);
    if (k >= s.length || s[k] != ':') return null;
    final vStart = k + 1;
    final vEnd = _skipValue(s, vStart);
    final raw = s.substring(vStart, vEnd);
    final value = raw.trimRight().endsWith(',')
        ? raw.substring(0, raw.length - 1).trim()
        : raw.trim();
    out[idm.group(0)!] = value;
    i = vEnd;
  }
  return out;
}

/// Every `SliverFillRemaining(` call site in [src] with its parsed args.
List<({int line, Map<String, String>? args})> _sliverSites(String src) {
  final out = <({int line, Map<String, String>? args})>[];
  for (final m in _sliverRe.allMatches(src)) {
    final lineStart = src.lastIndexOf('\n', m.start) + 1;
    final head = src.substring(lineStart, m.start);
    if (head.contains('//')) continue; // prose mentioning the ctor
    if (head.contains('class ')) continue;
    final line = src.substring(0, m.start).split('\n').length;
    final close = _findClose(src, m.end - 1);
    if (close == -1) {
      out.add((line: line, args: null));
      continue;
    }
    out.add((line: line, args: _argMap(src.substring(m.end, close))));
  }
  return out;
}

/// A violation is a `SliverFillRemaining` whose child is a recognised
/// non-scrollable state and which does NOT state `hasScrollBody: false`
/// (either implicit default `true`, or an explicit `true`). A child that is
/// itself scrollable is a legitimate exception; anything unrecognised is
/// ambiguous and left alone.
List<String> _violations(String path, String src) {
  final out = <String>[];
  for (final site in _sliverSites(src)) {
    final args = site.args;
    if (args == null) {
      out.add('$path:${site.line} unparsable SliverFillRemaining args');
      continue;
    }
    if (args['hasScrollBody'] == 'false') continue;
    final child = args['child'] ?? '';
    if (_scrollableMarkers.any(child.contains)) continue;
    if (_nonScrollableMarkers.any(child.contains)) {
      out.add(
        '$path:${site.line} non-scrollable state with '
        'hasScrollBody=${args['hasScrollBody'] ?? 'implicit true (default)'}',
      );
    }
  }
  return out;
}

Iterable<File> _libDartFiles() => Directory('lib')
    .listSync(recursive: true)
    .whereType<File>()
    .where((f) => f.path.endsWith('.dart'))
    .where((f) => !f.path.replaceAll(r'\', '/').contains('/generated/'));

String _norm(String p) => p.replaceAll(r'\', '/').replaceFirst('./', '');

// ============================================================================
// Canvas helpers
// ============================================================================

Widget _host(Widget child) =>
    MaterialApp(theme: AppTheme.lightTheme, home: Scaffold(body: child));

void _setViewport(WidgetTester tester, Size logical) {
  tester.view.physicalSize = logical;
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
}

void main() {
  // ==========================================================================
  // POSITIVE PROOF — the framework distinction, in rendered geometry
  // ==========================================================================
  group('BOUNDED-TAB-CELL — the primitive defers to intrinsic only when false',
      () {
    testWidgets(
      'hasScrollBody:false grows to the intrinsic height; true clamps to the '
      'viewport',
      (tester) async {
        _setViewport(tester, const Size(400, 200));

        Future<double> scrollExtentFor({required bool hasScrollBody}) async {
          await tester.pumpWidget(
            _host(
              CustomScrollView(
                slivers: <Widget>[
                  SliverFillRemaining(
                    hasScrollBody: hasScrollBody,
                    child: const SizedBox(
                      key: ValueKey<String>('probe'),
                      height: 300,
                      width: double.infinity,
                    ),
                  ),
                ],
              ),
            ),
          );
          final sliver = tester.renderObject<RenderSliver>(
            find.byType(SliverFillRemaining),
          );
          return sliver.geometry!.scrollExtent;
        }

        final clamped = await scrollExtentFor(hasScrollBody: true);
        final deferred = await scrollExtentFor(hasScrollBody: false);

        expect(
          clamped,
          closeTo(200, 0.01),
          reason:
              'hasScrollBody:true must clamp its non-scrollable child to the '
              'remaining viewport (this is the Profile Reviews overflow path)',
        );
        expect(
          deferred,
          closeTo(300, 0.01),
          reason:
              'hasScrollBody:false must defer to the intrinsic height and grow '
              'the scroll extent — the canonical rule',
        );
        expect(tester.takeException(), isNull);
      },
    );

    testWidgets(
      'a tall-chrome short viewport overflows nothing when the empty state is '
      'hasScrollBody:false',
      (tester) async {
        // 300 px viewport, 260 px of fixed chrome above the state — the exact
        // shape (header + tabs + summary + filter) that produced the 30 px
        // device overflow when the empty state was clamped to the leftover 40.
        _setViewport(tester, const Size(400, 300));

        await tester.pumpWidget(
          _host(
            CustomScrollView(
              slivers: <Widget>[
                const SliverToBoxAdapter(
                  child: SizedBox(height: 260, child: Text('chrome')),
                ),
                SliverFillRemaining(
                  hasScrollBody: false,
                  child: const EmptyState(title: 'Belum ada ulasan'),
                ),
              ],
            ),
          ),
        );

        expect(
          tester.takeException(),
          isNull,
          reason: 'the empty state overflowed its bounded tab cell',
        );
        // The state kept its intrinsic height instead of being squeezed into
        // the 40 px leftover.
        expect(
          tester.getSize(find.byType(EmptyState)).height,
          greaterThan(40),
          reason:
              'the empty state was clamped to the leftover viewport instead '
              'of deferring to its intrinsic height',
        );
      },
    );
  });

  // ==========================================================================
  // STRUCTURAL PROOF — no non-scrollable state may use implicit/default true
  // ==========================================================================
  group('BOUNDED-TAB-CELL — source contract', () {
    test('every canonical non-scrollable consumer states hasScrollBody:false',
        () {
      final offenders = <String>[];
      var total = 0;
      for (final path in _canonicalNonScrollableStateFiles) {
        final file = File(path);
        expect(file.existsSync(), isTrue, reason: '$path does not exist');
        for (final site in _sliverSites(file.readAsStringSync())) {
          total++;
          final hsb = site.args?['hasScrollBody'];
          if (hsb != 'false') {
            offenders.add(
              '$path:${site.line} hasScrollBody=${hsb ?? 'implicit true'} — '
              'a non-scrollable state must state false',
            );
          }
        }
      }

      // Anti-vacuum: 10 canonical files carry 30 non-scrollable state slivers
      // (8 clients x 3 + the profile pair x 3). A collapse means the parser
      // stopped seeing the app and the sweep would pass by looking at nothing.
      expect(
        total,
        greaterThanOrEqualTo(30),
        reason: 'canonical SliverFillRemaining count collapsed ($total)',
      );
      expect(offenders, isEmpty, reason: offenders.join('\n'));
    });

    test('no non-scrollable state anywhere in lib uses default/true', () {
      final offenders = <String>[];
      for (final f in _libDartFiles()) {
        offenders.addAll(_violations(_norm(f.path), f.readAsStringSync()));
      }
      expect(
        offenders,
        isEmpty,
        reason:
            'a non-scrollable state is clamped by the default hasScrollBody:true '
            'and will overflow on a short viewport:\n${offenders.join('\n')}',
      );
    });

    test('the framework-proven contract is documented and pinned', () {
      final doc = File(_contractDocPath);
      expect(
        doc.existsSync(),
        isTrue,
        reason: 'the canonical contract document is missing: $_contractDocPath',
      );
      final src = doc.readAsStringSync();
      for (final needle in const <String>[
        'hasScrollBody: false',
        'hasScrollBody: true',
        'getMaxIntrinsicHeight',
        'viewportMainAxisExtent',
        'NestedScrollView',
        'Rule 1',
        'Rule 6',
      ]) {
        expect(src, contains(needle), reason: 'contract lost `$needle`');
      }
    });

    test(
      'the detector has teeth: planted violations fail, legitimate scrollable '
      'exceptions pass',
      () {
        // Implicit default true wrapping a non-scrollable state → violation.
        expect(
          _violations(
            'lib/planted_default.dart',
            'SliverFillRemaining(child: EmptyState(title: "x"))',
          ),
          isNotEmpty,
        );
        // Explicit true wrapping a non-scrollable state → violation.
        expect(
          _violations(
            'lib/planted_true.dart',
            'SliverFillRemaining(hasScrollBody: true, '
            'child: Center(child: Text("x")))',
          ),
          isNotEmpty,
        );
        // The historical Profile Reviews shape → violation.
        expect(
          _violations(
            'lib/planted_reviews.dart',
            'SliverFillRemaining(child: ReviewsEmptyState())',
          ),
          isNotEmpty,
        );
        // The canonical rule on a non-scrollable state → clean.
        expect(
          _violations(
            'lib/planted_clean.dart',
            'SliverFillRemaining(hasScrollBody: false, '
            'child: EmptyState(title: "x"))',
          ),
          isEmpty,
        );
        // Legitimate exception: a genuinely scrollable child under true.
        expect(
          _violations(
            'lib/planted_scrollable.dart',
            'SliverFillRemaining(hasScrollBody: true, '
            'child: ListView(children: []))',
          ),
          isEmpty,
        );
      },
    );
  });
}
