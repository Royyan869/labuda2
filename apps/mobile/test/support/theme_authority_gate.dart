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
  r'|fontSize:\s*[0-9]'
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
