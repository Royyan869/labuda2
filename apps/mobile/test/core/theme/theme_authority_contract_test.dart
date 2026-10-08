// Tahap 0 — Theme authority contract (FOUNDATION, not a screen).
//
// CANONICAL TRUTH: `AppTheme.lightTheme` / `AppTheme.darkTheme` is the single
// colour/type authority. Widgets read from `Theme.of(context)`; `AppColors`
// is the raw token store for `app_colors.dart`/`app_theme.dart` only.
//
// This file locks the foundation in two halves:
//  1. Positive proof: both themes expose complete, scheme-derived slots with
//     proper M3 tonal direction (light containers step darker off the
//     surface, dark containers step lighter).
//  2. Negative gate: a FULL `lib/` sweep for competing colour authority (no
//     palette binds, no raw Material colours, no raw hex, no local
//     isDark/brightness branches, no alias name for a colour the theme already
//     owns, no legacy Material role bypassing `colorScheme`). The per-scope
//     registry is gone — locking
//     UI one domain at a time left cleaned files unguarded and demanded a
//     manual entry at every closure. Only the authority island may hold colour.
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:labuda/core/core.dart';

import '../../support/theme_authority_gate.dart';

/// The 15 M3 2021 style names the ladder must pin.
const _m3StyleNames = <String>[
  'displayLarge',
  'displayMedium',
  'displaySmall',
  'headlineLarge',
  'headlineMedium',
  'headlineSmall',
  'titleLarge',
  'titleMedium',
  'titleSmall',
  'bodyLarge',
  'bodyMedium',
  'bodySmall',
  'labelLarge',
  'labelMedium',
  'labelSmall',
];

TextStyle? _pick(TextTheme t, String name) => switch (name) {
  'displayLarge' => t.displayLarge,
  'displayMedium' => t.displayMedium,
  'displaySmall' => t.displaySmall,
  'headlineLarge' => t.headlineLarge,
  'headlineMedium' => t.headlineMedium,
  'headlineSmall' => t.headlineSmall,
  'titleLarge' => t.titleLarge,
  'titleMedium' => t.titleMedium,
  'titleSmall' => t.titleSmall,
  'bodyLarge' => t.bodyLarge,
  'bodyMedium' => t.bodyMedium,
  'bodySmall' => t.bodySmall,
  'labelLarge' => t.labelLarge,
  'labelMedium' => t.labelMedium,
  _ => t.labelSmall,
};

/// WCAG relative-luminance contrast ratio between two colours.
double _contrastRatio(Color a, Color b) {
  final la = a.computeLuminance();
  final lb = b.computeLuminance();
  final hi = la > lb ? la : lb;
  final lo = la > lb ? lb : la;
  return (hi + 0.05) / (lo + 0.05);
}

void main() {
  group('theme foundation positive proof', () {
    test('both themes expose the status palette through the extension', () {
      // The four semantic tones have no ColorScheme role, so they ship as a
      // ThemeExtension — registered by BOTH themes, or dark mode silently
      // renders the light tones (the accessor's default).
      final light = AppTheme.lightTheme.extension<AppStatusColors>();
      final dark = AppTheme.darkTheme.extension<AppStatusColors>();
      expect(
        light,
        isNotNull,
        reason: 'lightTheme must register AppStatusColors',
      );
      expect(
        dark,
        isNotNull,
        reason: 'darkTheme must register AppStatusColors',
      );
      expect(light, AppStatusColors.light);
      expect(dark, AppStatusColors.dark);
      // Light stays pixel-identical to the tokens the app shipped with.
      expect(light!.success, AppColors.statusSuccess);
      expect(light.warning, AppColors.statusWarning);
      expect(light.error, AppColors.statusError);
      expect(light.info, AppColors.statusInfo);
      // The scheme role and the extension can never disagree on error.
      expect(dark!.error, AppTheme.darkTheme.colorScheme.error);
      expect(light.error, AppTheme.lightTheme.colorScheme.error);
    });

    test('dark status tones meet WCAG AA on the dark surface', () {
      // Measured failure before the retune: #DC2626 on #161B22 = 3.6:1 and
      // #0284C7 = 4.3:1, both below 4.5:1 for normal text.
      final dark = AppTheme.darkTheme;
      final status = dark.extension<AppStatusColors>()!;
      final surface = dark.colorScheme.surface;
      final tones = <String, Color>{
        'success': status.success,
        'warning': status.warning,
        'error': status.error,
        'info': status.info,
      };
      expect(
        status.error,
        isNot(AppColors.statusError),
        reason: 'error must be retuned',
      );
      tones.forEach((name, tone) {
        expect(
          _contrastRatio(tone, surface),
          greaterThanOrEqualTo(4.5),
          reason: 'dark $name tone must stay readable on the dark surface',
        );
      });
    });

    test('app.dart wires AppTheme as the single runtime authority', () {
      final source = File('lib/app.dart').readAsStringSync();
      // ONE authority: MaterialApp must resolve themes exclusively through
      // AppTheme's static ThemeData pair, never a factory/helper that could
      // silently resolve a different palette.
      expect(source.contains('theme: AppTheme.lightTheme'), isTrue);
      expect(source.contains('darkTheme: AppTheme.darkTheme'), isTrue);
      expect(source.contains('themeMode: themeMode'), isTrue);
    });

    test('both themes carry the canonical scheme roles', () {
      for (final theme in [AppTheme.lightTheme, AppTheme.darkTheme]) {
        final s = theme.colorScheme;
        // Identity roles the rest of the app already depends on — unchanged.
        expect(s.primary, AppColors.primaryRed);
        expect(s.onPrimary, AppColors.neutralWhite);
        // Foundation roles Tahap 0 introduced — must exist and differ from
        // the flat defaults (pure black/white outline, containers == surface).
        expect(s.onSurfaceVariant, isNot(s.onSurface));
        expect(s.outlineVariant, isNot(s.onSurface));
        expect(s.surfaceContainerHighest, isNot(s.surface));
        expect(s.surfaceContainerLow, isNot(isNull));
      }
    });

    test('tonal direction is proper M3 in both modes', () {
      final light = AppTheme.lightTheme.colorScheme;
      final dark = AppTheme.darkTheme.colorScheme;
      // Light containers step DARKER off the surface; dark steps LIGHTER.
      expect(
        light.surfaceContainerHighest.computeLuminance(),
        lessThan(light.surface.computeLuminance()),
      );
      expect(
        dark.surfaceContainerHighest.computeLuminance(),
        greaterThan(dark.surface.computeLuminance()),
      );
      // Lowest stays at/beyond the surface on each side.
      expect(
        light.surfaceContainerLowest.computeLuminance(),
        greaterThanOrEqualTo(light.surface.computeLuminance()),
      );
      expect(
        dark.surfaceContainerLowest.computeLuminance(),
        lessThanOrEqualTo(dark.surface.computeLuminance()),
      );
    });

    test('component themes are pinned to scheme roles (both modes)', () {
      for (final theme in [AppTheme.lightTheme, AppTheme.darkTheme]) {
        final s = theme.colorScheme;
        expect(theme.scaffoldBackgroundColor, s.surface);
        expect(theme.dialogTheme.backgroundColor, s.surfaceContainerHigh);
        expect(theme.bottomSheetTheme.backgroundColor, s.surfaceContainerLow);
        expect(theme.dividerTheme.color, s.outlineVariant);
        expect(theme.listTileTheme.iconColor, s.onSurfaceVariant);
        // There is NO global snackBarTheme: AppSnackBar owns the toast palette
        // from the status foundation, so a competing global palette is banned.
        expect(theme.snackBarTheme.backgroundColor, isNull);
        expect(theme.snackBarTheme.actionTextColor, isNull);
        // Component themes that used to bind raw palette tokens directly.
        expect(theme.appBarTheme.backgroundColor, s.surface);
        expect(theme.appBarTheme.foregroundColor, s.onSurface);
        expect(theme.cardTheme.color, s.surface);
        expect(
          theme.elevatedButtonTheme.style?.backgroundColor?.resolve(
            const <WidgetState>{},
          ),
          s.primary,
        );
        expect(
          theme.elevatedButtonTheme.style?.foregroundColor?.resolve(
            const <WidgetState>{},
          ),
          s.onPrimary,
        );
        expect(
          (theme.inputDecorationTheme.enabledBorder! as OutlineInputBorder)
              .borderSide
              .color,
          s.outlineVariant.withValues(alpha: 0.5),
        );
        expect(
          (theme.inputDecorationTheme.focusedBorder! as OutlineInputBorder)
              .borderSide
              .color,
          s.primary.withValues(alpha: 0.7),
        );
        // Owner-locked form-field fill: ordinary data-entry fields are FILLED
        // with `surfaceContainerHigh`. Pinned here so a call site cannot
        // re-own the fill and so removing it from the authority fails loud.
        expect(
          theme.inputDecorationTheme.filled,
          isTrue,
          reason: 'form fields are filled by the canonical authority',
        );
        expect(
          theme.inputDecorationTheme.fillColor,
          s.surfaceContainerHigh,
          reason: 'the form-field fill role is surfaceContainerHigh',
        );
        // Geometry stays locked to the canonical values while the fill moves.
        expect(
          (theme.inputDecorationTheme.border! as OutlineInputBorder)
              .borderRadius,
          AppShape.containerRadius,
        );
        expect(
          (theme.inputDecorationTheme.focusedBorder! as OutlineInputBorder)
              .borderSide
              .width,
          AppMetrics.focusedBorderWidth,
        );
        // Button family: one geometry, one ink per variant. The neutral
        // outlined pair (onSurface / outlineVariant) is the canonical
        // secondary — a site re-stating it is a competing authority.
        expect(
          theme.outlinedButtonTheme.style?.foregroundColor?.resolve(
            const <WidgetState>{},
          ),
          s.onSurface,
        );
        expect(
          theme.outlinedButtonTheme.style?.side
              ?.resolve(const <WidgetState>{})
              ?.color,
          s.outlineVariant,
        );
        expect(
          theme.textButtonTheme.style?.foregroundColor?.resolve(
            const <WidgetState>{},
          ),
          s.primary,
        );
        expect(
          theme.filledButtonTheme.style?.backgroundColor?.resolve(
            const <WidgetState>{},
          ),
          s.primary,
        );
        expect(
          theme.filledButtonTheme.style?.foregroundColor?.resolve(
            const <WidgetState>{},
          ),
          s.onPrimary,
        );
        // ALL four button families share the one canonical radius.
        for (final (name, style) in <(String, ButtonStyle?)>[
          ('elevated', theme.elevatedButtonTheme.style),
          ('outlined', theme.outlinedButtonTheme.style),
          ('text', theme.textButtonTheme.style),
          ('filled', theme.filledButtonTheme.style),
        ]) {
          final shape = style?.shape?.resolve(const <WidgetState>{});
          expect(
            shape,
            isA<RoundedRectangleBorder>(),
            reason: '$name button shape must be a rounded rectangle',
          );
          expect(
            (shape! as RoundedRectangleBorder).borderRadius,
            AppShape.buttonRadius,
            reason: '$name button must use the canonical button radius',
          );
        }
        // Text defaults resolve to the scheme surface ink in both modes.
        expect(theme.textTheme.bodyLarge?.color, s.onSurface);
        expect(theme.textTheme.labelLarge?.color, s.onSurface);
      }
    });

    test('AppTheme binds only the scheme pair and the status tones', () {
      // app_colors.dart owns the hex; app_theme.dart may only turn the scheme
      // pair and the status tones into ThemeData. Any other `AppColors.*` bind
      // here (neutral/gray/primary palette tokens) is a competing authority
      // that a per-mode retune can no longer reach.
      final source = File(
        'lib/core/src/theme/app_theme.dart',
      ).readAsStringSync();
      final allowed = RegExp(
        r'AppColors\.(lightColorScheme|darkColorScheme|status\w*|darkStatus\w*)'
        r'|^///',
      );
      final offenders = <String>[];
      for (final line in source.split('\n')) {
        final trimmed = line.trim();
        if (!trimmed.contains('AppColors.')) continue;
        if (allowed.hasMatch(trimmed)) continue;
        offenders.add(trimmed);
      }
      expect(
        offenders,
        isEmpty,
        reason:
            'AppTheme may only bind the scheme pair and status tones, found: '
            '$offenders',
      );
    });

    test('no ColorScheme role falls back to the Material baseline', () {
      // Every role below was previously UNDEFINED, so Flutter's baseline
      // palette leaked in (Material purple for tertiary, a lavender
      // inversePrimary, an unbranded inverseSurface under the snackbar).
      // Pinned per token: deleting the definition silently re-opens the leak.
      final light = AppTheme.lightTheme.colorScheme;
      final dark = AppTheme.darkTheme.colorScheme;

      expect(light.tertiary, AppColors.primaryPurple);
      expect(dark.tertiary, AppColors.primaryPurple);
      expect(light.onTertiary, AppColors.neutralWhite);
      expect(dark.onTertiary, AppColors.neutralWhite);

      expect(light.primaryContainer, AppColors.neutralGray200);
      expect(dark.primaryContainer, AppColors.darkGray700);
      expect(light.onPrimaryContainer, AppColors.neutralGray900);
      expect(dark.onPrimaryContainer, AppColors.neutralGray100);
      expect(light.secondaryContainer, AppColors.neutralGray100);
      expect(dark.secondaryContainer, AppColors.darkGray700);
      expect(light.tertiaryContainer, AppColors.neutralGray100);
      expect(dark.tertiaryContainer, AppColors.darkGray700);
      expect(light.errorContainer, AppColors.neutralGray200);
      expect(dark.errorContainer, AppColors.darkGray700);

      expect(light.inverseSurface, AppColors.neutralGray900);
      expect(dark.inverseSurface, AppColors.neutralGray100);
      expect(light.onInverseSurface, AppColors.neutralWhite);
      expect(dark.onInverseSurface, AppColors.neutralGray900);
      expect(light.inversePrimary, AppColors.neutralGray100);
      expect(dark.inversePrimary, AppColors.darkGray900);

      expect(light.scrim, AppColors.neutralBlack);
      expect(dark.scrim, AppColors.neutralBlack);
      expect(light.shadow, AppColors.neutralBlack);
      expect(dark.shadow, AppColors.neutralBlack);
      expect(light.surfaceTint, AppColors.primaryRed);
      expect(dark.surfaceTint, AppColors.primaryRed);
      expect(light.surfaceDim, AppColors.neutralGray200);
      expect(dark.surfaceDim, AppColors.darkGray900);
      expect(light.surfaceBright, AppColors.neutralWhite);
      expect(dark.surfaceBright, AppColors.darkGray700);

      // The snackbar renders on the inverse pair, so it must not inherit the
      // baseline's unbranded surface.
      expect(
        light.inverseSurface,
        isNot(const ColorScheme.light().inverseSurface),
      );
      expect(
        light.inversePrimary,
        isNot(const ColorScheme.light().inversePrimary),
      );
    });

    test('both schemes define the full M3 role surface in source', () {
      // Closure gate for the leak above: a role that is simply absent from the
      // definition block silently resolves to the Material baseline again.
      const required = <String>[
        'primary',
        'onPrimary',
        'primaryContainer',
        'onPrimaryContainer',
        'secondary',
        'onSecondary',
        'secondaryContainer',
        'onSecondaryContainer',
        'tertiary',
        'onTertiary',
        'tertiaryContainer',
        'onTertiaryContainer',
        'error',
        'onError',
        'errorContainer',
        'onErrorContainer',
        'surface',
        'onSurface',
        'onSurfaceVariant',
        'outline',
        'outlineVariant',
        'surfaceContainerLowest',
        'surfaceContainerLow',
        'surfaceContainer',
        'surfaceContainerHigh',
        'surfaceContainerHighest',
        'surfaceDim',
        'surfaceBright',
        'inverseSurface',
        'onInverseSurface',
        'inversePrimary',
        'scrim',
        'shadow',
        'surfaceTint',
        'brightness',
      ];
      final source = File(
        'lib/core/src/theme/app_colors.dart',
      ).readAsStringSync();
      for (final (name, block) in <(String, String)>[
        ('light', 'lightColorScheme = ColorScheme.light('),
        ('dark', 'darkColorScheme = ColorScheme.dark('),
      ]) {
        final start = source.indexOf(block);
        expect(start, greaterThan(-1), reason: '$name scheme block must exist');
        final body = source.substring(start, source.indexOf(');', start));
        final missing = required
            .where(
              (role) => !body.contains(RegExp('^\\s+$role:', multiLine: true)),
            )
            .toList();
        expect(missing, isEmpty, reason: '$name scheme must define $missing');
      }
    });

    test('both modes are produced by one ThemeData builder', () {
      final source = File(
        'lib/core/src/theme/app_theme.dart',
      ).readAsStringSync();
      // ONE builder: light and dark are the same code path with different
      // inputs, so a component theme added later cannot be applied to only
      // one mode (the drift this file previously had).
      //
      // The detector counts CONSTRUCTIONS (`\bThemeData(`, which excludes
      // `DialogThemeData(`/`CardThemeData(` substrings and prose such as
      // "plain-`ThemeData()` default") rather than the literal `return
      // ThemeData(`: the builder now registers its extensions after the theme
      // exists, so the type-role extension can derive from the theme's OWN
      // body style instead of restating the M3 metrics in this file. One
      // construction is still the whole rule.
      final code = source
          .split('\n')
          .where((line) => !line.trimLeft().startsWith('//'))
          .join('\n');
      expect(RegExp(r'\bThemeData\(').allMatches(code).length, 1);
      expect(
        code.contains('_build(AppColors.lightColorScheme'),
        isTrue,
        reason: 'the single construction must live in the shared _build',
      );
      expect(source.contains('_build(AppColors.lightColorScheme'), isTrue);
      expect(source.contains('_build(AppColors.darkColorScheme'), isTrue);
    });

    test('geometry comes from the shared shape/metrics scales', () {
      final theme = AppTheme.lightTheme;
      expect(
        (theme.cardTheme.shape! as RoundedRectangleBorder).borderRadius,
        AppShape.containerRadius,
      );
      expect(
        AppShape.containerRadius,
        const BorderRadius.all(Radius.circular(12)),
      );
      expect(AppShape.buttonRadius, const BorderRadius.all(Radius.circular(8)));
      expect(
        theme.inputDecorationTheme.contentPadding,
        AppMetrics.inputPadding,
      );
      expect(
        theme.elevatedButtonTheme.style?.padding?.resolve(
          const <WidgetState>{},
        ),
        AppMetrics.buttonPadding,
      );
    });

    test('composer fields share one pill factory (five consumers)', () {
      // ONE composer spec: r24 pill, surfaceContainerHigh fill,
      // inputPadding, borderless — chat, comment, share-to-chat, support,
      // mention field. The old reality was two mechanisms (Container-wrap
      // vs decoration-fill) that had already drifted in fill and padding.
      for (final theme in [AppTheme.lightTheme, AppTheme.darkTheme]) {
        final scheme = theme.colorScheme;
        final d = AppTheme.composerDecoration(scheme, hintText: 'x');
        expect(
          (d.border! as OutlineInputBorder).borderRadius,
          const BorderRadius.all(Radius.circular(AppShape.r24)),
        );
        expect((d.border! as OutlineInputBorder).borderSide, BorderSide.none);
        expect(d.filled, isTrue);
        expect(d.fillColor, scheme.surfaceContainerHigh);
        expect(d.contentPadding, AppMetrics.inputPadding);
        expect(d.counterText, '');
      }
      // Consumers consume the factory; none may restate the pill radius.
      final consumers = <String>[
        'lib/domains/chat/chat/presentation/widgets/chat_input_area.dart',
        'lib/domains/social/comment/presentation/widgets/comment_input_with_commerce_reference.dart',
        'lib/domains/system/support/presentation/screens/support_ticket_thread_screen.dart',
        'lib/shared/widgets/mentions/mention_text_field.dart',
      ];
      for (final path in consumers) {
        final src = File(path).readAsStringSync();
        expect(
          src.contains('composerDecoration('),
          isTrue,
          reason: '$path must consume the composer factory',
        );
        expect(
          src.contains('AppShape.r24'),
          isFalse,
          reason: '$path must not restate the pill radius',
        );
      }
    });

    test(
      'composer action row is canonical (send always visible, + on the right)',
      () {
        // ONE action row spec: [pill] [ComposerAddButton?] [ComposerSendButton].
        // The send glyph lives only in the shared widget — consumers may not
        // draw their own — and nothing may hide the send button while typing
        // (the old showSend conditional caused layout shift).
        final canonical = File(
          'lib/shared/widgets/composer_action_buttons.dart',
        ).readAsStringSync();
        expect(
          canonical.contains('Icons.send'),
          isTrue,
          reason: 'the shared widget owns the send glyph',
        );
        expect(canonical.contains('ComposerAddButton'), isTrue);

        const chat =
            'lib/domains/chat/chat/presentation/widgets/chat_input_area.dart';
        const comment =
            'lib/domains/social/comment/presentation/widgets/comment_input_with_commerce_reference.dart';
        const support =
            'lib/domains/system/support/presentation/screens/support_ticket_thread_screen.dart';

        for (final path in const [chat, comment, support]) {
          final src = File(path).readAsStringSync();
          expect(
            src.contains('ComposerSendButton('),
            isTrue,
            reason: '$path must use the canonical send button',
          );
          expect(
            src.contains('Icons.send'),
            isFalse,
            reason: '$path may not draw its own send glyph',
          );
        }

        // The attach `+` exists only where an attach flow exists.
        for (final path in const [chat, comment]) {
          expect(
            File(path).readAsStringSync().contains('ComposerAddButton('),
            isTrue,
            reason: '$path must expose the canonical attach button',
          );
        }
        for (final path in const [support]) {
          expect(
            File(path).readAsStringSync().contains('ComposerAddButton('),
            isFalse,
            reason: '$path has no attach flow — no `+` row slot',
          );
        }

        // Chat used to hide send until typing; that conditional is gone.
        final chatSrc = File(chat).readAsStringSync();
        expect(chatSrc.contains('showSend'), isFalse);
        expect(chatSrc.contains('_isTyping'), isFalse);
      },
    );

    test('elevation and density are explicit theme data', () {
      for (final theme in [AppTheme.lightTheme, AppTheme.darkTheme]) {
        expect(theme.appBarTheme.elevation, AppElevation.none);
        // Owner decision (card foundation): ordinary cards are flat. Depth
        // is opt-in per semantic surface via AppElevation — never inherited
        // from the cardTheme fallback.
        expect(theme.cardTheme.elevation, AppElevation.none);
        expect(theme.visualDensity, AppDensity.visualDensity);
        expect(theme.materialTapTargetSize, AppDensity.tapTargetSize);
      }
    });

    test('typography stays on the M3 ladder — the migration target', () {
      // The per-widget `fontSize` literals migrate onto `textTheme.*` names.
      // The ladder is therefore NOT forked: it must equal the M3 2021 scale
      // Flutter ships (Inter only changes the family), otherwise every migrated
      // site would shift pixels.
      //
      // RESOLVE, DO NOT READ THE RAW THEME. `ThemeData.textTheme` as
      // constructed carries only colour and family — the geometry (size,
      // weight, height, letter spacing) arrives from `englishLike` 2021 when
      // `Theme.of()` runs `ThemeData.localize(...)`. The pre-fix version of
      // this test compared `AppTheme.lightTheme.textTheme` against
      // `ThemeData(useMaterial3: true).textTheme`, BOTH raw, so all four metric
      // assertions passed as `null == null`: the gate could not fail on
      // anything but the family, i.e. it certified nothing about the ladder it
      // claims to pin. Both sides now resolve through the shared
      // [resolvedTextTheme] helper, and every metric is floored non-null
      // BEFORE it is compared — a comparison that measures nothing must fail
      // first, not pass.
      final reference = resolvedTextTheme(ThemeData(useMaterial3: true));

      for (final theme in [AppTheme.lightTheme, AppTheme.darkTheme]) {
        final ours = resolvedTextTheme(theme);
        for (final name in _m3StyleNames) {
          final ourStyle = _pick(ours, name);
          final theirStyle = _pick(reference, name);
          expect(ourStyle, isNotNull, reason: '$name must exist on the ladder');
          expect(
            theirStyle?.fontSize,
            isNotNull,
            reason: 'the M3 reference must carry a size to compare against',
          );
          expect(
            ourStyle!.fontSize,
            isNotNull,
            reason:
                '$name size must be measured — a null here means this gate is '
                'back to comparing nothing',
          );
          expect(ourStyle.fontWeight, isNotNull, reason: '$name weight');
          expect(ourStyle.height, isNotNull, reason: '$name height');
          expect(
            ourStyle.letterSpacing,
            isNotNull,
            reason: '$name letterSpacing',
          );
          expect(
            ourStyle.fontSize,
            theirStyle?.fontSize,
            reason: '$name fontSize',
          );
          expect(
            ourStyle.fontWeight,
            theirStyle?.fontWeight,
            reason: '$name weight',
          );
          expect(ourStyle.height, theirStyle?.height, reason: '$name height');
          expect(
            ourStyle.letterSpacing,
            theirStyle?.letterSpacing,
            reason: '$name letterSpacing',
          );
        }
        expect(theme.textTheme.bodyMedium?.fontFamily, 'Inter');
        expect(
          _pick(ours, 'bodyMedium')!.fontFamily,
          'Inter',
          reason: 'the family must survive the geometry merge',
        );
      }
    });

    test('the ladder comparison can actually fail (negative proof)', () {
      // A gate that cannot fail certifies nothing. Two proofs that this one can:
      //
      // (1) A FORKED ladder — one role moved off its M3 size — is caught by the
      //     exact comparison the gate runs.
      final reference = resolvedTextTheme(ThemeData(useMaterial3: true));
      final real = resolvedTextTheme(AppTheme.lightTheme);
      final forked = real.copyWith(
        labelSmall: real.labelSmall!.copyWith(fontSize: 13),
      );

      final mismatches = <String>[];
      for (final name in _m3StyleNames) {
        final ours = _pick(forked, name);
        final theirs = _pick(reference, name);
        if (ours?.fontSize != theirs?.fontSize)
          mismatches.add('$name fontSize');
        if (ours?.fontWeight != theirs?.fontWeight) {
          mismatches.add('$name weight');
        }
        if (ours?.height != theirs?.height) mismatches.add('$name height');
        if (ours?.letterSpacing != theirs?.letterSpacing) {
          mismatches.add('$name letterSpacing');
        }
      }
      expect(
        mismatches,
        contains('labelSmall fontSize'),
        reason:
            'a forked size must be reported — the gate must be able to fail',
      );
      expect(
        mismatches.length,
        1,
        reason: 'the probe touches exactly one role',
      );

      // (2) WHY the gate resolves: the raw ThemeData text theme carries no
      //     geometry at all, which is precisely what made the old comparison
      //     vacuous. If a future Flutter bakes geometry into ThemeData, this
      //     canary fails on purpose — resolution then becomes redundant and can
      //     be simplified, but the floors above must keep the ladder pinned.
      expect(
        AppTheme.lightTheme.textTheme.labelSmall?.fontSize,
        isNull,
        reason:
            'raw ThemeData text themes carry colour/family only — resolving '
            'them is what the ladder proof depends on',
      );
      expect(real.labelSmall?.fontSize, isNotNull);
      expect(real.labelSmall?.fontSize, 11);

      // (3) END-TO-END, through the route a fork would really take: a custom
      //     `typography` whose englishLike geometry moves a size. It flows
      //     through the same path the gate uses (ThemeData ->
      //     resolvedTextTheme -> compare) and must be rejected. The geometry
      //     merge wins for metrics, so a metric fork enters through
      //     `typography` (a `textTheme:` fork only reaches colour/family) —
      //     which is exactly the route this proof takes.
      final forkedTypography = Typography.material2021(
        englishLike: Typography.material2021().englishLike.copyWith(
          bodyMedium: Typography.material2021().englishLike.bodyMedium!
              .copyWith(fontSize: 15),
        ),
      );
      final forkedLadder = resolvedTextTheme(
        ThemeData(
          useMaterial3: true,
          fontFamily: 'Inter',
          typography: forkedTypography,
        ),
      );
      final forkedBody = _pick(forkedLadder, 'bodyMedium')!;
      expect(
        forkedBody.fontSize,
        15,
        reason: 'the fork must reach the gate unchanged',
      );
      expect(
        forkedBody.fontSize,
        isNot(_pick(reference, 'bodyMedium')!.fontSize),
        reason:
            'a ladder that moved off M3 must fail this comparison — the gate '
            'certifies only because nothing has moved',
      );
    });

    test('the second typography ladder stays deleted', () {
      // The purged second ladder was a PARALLEL scale (h1–h6,
      // body/label/caption, labelSmall 10, h4 20, h5 18, height 1.5) exported
      // from `core.dart` and read by 11 widgets: a second type authority that
      // could drift from `textTheme` in size, weight AND line height. Its
      // consumers now read `Theme.of(context).textTheme`, so the file is gone
      // and the revival route — an import/export of the path, or the class
      // name — is locked.
      //
      // The forbidden class name is spelled from fragments so THIS gate does
      // not name it in a way its own sweep would catch; it is never written in
      // prose here for the same reason.
      final ladderClass =
          'AppTypog'
          'raphy';
      final ladderPath =
          'app_'
          'typography.dart';
      expect(
        File('lib/core/src/theme/$ladderPath').existsSync(),
        isFalse,
        reason: 'the parallel typography ladder must stay deleted',
      );
      final namedAgain = <String>[];
      for (final path in themeAuthorityDartFiles()) {
        if (File(path).readAsStringSync().contains(ladderClass)) {
          namedAgain.add(path);
        }
      }
      for (final f
          in Directory('test')
              .listSync(recursive: true)
              .whereType<File>()
              .where((f) => f.path.endsWith('.dart'))) {
        if (f.readAsStringSync().contains(ladderClass)) {
          namedAgain.add(f.path.replaceAll(r'\', '/'));
        }
      }
      expect(
        namedAgain,
        isEmpty,
        reason:
            'type role must come from textTheme, found at:\n'
            '${namedAgain.join('\n')}',
      );
      expect(
        File('lib/core/core.dart').readAsStringSync().contains(ladderPath),
        isFalse,
        reason: 'the barrel must not re-export a deleted ladder',
      );
    });

    test('motion ladder pins the durations widgets used to spell inline', () {
      // Stage 2: every literal became a step with the SAME value, so these pins
      // are what makes that migration behaviour-preserving — editing a value
      // here is a deliberate change, not an accident.
      expect(AppMotion.none, Duration.zero);
      expect(AppMotion.quick, const Duration(milliseconds: 100));
      expect(AppMotion.brisk, const Duration(milliseconds: 150));
      expect(AppMotion.fast, const Duration(milliseconds: 200));
      expect(AppMotion.steady, const Duration(milliseconds: 250));
      expect(AppMotion.settled, const Duration(milliseconds: 300));
      expect(AppMotion.relaxed, const Duration(milliseconds: 400));
      expect(AppMotion.slow, const Duration(milliseconds: 500));
      expect(AppMotion.slower, const Duration(milliseconds: 600));
      expect(AppMotion.deliberate, const Duration(milliseconds: 800));
      expect(AppMotion.ambient, const Duration(milliseconds: 1200));
      expect(AppMotion.longest, const Duration(milliseconds: 1500));
      expect(AppMotion.curve, Curves.easeInOut);
    });

    test('shape ladder pins the radii widgets used to spell inline', () {
      // Stage 3: every inline radius became a step with the SAME value, so
      // these pins are what make that migration pixel-identical.
      expect(AppShape.r2, 2);
      expect(AppShape.r4, 4);
      expect(AppShape.r6, 6);
      expect(AppShape.r8, 8);
      expect(AppShape.r10, 10);
      expect(AppShape.r11, 11);
      expect(AppShape.r12, 12);
      expect(AppShape.r14, 14);
      expect(AppShape.r16, 16);
      expect(AppShape.r18, 18);
      expect(AppShape.r20, 20);
      expect(AppShape.r24, 24);
      expect(AppShape.r28, 28);
      expect(AppShape.pill, 999);
      // Derived radii hold no independent value: they read the ladder.
      expect(
        AppShape.containerRadius,
        const BorderRadius.all(Radius.circular(AppShape.r12)),
      );
      expect(
        AppShape.buttonRadius,
        const BorderRadius.all(Radius.circular(AppShape.r8)),
      );
    });

    test(
      'type and spacing ladders pin the values widgets used to spell inline',
      () {
        // FOUNDATION (owner decision 2026-10-02): no more one step per pixel a
        // screen once wanted. Five icon steps, seven spacing steps on a 4pt grid.
        // These pins ARE the ladder — retuning a step is a deliberate edit HERE,
        // never a local literal. (The five TYPE steps are the semantic roles and
        // are pinned by `type_role_migration_ratchet_test.dart`.)
        expect(AppIconSize.inlineGlyph, 16);
        expect(AppIconSize.action, 20);
        expect(AppIconSize.header, 24);
        expect(AppIconSize.emphasis, 32);
        expect(AppIconSize.display, 48);

        // Content/media ladder — role-named like the icon ladder, one literal
        // per value. These pins ARE the policy: a call site reads a name, never
        // a number, and the folds documented on [AppContentSize] land exactly
        // here.
        expect(AppContentSize.badge, 24);
        expect(AppContentSize.controlCompact, 36);
        expect(AppContentSize.control, 48);
        expect(AppContentSize.termLabel, 100);
        expect(AppContentSize.thumbnail, 112);
        expect(AppContentSize.panel, 120);
        expect(AppContentSize.mediaCard, 140);
        expect(AppContentSize.preview, 180);
        expect(AppContentSize.capture, 200);
        expect(AppContentSize.overlay, 250);
        expect(AppContentSize.actionWidth, 280);
        expect(AppContentSize.dialogWidth, 340);
        expect(AppContentSize.cropperCanvas, const Size(500, 600));

        expect(AppMetrics.p0, 0);
        expect(AppMetrics.p4, 4);
        expect(AppMetrics.p8, 8);
        expect(AppMetrics.p12, 12);
        expect(AppMetrics.p16, 16);
        expect(AppMetrics.p24, 24);
        expect(AppMetrics.p32, 32);
        expect(AppMetrics.p48, 48);
        // A layout role derived from the step, not a step of its own.
        expect(AppMetrics.fabClearance, 96);
        // Derived paddings hold no value of their own.
        expect(
          AppMetrics.buttonPadding,
          const EdgeInsets.symmetric(
            horizontal: AppMetrics.p24,
            vertical: AppMetrics.p12,
          ),
        );
        expect(
          AppMetrics.inputPadding,
          const EdgeInsets.symmetric(
            horizontal: AppMetrics.p16,
            vertical: AppMetrics.p12,
          ),
        );
      },
    );

    test('content size ladder holds one name per value', () {
      // The AppElevation doctrine, applied to the content ladder: two members
      // may never spell the same number — that is how a ladder becomes the
      // drift it was cut out to kill. (Cross-LADDER duplicates are fine:
      // the type role `titleProminent` (24), `AppMetrics.p24`,
      // `AppIconSize.header` and `AppContentSize.badge` are four different
      // concepts.)
      final contentValues = <double>[
        AppContentSize.badge,
        AppContentSize.controlCompact,
        AppContentSize.control,
        AppContentSize.termLabel,
        AppContentSize.thumbnail,
        AppContentSize.panel,
        AppContentSize.mediaCard,
        AppContentSize.preview,
        AppContentSize.capture,
        AppContentSize.overlay,
        AppContentSize.actionWidth,
        AppContentSize.dialogWidth,
      ];
      expect(
        contentValues.toSet().length,
        contentValues.length,
        reason: 'two content-size names share one value — merge them',
      );
    });

    test('system mode resolves through the theme, including OS chrome', () {
      // `ThemeMode.system` is a SELECTOR, not a third palette: it picks one of
      // the two ThemeData below. What it additionally needs is OS chrome whose
      // polarity follows the resolved mode — defined once as theme data so no
      // screen has to branch on brightness.
      final light = AppTheme.lightTheme;
      final dark = AppTheme.darkTheme;
      expect(
        light.appBarTheme.systemOverlayStyle?.statusBarIconBrightness,
        Brightness.dark,
      );
      expect(
        dark.appBarTheme.systemOverlayStyle?.statusBarIconBrightness,
        Brightness.light,
      );
      expect(
        light.appBarTheme.systemOverlayStyle?.systemNavigationBarColor,
        light.colorScheme.surface,
      );
      expect(
        dark.appBarTheme.systemOverlayStyle?.systemNavigationBarColor,
        dark.colorScheme.surface,
      );

      // The selector itself: default is `system`, and the choice survives a
      // restart by index — order of ThemeMode.values is system/light/dark.
      final provider = File(
        'lib/core/src/theme/theme_provider.dart',
      ).readAsStringSync();
      expect(
        provider.contains(
          'return const ThemeState(themeMode: ThemeMode.system)',
        ),
        isTrue,
        reason: 'system must stay the default',
      );
      expect(provider.contains('ThemeMode.values[themeIndex]'), isTrue);
      expect(ThemeMode.values.first, ThemeMode.system);
    });

    test('dark and light stay distinct authorities', () {
      final light = AppTheme.lightTheme.colorScheme;
      final dark = AppTheme.darkTheme.colorScheme;
      expect(dark.surface, isNot(light.surface));
      expect(dark.onSurface, isNot(light.onSurface));
      expect(
        dark.surfaceContainerHighest,
        isNot(light.surfaceContainerHighest),
      );
      expect(dark.outlineVariant, isNot(light.outlineVariant));
    });
  });

  group('lib-wide colour authority gate', () {
    test('authority island stays the pinned three files', () {
      // Growing this set is the only way to legitimise a competing palette,
      // so it is pinned: a deliberate edit with a reason, never an accident.
      expect(themeAuthorityFiles, hasLength(3));
      for (final path in themeAuthorityFiles) {
        expect(
          File(path).existsSync(),
          isTrue,
          reason: 'authority file missing: $path',
        );
      }
    });

    test('the sweep covers the whole app and cannot go vacuous', () {
      final scanned = themeAuthorityDartFiles();
      // A floor, not an exact count: the lock must always cover the real app
      // instead of passing because the sweep found nothing to read.
      expect(scanned.length, greaterThan(1000));
      expect(scanned, containsAll(themeAuthorityFiles));
    });

    test('pattern still fires on a resurrection and spares real tokens', () {
      // Negative proof: the gate is only worth its runtime if it actually
      // catches a palette coming back. Paired with the lines it must NOT
      // touch, so a future "fix" cannot neuter it from either side.
      for (final resurrection in [
        'color: AppColors.neutralWhite,',
        'color: AppColors.dark,',
        'backgroundColor: Colors.white,',
        'color: Color(0xFF123456),',
        'final isDark = brightness == Brightness.dark;',
        // Scope A: alias of a role the theme already owns.
        'color: AppColors.primary,',
        'color: AppColors.successGreen,',
        // Scope A: legacy Material role bypassing colorScheme.
        'color: Theme.of(context).primaryColor,',
        // Status tones: theme data now, not a widget-side token bind.
        'color: AppColors.statusError,',
        'color: AppColors.primaryGreen,',
        // Closed holes: hidden colour forms the older, weaker gate copy let
        // through, plus OS chrome, which is theme data now.
        'color: Colors.white70,',
        'color: Color.fromARGB(255, 1, 2, 3),',
        'color: scheme.primary.withOpacity(0.5),',
        'statusBarBrightness: Brightness.dark,',
        'Theme.of(context).platformBrightness == Brightness.light;',
        // Stage 1 (elevation): the ladder is the only source of a step.
        'elevation: 2,',
        // Stage 2 (motion): same rule for timing.
        'duration: const Duration(milliseconds: 300),',
        // Stage 3 (shape): same rule for corner radii.
        'borderRadius: BorderRadius.circular(12)',
        // Stage 4 & 5 (type size / spacing): same rule.
        'fontSize: 14,',
        'margin: const EdgeInsets.only(left: 8),',
        // The literal hiding inside a ternary / null-coalesce — the hole this
        // scope closed (7 sites migrated onto the semantic type roles).
        'fontSize: isTotal ? 18 : 14,',
        'fontSize: widget.style?.fontSize ?? 14,',
        'fontSize: isActive ? 10 : 8.5,',
        // Scheme-role token bound directly instead of through colorScheme.
        'color: AppColors.primaryPurple,',
        // The killed second category-colour map (chat_card's statusColors
        // switch) — CategoryConfig is the one authority.
        'SupportCategory.dispute => context.statusColors.error,',
        'SupportCategory.other || null =>\n          Theme.of(context).colorScheme.onSurfaceVariant,',
      ]) {
        expect(
          themeForbiddenColour.hasMatch(resurrection),
          isTrue,
          reason: 'gate would miss: $resurrection',
        );
      }
      for (final legitimate in [
        'color: scheme.onSurfaceVariant,',
        'color: scheme.primary,',
        'color: context.statusColors.success,',
        'color: context.statusColors.error,',
        'foregroundColor: AppColors.coinPrimary,',
        'backgroundColor: Colors.transparent,', // mode-independent, see above
        'elevation: AppElevation.none,',
        'duration: AppMotion.settled,',
        'borderRadius: BorderRadius.circular(AppShape.r12),',
        'fontSize: context.typeRoles.bodyDense.fontSize,',
        'fontSize: Theme.of(context).textTheme.bodyMedium?.fontSize,',
        'margin: const EdgeInsets.only(left: AppMetrics.p8),',
        // Computed proportional geometry is not a size decision.
        'fontSize: stepSize * 0.42,',
        'fontSize: handleSize,',
        // Brand/identity hues with no scheme role stay bindable.
        'color: AppColors.primaryYellow,',
        'color: AppColors.koiGold,',
        // Icon/label switches over the category enum are not colour maps.
        'SupportCategory.paymentIssue => Icons.payment,',
      ]) {
        expect(
          themeForbiddenColour.hasMatch(legitimate),
          isFalse,
          reason: 'gate would false-positive: $legitimate',
        );
      }
    });

    test('button geometry is theme-owned, never restated at the call site', () {
      // The canonical radius decision (container 12 / button 8) lives in
      // AppShape.buttonRadius and flows through the four button themes.
      // A call site that re-states a shape forks the geometry authority —
      // the exact split (r12 sites vs r8 sites) this scope killed.
      final violations = <String>[];
      final ctors = <String>[
        'ElevatedButton.styleFrom(',
        'OutlinedButton.styleFrom(',
        'TextButton.styleFrom(',
        'FilledButton.styleFrom(',
      ];
      for (final path in themeAuthorityDartFiles()) {
        if (themeAuthorityFiles.contains(path)) continue;
        final src = File(path).readAsStringSync();
        for (final ctor in ctors) {
          var from = 0;
          while (true) {
            final idx = src.indexOf(ctor, from);
            if (idx < 0) break;
            var depth = 0;
            var end = idx + ctor.length - 1;
            for (; end < src.length; end++) {
              final ch = src[end];
              if (ch == '(') depth++;
              if (ch == ')') {
                depth--;
                if (depth == 0) break;
              }
            }
            final block = src.substring(idx, end + 1);
            if (block.contains('BorderRadius') || block.contains('shape:')) {
              final line = src.substring(0, idx).split('\n').length + 1;
              violations.add('$path:$line');
            }
            from = idx + 1;
          }
        }
      }
      expect(
        violations,
        isEmpty,
        reason:
            'button shape must come from the theme, found at:\n'
            '${violations.join('\n')}',
      );
    });

    test('ink roles never become a surface (fill or background)', () {
      // The address and verification surfaces shipped a whole family of these:
      // scaffold + app bar + dialog + sheet painted with `onSurfaceVariant`
      // (title and back arrow the same grey as their own background), a form
      // sheet whose header ink equalled its slab, an input `fillColor`, a
      // dropdown menu surface, a notes box, a badge row, ink borders and ink
      // sheet handles. `inverseSurface`, `surfaceContainer*` and
      // `outlineVariant` are the canonical answers.
      final scan = inkAsFillScan();
      expect(
        scan.hosts,
        greaterThan(300),
        reason: 'anti-vacuum: the sweep must inspect real fill blocks',
      );
      expect(
        scan.violations,
        isEmpty,
        reason: 'ink used as a surface at:\n${scan.violations.join('\n')}',
      );
    });

    test('ink-as-fill detector fires on a resurrection and spares real roles', () {
      ({List<String> violations, int hosts}) scan(String src) =>
          inkAsFillViolationsIn(src, path: 'probe.dart');

      // Fires: a fill inside a decoration block…
      expect(
        scan(
          'Container(decoration: BoxDecoration(color: scheme.onSurfaceVariant))',
        ).violations,
        hasLength(1),
      );
      // …an input fill…
      expect(
        scan(
          'InputDecoration(fillColor: scheme.onSurfaceVariant, filled: true)',
        ).violations,
        hasLength(1),
      );
      // …and any widget/data object naming a background with an ink role.
      expect(
        scan('Scaffold(backgroundColor: scheme.onSurface, body: x)').violations,
        hasLength(1),
      );

      // Spares: the legitimate uses. An ink used as INK…
      expect(
        scan(
          'Text(style: TextStyle(color: scheme.onSurfaceVariant))',
        ).violations,
        isEmpty,
      );
      // …an ink tint with alpha (M3 state layers paint onSurface at 8–12%)…
      expect(
        scan(
          'BoxDecoration(color: scheme.onSurfaceVariant.withValues(alpha: 0.05))',
        ).violations,
        isEmpty,
      );
      // …a real surface role…
      expect(
        scan('BoxDecoration(color: scheme.surfaceContainerHigh)').violations,
        isEmpty,
      );
      // …and an on-media control over a camera preview (`onPrimary` is the
      // always-light ink).
      expect(
        scan(
          'BoxDecoration(color: scheme.onPrimary, shape: BoxShape.circle)',
        ).violations,
        isEmpty,
      );
    });

    test('no file outside the authority owns a colour decision', () {
      // ONE implementation of the rule (test/support/theme_authority_gate.dart):
      // a second copy of the pattern would be a second, lying truth.
      final violations = themeAuthorityViolations();
      expect(
        violations,
        isEmpty,
        reason:
            'competing colour authority outside '
            '${themeAuthorityFiles.length} pinned files:\n'
            '${violations.join('\n')}',
      );
    });
  });

  group('foundation zombie/alias gate', () {
    test('AppColors carries no backward-compat colour aliases', () {
      final source = File(
        'lib/core/src/theme/app_colors.dart',
      ).readAsStringSync();
      // Killed in Scope F: flat light/dark/neutral binds had no scheme
      // meaning and invited off-authority colour picks. Must not return.
      expect(source.contains('Color light ='), isFalse);
      expect(source.contains('Color dark ='), isFalse);
      expect(source.contains('Color neutral ='), isFalse);
      // Killed in Scope A: one colour, one name. Each was a second spelling of
      // a colour the theme/status palette already owns (status* / scheme role),
      // so every screen could pick its own word for the same semantic.
      for (final alias in [
        'Color primary =',
        'Color error =',
        'Color success =',
        'Color warning =',
        'Color successGreen',
        'Color warningYellow',
      ]) {
        expect(
          source.contains(alias),
          isFalse,
          reason: 'alias must stay dead: $alias',
        );
      }
    });

    test('ThemeState exposes no brightness-branch helpers', () {
      final source = File(
        'lib/core/src/theme/theme_provider.dart',
      ).readAsStringSync();
      // Zombie helpers purged in Scope F: zero callers — widgets read
      // Theme.of(context), never ThemeState brightness forks.
      expect(source.contains('isDarkMode'), isFalse);
      expect(source.contains('getCurrentBrightness'), isFalse);
    });

    test('no second ThemeData builder survives in the theme layer', () {
      final source = File(
        'lib/core/src/theme/theme_provider.dart',
      ).readAsStringSync();
      // Killed: ThemeHelper.getThemeData duplicated AppTheme's job by
      // building ThemeData from a Brightness fork — a second authority.
      expect(source.contains('ThemeHelper'), isFalse);
      expect(source.contains('getThemeData'), isFalse);
    });

    test('component factory authority stays deleted', () {
      // Killed: the factory resolved component palettes off a locked light
      // brightness, competing with Theme.of(context). Must not be recreated.
      expect(
        File('lib/shared/ui/factory/component_factory.dart').existsSync(),
        isFalse,
      );
      expect(Directory('lib/shared/ui/factory').existsSync(), isFalse);
    });
  });

  group('snackbar single-authority gate', () {
    test('context extension hosts no Snackbar surface', () {
      final source = File(
        'lib/core/src/utils/extensions/context_extensions.dart',
      ).readAsStringSync();
      // Killed twice over: the extension first forked error/success/info off a
      // raw `backgroundColor` argument, then survived as a delegating trio.
      // Both are gone — the canonical Snapbar widget is the ONLY surface.
      expect(source.contains('backgroundColor'), isFalse);
      expect(source.contains('showSnackBar'), isFalse);
      expect(source.contains('AppSnackBar'), isFalse);
    });

    test('no per-screen SnackBar re-decides its palette', () {
      // Killed: ~40 call sites passed `backgroundColor:` into a raw SnackBar,
      // a second authority beside AppSnackBar's type → colour map. Every
      // toast goes through AppSnackBar.show{Success,Error,Info,Warning}; a
      // bare SnackBar may still exist but must not paint itself.
      final violations = <String>[];
      for (final path in themeAuthorityDartFiles()) {
        if (path.endsWith('shared/widgets/app_snackbar.dart')) continue;
        final src = File(path).readAsStringSync();
        var from = 0;
        while (true) {
          final idx = src.indexOf('SnackBar(', from);
          if (idx < 0) break;
          // Only the bare Material widget counts: `ScaffoldMessenger…
          // showSnackBar(` and `AppSnackBar(` are not scoped here.
          final before = idx == 0 ? '' : src[idx - 1];
          final bare = !RegExp(r'[A-Za-z0-9_]').hasMatch(before);
          var depth = 0;
          var end = idx + 'SnackBar'.length;
          for (; end < src.length; end++) {
            final ch = src[end];
            if (ch == '(') depth++;
            if (ch == ')') {
              depth--;
              if (depth == 0) break;
            }
          }
          final block = src.substring(idx, end + 1);
          if (bare && block.contains('backgroundColor')) {
            final line = '\n'.allMatches(src.substring(0, idx)).length + 1;
            violations.add('$path:$line');
          }
          from = idx + 1;
        }
      }
      expect(
        violations,
        isEmpty,
        reason:
            'SnackBar painted outside the AppSnackBar authority:\n'
            '${violations.join('\n')}',
      );
    });
  });
}
