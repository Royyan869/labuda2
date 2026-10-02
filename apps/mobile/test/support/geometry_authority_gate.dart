// GEOMETRY AUTHORITY — the census behind the ratchet.
//
// The colour, type, corner, elevation and motion ladders are defended by HARD
// gates (`theme_authority_gate.dart`): one raw literal outside the three
// authority files fails the lib-wide sweep outright. Five geometry decisions
// never got that protection, so each grew a second, unguarded spelling beside
// the ladder that already names it:
//
//   spacing      `SizedBox(height: 16)`  — `AppMetrics.p16` already names the
//                                          step
//   contentDimension `SizedBox(width: 280, child: button)`, a 64×64 avatar box —
//                                          the SIZE OF something, not a step
//                                          between things; the spacing ladder
//                                          cannot name it (see [_isSizedContent]
//                                          for where the line is drawn)
//   iconSize     `Icon(size: 20)`        — now owned by `AppIconSize` (the
//                                          ladder landed and the 657-site
//                                          backlog migrated to cap 0 on
//                                          2026-10-02)
//   strokeWidth  `Border.all(width: 1)`  — no ladder at all
//   lineMetric   `letterSpacing: 0.5` / a text `height: 1.1` — a text role owns
//                                          both, not the call site
//   shadow       `blurRadius: 10` / `Offset(0, 2)` — `AppElevation` names the step
//
// plus the dodge that hides a literal from every `property: <digit>` pattern:
// `BorderRadius.circular(borderRadius ?? 12)` passes a regex that only looks
// for a digit right after `circular(`, so the radius decision survives where
// the shape ladder cannot see it.
//
// and the one that let a real regression ship on 2026-10-02 — a box that
// FREEZES an axis with a raw literal while the ladder decides how big what goes
// inside it comes out:
//
//   frozenExtent `Container(height: 60, padding: EdgeInsets.symmetric(
//                  vertical: AppMetrics.p8), child: …)`
//
// The budget is `literal - everything the ladder decides`, and NOTHING in the
// source proves any slack exists, so the day a step moves the content silently
// overflows: that toolbar kept `height: 60` while its label moved
// `AppType.s10 → s12` and the icon+label column overflowed by 3 px, which only
// a widget test could see. The literal never changed; the ladder did. This is
// NOT the same finding as `contentDimension`: that one says "this literal is not
// on a ladder", this one says "this literal is a frozen BUDGET holding
// ladder-driven content". One site can honestly carry both.
//
// The first decision is ONE concept, counted as ONE `gap` until 2026-10-02. That
// single cap mixed two different jobs — "migrate this step to the spacing
// ladder" and "this size is not a step at all" — and the mix was lopsided
// enough to hide the second job entirely. It is now reported as two categories:
// `spacing` and `contentDimension`.
//
// WHY A CENSUS AND NOT A HARD GATE YET. A hard gate can only be switched on for
// a rule the app already obeys. For these five it does not: the real app holds
// thousands of them (measured by [geometryCensus]). Turning the rule on first
// would clean nothing; it would only paint the suite red. So the ratchet that
// tamed `AppType` is applied here: the census counts today's call sites, every
// category is capped so it can only go DOWN, a finished file is locked to zero
// BY PATH, and the detectors must fire on a planted resurrection before they
// are allowed to certify anything.
//
// SCOPE, stated honestly: a `property:` value is counted only when its host
// block is recognised AND the value carries a literal WITHOUT arithmetic —
// `width * 0.32` is proportional geometry, not a step decision, and stays
// legal, exactly as the colour gate already rules for sizes. `copyWith` chains
// that set a line metric are NOT counted (the property is not inside a
// `TextStyle(` block): a known blind spot of the census, not a licence to write
// new ones. `?? 0` is a legitimate "no size" default and is not counted either.
// A property must also sit on the HOST'S OWN argument level: a `width:` nested
// inside `Border.all(` belongs to the stroke, not to the box around it.
library;

import 'dart:io';

/// Files allowed to hold raw geometry digits — the same island as colour and
/// type: the ladder definitions themselves.
const geometryAuthorityFiles = <String>{
  'lib/core/src/theme/app_colors.dart',
  'lib/core/src/theme/app_theme.dart',
  'lib/core/src/theme/theme_provider.dart',
};

/// The categories the ratchet caps. A new category must be added here AND to
/// the frozen caps in the test — i.e. deliberately.
const geometryCategories = <String>[
  'spacing',
  'contentDimension',
  'frozenExtent',
  'iconSize',
  'strokeWidth',
  'lineMetric',
  'shadow',
  'defaultedLiteral',
];

/// ONE geometry decision, as a host block plus the properties that decide it.
///
/// The decision is counted PER SITE, not per property: a
/// `SizedBox(width: 8, height: 8)` is one thing to migrate, not two.
class GeometryRule {
  const GeometryRule(this.category, this.hosts, this.property);

  final String category;

  /// Block openers that introduce the decision.
  final List<String> hosts;

  /// The property spelling inside the host.
  final RegExp property;
}

final RegExp _dimension = RegExp(r'(?<![A-Za-z])(?:width|height):');
final RegExp _stroke = RegExp(r'(?<![A-Za-z])(?:width|thickness):');
final RegExp _iconSize = RegExp(r'(?<![A-Za-z])size:');
final RegExp _lineMetric = RegExp(
  r'(?<![A-Za-z])(?:letterSpacing|height):',
);
final RegExp _shadow = RegExp(
  r'(?<![A-Za-z])(?:blurRadius|spreadRadius|offset):',
);

final List<GeometryRule> _rules = <GeometryRule>[
  GeometryRule('iconSize', ['Icon(', 'IconThemeData('], _iconSize),
  GeometryRule(
    'strokeWidth',
    ['Border.all(', 'BorderSide(', 'Divider('],
    _stroke,
  ),
  GeometryRule('lineMetric', ['TextStyle('], _lineMetric),
  GeometryRule('shadow', ['BoxShadow('], _shadow),
];

/// The sizing family (`SizedBox`/`Container`) — the ONE rule SPLIT IN TWO.
///
/// A bare box is the gap between things (`spacing`, and `AppMetrics` already
/// owns those steps). A box that IS something — it wraps a child, paints a
/// surface, or decides both axes — is content/media geometry
/// (`contentDimension`): a thumbnail, an avatar, a fixed-width button, a
/// hairline. The spacing ladder cannot name those, so they need a size policy
/// of their own; mixing the two under one cap is what this category exists to
/// stop.
///
/// The split is STRUCTURAL, keyed on the site's own syntax ([_isSizedContent]),
/// not on whether the number happens to be on the ladder: keying it on the
/// ladder would silently reclassify sites whenever a step is added, and would
/// still call a 40×40 avatar a "gap" because 40 is a spacing step.
const List<String> _sizingHosts = <String>['SizedBox(', 'Container('];

final RegExp _ownBoxSignal = RegExp(
  r'(?<![A-Za-z])(?:child|color|decoration|foregroundDecoration):',
);

/// The block with everything inside a NESTED call blanked out, so a property is
/// only seen at the host's own argument level.
///
/// Same length as the input (positions stay valid for [_valueWindow]): a
/// `width:` inside `Border.all(` on a `Container(height: 1, …)` is the STROKE's
/// decision, and reading it as the box's second axis would both report a
/// separator as content and count a stroke width as a gap.
String _ownLevel(String block) {
  final chars = block.split('');
  var depth = 0;
  for (var i = 0; i < chars.length; i++) {
    final ch = chars[i];
    if (ch == '(') {
      depth++;
      continue;
    }
    if (ch == ')') {
      depth--;
      continue;
    }
    if (depth > 1) chars[i] = ' ';
  }
  return chars.join();
}

/// True when the sizing block IS something (content/media), false when it is
/// only empty space between things.
///
/// `SizedBox(height: 16)` is a gap. `SizedBox(width: 280, child: button)` is a
/// button width; `Container(width: 64, height: 64)` is an avatar box;
/// `Container(height: 1, color: outlineVariant)` is a drawn hairline. One axis
/// and no paint, no child — that is the only thing left in `spacing`.
bool _isSizedContent(String block) {
  final own = _ownLevel(block);
  if (_ownBoxSignal.hasMatch(own)) return true;
  return _dimension.allMatches(own).length > 1;
}

/// The FROZEN BUDGET lens.
///
/// `Container(height: 60, … child: label)` is not wrong because 60 is a
/// literal — `contentDimension` already reports that. It is wrong as a BUDGET:
/// the box promises exactly 60 logical pixels while the ladder decides how tall
/// the content inside it comes out, so a step move consumes slack the source
/// never proved existed. That is exactly how the toolbar regression shipped: it
/// kept `height: 60` while its label moved `AppType.s10 → s12`, and the
/// icon+label column overflowed by 3 px — a failure only a widget test could
/// see. The literal did not change; the ladder did.
///
/// WHERE THE EVIDENCE IS READ — the one place this census deliberately looks
/// past the own level. Everywhere else a property is only read at the host's
/// own argument level, so a nested call owns its own decision. Here the
/// DECISION (the frozen literal) is still read at own level — that is what
/// makes the finding a promise the box itself made — while the ladder evidence
/// is read from the whole block.
///
/// Why the wider reading, and what it costs. A same-axis own-level reading was
/// tried first (frozen `height` + `vertical:` padding naming a step) and
/// measured ZERO live sites, because the one site that shape existed at was the
/// toolbar — already fixed when this rule was written. The subtree reading
/// measures 72 sites, and each one is the family the toolbar belonged to: a
/// frozen extent holding content the ladder measures.
///
/// BLIND SPOT, documented rather than hidden: evidence inside a SEPARATE widget
/// class that the box only names (`child: _ToolbarIconRow()`) is invisible to a
/// source-level census. The toolbar site was caught because its OWN
/// `padding: … vertical: AppMetrics.p8` sat on the frozen axis; the text that
/// actually overflowed lived one class down and is NOT what this rule read. The
/// rule therefore guards the SHAPE, not the exact overflow: a box that freezes
/// an extent while ladder-measured content sits inside it, and a same-axis step
/// on that frozen axis — the thing that eats the budget — is the strongest
/// evidence of all.

/// SCOPE, stated honestly:
/// (1) only the sizing family ([_sizingHosts]) is read — the shape is about a
/// box's own promised extent, and `ConstrainedBox`/`BoxConstraints` are not
/// host spellings the app uses for it today;
/// (2) the frozen axis must be a RAW literal — a step, a role or a computed
/// value is not frozen, so `height: AppMetrics.p48` is not a finding;
/// (3) the box must CONTAIN something. A childless `height: 1` divider or a
/// bare 40×4 bar cannot overflow — nothing has to fit inside it — so it is
/// reported by `contentDimension` (the size OF a thing) and not here. Requiring
/// the child is what makes "frozen budget holding ladder-driven content" true
/// of every finding, rather than true of most of them;
/// (4) the ladder evidence must be a ladder this app owns and that decides an
/// extent: `AppMetrics` (spacing + padding + roles), `AppIconSize`,
/// `AppContentSize` (the content/media ladder, added with its policy on
/// 2026-10-02 — a content-size token inside the box measures what has to fit,
/// exactly like a spacing step), an `AppType.s*` step and the
/// `context.typeRoles` view of it. A bare colour, radius or shadow token
/// inside the box changes no extent and is not counted;
/// (5) a box nested inside another sizing host IS still read here, unlike the
/// split rule: the outer box's promise is its own decision, not a duplicate of
/// the inner box's literal. Measured, the two readings differ by a handful of
/// sites, and dropping this one would blind the lens to a frozen box whose
/// child is itself a box.
final RegExp _frozenAxis = RegExp(r'(?<![A-Za-z])(height|width):');
final RegExp _holdsContent = RegExp(r'(?<![A-Za-z])child(?:ren)?:');
final RegExp _ladderContent = RegExp(
  r'AppMetrics\.|AppIconSize\.|AppContentSize\.|AppType\.s[0-9]|typeRoles\.',
);

/// True when the block FREEZES an axis with a raw literal while its own
/// subtree is measured by the ladder — a promise with no slack budget.
bool _ownsFrozenExtentOverLadderContent(String block) {
  final own = _ownLevel(block);
  var frozen = false;
  for (final axis in _frozenAxis.allMatches(own)) {
    if (geometryStepDecision(_valueWindow(block, axis.end))) {
      frozen = true;
      break;
    }
  }
  if (!frozen) return false;
  // Nothing to fit inside ⇒ nothing to overflow. The bar and the divider are
  // reported as `contentDimension` (their size IS the decision) and must not
  // inflate this lens.
  if (!_holdsContent.hasMatch(own)) return false;
  return _ladderContent.hasMatch(block);
}

/// A geometry literal hidden behind a default: `borderRadius ?? 12`. Captures
/// the literal so `?? 0` (a legitimate "no size") can be spared. Scope: the
/// default must sit on the property's own line (a `??` buried inside a nested
/// call is not counted — a known blind spot, not a licence).
final RegExp _defaultedLiteral = RegExp(
  r'(?<![A-Za-z])(?:width|height|size|radius|borderRadius|blurRadius'
  r'|spreadRadius|letterSpacing|thickness|elevation):\s*[^,\n]*\?\?\s*'
  r'(-?[0-9]+(?:\.[0-9]+)?)',
);

/// A number in a geometry position. Digits glued to a word (`AppMetrics.p16`,
/// `AppType.s12`) are token references, not literals.
final RegExp _number = RegExp(r'(?<![\w.])-?[0-9]+(?:\.[0-9]+)?(?![\d.])');

/// The value text starting at [valueStart], walked to its real end.
///
/// A plain `[^,\n]*` window is wrong here: `offset: Offset(0, size * 0.1)`
/// cuts at the comma INSIDE `Offset(`, hides the `*`, and reports proportional
/// geometry as a step decision. Depth tracking keeps the whole expression, and
/// only a TOP-LEVEL comma or a newline outside any call ends it.
String _valueWindow(String source, int valueStart) {
  var end = valueStart;
  var depth = 0;
  while (end < source.length) {
    final ch = source[end];
    if (ch == '\n' && depth == 0) break;
    if (ch == '(') depth++;
    if (ch == ')') {
      if (depth == 0) break; // the host block itself closes here
      depth--;
      if (depth == 0) {
        end++;
        break;
      }
    }
    if (ch == ',' && depth == 0) break;
    end++;
  }
  return source.substring(valueStart, end);
}

/// A host that CONTAINS another host is not the innermost decision: `_blocks`
/// yields the inner one too, so counting the outer as well would report one
/// literal twice and keep the outer's count alive after the inner is fixed.
bool _hasNestedHost(String block, List<String> hosts) {
  for (final opener in hosts) {
    // A sibling opener can be longer than the block it is tested against
    // (`Divider()` is shorter than `Border.all(`), so the search starts at the
    // block's own end rather than past it.
    final from = opener.length < block.length ? opener.length : block.length;
    if (block.indexOf(opener, from) > 0) return true;
  }
  return false;
}

/// True when any [property] declared at the block's OWN level carries a step
/// decision. The value text is read from the original block, so a nested call
/// inside the value (`width: size * 0.5`) still reads as one expression.
bool _blockDecidesGeometry(String block, RegExp property) {
  final own = _ownLevel(block);
  for (final match in property.allMatches(own)) {
    if (geometryStepDecision(_valueWindow(block, match.end))) return true;
  }
  return false;
}

/// True when [expression] is a STEP DECISION: it carries a number and no
/// arithmetic.
///
/// `AppMetrics.p16` is spared because its digits are glued to a word;
/// `size.width * 0.32` and `height / 2` are spared because derived geometry is
/// not a step; `16`, `1.5` and `-0.5` are decisions the ladder must own.
bool geometryStepDecision(String expression) {
  if (expression.contains('*') || expression.contains('/')) return false;
  return _number.hasMatch(expression);
}

/// Line comments are BLANKED, not removed, so every reported violation keeps an
/// exact line number and prose about a ladder is never mistaken for a call site.
String blankLineComments(String source) {
  final lines = source.split('\n');
  for (var i = 0; i < lines.length; i++) {
    final cut = lines[i].indexOf('//');
    if (cut >= 0) lines[i] = lines[i].substring(0, cut);
  }
  return lines.join('\n');
}

/// Balanced-paren blocks opened by [opener], yielded with their offset so
/// violations keep real line numbers.
Iterable<({int start, String block})> _blocks(
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
    if (end >= source.length) return; // unbalanced (a string, not a block)
    yield (start: idx, block: source.substring(idx, end + 1));
    from = idx + opener.length;
  }
}

int _lineAt(String source, int index) =>
    '\n'.allMatches(source.substring(0, index)).length + 1;

String _textAt(String source, int index) {
  final start = source.lastIndexOf('\n', index) + 1;
  var end = source.indexOf('\n', index);
  if (end < 0) end = source.length;
  return source.substring(start, end).trim();
}

/// EVERY geometry call site in one source, by category, as
/// `path:line: text`. Split out so the ratchet test can prove the detectors
/// fire on a planted resurrection without writing a file.
Map<String, List<String>> geometryViolationsIn(
  String source, {
  required String path,
}) {
  final blanked = blankLineComments(source);
  final out = <String, List<String>>{
    for (final category in geometryCategories) category: <String>[],
  };

  void record(String category, int index) {
    out[category]!.add(
      '$path:${_lineAt(blanked, index)}: ${_textAt(blanked, index)}',
    );
  }

  // Every block rule, innermost host only, one finding per site.
  for (final rule in _rules) {
    for (final host in rule.hosts) {
      for (final found in _blocks(blanked, host)) {
        if (_hasNestedHost(found.block, rule.hosts)) continue;
        if (_blockDecidesGeometry(found.block, rule.property)) {
          record(rule.category, found.start);
        }
      }
    }
  }

  // The split sizing rule: one host family, two categories, decided per site by
  // the site's own syntax ([_isSizedContent]).
  for (final host in _sizingHosts) {
    for (final found in _blocks(blanked, host)) {
      if (_hasNestedHost(found.block, _sizingHosts)) continue;
      if (!_blockDecidesGeometry(found.block, _dimension)) continue;
      record(
        _isSizedContent(found.block) ? 'contentDimension' : 'spacing',
        found.start,
      );
    }
  }

  // The frozen-budget lens: a host that promises an extent with a raw literal
  // while the ladder measures what goes inside it. Counted beside the split
  // rule rather than folded into it — one site can honestly carry both findings
  // ("this literal is not on a ladder" AND "this literal is a budget with no
  // slack"), and merging them would hide the budget question again. Nested
  // sizing hosts are NOT skipped here: see [_ownsFrozenExtentOverLadderContent].
  for (final host in _sizingHosts) {
    for (final found in _blocks(blanked, host)) {
      if (_ownsFrozenExtentOverLadderContent(found.block)) {
        record('frozenExtent', found.start);
      }
    }
  }

  // the `?? literal` dodge — a `?? 0` default is not a step decision.
  for (final match in _defaultedLiteral.allMatches(blanked)) {
    final value = double.tryParse(match.group(1)!);
    if (value == null || value == 0) continue;
    record('defaultedLiteral', match.start);
  }

  return out;
}

/// Every Dart file under [dir], as normalised `lib/...` paths. Generated code is
/// skipped: it cannot hold a ladder decision.
List<File> geometryCensusFiles({String dir = 'lib'}) => Directory(dir)
    .listSync(recursive: true)
    .whereType<File>()
    .where((file) => file.path.endsWith('.dart'))
    .where(
      (file) => !file.path.replaceAll(r'\', '/').contains('/generated/'),
    )
    .toList();

/// Category -> call-site count across `lib`.
Map<String, int> geometryCensus({String dir = 'lib'}) {
  final census = <String, int>{for (final c in geometryCategories) c: 0};
  for (final file in geometryCensusFiles(dir: dir)) {
    final path = file.path.replaceAll(r'\', '/');
    if (geometryAuthorityFiles.contains(path)) continue;
    final found = geometryViolationsIn(file.readAsStringSync(), path: path);
    for (final category in geometryCategories) {
      census[category] = census[category]! + found[category]!.length;
    }
  }
  return census;
}

/// Category -> call-site count for ONE file. A migrated slice must read zeros.
Map<String, int> geometryViolationsInFile(String path) {
  final file = File(path);
  if (!file.existsSync()) return <String, int>{};
  final found = geometryViolationsIn(file.readAsStringSync(), path: path);
  return <String, int>{
    for (final category in geometryCategories)
      category: found[category]!.length,
  };
}

/// Total call sites per file — the map used to pick the next smallest slice.
Map<String, int> geometryCensusByFile({String dir = 'lib'}) {
  final byFile = <String, int>{};
  for (final file in geometryCensusFiles(dir: dir)) {
    final path = file.path.replaceAll(r'\', '/');
    if (geometryAuthorityFiles.contains(path)) continue;
    final found = geometryViolationsIn(file.readAsStringSync(), path: path);
    final total = found.values.fold(0, (sum, list) => sum + list.length);
    if (total > 0) byFile[path] = total;
  }
  return byFile;
}

int geometryCensusTotal(Map<String, int> census) =>
    census.values.fold(0, (sum, count) => sum + count);
