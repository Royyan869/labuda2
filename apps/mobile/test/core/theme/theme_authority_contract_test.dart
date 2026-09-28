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
      expect(light, isNotNull, reason: 'lightTheme must register AppStatusColors');
      expect(dark, isNotNull, reason: 'darkTheme must register AppStatusColors');
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
      expect(status.error, isNot(AppColors.statusError), reason: 'error must be retuned');
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
        expect(
          theme.bottomSheetTheme.backgroundColor,
          s.surfaceContainerLow,
        );
        expect(theme.dividerTheme.color, s.outlineVariant);
        expect(theme.listTileTheme.iconColor, s.onSurfaceVariant);
        expect(theme.snackBarTheme.backgroundColor, s.inverseSurface);
        expect(theme.snackBarTheme.actionTextColor, s.inversePrimary);
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
      expect(light.inverseSurface, isNot(const ColorScheme.light().inverseSurface));
      expect(light.inversePrimary, isNot(const ColorScheme.light().inversePrimary));
    });

    test('both schemes define the full M3 role surface in source', () {
      // Closure gate for the leak above: a role that is simply absent from the
      // definition block silently resolves to the Material baseline again.
      const required = <String>[
        'primary', 'onPrimary', 'primaryContainer', 'onPrimaryContainer',
        'secondary', 'onSecondary', 'secondaryContainer',
        'onSecondaryContainer', 'tertiary', 'onTertiary', 'tertiaryContainer',
        'onTertiaryContainer', 'error', 'onError', 'errorContainer',
        'onErrorContainer', 'surface', 'onSurface', 'onSurfaceVariant',
        'outline', 'outlineVariant', 'surfaceContainerLowest',
        'surfaceContainerLow', 'surfaceContainer', 'surfaceContainerHigh',
        'surfaceContainerHighest', 'surfaceDim', 'surfaceBright',
        'inverseSurface', 'onInverseSurface', 'inversePrimary', 'scrim',
        'shadow', 'surfaceTint', 'brightness',
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
            .where((role) => !body.contains(RegExp('^\\s+$role:', multiLine: true)))
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
      expect(RegExp(r'return ThemeData\(').allMatches(source).length, 1);
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
      expect(theme.inputDecorationTheme.contentPadding, AppMetrics.inputPadding);
      expect(
        theme.elevatedButtonTheme.style?.padding?.resolve(
          const <WidgetState>{},
        ),
        AppMetrics.buttonPadding,
      );
    });

    test('elevation and density are explicit theme data', () {
      for (final theme in [AppTheme.lightTheme, AppTheme.darkTheme]) {
        expect(theme.appBarTheme.elevation, AppElevation.none);
        expect(theme.cardTheme.elevation, AppElevation.card);
        expect(theme.visualDensity, AppDensity.visualDensity);
        expect(theme.materialTapTargetSize, AppDensity.tapTargetSize);
      }
    });

    test('typography stays on the M3 ladder — the migration target', () {
      // The 1236 per-widget `fontSize` literals migrate onto `textTheme.*`
      // names. The ladder is therefore NOT forked: it must equal the M3 2021
      // scale Flutter ships (Inter only changes the family), otherwise every
      // migrated site would shift pixels.
      final reference = ThemeData(useMaterial3: true).textTheme;
      const styleNames = <String>[
        'displayLarge', 'displayMedium', 'displaySmall',
        'headlineLarge', 'headlineMedium', 'headlineSmall',
        'titleLarge', 'titleMedium', 'titleSmall',
        'bodyLarge', 'bodyMedium', 'bodySmall',
        'labelLarge', 'labelMedium', 'labelSmall',
      ];

      TextStyle? pick(TextTheme t, String name) => switch (name) {
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

      for (final theme in [AppTheme.lightTheme, AppTheme.darkTheme]) {
        for (final name in styleNames) {
          final ours = pick(theme.textTheme, name);
          final theirs = pick(reference, name);
          expect(ours?.fontSize, theirs?.fontSize, reason: '$name fontSize');
          expect(ours?.fontWeight, theirs?.fontWeight, reason: '$name weight');
          expect(ours?.height, theirs?.height, reason: '$name height');
          expect(
            ours?.letterSpacing,
            theirs?.letterSpacing,
            reason: '$name letterSpacing',
          );
        }
        expect(theme.textTheme.bodyMedium?.fontFamily, 'Inter');
      }
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

    test('type and spacing ladders pin the values widgets used to spell inline', () {
      // Stages 4 & 5: every inline size/padding became a step with the SAME
      // value, so these pins are what make those migrations pixel-identical.
      expect(AppType.s8, 8);
      expect(AppType.s9, 9);
      expect(AppType.s10, 10);
      expect(AppType.s11, 11);
      expect(AppType.s12, 12);
      expect(AppType.s13, 13);
      expect(AppType.s14, 14);
      expect(AppType.s15, 15);
      expect(AppType.s16, 16);
      expect(AppType.s18, 18);
      expect(AppType.s20, 20);
      expect(AppType.s22, 22);
      expect(AppType.s24, 24);
      expect(AppType.s28, 28);
      expect(AppType.s32, 32);
      expect(AppType.s36, 36);

      expect(AppMetrics.p0, 0);
      expect(AppMetrics.p1, 1);
      expect(AppMetrics.p1_5, 1.5);
      expect(AppMetrics.p2, 2);
      expect(AppMetrics.p3, 3);
      expect(AppMetrics.p4, 4);
      expect(AppMetrics.p5, 5);
      expect(AppMetrics.p6, 6);
      expect(AppMetrics.p8, 8);
      expect(AppMetrics.p9, 9);
      expect(AppMetrics.p10, 10);
      expect(AppMetrics.p12, 12);
      expect(AppMetrics.p14, 14);
      expect(AppMetrics.p16, 16);
      expect(AppMetrics.p20, 20);
      expect(AppMetrics.p24, 24);
      expect(AppMetrics.p32, 32);
      expect(AppMetrics.p40, 40);
      expect(AppMetrics.p48, 48);
      expect(AppMetrics.p60, 60);
      expect(AppMetrics.p80, 80);
      expect(AppMetrics.p96, 96);
      expect(AppMetrics.p99, 99);
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
        provider.contains('return const ThemeState(themeMode: ThemeMode.system)'),
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
      expect(dark.surfaceContainerHighest, isNot(light.surfaceContainerHighest));
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
        'elevation: AppElevation.card,',
        'duration: AppMotion.settled,',
        'borderRadius: BorderRadius.circular(AppShape.r12),',
        'fontSize: AppType.s14,',
        'margin: const EdgeInsets.only(left: AppMetrics.p8),',
      ]) {
        expect(
          themeForbiddenColour.hasMatch(legitimate),
          isFalse,
          reason: 'gate would false-positive: $legitimate',
        );
      }
    });

    test('no file outside the authority owns a colour decision', () {
      // ONE implementation of the rule (test/support/theme_authority_gate.dart):
      // a second copy of the pattern would be a second, lying truth.
      final violations = themeAuthorityViolations();
      expect(
        violations,
        isEmpty,
        reason: 'competing colour authority outside '
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
      expect(
        Directory('lib/shared/ui/factory').existsSync(),
        isFalse,
      );
    });

    test('base_component owns no colour/size authority', () {
      final source = File(
        'lib/shared/ui/base/base_component.dart',
      ).readAsStringSync();
      // Killed: ComponentSize/ComponentSpacing(value) duplicated the theme's
      // layout scale and had zero consumers.
      expect(source.contains('ComponentSize'), isFalse);
      expect(source.contains('ComponentSpacing'), isFalse);
    });
  });

  group('snackbar single-authority gate', () {
    test('showSnackBar extension owns no colour decision', () {
      final source = File(
        'lib/core/src/utils/extensions/context_extensions.dart',
      ).readAsStringSync();
      // Killed: the extension forked error/success/info off a raw
      // `backgroundColor` argument — the same decision AppSnackBar already
      // owns. It now delegates with no colour parameter at all.
      expect(source.contains('backgroundColor'), isFalse);
      expect(source.contains('AppSnackBar.showInfo'), isTrue);
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
        reason: 'SnackBar painted outside the AppSnackBar authority:\n'
            '${violations.join('\n')}',
      );
    });
  });
}
