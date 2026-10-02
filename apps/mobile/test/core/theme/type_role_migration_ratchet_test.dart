/// TYPOGRAPHY MIGRATION RATCHET.
///
/// `AppType.s*` is the LAST parallel type authority: a size token that the
/// theme's own documentation says a ROLE must replace
/// (`Theme.of(context).textTheme.bodySmall`). The migration is staged — see
/// `TIPOGRAFI_MIGRATION_PLAN.md` — and this gate makes it one-way:
///
/// 1. no token may grow past the count frozen here, so a new
///    `fontSize: AppType.s14` cannot join the backlog;
/// 2. every slice that finishes must be locked to zero BY PATH, so a migrated
///    file cannot quietly regain a size token;
/// 3. the census has floors, so passing because it read nothing is impossible.
library;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';

import '../../support/theme_authority_gate.dart';
import '../../support/type_role_migration_gate.dart';

/// Frozen counts, measured by [typeReferenceCensus] on 2026-10-02 — the day the
/// foundation pass folded seventeen sizes onto five. That pass is the only
/// reason a cap may be re-based, and it was a DELIBERATE act: from here the
/// numbers may only go DOWN, and lowering one is the reward for finishing a
/// slice. A retired token has no cap because `AppType` no longer declares it.
const _frozenCaps = <String, int>{
  's12': 387,
  's14': 403,
  's16': 205,
  's20': 97,
  's24': 25,
};

/// Slices that are DONE. Each path must read zero tokens forever; the lock is
/// by path (not by token) because a finished file is the unit of work.
const _slicesLockedToZero = <String>[
  // Tahap 2, irisan 1: the reference card's type caption is `labelSmall` now.
  'lib/shared/object/presentation/widgets/object_preview_card.dart',
  // Tahap 2, irisan 2: the marketplace tab labels are `labelLarge` now.
  'lib/features/marketplace/presentation/screens/marketplace_screen.dart',
  // Tahap 2, irisan 3: the router error page reads headline/body/button roles.
  'lib/core/src/router/router_error_page.dart',
];

/// The FIVE foundation steps (owner decision 2026-10-02), role by role. The
/// extension first legalised off-ladder sizes (10, 13, 15, 18, 20); the
/// foundation pass folded those onto these five, so a role now names a step of
/// `AppType` instead of a size the ladder does not have.
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
TextStyle _resolvedBody(ThemeData theme) => resolvedTextTheme(theme).bodyMedium!;

void main() {
  group('the extended type ladder', () {
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
          expect(style.fontWeight, body.fontWeight, reason: '${style.fontSize}');
          expect(style.height, body.height, reason: '${style.fontSize}');
          expect(
            style.letterSpacing,
            body.letterSpacing,
            reason: '${style.fontSize}',
          );
          expect(style.fontFamily, body.fontFamily, reason: '${style.fontSize}');
        }
      }
    });

    test('the roles ARE the five foundation steps, one name per value', () {
      for (final theme in [AppTheme.lightTheme, AppTheme.darkTheme]) {
        final sizes = _roles(
          theme.extension<AppTypeRoles>()!,
        ).map((style) => style.fontSize).toSet();
        expect(
          sizes,
          _enshrinedSteps.values.toSet(),
          reason:
              'a role that names a step the ladder does not have is a second '
              'type authority',
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
      expect(body.fontSize, isNotNull, reason: 'englishLike carries the geometry');
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

  test('the census reads the real app and cannot go vacuous', () {
    final census = typeReferenceCensus();
    expect(
      typeCensusFiles().length,
      greaterThan(typeCensusFileFloor),
      reason: 'the typography census must sweep the whole app',
    );
    expect(
      typeReferenceTotal(census),
      greaterThan(typeCensusReferenceFloor),
      reason:
          'the census found almost no size tokens — it is reading the wrong '
          'tree or stripping too much, not observing a finished migration',
    );
  });

  test('no size token grows past its frozen cap', () {
    final census = typeReferenceCensus();
    final offenders = <String>[];
    for (final entry in census.entries) {
      final cap = _frozenCaps[entry.key];
      if (cap == null) {
        offenders.add('AppType.${entry.key} is a NEW size token (${entry.value})');
        continue;
      }
      if (entry.value > cap) {
        offenders.add('AppType.${entry.key}: ${entry.value} > cap $cap');
      }
    }
    expect(
      offenders,
      isEmpty,
      reason:
          'the AppType backlog grew. A size token is a SECOND type authority — '
          'the theme owns type through `textTheme`. Migrate the call site onto '
          'a role (`AppTheme` documents the mapping) or lower the cap in the '
          'same commit as the migration:\n'
          'census: ${_sorted(census)}\n${offenders.join('\n')}',
    );
  });

  test('finished slices stay at zero', () {
    final offenders = <String>[];
    for (final path in _slicesLockedToZero) {
      final census = typeReferencesInFile(path);
      if (census.isNotEmpty) {
        offenders.add('$path regained ${_sorted(census)}');
      }
    }
    expect(
      offenders,
      isEmpty,
      reason:
          'a migrated slice grew a size token back. Finished files read '
          '`Theme.of(context).textTheme.*`: the ladder is a ROLE, not a size:'
          '\n${offenders.join('\n')}',
    );
  });

  test('the census counts inline sizes and spares prose (negative proof)', () {
    // The direct spelling and the conditional spelling are both call sites.
    expect(typeReference.hasMatch('fontSize: AppType.s14'), isTrue);
    expect(
      typeReference.allMatches('fontSize: isActive ? AppType.s12 : AppType.s24'),
      hasLength(2),
      reason: 'an inline size decision is a call site too',
    );
    // Prose and the token declaration are not call sites.
    expect(typeReference.hasMatch('widgets spell fontSize: 14, never a raw'), isFalse);
    expect(
      stripLineComments('/// spell `fontSize: AppType.s14`, never a raw number')
          .contains('AppType.s14'),
      isFalse,
      reason: 'documentation about the migration is not a call site',
    );
    // The declaring file OWNS the tokens and reads its own ladder (the role
    // factory derives from it instead of restating numbers). It is never a call
    // site — the census skips it, which is why the caps above never see these
    // references.
    final ownerRefs = typeReferencesInFile(typeTokenOwnerFile);
    expect(
      ownerRefs.keys.toList()..sort(),
      _frozenCaps.keys.toList()..sort(),
      reason: 'the owner file may only reference the ladder it declares',
    );
  });
}

String _sorted(Map<String, int> census) {
  final keys = census.keys.toList()..sort();
  return '{${keys.map((key) => '$key: ${census[key]}').join(', ')}}';
}
