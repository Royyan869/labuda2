import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:hishumi/core/core.dart';

// ============================================================================
// TAB BAR — ACCENT AUTHORITY CONTRACT (TabBar convergence).
//
// The app shipped ONE TabBar accent dialect spelled NINE times: verbatim
// `indicatorColor: scheme.primary` / `labelColor: scheme.primary` /
// `unselectedLabelColor: scheme.onSurfaceVariant` copies across eight files
// (order list, address list, search results, marketplace, profile ×2, profile
// reviews, link picker, commerce resource picker). Eight of the nine restated
// what this Flutter version already defaults for a primary M3 tab bar; the
// ninth (commerce resource picker) omitted indicatorColor and inherited the
// same value. Restatement is exactly how dialects drift: a copy is free to be
// edited independently of the original, and forgetting one is invisible.
//
// THE TRUTH: AppTheme.tabBarTheme owns every accent pixel of every TabBar.
// This locks:
// 1. NO TabBar ctor in lib states indicatorColor/labelColor/
//    unselectedLabelColor — NO exceptions (the allowlist is empty and must
//    stay equal to the set of files that actually violate);
// 2. AppTheme still carries the full accent statement (nothing is silently
//    deleted from the authority);
// 3. both light and dark resolve the accent through the scheme — the
//    statement is pixel-identical to the M3 primary-tab defaults it replaced,
//    so the move changed zero pixels;
// 4. anti-vakum floor + planted-violation probes, so the sweep cannot rot
//    into a detector that passes by never looking.
//
// NOT banned: `indicatorWeight` as a layout value. This Flutter version
// deliberately ignores TabBarThemeData.indicatorWeight when painting — only
// the widget's own parameter participates, clamped UP to the primary-tab
// floor by max() — so a theme statement would be a lie the renderer never
// reads; the parameter stays call-site because its raw value is also the
// tab's bottom padding. BANNED anyway: restating the ctor default
// `indicatorWeight: 2` — three of those were deleted this batch, every one
// of their effects identical to the absent param. ONE layout value remains:
// address list's `: 3` (bottom padding 3 ≠ default 2 — a real pixel, kept
// and documented at the site).
// ============================================================================

/// Accent keys that belong to `tabBarTheme`, never to a call site.
const _accentKeys = {
  'indicatorColor',
  'labelColor',
  'unselectedLabelColor',
};

/// No file may state its own accent. The sweep asserts this set EQUALS the
/// set of files that actually violate — a future exception must be a
/// conscious edit of this list, never silent drift.
const _accentException = <String>{};

/// Brand/status tokens that would smuggle a second accent dialect past a
/// key-only sweep — a custom indicator painted in the brand red, or a label
/// style inked with the scheme.
///
/// NOT scanned: the top-level `tabs:` subtree ([_contentKeys]). That is
/// screen content embedded in the bar (the AppBar gate draws the same line
/// for `actions:`/`bottom:`); tab labels inherit the accent resolved by
/// _TabStyle and do not restate it.
final _accentTokens = [
  RegExp(r'\bstatusColors\b'),
  RegExp(r'\bcolorScheme\.primary\b'),
  RegExp(r'\bscheme\.primary\b'),
  RegExp(r'\bprimaryRed\b'),
  RegExp(r'\bprimaryGradient\b'),
  RegExp(r'\bColors\.[A-Za-z_]+\b'),
];

/// Top-level args whose subtrees hold screen content, not accent.
const _contentKeys = {'tabs'};

/// Plain `TabBar(` plus the named secondary dialect — a `TabBar.secondary(`
/// would still resolve its accent through tabBarTheme, so it must obey the
/// same contract. `TabBarView(`/`TabBarTheme(` never match (name continues
/// past `TabBar` before the paren).
final _ctorRe = RegExp(r'\bTabBar(?:\.\w+)?\s*\(');
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

/// All accent violations in one source file. Deliberately allowlist-blind:
/// the exception list only lives in the expectations, so a violation in an
/// "allowed" file is still counted and the allowlist can never silently grow.
List<String> findAccentViolations(String path, String src) {
  final hits = <String>[];
  for (final m in _ctorRe.allMatches(src)) {
    final lineStart = src.lastIndexOf('\n', m.start) + 1;
    final head = src.substring(lineStart, m.start);
    if (head.contains('class ')) continue; // `class Foo extends TabBar(`
    if (head.contains('//')) continue; // prose mentioning TabBar(
    final lineNo = src.substring(0, m.start).split('\n').length;
    final open = m.end - 1;
    final close = _findClose(src, open);
    if (close == -1) {
      hits.add('$path:$lineNo unbalanced TabBar ctor');
      continue;
    }
    final args = src.substring(open + 1, close);
    final map = _argMap(args);
    if (map == null) {
      hits.add('$path:$lineNo unparsable TabBar args (gate cannot vouch)');
      continue;
    }
    for (final key in map.keys) {
      if (_accentKeys.contains(key)) {
        hits.add(
          '$path:$lineNo `$key:` restated at call site — tabBarTheme owns it',
        );
      }
    }
    final weight = map['indicatorWeight'];
    if (weight == '2' || weight == '2.0') {
      hits.add(
        '$path:$lineNo `indicatorWeight: $weight` restates the TabBar ctor '
        'default — delete it; the painted line is clamped to the primary '
        'floor regardless',
      );
    }
    for (final entry in map.entries) {
      if (_contentKeys.contains(entry.key)) continue; // content, not accent
      for (final tok in _accentTokens) {
        for (final mm in tok.allMatches(entry.value)) {
          hits.add(
            '$path:$lineNo `${entry.key}` paints with `${mm.group(0)}` — '
            'tab accent comes from tabBarTheme',
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
  test('every tab bar defers its accent to tabBarTheme (source sweep)', () {
    final violationsByFile = <String, List<String>>{};
    var ctors = 0;
    for (final f in _libDartFiles()) {
      final path = _norm(f.path);
      final src = f.readAsStringSync();
      ctors += _ctorRe.allMatches(src).length;
      final hits = findAccentViolations(path, src);
      if (hits.isNotEmpty) violationsByFile[path] = hits;
    }

    // Anti-vakum: the audit counted 10 TabBar ctors across 10 files. If the
    // sweep ever sees almost nothing, its parser broke — it must not "pass"
    // by looking at an empty world.
    expect(
      ctors,
      greaterThanOrEqualTo(10),
      reason: 'ctor count collapsed — the sweep is no longer seeing the app',
    );

    final unexpected = violationsByFile.entries
        .map((e) => e.value)
        .expand((v) => v)
        .toList();
    expect(
      unexpected,
      isEmpty,
      reason:
          'accent restated at a call site. Delete it — tabBarTheme already '
          'decides label/indicator/unselected:\n${unexpected.join('\n')}',
    );

    expect(
      violationsByFile.keys.toSet(),
      _accentException,
      reason:
          'the accent allowlist is EMPTY. If a file appeared here, justify it '
          'or strip its accent; if none did, it must stay empty.',
    );
  });

  test('appTheme still states the whole accent contract', () {
    final themeSrc =
        File('lib/core/src/theme/app_theme.dart').readAsStringSync();
    final matches = RegExp(r'tabBarTheme:\s*TabBarThemeData\s*\(')
        .allMatches(themeSrc)
        .toList();
    expect(
      matches,
      hasLength(1),
      reason: 'exactly ONE tabBarTheme statement — the accent authority',
    );

    final m = matches.single;
    final close = _findClose(themeSrc, m.end - 1);
    expect(close, greaterThan(0), reason: 'tabBarTheme ctor must balance');
    final map = _argMap(themeSrc.substring(m.end, close));
    expect(map, isNotNull, reason: 'tabBarTheme args must be plain named list');
    expect(
      map!.keys.toSet(),
      _accentKeys,
      reason: 'the authority must keep the FULL statement — no key silently '
          'deleted (and no unvetted key smuggled in)',
    );

    expect(map['labelColor'], 'scheme.primary');
    expect(
      map['unselectedLabelColor'],
      'scheme.onSurfaceVariant',
      reason: 'unselected tabs stay muted against the brand-ink selection',
    );
    expect(map['indicatorColor'], 'scheme.primary');
  });

  test('both modes resolve the accent through the scheme (zero-pixel)', () {
    for (final theme in [AppTheme.lightTheme, AppTheme.darkTheme]) {
      final t = theme.tabBarTheme;
      expect(t.labelColor, theme.colorScheme.primary);
      expect(t.unselectedLabelColor, theme.colorScheme.onSurfaceVariant);
      expect(t.indicatorColor, theme.colorScheme.primary);
    }
  });

  test('the sweep has teeth: planted accents are caught, clean bars pass',
      () {
    // The exact nine-copy dialect that was purged.
    final bad = findAccentViolations(
      'lib/planted_accent.dart',
      '''
TabBar(
  controller: _c,
  labelColor: scheme.primary,
  unselectedLabelColor: scheme.onSurfaceVariant,
  indicatorColor: Colors.red,
  tabs: [Tab(text: 'A')],
)''',
    );
    for (final key in _accentKeys) {
      expect(
        bad.any((h) => h.contains('`$key:')),
        isTrue,
        reason: 'planted `$key:` must be flagged, got: $bad',
      );
    }
    expect(
      bad.any((h) => h.contains('Colors.red')),
      isTrue,
      reason: 'planted brand token must be flagged, got: $bad',
    );

    // Token paths that carry no banned KEY: a custom indicator and a label
    // style inked with the scheme.
    final smuggled = findAccentViolations(
      'lib/planted_smuggle.dart',
      '''
TabBar(
  indicator: UnderlineTabIndicator(borderSide: BorderSide(color: Colors.red)),
  labelStyle: TextStyle(color: scheme.primary),
  tabs: [Tab(text: 'A')],
)''',
    );
    expect(
      smuggled,
      isNotEmpty,
      reason: 'accent smuggled through a value must be flagged: $smuggled',
    );

    // Unparsable shape must be reported, never waved through.
    final spread = findAccentViolations(
      'lib/planted_spread.dart',
      'TabBar(..._rest, tabs: const [Tab(text: "A")])',
    );
    expect(
      spread,
      isNotEmpty,
      reason: 'a shape the gate cannot vouch for must fail loudly',
    );

    // And the post-convergence reality — a bar with only structural params —
    // must pass, or the gate is just noise nobody can satisfy.
    final clean = findAccentViolations(
      'lib/planted_clean.dart',
      '''
TabBar(
  controller: _tabController,
  isScrollable: true,
  onTap: _onTabSelected,
  indicatorWeight: 3,
  tabs: [Tab(text: 'A'), Tab(text: 'B')],
)''',
    );
    expect(clean, isEmpty, reason: 'a clean tab bar must pass: $clean');

    // Restating the ctor default is junk even though it is not accent —
    // but a real layout value (`: 3`, inside `clean` above) passes.
    final defaulted = findAccentViolations(
      'lib/planted_weight.dart',
      '''
TabBar(
  indicatorWeight: 2,
  tabs: [Tab(text: 'A')],
)''',
    );
    expect(
      defaulted.any((h) => h.contains('indicatorWeight')),
      isTrue,
      reason:
          'planted `indicatorWeight: 2` (ctor default restatement) must be '
          'flagged, got: $defaulted',
    );
  });
}
