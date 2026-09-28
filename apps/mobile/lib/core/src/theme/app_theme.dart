import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'app_colors.dart';

/// Canonical status palette (success / warning / error / info).
///
/// These four semantic tones have NO M3 `ColorScheme` role, so they live here
/// as a [ThemeExtension] — the one place that may differ between light and
/// dark. Widgets read it as `context.statusColors.*`; binding the raw
/// `AppColors.status*` tokens from a widget is forbidden by
/// `theme_authority_contract_test`.
///
/// Light is pixel-identical to the old tokens. Dark retunes the tones (see
/// `AppColors.darkStatus*`): the previous light tokens failed WCAG AA on the
/// dark surface (error 3.6:1, info 4.3:1); the retuned ones pass (>= 4.5:1).
class AppStatusColors extends ThemeExtension<AppStatusColors> {
  const AppStatusColors({
    required this.success,
    required this.warning,
    required this.error,
    required this.info,
  });

  final Color success;
  final Color warning;
  final Color error;
  final Color info;

  static const AppStatusColors light = AppStatusColors(
    success: AppColors.statusSuccess,
    warning: AppColors.statusWarning,
    error: AppColors.statusError,
    info: AppColors.statusInfo,
  );

  static const AppStatusColors dark = AppStatusColors(
    success: AppColors.darkStatusSuccess,
    warning: AppColors.darkStatusWarning,
    error: AppColors.darkStatusError,
    info: AppColors.darkStatusInfo,
  );

  @override
  AppStatusColors copyWith({
    Color? success,
    Color? warning,
    Color? error,
    Color? info,
  }) {
    return AppStatusColors(
      success: success ?? this.success,
      warning: warning ?? this.warning,
      error: error ?? this.error,
      info: info ?? this.info,
    );
  }

  @override
  AppStatusColors lerp(AppStatusColors? other, double t) {
    if (other == null) return this;
    return AppStatusColors(
      success: Color.lerp(success, other.success, t)!,
      warning: Color.lerp(warning, other.warning, t)!,
      error: Color.lerp(error, other.error, t)!,
      info: Color.lerp(info, other.info, t)!,
    );
  }
}

/// One-line read of the status palette: `context.statusColors.error`.
///
/// The `??` is the plain-`ThemeData()` default (light) — never an in-app
/// fallback: both app themes register the extension and that wiring is pinned
/// by `theme_authority_contract_test`, so in-app dark mode cannot resolve to
/// light silently.
extension AppStatusColorsContext on BuildContext {
  AppStatusColors get statusColors =>
      Theme.of(this).extension<AppStatusColors>() ?? AppStatusColors.light;
}

/// Shape scale — the ONLY place corner radii live.
///
/// ONE NAME PER VALUE: `r12` is not also `container`, `r8` is not also
/// `button` — two names for one radius is exactly the duplication this file
/// exists to kill. Component themes (and any component theme added later) take
/// their radius from here, so a shape change is one edit instead of a hunt
/// through inline `circular(...)` literals — 856 of them existed before this
/// ladder.
class AppShape {
  AppShape._();

  static const double r2 = 2;
  static const double r4 = 4;
  static const double r6 = 6;
  static const double r8 = 8;
  static const double r10 = 10;
  static const double r11 = 11;
  static const double r12 = 12;
  static const double r14 = 14;
  static const double r16 = 16;
  static const double r18 = 18;
  static const double r20 = 20;
  static const double r24 = 24;
  static const double r28 = 28;

  /// Full round (pills, circular avatars) — a named step, never a magic 999.
  static const double pill = 999;

  /// Container surfaces (cards, dialogs, fields) share one radius.
  static const BorderRadius containerRadius = BorderRadius.all(
    Radius.circular(r12),
  );
  static const BorderRadius buttonRadius = BorderRadius.all(
    Radius.circular(r8),
  );
}

/// Type size scale — the ONLY place a numeric font size may live.
///
/// ONE NAME PER SIZE, exactly like [AppShape] and [AppElevation]: widgets
/// spell `fontSize: AppType.s14`, never a raw number, so a size can be
/// retuned in one place instead of across 1216 call sites. Targeting a
/// `Theme.of(context).textTheme` style name instead of a size is the next
/// rung — that changes pixels, so it is a deliberate follow-up, not this.
class AppType {
  AppType._();

  static const double s8 = 8;
  static const double s9 = 9;
  static const double s10 = 10;
  static const double s11 = 11;
  static const double s12 = 12;
  static const double s13 = 13;
  static const double s14 = 14;
  static const double s15 = 15;
  static const double s16 = 16;
  static const double s18 = 18;
  static const double s20 = 20;
  static const double s22 = 22;
  static const double s24 = 24;
  static const double s28 = 28;
  static const double s32 = 32;
  static const double s36 = 36;
}

/// Spacing scale for component themes — same doctrine as [AppShape]: geometry
/// is defined once, component themes consume it.
/// Spacing scale — the ONLY place an EdgeInsets numeric literal may live.
///
/// ONE NAME PER STEP, exactly like [AppShape], [AppType] and [AppElevation].
/// The paddings below derive from these steps so they hold no value of their
/// own.
class AppMetrics {
  AppMetrics._();

  static const double p0 = 0;
  static const double p1 = 1;
  // Half-step — the one 1.5 gutter that exists (star-rating stack).
  static const double p1_5 = 1.5;
  static const double p2 = 2;
  static const double p3 = 3;
  static const double p4 = 4;
  static const double p5 = 5;
  static const double p6 = 6;
  static const double p8 = 8;
  static const double p9 = 9;
  static const double p10 = 10;
  static const double p12 = 12;
  static const double p14 = 14;
  static const double p16 = 16;
  static const double p20 = 20;
  static const double p24 = 24;
  static const double p32 = 32;
  static const double p40 = 40;
  static const double p48 = 48;
  static const double p60 = 60;
  static const double p80 = 80;
  static const double p96 = 96;
  static const double p99 = 99;

  static const EdgeInsets buttonPadding = EdgeInsets.symmetric(
    horizontal: p24,
    vertical: p12,
  );
  static const EdgeInsets inputPadding = EdgeInsets.symmetric(
    horizontal: p16,
    vertical: p12,
  );

  /// Pressed/outlined field border width used by the focused state.
  static const double focusedBorderWidth = 1.5;
}

/// Elevation scale — the ONLY place elevation values live.
///
/// ONE NAME PER VALUE: `none` is not also `flat`, `card` is not also `raised`.
/// Two names for one number is the duplication this whole file exists to kill.
class AppElevation {
  AppElevation._();

  /// Flat chrome: app bars and outlined surfaces that sit ON the surface.
  static const double none = 0;

  /// Barely lifted (chips, subtle inline cards).
  static const double raised = 1;

  /// The card step — also the `cardTheme` default.
  static const double card = 2;

  /// Toasts.
  static const double snackBar = 6;

  /// Overlay surfaces: banners, bottom navigation, popup menus, mention cards.
  static const double overlay = 8;
}

/// Density / hit-target policy — explicit instead of implicit per platform.
class AppDensity {
  AppDensity._();

  /// Touch-first product: keep the standard density but never shrink tap
  /// targets, even on desktop builds where Flutter would default to
  /// `shrinkWrap`.
  static const VisualDensity visualDensity = VisualDensity.standard;
  static const MaterialTapTargetSize tapTargetSize =
      MaterialTapTargetSize.padded;
}

/// Interaction timing — the ONE ladder for animations AND waits (debounce,
/// hold, redirect delay).
///
/// ONE NAME PER VALUE: two names for the same duration is the duplication this
/// file exists to kill. The ladder also stops the drift where the same intent
/// was spelled 300/250/200 in different screens.
///
/// The page-transition *character* is deliberately NOT overridden (which
/// builder, what duration) — that is a UX decision, not a foundation default.
class AppMotion {
  AppMotion._();

  static const Duration none = Duration.zero;
  static const Duration quick = Duration(milliseconds: 100);
  static const Duration brisk = Duration(milliseconds: 150);
  static const Duration fast = Duration(milliseconds: 200);
  static const Duration steady = Duration(milliseconds: 250);
  static const Duration settled = Duration(milliseconds: 300);
  static const Duration relaxed = Duration(milliseconds: 400);
  static const Duration slow = Duration(milliseconds: 500);
  static const Duration slower = Duration(milliseconds: 600);
  static const Duration deliberate = Duration(milliseconds: 800);
  static const Duration ambient = Duration(milliseconds: 1200);
  static const Duration longest = Duration(milliseconds: 1500);

  static const Curve curve = Curves.easeInOut;
}

/// THEME AUTHORITY.
///
/// `AppTheme` is the single colour/type/geometry authority. Widgets must read
/// from `Theme.of(context)` (`colorScheme`, `textTheme`, `context.statusColors`,
/// component themes) and must NOT branch on `isDark`/`brightness`, bind
/// `AppColors` directly, or hardcode raw Material/hex colours.
///
/// STRUCTURE: `lightTheme` and `darkTheme` are produced by ONE builder
/// ([_build]) that takes only a [ColorScheme] and an [AppStatusColors]. Light
/// and dark are therefore the same code path with different inputs — they
/// cannot drift apart, and a new component theme is added in exactly one
/// place. `AppColors` is the raw token store for `app_colors.dart` only.
///
/// Canonical surface mapping (both modes):
/// screen → `surface`; cards → `surface`; subtle fills →
/// `surfaceContainerHighest`; borders/dividers → `outlineVariant`;
/// secondary text/icons → `onSurfaceVariant`; status → `context.statusColors`.
///
/// TYPOGRAPHY is defined as "the M3 2021 ladder + Inter family" and pinned by
/// contract test, so widgets can migrate their per-widget `fontSize` literals
/// onto these names without any pixel drift. The canonical role mapping:
/// screen title → `titleLarge`; section header → `titleMedium`; card title →
/// `titleSmall`; body → `bodyMedium`; dense/meta text → `bodySmall`; button and
/// chip label → `labelLarge`; caption/badge → `labelSmall`; money/emphasis →
/// `titleMedium` with `FontWeight.w700`. Forking the ladder (a fourth size,
/// letter-spacing tweak, etc.) is a deliberate decision that must land HERE,
/// not in a widget.
class AppTheme {
  AppTheme._();

  /// Light mode. Same builder, light scheme + light status tones.
  static ThemeData get lightTheme =>
      _build(AppColors.lightColorScheme, AppStatusColors.light);

  /// Dark mode. Same builder, dark scheme + dark status tones.
  static ThemeData get darkTheme =>
      _build(AppColors.darkColorScheme, AppStatusColors.dark);

  /// OS chrome for the resolved mode.
  ///
  /// This is the ONLY place allowed to read brightness: the platform needs an
  /// explicit icon polarity, and the contract gate exempts
  /// `statusBarIconBrightness` for exactly this reason. Because it is theme
  /// data, `ThemeMode.system` gets the right status/navigation bar without any
  /// screen branching on brightness.
  static SystemUiOverlayStyle _overlayStyle(ColorScheme scheme) {
    final isDark = scheme.brightness == Brightness.dark;
    return SystemUiOverlayStyle(
      statusBarColor: Colors.transparent,
      statusBarIconBrightness: isDark ? Brightness.light : Brightness.dark,
      systemNavigationBarColor: scheme.surface,
      systemNavigationBarIconBrightness:
          isDark ? Brightness.light : Brightness.dark,
    );
  }

  /// OS chrome for full-bleed dark-room surfaces (crop editor, immersive
  /// viewers): light icons on black, in BOTH modes, because the surface behind
  /// them is black regardless of theme. Owned here so no widget holds its own
  /// overlay style (the gate rejects `Brightness.*` outside this file).
  static const SystemUiOverlayStyle immersiveOverlayStyle =
      SystemUiOverlayStyle(
        statusBarColor: Colors.transparent,
        statusBarIconBrightness: Brightness.light,
        statusBarBrightness: Brightness.dark,
        systemNavigationBarColor: Colors.transparent,
        systemNavigationBarIconBrightness: Brightness.light,
      );

  /// THE one ThemeData builder. Both modes flow through here, so any component
  /// theme or scale defined below is automatically correct in light and dark.
  static ThemeData _build(ColorScheme scheme, AppStatusColors status) {
    return ThemeData(
      useMaterial3: true,
      colorScheme: scheme,
      fontFamily: 'Inter',
      visualDensity: AppDensity.visualDensity,
      materialTapTargetSize: AppDensity.tapTargetSize,
      scaffoldBackgroundColor: scheme.surface,
      extensions: <ThemeExtension<dynamic>>[status],

      // AppBar theme
      appBarTheme: AppBarTheme(
        backgroundColor: scheme.surface,
        foregroundColor: scheme.onSurface,
        elevation: AppElevation.none,
        centerTitle: true,
        systemOverlayStyle: _overlayStyle(scheme),
      ),

      // Dialog / bottom sheet / divider / list tile / snackbar follow the
      // scheme roles — pinned here so the authority is explicit and locked by
      // contract test.
      dialogTheme: DialogThemeData(
        backgroundColor: scheme.surfaceContainerHigh,
      ),
      bottomSheetTheme: BottomSheetThemeData(
        backgroundColor: scheme.surfaceContainerLow,
      ),
      dividerTheme: DividerThemeData(color: scheme.outlineVariant),
      listTileTheme: ListTileThemeData(iconColor: scheme.onSurfaceVariant),
      snackBarTheme: SnackBarThemeData(
        backgroundColor: scheme.inverseSurface,
        actionTextColor: scheme.inversePrimary,
      ),

      // Card theme
      cardTheme: CardThemeData(
        color: scheme.surface,
        elevation: AppElevation.card,
        shape: const RoundedRectangleBorder(
          borderRadius: AppShape.containerRadius,
        ),
      ),

      // Elevated button theme
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: scheme.primary,
          foregroundColor: scheme.onPrimary,
          shape: const RoundedRectangleBorder(
            borderRadius: AppShape.buttonRadius,
          ),
          padding: AppMetrics.buttonPadding,
        ),
      ),

      // Input decoration theme
      inputDecorationTheme: InputDecorationTheme(
        border: OutlineInputBorder(
          borderRadius: AppShape.containerRadius,
          borderSide: BorderSide(
            color: scheme.outlineVariant.withValues(alpha: 0.5),
          ),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: AppShape.containerRadius,
          borderSide: BorderSide(
            color: scheme.outlineVariant.withValues(alpha: 0.5),
          ),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: AppShape.containerRadius,
          borderSide: BorderSide(
            color: scheme.primary.withValues(alpha: 0.7),
            width: AppMetrics.focusedBorderWidth,
          ),
        ),
        contentPadding: AppMetrics.inputPadding,
      ),
    );
  }
}
