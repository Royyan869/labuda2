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
/// FOUNDATION (owner decision 2026-10-02): FIVE steps. The old ladder had grown
/// one token per pixel a screen happened to want — 8, 8.5, 9, 10, 11, 12, 13,
/// 14, 15, 16, 18, 20, 22, 24, 28, 32, 36 — and the usage census showed that
/// was drift, not design: four sizes carried 77% of the app, the tail appeared
/// 1–4 times, and one token had gone to zero everywhere. The scale is now the
/// industry-standard core (Tailwind/Material): caption 12, body 14, title 16,
/// section 20, display 24.
///
/// MAPPING of the retired tokens (nearest step, ties rounded UP, clamped at the
/// ends): 8 · 8.5 · 9 · 10 · 11 → s12; 13 → s14; 15 → s16; 18 → s20;
/// 22 · 28 · 32 · 36 → s24. A ROLE beats a size: [AppTypeRoles] exposes these
/// five steps, and `Theme.of(context).textTheme.*` (mapping documented on
/// [AppTheme]) stays the generic authority.
class AppType {
  AppType._();

  /// Caption, meta and badge text — the smallest step.
  static const double s12 = 12;

  /// Body and helper text — the default.
  static const double s14 = 14;

  /// Emphasis: item titles and body-large.
  static const double s16 = 16;

  /// Section header inside a screen.
  static const double s20 = 20;

  /// Display: hero numbers and prominent state titles.
  static const double s24 = 24;
}

/// Icon size ladder — the ONLY place an icon size may live.
///
/// FOUNDATION (owner decision 2026-10-02): FIVE steps, the range large apps
/// use — inline 16, action 20, header 24, emphasis 32, display 48. The raw
/// `size:` literals still in the app (counted per file by the geometry census,
/// category `iconSize`) migrate onto these; nothing else may spell a number.
class AppIconSize {
  AppIconSize._();

  /// Inline glyph beside body text.
  static const double inlineGlyph = 16;

  /// Default action icon (app bars, buttons).
  static const double action = 20;

  /// Header and navigation icon.
  static const double header = 24;

  /// Emphasised icon inside a card or tile.
  static const double emphasis = 32;

  /// Display / empty-state icon.
  static const double display = 48;
}

/// Content and media size ladder — the ONLY place a content extent may live.
///
/// FOUNDATION (owner decision 2026-10-02): the SIZE OF something is a
/// different decision from a gap between things ([AppMetrics]) and from an
/// icon ([AppIconSize]) — a button's height, a label column, a thumbnail, a
/// preview surface cannot be named by the spacing ladder, so each gets a name
/// of its own here. Like [AppIconSize] the steps are ROLE-named, never
/// value-named: the old type ladder grew one token per pixel a screen wanted,
/// and that is the drift this file exists to kill. The census behind the
/// geometry ratchet (`frozenExtent` + `contentDimension`) counts the raw
/// literals left; a call site may only read an extent from here, from a
/// component-local named policy, or from its own content.
///
/// MAPPING of the literals migrated onto this ladder (same rule as [AppType]:
/// same-role drift folds to ONE step, ties rounded UP): button heights `52` →
/// [control]; in-card action `36` and header-select `32` → [controlCompact];
/// label columns `110`/`120` → [termLabel]; CTA widths `240` → [actionWidth];
/// the visibility dropdown's hand-fit `116` → [panel]. Sites that were
/// hand-summed budgets for content the ladder already measures went
/// CONTENT-DRIVEN instead — the helper reserve, the rating count column, the
/// upload dropzone and the app-bar search pill now size from their content —
/// and two component-local policies named their numbers in-file instead
/// (the time-picker wheels derive their viewport from one named item extent;
/// the wizard's label clamp spells its bounds once, not twice).
class AppContentSize {
  AppContentSize._();

  /// Micro slot: numbered step badges and right-aligned count columns.
  static const double badge = 24;

  /// Compact control height: in-card actions and header selects.
  static const double controlCompact = 36;

  /// Standard button height.
  static const double control = 48;

  /// Label column of a description row (fixed label + `Expanded` value).
  static const double termLabel = 100;

  /// Media thumbnail inside a grid or strip.
  static const double thumbnail = 112;

  /// A small fixed surface: a tray's viewport, a header control's width.
  static const double panel = 120;

  /// Media card width inside a horizontal rail.
  static const double mediaCard = 140;

  /// Preview surface height: a static map snapshot, a recommendation rail.
  static const double preview = 180;

  /// Capture area (ID card / selfie upload).
  static const double capture = 200;

  /// Popover budget above the keyboard (mention suggestions).
  static const double overlay = 250;

  /// CTA / action button width (empty states and hero actions).
  static const double actionWidth = 280;

  /// Dialog body width.
  static const double dialogWidth = 340;

  /// The image cropper's canvas — ONE name for the pair, so width and height
  /// cannot drift apart at the call site.
  static const Size cropperCanvas = Size(500, 600);
}

/// Type-role extension — the app's five type steps, named.
///
/// WHY THESE EXIST. [AppType] names RAW SIZES: a widget that spells
/// `fontSize: AppType.s14` states a size and delegates everything else to
/// whatever `DefaultTextStyle` wraps it (inside a button that ambient is
/// `labelLarge` w500, inside an app bar `titleLarge`), so the same 14 px can
/// render at two weights depending on where it sits. A ROLE states the whole
/// style once, in the theme — forking the ladder is a deliberate act that
/// belongs in the theme, never in a widget. Since the 2026-10-02 foundation
/// pass the roles ARE the five ladder steps, so the migration's destination is
/// a role, never a number.
///
/// METRICS come from this theme's own `bodyMedium` — see [fromBody]. Every role
/// is the body family at another size, so a role never claims a weight the
/// design did not ask for: a site that wants bold keeps `fontWeight:` at the
/// call site. A site whose old, off-ladder size was folded onto a step moves by
/// the rounding documented on [AppType]. The migration recipe and its proof
/// obligation live in the plan.
///
/// THE FOUNDATION'S FIVE STEPS, as roles (owner decision 2026-10-02): 12, 14,
/// 16, 20, 24 — one name per value, no off-ladder step legalised here. Four of
/// them are M3 2021 steps (`bodySmall`, `bodyMedium`, `bodyLarge`,
/// `headlineSmall`), so a widget that needs no app-specific name can still ride
/// the generic ladder; 20 is the one step M3 2021 lacks, which is why the
/// extension exists at all.
class AppTypeRoles extends ThemeExtension<AppTypeRoles> {
  const AppTypeRoles({
    required this.labelMicro,
    required this.bodyDense,
    required this.titleCompact,
    required this.titleSection,
    required this.titleProminent,
  });

  /// 12 px — the caption under an icon action, and dense timestamps.
  final TextStyle labelMicro;

  /// 14 px — helper and explanatory body text.
  final TextStyle bodyDense;

  /// 16 px — compact item titles and amounts.
  final TextStyle titleCompact;

  /// 20 px — a section header inside a screen.
  final TextStyle titleSection;

  /// 24 px — a prominent state title (empty state) or a display glyph.
  final TextStyle titleProminent;

  /// Builds every role from ONE body style, so no metric is restated and the
  /// only decision this class makes is WHICH ladder step a role means.
  factory AppTypeRoles.fromBody(TextStyle body) => AppTypeRoles(
    labelMicro: body.copyWith(fontSize: AppType.s12),
    bodyDense: body.copyWith(fontSize: AppType.s14),
    titleCompact: body.copyWith(fontSize: AppType.s16),
    titleSection: body.copyWith(fontSize: AppType.s20),
    titleProminent: body.copyWith(fontSize: AppType.s24),
  );

  /// Plain-`ThemeData()` default: the stock M3 2021 body geometry — the same
  /// ladder the contract test pins. Derived from `englishLike` (the geometry
  /// half of `Typography`, where sizes actually live) rather than `black`,
  /// which in this Flutter version carries only colour/family. Same posture as
  /// [AppStatusColors.light]: a widget-test fallback, never an in-app path
  /// (both app themes register this extension and that wiring is pinned by
  /// contract test).
  static AppTypeRoles get fallback =>
      AppTypeRoles.fromBody(Typography.material2021().englishLike.bodyMedium!);

  @override
  AppTypeRoles copyWith({
    TextStyle? labelMicro,
    TextStyle? bodyDense,
    TextStyle? titleCompact,
    TextStyle? titleSection,
    TextStyle? titleProminent,
  }) {
    return AppTypeRoles(
      labelMicro: labelMicro ?? this.labelMicro,
      bodyDense: bodyDense ?? this.bodyDense,
      titleCompact: titleCompact ?? this.titleCompact,
      titleSection: titleSection ?? this.titleSection,
      titleProminent: titleProminent ?? this.titleProminent,
    );
  }

  @override
  AppTypeRoles lerp(AppTypeRoles? other, double t) {
    if (other == null) return this;
    return AppTypeRoles(
      labelMicro: TextStyle.lerp(labelMicro, other.labelMicro, t)!,
      bodyDense: TextStyle.lerp(bodyDense, other.bodyDense, t)!,
      titleCompact: TextStyle.lerp(titleCompact, other.titleCompact, t)!,
      titleSection: TextStyle.lerp(titleSection, other.titleSection, t)!,
      titleProminent: TextStyle.lerp(titleProminent, other.titleProminent, t)!,
    );
  }
}

/// One-line read of the extended ladder: `context.typeRoles.labelMicro`.
extension AppTypeRolesContext on BuildContext {
  AppTypeRoles get typeRoles =>
      Theme.of(this).extension<AppTypeRoles>() ?? AppTypeRoles.fallback;
}

/// Spacing scale — the ONLY place a spacing number may live.
///
/// FOUNDATION (owner decision 2026-10-02): SEVEN steps on a 4pt grid with an
/// 8pt base — 4 · 8 · 12 · 16 · 24 · 32 · 48 — the scale Material/Atlassian
/// style systems converge on. The old ladder held 23 values (1, 1.5, 2, 3, 5,
/// 6, 9, 10, 14, 20, 40, 60, 80, 96, 99 …) and the census showed seven of them
/// carrying ~97% of usage: the rest was drift. Off-ladder call sites were
/// folded onto the nearest step (ties rounded UP, clamped at the ends) so
/// spacing never shrinks by accident. `p0` survives as "no space" — the
/// absence of a step, not a step. Component themes consume these; the roles
/// below name intents that sit on the grid.
class AppMetrics {
  AppMetrics._();

  /// No space — the absence of a step, not a step.
  static const double p0 = 0;

  /// Micro gap: icon-to-label, badge padding.
  static const double p4 = 4;

  /// Tight gap: chips, dense rows.
  static const double p8 = 8;

  /// Default inline gap.
  static const double p12 = 12;

  /// Default padding and screen gutter.
  static const double p16 = 16;

  /// Section gap.
  static const double p24 = 24;

  /// Large separation.
  static const double p32 = 32;

  /// Screen-level separation.
  static const double p48 = 48;

  static const EdgeInsets buttonPadding = EdgeInsets.symmetric(
    horizontal: p24,
    vertical: p12,
  );
  static const EdgeInsets inputPadding = EdgeInsets.symmetric(
    horizontal: p16,
    vertical: p12,
  );

  /// Pressed/outlined field border width used by the focused state. A STROKE
  /// role, not a spacing step.
  static const double focusedBorderWidth = 1.5;

  /// Inset that keeps scroll content clear of a fixed bottom bar. A LAYOUT
  /// role, not a spacing step: derived from the ladder so it cannot drift.
  static const double bottomBarClearance = p48 * 2;
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

  /// THE composer decoration — one factory, one pill, four consumers.
  ///
  /// Message-entry fields (chat composer, comment composer, share-to-chat
  /// message, support thread reply) share ONE spec: radius [AppShape.r24],
  /// fill `surfaceContainerHigh`, [AppMetrics.inputPadding], no border. Form
  /// fields stay on [ThemeData.inputDecorationTheme] (box r12) — composer and
  /// data-entry form are different classes and must not drift into each
  /// other. The scheme is passed in (island rule): this file binds the
  /// scheme pair, never widget-side colours.
  static InputDecoration composerDecoration(
    ColorScheme scheme, {
    required String hintText,
  }) =>
      InputDecoration(
        hintText: hintText,
        hintStyle: TextStyle(color: scheme.onSurfaceVariant),
        filled: true,
        fillColor: scheme.surfaceContainerHigh,
        border: OutlineInputBorder(
          borderRadius: BorderRadius.circular(AppShape.r24),
          borderSide: BorderSide.none,
        ),
        contentPadding: AppMetrics.inputPadding,
        counterText: '',
      );

  /// THE one ThemeData builder. Both modes flow through here, so any component
  /// theme or scale defined below is automatically correct in light and dark.
  static ThemeData _build(ColorScheme scheme, AppStatusColors status) {
    final theme = ThemeData(
      useMaterial3: true,
      colorScheme: scheme,
      fontFamily: 'Inter',
      visualDensity: AppDensity.visualDensity,
      materialTapTargetSize: AppDensity.tapTargetSize,
      scaffoldBackgroundColor: scheme.surface,

      // AppBar theme — THE single chrome authority. Every AppBar/SliverAppBar
      // inherits flat surface (no tint, no shadow, no scrolled-under bump);
      // call sites must not restate these (locked by
      // test/core/theme/app_bar_authority_contract_test.dart).
      appBarTheme: AppBarTheme(
        backgroundColor: scheme.surface,
        foregroundColor: scheme.onSurface,
        elevation: AppElevation.none,
        scrolledUnderElevation: 0,
        surfaceTintColor: Colors.transparent,
        centerTitle: true,
        systemOverlayStyle: _overlayStyle(scheme),
      ),

      // Tab bar — THE single accent authority. Every TabBar inherits the one
      // dialect (brand-ink label + indicator, muted unselected); call sites
      // must not restate these (locked by
      // test/core/theme/tab_bar_authority_contract_test.dart). The three
      // values equal this Flutter version's M3 primary-tab defaults ON
      // PURPOSE: the accent is a design decision owned here, not a framework
      // default the app happens to inherit.
      tabBarTheme: TabBarThemeData(
        labelColor: scheme.primary,
        unselectedLabelColor: scheme.onSurfaceVariant,
        indicatorColor: scheme.primary,
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

      // Outlined button — CANONICAL NEUTRAL secondary action (scope: button
      // authority): onSurface ink on an outlineVariant border. Call sites may
      // only express VARIANTS (e.g. destructive via context.statusColors),
      // never re-state these defaults.
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          foregroundColor: scheme.onSurface,
          side: BorderSide(color: scheme.outlineVariant),
          shape: const RoundedRectangleBorder(
            borderRadius: AppShape.buttonRadius,
          ),
          padding: AppMetrics.buttonPadding,
        ),
      ),

      // Text button — tertiary action, brand ink.
      textButtonTheme: TextButtonThemeData(
        style: TextButton.styleFrom(
          foregroundColor: scheme.primary,
          shape: const RoundedRectangleBorder(
            borderRadius: AppShape.buttonRadius,
          ),
          padding: AppMetrics.buttonPadding,
        ),
      ),

      // Filled button — same canonical pair as the elevated theme.
      filledButtonTheme: FilledButtonThemeData(
        style: FilledButton.styleFrom(
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

    // Extensions are registered AFTER the theme exists so the type roles can
    // derive from the body style WIDGETS actually get. That is not the raw
    // `theme.textTheme`: `Theme.of()` resolves it through
    // `ThemeData.localize(theme, typography.geometryThemeFor(category))`, which
    // merges the englishLike-2021 geometry (size, weight, height, letter
    // spacing) on top. The raw ThemeData carries only colour and family, so
    // deriving from it would produce roles with a size and NO line metrics —
    // exactly the ambient dependence these roles exist to kill. Resolving here
    // through the same framework call keeps one source and none of it restated.
    final resolved = ThemeData.localize(
      theme,
      theme.typography.geometryThemeFor(ScriptCategory.englishLike),
    );
    return theme.copyWith(
      extensions: <ThemeExtension<dynamic>>[
        status,
        AppTypeRoles.fromBody(resolved.textTheme.bodyMedium!),
      ],
    );
  }
}
