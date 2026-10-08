/// TYPOGRAPHY AUTHORITY GUARD.
///
/// The canonical type authority is [AppTypeRoles] (`context.typeRoles`), derived
/// from the theme's RESOLVED M3 geometry — never from a numeric size ladder. The
/// legacy `AppType` ladder (five `AppType.s*` tokens) has been deleted; this
/// gate keeps it dead:
///
/// 1. no `AppType.s*` size token may exist anywhere in `lib`, and the detector
///    is proved to fire on a planted token (the resurrection path is a paste);
/// 2. the roles ARE the five enshrined steps, one name per value, and no retired
///    size may come back as a role;
/// 3. the roles derive from the theme's resolved geometry, proved by forking it.
///
/// This is the terminal, SIMPLE guard. The staged machinery it replaced
/// (per-token caps, per-slice path locks, a consumer census) tracked the
/// migration backlog and went away with it.
library;

import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';

import '../../support/theme_authority_gate.dart';

/// A retired `AppType.s<n>` size token. Named after the deleted authority on
/// purpose: a reintroduction is caught by the exact spelling it would use.
final RegExp _retiredSizeToken = RegExp(r'AppType\.s\w+');

/// The anti-vacuum floor: the sweep must read the real app, not an empty tree.
const _censusFileFloor = 900;

/// Line comments are stripped so prose ABOUT the deleted token is never counted
/// as a call site; `///` is the spelling this codebase uses.
String _stripLineComments(String source) {
  final buffer = StringBuffer();
  for (final line in source.split('\n')) {
    if (line.trimLeft().startsWith('//')) continue;
    final cut = line.indexOf('//');
    buffer.writeln(cut >= 0 ? line.substring(0, cut) : line);
  }
  return buffer.toString();
}

List<File> _libDartFiles({String dir = 'lib'}) => Directory(dir)
    .listSync(recursive: true)
    .whereType<File>()
    .where((file) => file.path.endsWith('.dart'))
    .where((file) => !file.path.replaceAll(r'\', '/').contains('/generated/'))
    .toList();

/// Token -> occurrence count for one SOURCE string.
Map<String, int> _sizeTokensIn(String source) {
  final census = <String, int>{};
  for (final match in _retiredSizeToken.allMatches(
    _stripLineComments(source),
  )) {
    final token = match.group(0)!;
    census[token] = (census[token] ?? 0) + 1;
  }
  return census;
}

/// The FIVE enshrined steps (owner decision 2026-10-02), role by role. The
/// extension first legalised off-ladder sizes (10, 13, 15, 18, 20); the
/// foundation pass folded those onto these five, so a role names one of the five
/// steps instead of a size the retired numeric ladder did not have.
const _enshrinedSteps = <String, double>{
  'labelMicro': 12,
  'bodyDense': 14,
  'titleCompact': 16,
  'titleSection': 20,
  'titleProminent': 24,
};

/// Sizes the foundation RETIRED. They may not reappear as a role or a token:
/// a role that names one would be a second type authority again.
const _retiredTypeSizes = <double>[
  8,
  8.5,
  9,
  10,
  11,
  13,
  15,
  18,
  22,
  28,
  32,
  36,
];

List<TextStyle> _roles(AppTypeRoles roles) => [
  roles.labelMicro,
  roles.bodyDense,
  roles.titleCompact,
  roles.titleSection,
  roles.titleProminent,
];

/// Widgets never read `theme.textTheme` directly: `Theme.of()` resolves it
/// through `ThemeData.localize(theme, typography.geometryThemeFor(category))`,
/// merging the englishLike-2021 geometry (size, weight, height, letter
/// spacing) on top — the raw ThemeData carries only colour and family. This
/// helper resolves the same way, so the derivation proof compares what
/// actually renders instead of silently passing on `null == null`.
TextStyle _resolvedBody(ThemeData theme) =>
    resolvedTextTheme(theme).bodyMedium!;

void main() {
  group('the canonical type roles', () {
    test('both themes register it, derived from their own body style', () {
      for (final theme in [AppTheme.lightTheme, AppTheme.darkTheme]) {
        final roles = theme.extension<AppTypeRoles>();
        expect(roles, isNotNull, reason: 'AppTheme must register AppTypeRoles');

        final body = _resolvedBody(theme);
        // Anti-vacuity: the proof below must compare real metrics. The raw
        // ThemeData's text theme is colour/family only, so a floor here is what
        // stops `null == null` from certifying an empty derivation.
        expect(body.fontSize, isNotNull, reason: 'geometry must be resolved');
        expect(body.fontWeight, isNotNull);
        expect(body.height, isNotNull);
        final sizes = _roles(roles!).map((style) => style.fontSize).toList();
        expect(sizes, _enshrinedSteps.values.toList());
        for (final style in _roles(roles)) {
          // One body style at another size: no metric is restated, and a
          // ladder retune reaches the roles automatically.
          expect(
            style.fontWeight,
            body.fontWeight,
            reason: '${style.fontSize}',
          );
          expect(style.height, body.height, reason: '${style.fontSize}');
          expect(
            style.letterSpacing,
            body.letterSpacing,
            reason: '${style.fontSize}',
          );
          expect(
            style.fontFamily,
            body.fontFamily,
            reason: '${style.fontSize}',
          );
        }
      }
    });

    test('the roles ARE the five enshrined steps, one name per value', () {
      for (final theme in [AppTheme.lightTheme, AppTheme.darkTheme]) {
        final sizes = _roles(
          theme.extension<AppTypeRoles>()!,
        ).map((style) => style.fontSize).toSet();
        expect(
          sizes,
          _enshrinedSteps.values.toSet(),
          reason:
              'a role that names a step the foundation does not have is a '
              'second type authority',
        );
      }
      // The retired sizes must not come back as a role: the foundation folded
      // them onto the five steps (nearest step, ties rounded up).
      for (final retired in _retiredTypeSizes) {
        expect(
          _enshrinedSteps.values,
          isNot(contains(retired)),
          reason: '$retired was folded onto a foundation step',
        );
      }
      expect(_enshrinedSteps, hasLength(5));
    });

    test('a bare ThemeData falls back to the stock M3 body metrics', () {
      final fallback = AppTypeRoles.fallback;
      final body = Typography.material2021().englishLike.bodyMedium!;
      expect(
        body.fontSize,
        isNotNull,
        reason: 'englishLike carries the geometry',
      );
      expect(
        _roles(fallback).map((style) => style.fontSize).toList(),
        _enshrinedSteps.values.toList(),
      );
      for (final style in _roles(fallback)) {
        expect(style.fontWeight, body.fontWeight);
        expect(style.height, body.height);
        expect(style.letterSpacing, body.letterSpacing);
      }
    });

    testWidgets('context.typeRoles resolves under the real theme', (
      tester,
    ) async {
      late AppTypeRoles read;
      await tester.pumpWidget(
        MaterialApp(
          theme: AppTheme.lightTheme,
          home: Builder(
            builder: (context) {
              read = context.typeRoles;
              return const SizedBox();
            },
          ),
        ),
      );
      expect(read.labelMicro.fontSize, 12);
      expect(read.titleSection.fontSize, 20);

      // A bare ThemeData (widget tests pump this constantly) must not throw.
      late AppTypeRoles bare;
      await tester.pumpWidget(
        MaterialApp(
          home: Builder(
            builder: (context) {
              bare = context.typeRoles;
              return const SizedBox();
            },
          ),
        ),
      );
      expect(bare.titleProminent.fontSize, 24);
    });
  });

  group('the retired AppType authority stays dead', () {
    test('no AppType.s* size token exists anywhere in lib', () {
      final files = _libDartFiles();
      expect(
        files.length,
        greaterThan(_censusFileFloor),
        reason: 'the typography census must sweep the whole app',
      );
      final offenders = <String>[];
      for (final file in files) {
        final path = file.path.replaceAll(r'\', '/');
        for (final entry in _sizeTokensIn(file.readAsStringSync()).entries) {
          offenders.add('$path: ${entry.key}');
        }
      }
      expect(
        offenders,
        isEmpty,
        reason:
            'the legacy AppType size ladder must never come back. Type is a '
            'ROLE (`context.typeRoles`), never a numeric token:\n'
            '${offenders.join('\n')}',
      );
    });

    test('the detector fires on a planted size token (negative proof)', () {
      const planted =
          'Text(t, style: const TextStyle(fontSize: AppType.s14));\n';
      expect(
        _sizeTokensIn(planted),
        {'AppType.s14': 1},
        reason: 'a resurrection must be caught the moment it is planted',
      );
      // Prose about the deleted token is not a call site.
      expect(
        _stripLineComments(
          '/// spell `fontSize: AppType.s14`, never raw',
        ).contains('AppType.s14'),
        isFalse,
        reason: 'documentation about the migration is not a call site',
      );
    });

    test('roles derive from the resolved theme geometry, not a ladder', () {
      // Behavioural proof of the authority separation: FORK the theme's
      // bodySmall geometry and watch `labelMicro` follow it. If the roles were
      // built from a fixed numeric ladder, the fork would be invisible and the
      // role would stay 12.
      final geometry = Typography.material2021().englishLike;
      final forkedTypography = Typography.material2021(
        englishLike: geometry.copyWith(
          bodySmall: geometry.bodySmall!.copyWith(fontSize: 13),
        ),
      );
      final forkedTheme = ThemeData(
        useMaterial3: true,
        fontFamily: 'Inter',
        typography: forkedTypography,
      );
      final roles = AppTypeRoles.fromResolved(resolvedTextTheme(forkedTheme));

      expect(
        roles.labelMicro.fontSize,
        13,
        reason:
            'labelMicro must follow the theme bodySmall geometry; a fixed 12 '
            'means a numeric ladder is still the source',
      );
      // The rest of the roles ride the theme steps unchanged.
      expect(roles.bodyDense.fontSize, 14);
      expect(roles.titleCompact.fontSize, 16);
      expect(roles.titleSection.fontSize, AppTypeRoles.sectionStep);
      expect(roles.titleProminent.fontSize, 24);

      // The real themes still resolve the five enshrined sizes (no regression).
      for (final theme in [AppTheme.lightTheme, AppTheme.darkTheme]) {
        final resolved = theme.extension<AppTypeRoles>()!;
        expect(
          _roles(resolved).map((s) => s.fontSize).toList(),
          _enshrinedSteps.values.toList(),
        );
      }
    });
  });
}
