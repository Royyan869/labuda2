import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

// ============================================================================
// APP BAR — CHROME AUTHORITY CONTRACT (AppBar consistency cleanup).
//
// The app shipped three chrome dialects: ~59 flat surface bars, 2 solid red
// bars (seller earnings / shipping), and 1 red-gradient SliverAppBar whose
// title GREW on scroll (seller dashboard — explicitly rejected by the owner
// as "jelek, jangan pakai"). On top of that, ~74 lines across 22 files
// restated the chrome at the call site (scrolledUnderElevation: 0,
// surfaceTintColor, elevation, background/foreground colours) — a verbatim
// copy of what appBarTheme already decided. The restatement is exactly how
// the drift happened: Flutter's M3 default (scrolled-under elevation 3,
// tinted shadow) leaked back in everywhere someone forgot the pin.
//
// THE TRUTH: AppTheme.appBarTheme owns every chrome pixel. This locks:
// 1. NO AppBar/SliverAppBar ctor in lib states its own chrome (colours,
//    tint, elevation, overlay) — with ONE documented exception: the media
//    viewer, the only screen whose bar renders over live media;
// 2. AppTheme still carries the full flat-chrome statement (nothing is
//    silently deleted from the authority);
// 3. no screen pushes its own status-bar overlay style;
// 4. the seller bars stay converged and the growing FlexibleSpaceBar header
//    stays dead;
// 5. anti-vakum floor + planted-violation probes, so the sweep cannot rot
//    into a detector that passes by never looking.
// ============================================================================

/// The ONLY file allowed to state its own bar chrome: the media viewer
/// overlays its bar on live video/photo content, so it must be transparent
/// with a contrasting foreground. The sweep asserts this set EQUALS the set
/// of files that actually violate — retiring the exception must be a
/// conscious edit of this list, never silent drift.
const _chromeException = {'lib/shared/widgets/media_viewer_widget.dart'};

/// The only screen allowed to push an overlay style of its own (immersive
/// full-screen crop UI). The STYLE itself still comes from AppTheme
/// (`AppTheme.immersiveOverlayStyle`); the screen merely applies it.
const _overlayException = {'lib/shared/widgets/flutter_crop_image.dart'};

/// Chrome keys that belong to `appBarTheme`, never to a call site.
/// `systemOverlayStyle` is included: the status bar above the app bar is
/// chrome too.
const _chromeKeys = {
  'backgroundColor',
  'foregroundColor',
  'elevation',
  'scrolledUnderElevation',
  'surfaceTintColor',
  'systemOverlayStyle',
};

/// Brand/status tokens that turn a bar into a second dialect even when no
/// key above is present — a red `flexibleSpace` gradient or a title painted
/// in the brand red would slip past a key-only sweep.
///
/// NOT scanned: the top-level `actions:` and `bottom:` subtrees
/// ([_contentKeys]). Those are screen content embedded in the bar, and the
/// owner's rule keeps red accents in content: submit spinners/labels in
/// actions, and tab bars under bottom — whose accent was converged onto
/// `tabBarTheme` and is locked by tab_bar_authority_contract_test. Chrome is
/// what the BAR paints; content is what the screen puts in it.
final _chromeTokens = [
  RegExp(r'\bstatusColors\b'),
  RegExp(r'\bcolorScheme\.primary\b'),
  RegExp(r'\bscheme\.primary\b'),
  RegExp(r'\bprimaryRed\b'),
  RegExp(r'\bprimaryGradient\b'),
  RegExp(r'\bColors\.[A-Za-z_]+\b'),
];

/// Top-level args whose subtrees hold screen content, not bar chrome.
const _contentKeys = {'actions', 'bottom'};

final _ctorRe = RegExp(r'\b(SliverAppBar|AppBar)\s*\(');
final _idRe = RegExp(r'[A-Za-z_][A-Za-z0-9_]*');

bool _isWs(int c) => c == 0x20 || c == 0x09 || c == 0x0A || c == 0x0D;

/// Skip whitespace and // comments; returns the index of the next token.
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
      i = nl + 1; // skip the comment, keep scanning trivia
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

/// i points at the opening quote (or a raw-string `r` prefix).
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

/// i points just past the opening `(` of the constructor.
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

/// Scan one argument value; returns index just past its terminating comma
/// (or the end of [s] when the argument has no trailing comma).
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

/// Top-level named args of one constructor: name -> raw value text.
/// Returns null when the shape is not the plain `name: value,` list this
/// gate can vouch for — treated as a violation, never silently passed.
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
        ? raw.substring(0, raw.length - 1).trimRight().trimLeft()
        : raw.trim();
    out[idm.group(0)!] = value;
    i = vEnd;
  }
  return out;
}

/// All chrome violations in one source file. Deliberately allowlist-blind:
/// the exception list only lives in the expectations, so a violation in an
/// "allowed" file is still counted and the allowlist can never silently grow.
List<String> findChromeViolations(String path, String src) {
  final hits = <String>[];
  for (final m in _ctorRe.allMatches(src)) {
    final lineStart = src.lastIndexOf('\n', m.start) + 1;
    final head = src.substring(lineStart, m.start);
    if (head.contains('class ')) continue; // `class Foo extends AppBar(`
    if (head.contains('//')) continue; // prose mentioning AppBar(
    final lineNo = src.substring(0, m.start).split('\n').length;
    final open = m.end - 1;
    final close = _findClose(src, open);
    if (close == -1) {
      hits.add('$path:$lineNo unbalanced AppBar ctor');
      continue;
    }
    final args = src.substring(open + 1, close);
    final map = _argMap(args);
    if (map == null) {
      hits.add('$path:$lineNo unparsable AppBar args (gate cannot vouch)');
      continue;
    }
    for (final key in map.keys) {
      if (_chromeKeys.contains(key)) {
        hits.add(
          '$path:$lineNo `$key:` restated at call site — appBarTheme owns it',
        );
      }
    }
    for (final entry in map.entries) {
      if (_contentKeys.contains(entry.key)) continue; // content, not chrome
      for (final tok in _chromeTokens) {
        for (final mm in tok.allMatches(entry.value)) {
          hits.add(
            '$path:$lineNo `${entry.key}` paints with `${mm.group(0)}` — '
            'bar ink comes from appBarTheme',
          );
        }
      }
    }
  }
  return hits;
}

Iterable<File> _libDartFiles() => Directory('lib')
    .listSync(recursive: true)
    .whereType<File>()
    .where((f) => f.path.endsWith('.dart'))
    .where((f) => !f.path.replaceAll(r'\', '/').contains('/generated/'));

String _norm(String p) => p.replaceAll(r'\', '/').replaceFirst('./', '');

void main() {
  test('every app bar defers its chrome to appBarTheme (source sweep)', () {
    final violationsByFile = <String, List<String>>{};
    var ctors = 0;
    for (final f in _libDartFiles()) {
      final path = _norm(f.path);
      final src = f.readAsStringSync();
      ctors += _ctorRe.allMatches(src).length;
      final hits = findChromeViolations(path, src);
      if (hits.isNotEmpty) violationsByFile[path] = hits;
    }

    // Anti-vakum: the audit counted 64 ctors across 51 files. If the sweep
    // ever sees almost nothing, its parser broke — it must not "pass" by
    // looking at an empty world.
    expect(
      ctors,
      greaterThanOrEqualTo(60),
      reason: 'ctor count collapsed — the sweep is no longer seeing the app',
    );

    final unexpected = violationsByFile.entries
        .where((e) => !_chromeException.contains(e.key))
        .map((e) => e.value)
        .expand((v) => v)
        .toList();
    expect(
      unexpected,
      isEmpty,
      reason:
          'chrome restated at a call site. Delete it — appBarTheme already '
          'decides surface/onSurface/flat/0:\n${unexpected.join('\n')}',
    );

    expect(
      violationsByFile.keys.toSet(),
      _chromeException,
      reason:
          'the media-viewer overlay is the ONE documented exception. If it '
          'converged too, shrink _chromeException consciously; if a new file '
          'appeared here, justify it or strip its chrome.',
    );
  });

  test('appTheme still states the whole flat-chrome contract', () {
    final themeSrc = File(
      'lib/core/src/theme/app_theme.dart',
    ).readAsStringSync();
    final m = RegExp(
      r'appBarTheme:\s*AppBarTheme\s*\(',
    ).firstMatch(themeSrc);
    expect(m, isNotNull, reason: 'appBarTheme is THE chrome authority');
    final close = _findClose(themeSrc, m!.end - 1);
    expect(close, greaterThan(0), reason: 'appBarTheme ctor must balance');
    final args = themeSrc.substring(m.end, close);

    final map = _argMap(args);
    expect(map, isNotNull, reason: 'appBarTheme args must be plain named list');
    expect(
      map!.keys.toSet(),
      containsAll(const [
        'backgroundColor',
        'foregroundColor',
        'elevation',
        'scrolledUnderElevation',
        'surfaceTintColor',
        'centerTitle',
        'systemOverlayStyle',
      ]),
      reason: 'the authority must keep the FULL statement',
    );

    expect(map['backgroundColor'], 'scheme.surface');
    expect(map['foregroundColor'], 'scheme.onSurface');
    expect(map['elevation'], 'AppElevation.none');
    expect(
      map['scrolledUnderElevation'],
      '0',
      reason: 'flat chrome: no shadow appears when content scrolls under',
    );
    expect(map['surfaceTintColor'], 'Colors.transparent');
    expect(map['centerTitle'], 'true');
    expect(map['systemOverlayStyle'], '_overlayStyle(scheme)');
  });

  test('no screen pushes its own status-bar overlay style', () {
    final offenders = <String>[];
    for (final f in _libDartFiles()) {
      final path = _norm(f.path);
      if (_overlayException.contains(path)) continue;
      final src = f.readAsStringSync();
      if (src.contains('SystemChrome.setSystemUIOverlayStyle') ||
          src.contains('AnnotatedRegion<SystemUiOverlayStyle>')) {
        offenders.add(path);
      }
    }
    expect(
      offenders,
      isEmpty,
      reason:
          'status-bar overlay is chrome: it is built by AppTheme._overlayStyle '
          'and applied nowhere but the immersive crop editor:\n$offenders',
    );
  });

  test('the seller bars converged; the growing dashboard header stays dead', () {
    const dir = 'lib/domains/user/preference/seller/presentation/screens';
    const earnings = '$dir/seller_earnings_screen.dart';
    const shipping = '$dir/seller_shipping_screen.dart';
    const dashboard = '$dir/seller_dashboard_screen.dart';

    for (final path in [earnings, shipping]) {
      expect(
        findChromeViolations(path, File(path).readAsStringSync()),
        isEmpty,
        reason:
            '$path once painted its bar solid brand red — the bar is surface '
            'now; red stays in the FAB/money/content',
      );
    }

    final dash = File(dashboard).readAsStringSync();
    // Code-shaped match: a prose comment may name the pattern (the purge
    // note above the bar does); only an actual instantiation is a relapse.
    expect(
      RegExp(r'FlexibleSpaceBar\s*\(').hasMatch(dash),
      isFalse,
      reason:
          'owner rejected the collapsing/growing header ("teksnya membesar, '
          'jelek, jangan pakai") — it must not come back',
    );
    expect(
      RegExp(r'expandedHeight\s*:').hasMatch(dash),
      isFalse,
      reason: 'no collapsing header height on the seller dashboard',
    );
    expect(
      findChromeViolations(dashboard, dash),
      isEmpty,
      reason: 'seller dashboard chrome belongs to appBarTheme',
    );
  });

  test('the sweep has teeth: planted violations are caught, clean bars pass', () {
    // Planted red/growing bar — the exact dialect that was purged.
    final bad = findChromeViolations(
      'lib/planted_red_bar.dart',
      '''
AppBar(
  title: const Text('Merah'),
  backgroundColor: Colors.red,
  foregroundColor: scheme.onPrimary,
  elevation: 4,
  scrolledUnderElevation: 3,
  surfaceTintColor: scheme.primary,
)''',
    );
    for (final key in _chromeKeys) {
      if (key == 'systemOverlayStyle') continue;
      expect(
        bad.any((h) => h.contains('`$key:')),
        isTrue,
        reason: 'planted `$key:` must be flagged, got: $bad',
      );
    }
    expect(
      bad.any((h) => h.contains('Colors.red')),
      isTrue,
      reason: 'planted brand/status token must be flagged, got: $bad',
    );

    // Unparsable shape must be reported, never waved through.
    final spread = findChromeViolations(
      'lib/planted_spread.dart',
      'AppBar(..._delegate, title: const Text("x"))',
    );
    expect(
      spread,
      isNotEmpty,
      reason: 'a shape the gate cannot vouch for must fail loudly',
    );

    // And a plain bar — the post-cleanup reality — must pass, or the gate
    // is just noise nobody can satisfy.
    final clean = findChromeViolations(
      'lib/planted_clean.dart',
      '''
SliverAppBar(
  pinned: true,
  title: const Text('Bersih'),
  actions: [IconButton(onPressed: _noop, icon: const Icon(Icons.close))],
  bottom: TabBar(tabs: [Tab(text: 'A'), Tab(text: 'B')]),
)''',
    );
    expect(clean, isEmpty, reason: 'a clean bar must pass: $clean');
  });
}
