// THE theme-authority rule. ONE definition, imported by every contract test
// that guards it.
//
// A second copy is a second truth: the checkout-scope guard used to carry its
// own, weaker pattern (no hex, no `isDark`, no status tokens, no
// `.primaryColor`), so it silently certified files the real gate rejects. That
// copy is deleted; the rule, the authority island and the scan live here only.
import 'dart:io';

import 'package:flutter/material.dart';

/// A ThemeData's text theme resolved the way WIDGETS see it.
///
/// `ThemeData.textTheme` as constructed carries only colour and family: the
/// geometry (size, weight, height, letter spacing) lives in `englishLike` 2021
/// and is merged onto the theme at read time — `Theme.of()` calls
/// `ThemeData.localize(theme, theme.typography.geometryThemeFor(category))`.
/// Reading the raw `ThemeData.textTheme` in a test therefore yields a style
/// with NO metrics, and any comparison built on it passes as `null == null`.
/// Every gate that pins type must resolve through this helper instead.
TextTheme resolvedTextTheme(ThemeData theme) => ThemeData.localize(
  theme,
  theme.typography.geometryThemeFor(ScriptCategory.englishLike),
).textTheme;

/// The ONLY files in `lib/` allowed to hold colour/geometry authority.
/// Everything else reads `Theme.of(context)`. Growing this set is the only way
/// to legitimise a competing palette, so it is pinned by a test.
const themeAuthorityFiles = <String>{
  // Raw token store — the only place hex literals and palette binds live.
  'lib/core/src/theme/app_colors.dart',
  // The only ThemeData builder (AppTheme.lightTheme / darkTheme) and the only
  // place that owns OS chrome polarity.
  'lib/core/src/theme/app_theme.dart',
  // System brightness -> ThemeMode bridge inside toggleTheme(); it addresses
  // the OS, never widget colour.
  'lib/core/src/theme/theme_provider.dart',
};

/// Competing colour authorities: palette binds, raw Material colours, raw hex,
/// local theme branches, alias names for colours the theme already owns, and
/// legacy Material roles that resolve outside `colorScheme`.
///
/// Legitimate and deliberately NOT matched: brand tokens with no scheme role
/// (coin*, koi*) and `Colors.transparent` — a mode-independent value used for
/// scrims and immersive chrome, not a palette choice.
///
/// OS chrome (`statusBarIconBrightness` / `statusBarBrightness`) is NOT
/// exempt: the polarity is theme data owned by `AppTheme.immersiveOverlayStyle`
/// and `AppTheme._build`, so a widget holding it is a second truth again.
final themeForbiddenColour = RegExp(
  // Raw palette binds, incl. the scheme-role aliases (one colour = one name:
  // AppColors.primary/error/success/warning/successGreen/warningYellow died).
  r'AppColors\.(neutral\w*|darkGray\w*|light|dark|neutral|primaryRed|primaryBlue)\b'
  r'|AppColors\.(primary|error|success|warning|successGreen|warningYellow)\b'
  // Status tones are theme data now — a widget binding them is a second
  // authority that cannot follow light/dark.
  r'|AppColors\.(status\w*|primaryGreen)\b'
  // Scheme role tokens: `primaryPurple` IS `tertiary` in both modes, so a
  // widget binding it bypasses the role authority (migrate to
  // `scheme.tertiary`). `primaryPink` has no role and no legitimate binder.
  // DELIBERATELY ALLOWED brand/identity hues with no scheme role:
  // `coin*`, `koi*`, and `primaryYellow` (pro-tier brand, owner-decided —
  // see seller_tier_badge.dart). Do not add them here.
  r'|AppColors\.(primaryPurple|primaryPink)\b'
  // Support category colour has ONE map: the CategoryConfig palette in the
  // support domain. A per-category switch onto statusColors/scheme hues is
  // the killed second map (nine categories collapsed into five, disagreeing
  // with the ticket screens that render the config palette).
  r'|SupportCategory\.\w+(\s*\|\|\s*null)?\s*=>\s*(context\.statusColors|Theme\.of\()'
  r'|Colors\.(white|black|grey|gray|green|orange|red|blue|yellow|amber)\b'
  r'|Colors\.(white|black|grey|gray)[0-9]'
  r'|Color\(0x'
  r'|Color\.from(ARGB|RGBO)'
  // The deprecated Color method, not a local helper that merely takes a colour.
  r'|\.withOpacity\('
  // Elevation (`AppElevation`) and interaction timing (`AppMotion`) are theme
  // data, not per-widget numbers: a raw value here is a second truth, and the
  // same intent used to be spelled 300/250/200 across different screens.
  r'|elevation:\s*[0-9]'
  r'|Duration\(milliseconds:\s*[0-9]'
  // Corner radii are theme data (`AppShape`), not per-widget numbers.
  r'|Radius\.circular\(\s*[0-9]'
  // Font sizes and padding steps are theme data too (`AppMetrics`; type is the
  // `AppTypeRoles` extension).
  // A call whose literal sits on a later line is caught by the whole-file pass
  // in `themeAuthorityViolations` — same regex, wider scope.
  //
  // The literal may appear ANYWHERE in the expression, not just first:
  // `fontSize: isTotal ? 18 : 14` and `fontSize: style?.fontSize ?? 14` used
  // to slip through un-tokenized. Values with a decimal point in a COMPUTED
  // expression (`size.width * 0.32`, `stepSize * 0.42`) are proportional
  // geometry, not a size decision, and stay legal (the guard fails on dots).
  // Tokenized references like `AppMetrics.p16` never match: the digit is glued
  // to a word character. The scan stops at the first comma so a later argument
  // on the same statement (`... fontSize: 14, size: AppIconSize.action`) is not
  // caught.
  r'|fontSize:\s*[^,\n]*(?<![\w.])[0-9]+(?![\d.])'
  r'|EdgeInsets\.\w+\([^)]*(?<![\w.])[0-9]+(?![\d.])'
  r'|isDark'
  r'|brightness\s*=='
  r'|Brightness\.dark'
  r'|platformBrightness'
  // Legacy Material role: primaryColor is the M2 answer to "what is the brand
  // colour" and bypasses the colourScheme authority entirely.
  r'|\.primaryColor\b',
);

/// Every Dart file under [dir], as normalised `lib/...` paths.
List<String> themeAuthorityDartFiles({String dir = 'lib'}) => Directory(dir)
    .listSync(recursive: true)
    .whereType<File>()
    .where((f) => f.path.endsWith('.dart'))
    .map((f) => f.path.replaceAll(r'\', '/'))
    .toList();

// ─────────────────────────────────────────────────────────────────────────────
// INK IS NOT A SURFACE.
//
// `onSurface` / `onSurfaceVariant` are the roles TEXT and ICONS are painted
// with. Using one as a FILL or a BORDER is how grey-on-grey ships: a screen
// title in exactly the colour of the app bar behind it, a dialog whose ink
// equals its own background, a text field `fillColor`ed with ink, a sheet
// header slab that ate its own heading. That is the defect found on the
// address and verification surfaces (scaffold, app bar, alert dialog, form
// sheet body + header + footer, notes box, purpose block, map picker, phone
// block, dropdown menu, date-of-birth field border, sheet drag handles) and in
// the seller wizard's address dialog, so the rule is locked here.
//
// The canonical answers: `inverseSurface`/`onInverseSurface` for inverted
// fills, `surfaceContainer*` for subtle fills, `outlineVariant` for borders,
// `Colors.transparent` for overlays. On-media controls painted over a camera
// preview or scrim use `onPrimary` (the always-light ink) and stay legal.
//
// Scope, stated honestly: a fill binding inside a `BoxDecoration`/
// `InputDecoration` block, and any `backgroundColor:` binding anywhere. Ink
// reads (`statusDisplay.tone`, a `TextStyle.color`) are out of scope by
// construction — a role used as INK is correct.
const inkFillRoles = <String>['onSurface', 'onSurfaceVariant'];

/// Blocks that hold a surface/border decision and NEVER a `TextStyle`, so every
/// fill-ish property inside them is a fill.
const inkFillHosts = <String>['BoxDecoration(', 'InputDecoration('];

final RegExp _inkFillProperty = RegExp(
  r'(?<![A-Za-z])(color|fillColor|backgroundColor):\s*[^,\n]*'
  r'\.(onSurface|onSurfaceVariant)\b',
);

final RegExp _inkBackgroundProperty = RegExp(
  r'(?<![A-Za-z])backgroundColor:\s*[^,\n]*'
  r'\.(onSurface|onSurfaceVariant)\b',
);

/// The WHOLE line holding the bind at [index], so the alpha check sees the
/// value and not just the indentation before the property name.
String _lineAround(String text, int index, int matchEnd) {
  final start = text.lastIndexOf('\n', index) + 1;
  var end = text.indexOf('\n', matchEnd);
  if (end < 0) end = text.length;
  return text.substring(start, end);
}

/// A solid ink bind is a bug; an ink bind WITH alpha is a legitimate tint
/// (M3 state layers paint `onSurface` at 8–12%).
bool _isSolidInkBind(String line) =>
    !line.contains('withValues(') && !line.contains('withOpacity(');

/// Balanced-paren blocks opened by [opener], e.g. every `BoxDecoration(...)`,
/// yielded with their offset in [source] so violations keep real line numbers.
Iterable<({int start, String block})> _hostBlocks(
  String source,
  String opener,
) sync* {
  var from = 0;
  while (true) {
    final idx = source.indexOf(opener, from);
    if (idx < 0) return;
    var depth = 0;
    var end = idx + opener.length - 1;
    for (; end < source.length; end++) {
      final ch = source[end];
      if (ch == '(') depth++;
      if (ch == ')') {
        depth--;
        if (depth == 0) break;
      }
    }
    yield (start: idx, block: source.substring(idx, end + 1));
    from = idx + opener.length;
  }
}

/// Scan ONE source string; [path] only labels the violations. Split out so the
/// contract test can prove the detector fires on a planted resurrection without
/// writing a file.
({List<String> violations, int hosts}) inkAsFillViolationsIn(
  String source, {
  required String path,
}) {
  final violations = <String>[];
  var hosts = 0;

  int lineOf(int index) => source.substring(0, index).split('\n').length;

  // 1. Anything that names a background: widgets and data objects alike. A
  //    status-tone field must not be called `backgroundColor` — name it what it
  //    is (`tone`) instead of widening this rule.
  for (final m in _inkBackgroundProperty.allMatches(source)) {
    final line = _lineAround(source, m.start, m.end);
    if (_isSolidInkBind(line)) {
      violations.add('$path:${lineOf(m.start)}: ${line.trim()}');
    }
  }

  // 2. Fills inside surface/border blocks.
  for (final host in inkFillHosts) {
    for (final found in _hostBlocks(source, host)) {
      hosts++;
      // An `InputDecoration` carries its own ink: `hintStyle: TextStyle(...)`
      // or `theme.textTheme.bodyMedium?.copyWith(...)`, `prefixIcon: Icon(...)`.
      // Those are legitimate ink READS, so any fill match landing inside one of
      // those sub-blocks is skipped.
      final inkSpans = <({int start, int end})>[];
      for (final opener in const [
        'TextStyle(',
        'copyWith(',
        'Icon(',
        'TextButton(',
      ]) {
        for (final ink in _hostBlocks(found.block, opener)) {
          inkSpans.add((start: ink.start, end: ink.start + ink.block.length));
        }
      }
      bool readsInk(int at) => inkSpans.any((s) => at >= s.start && at < s.end);

      for (final m in _inkFillProperty.allMatches(found.block)) {
        if (readsInk(m.start)) continue;
        final line = _lineAround(found.block, m.start, m.end);
        if (_isSolidInkBind(line)) {
          final at = found.start + m.start;
          violations.add('$path:${lineOf(at)}: ${line.trim()}');
        }
      }
    }
  }
  return (violations: violations, hosts: hosts);
}

/// Sweep [paths] (default: `lib/`) for ink used as a surface. [hosts] is the
/// anti-vacuum floor: it counts the fill blocks actually inspected.
({List<String> violations, int hosts}) inkAsFillScan({List<String>? paths}) {
  final violations = <String>[];
  var hosts = 0;
  for (final raw in paths ?? themeAuthorityDartFiles()) {
    final path = raw.replaceAll(r'\', '/');
    if (themeAuthorityFiles.contains(path)) continue;
    final file = File(path);
    if (!file.existsSync()) continue;
    final res = inkAsFillViolationsIn(file.readAsStringSync(), path: path);
    violations.addAll(res.violations);
    hosts += res.hosts;
  }
  return (violations: violations, hosts: hosts);
}

/// Scan [paths] (default: every Dart file under `lib/`) and return the
/// competing-authority lines as `path:line: text`.
///
/// Files in [themeAuthorityFiles] are skipped unless [skipAuthority] is false —
/// those are the island allowed to hold colour.
List<String> themeAuthorityViolations({
  List<String>? paths,
  bool skipAuthority = true,
}) {
  final violations = <String>[];
  for (final raw in paths ?? themeAuthorityDartFiles()) {
    final path = raw.replaceAll(r'\', '/');
    if (skipAuthority && themeAuthorityFiles.contains(path)) continue;
    final file = File(path);
    if (!file.existsSync()) continue;
    final content = file.readAsStringSync();
    final lines = content.split('\n');
    var caught = false;
    for (var i = 0; i < lines.length; i++) {
      if (themeForbiddenColour.hasMatch(lines[i])) {
        violations.add('$path:${i + 1}: ${lines[i].trim()}');
        caught = true;
      }
    }
    // Whole-file pass: the SAME regex over the whole buffer, because a call
    // that opens on one line and holds its literal on the next is invisible to
    // the per-line loop above — which is exactly how `EdgeInsets.only(` +
    // `left: 4,` on the following line used to slip through the gate.
    if (!caught) {
      final m = themeForbiddenColour.firstMatch(content);
      if (m != null) {
        final line = content.substring(0, m.start).split('\n').length;
        violations.add('$path:$line: ${lines[line - 1].trim()}');
      }
    }
  }
  return violations;
}
