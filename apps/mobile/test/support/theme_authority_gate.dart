// THE theme-authority rule. ONE definition, imported by every contract test
// that guards it.
//
// A second copy is a second truth: the checkout-scope guard used to carry its
// own, weaker pattern (no hex, no `isDark`, no status tokens, no
// `.primaryColor`), so it silently certified files the real gate rejects. That
// copy is deleted; the rule, the authority island and the scan live here only.
import 'dart:io';

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
  // Font sizes and padding steps are theme data too (`AppType` / `AppMetrics`).
  // A call whose literal sits on a later line is caught by the whole-file pass
  // in `themeAuthorityViolations` — same regex, wider scope.
  //
  // The literal may appear ANYWHERE in the expression, not just first:
  // `fontSize: isTotal ? 18 : 14` and `fontSize: style?.fontSize ?? 14` used
  // to slip through un-tokenized. Values with a decimal point in a COMPUTED
  // expression (`size.width * 0.32`, `stepSize * 0.42`) are proportional
  // geometry, not a size decision, and stay legal (the guard fails on dots).
  // Tokenized values like `AppType.s14` never match: the digit is glued to a
  // word character. The scan stops at the first comma so a later argument on
  // the same statement (`... fontSize: AppType.s14, size: 20`) is not caught.
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
