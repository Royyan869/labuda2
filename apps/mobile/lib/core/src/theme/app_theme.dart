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
    required this.onSuccess,
    required this.onWarning,
    required this.onError,
    required this.onInfo,
  });

  final Color success;
  final Color warning;
  final Color error;
  final Color info;

  /// Contrast-safe ink for text/icons sitting ON the matching status fill.
  /// Never substitute a blanket white.
  final Color onSuccess;
  final Color onWarning;
  final Color onError;
  final Color onInfo;

  static const AppStatusColors light = AppStatusColors(
    success: AppColors.statusSuccess,
    warning: AppColors.statusWarning,
    error: AppColors.statusError,
    info: AppColors.statusInfo,
    onSuccess: AppColors.statusOnSuccess,
    onWarning: AppColors.statusOnWarning,
    onError: AppColors.statusOnError,
    onInfo: AppColors.statusOnInfo,
  );

  static const AppStatusColors dark = AppStatusColors(
    success: AppColors.darkStatusSuccess,
    warning: AppColors.darkStatusWarning,
    error: AppColors.darkStatusError,
    info: AppColors.darkStatusInfo,
    onSuccess: AppColors.darkStatusOnSuccess,
    onWarning: AppColors.darkStatusOnWarning,
    onError: AppColors.darkStatusOnError,
    onInfo: AppColors.darkStatusOnInfo,
  );

  @override
  AppStatusColors copyWith({
    Color? success,
    Color? warning,
    Color? error,
    Color? info,
    Color? onSuccess,
    Color? onWarning,
    Color? onError,
    Color? onInfo,
  }) {
    return AppStatusColors(
      success: success ?? this.success,
      warning: warning ?? this.warning,
      error: error ?? this.error,
      info: info ?? this.info,
      onSuccess: onSuccess ?? this.onSuccess,
      onWarning: onWarning ?? this.onWarning,
      onError: onError ?? this.onError,
      onInfo: onInfo ?? this.onInfo,
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
      onSuccess: Color.lerp(onSuccess, other.onSuccess, t)!,
      onWarning: Color.lerp(onWarning, other.onWarning, t)!,
      onError: Color.lerp(onError, other.onError, t)!,
      onInfo: Color.lerp(onInfo, other.onInfo, t)!,
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
/// MAPPING of the literals migrated onto this ladder (same rule as the retired
/// type ladder: same-role drift folds to ONE step, ties rounded UP): button
/// heights `52` →
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
/// WHY THESE EXIST. The retired numeric ladder named RAW SIZES: a widget that
/// spelled a raw `fontSize:` stated a size and delegated everything else to
/// whatever `DefaultTextStyle` wrapped it (inside a button that ambient is
/// `labelLarge` w500, inside an app bar `titleLarge`), so the same 14 px could
/// render at two weights depending on where it sat. A ROLE states the whole
/// style once, in the theme — forking the ladder is a deliberate act that
/// belongs in the theme, never in a widget. The roles ARE the five enshrined
/// steps, so the destination is a role, never a number.
///
/// METRICS come from this theme's own `bodyMedium` — see [fromResolved]. Every
/// role is the body family at another size, so a role never claims a weight the
/// design did not ask for: a site that wants bold keeps `fontWeight:` at the
/// call site. A site whose old, off-ladder size was folded onto a step moves by
/// the foundation's rounding rule (nearest step, ties rounded up).
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

  /// The one app-specific type step: M3 2021 has no 20-px role, which is the
  /// only reason this extension exists. Named once, here, and never re-spelled
  /// — a widget that reads `titleSection` never states the number.
  static const double sectionStep = 20;

  /// Builds every role from the THEME'S OWN resolved type geometry.
  ///
  /// Four of the five steps are M3 2021 roles — `bodySmall` (12), `bodyMedium`
  /// (14), `bodyLarge` (16) and `headlineSmall` (24) — so each role's SIZE
  /// comes from the theme's resolved [TextTheme], never from a numeric size
  /// ladder. Every other metric (weight, height, letter spacing, family) comes
  /// from `bodyMedium`, so a role is still one body style at another size and a
  /// theme retune reaches every role.
  factory AppTypeRoles.fromResolved(TextTheme text) {
    final body = text.bodyMedium!;
    return AppTypeRoles(
      labelMicro: body.copyWith(fontSize: text.bodySmall!.fontSize),
      bodyDense: body.copyWith(fontSize: text.bodyMedium!.fontSize),
      titleCompact: body.copyWith(fontSize: text.bodyLarge!.fontSize),
      titleSection: body.copyWith(fontSize: sectionStep),
      titleProminent: body.copyWith(fontSize: text.headlineSmall!.fontSize),
    );
  }

  /// Plain-`ThemeData()` default: the stock M3 2021 geometry — the same ladder
  /// the contract test pins. Derived from `englishLike` (the geometry half of
  /// `Typography`, where sizes actually live) rather than `black`, which in
  /// this Flutter version carries only colour/family. Same posture as
  /// [AppStatusColors.light]: a widget-test fallback, never an in-app path
  /// (both app themes register this extension and that wiring is pinned by
  /// contract test).
  static AppTypeRoles get fallback =>
      AppTypeRoles.fromResolved(Typography.material2021().englishLike);

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

  /// Content clearance that keeps scrolled content clear of an overlaying
  /// [FloatingActionButton]. A LAYOUT role — not a spacing step, and NOT a
  /// system inset (the screen's `SafeArea` owns the live system inset).
  /// Measured ABOVE that live inset: content ends at
  /// `systemInset + fabClearance`, so the FAB clearance never shrinks as the
  /// system inset grows. Derived from the ladder so it cannot drift.
  static const double fabClearance = p48 * 2;
}

/// Elevation scale — the ONLY place elevation values live.
///
/// ONE NAME PER VALUE: `none` is not also `flat`, `raised` is not also
/// `overlay`. Two names for one number is the duplication this whole file
/// exists to kill.
///
/// Cards are flat ([none]) by owner decision — there is no card step: the
/// former `card` token (2) was purged with zero production consumers, so a
/// depth need must justify `raised` (or above) at its own call site.
class AppElevation {
  AppElevation._();

  /// Flat chrome: app bars and outlined surfaces that sit ON the surface.
  /// Also the canonical default for ordinary cards and surfaces.
  static const double none = 0;

  /// Barely lifted (chips, subtle inline cards).
  static const double raised = 1;

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
      systemNavigationBarIconBrightness: isDark
          ? Brightness.light
          : Brightness.dark,
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
  }) => InputDecoration(
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

  /// THE editable-search decoration — one factory for every ordinary flat
  /// search field (owner decision 2026-10-05). Canonical search appearance:
  /// filled `surfaceContainerHigh`, rounded [AppShape.containerRadius], NO
  /// border, [AppMetrics.inputPadding]. This is a SEPARATE class from a form
  /// field (which keeps its `outlineVariant` border) and from the composer
  /// pill (r24) — a search field is neither. The map overlay, the tappable
  /// home pill and the composer do NOT consume this factory.
  static InputDecoration searchDecoration(
    ColorScheme scheme, {
    required String hintText,
  }) => InputDecoration(
    hintText: hintText,
    hintStyle: TextStyle(color: scheme.onSurfaceVariant),
    filled: true,
    fillColor: scheme.surfaceContainerHigh,
    border: OutlineInputBorder(
      borderRadius: BorderRadius.circular(AppShape.r12),
      borderSide: BorderSide.none,
    ),
    contentPadding: AppMetrics.inputPadding,
  );

  /// THE form-field state authority (ordinary data-entry fields). Owner-locked:
  /// fields are FILLED with `surfaceContainerHigh`; the geometry (r12 container
  /// radius, outlineVariant/primary border colours, 1.5 focused stroke,
  /// inputPadding) is the locked canonical set. Errors are owned here too: the
  /// error and focused-error borders use the scheme `error` role, the disabled
  /// border mutes to `onSurface @ 0.12`, and label/helper/error/icon styles are
  /// stated ONCE so no call site has to. Text-state styles derive from the
  /// resolved type ladder, so a retune of `textTheme` reaches every field.
  static InputDecorationTheme _inputDecorationTheme(
    ColorScheme scheme,
    TextTheme text,
  ) => InputDecorationTheme(
    filled: true,
    fillColor: scheme.surfaceContainerHigh,
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
    errorBorder: OutlineInputBorder(
      borderRadius: AppShape.containerRadius,
      borderSide: BorderSide(color: scheme.error),
    ),
    focusedErrorBorder: OutlineInputBorder(
      borderRadius: AppShape.containerRadius,
      borderSide: BorderSide(
        color: scheme.error,
        width: AppMetrics.focusedBorderWidth,
      ),
    ),
    disabledBorder: OutlineInputBorder(
      borderRadius: AppShape.containerRadius,
      borderSide: BorderSide(
        color: scheme.onSurface.withValues(alpha: 0.12),
      ),
    ),
    contentPadding: AppMetrics.inputPadding,
    labelStyle: text.bodyLarge?.copyWith(color: scheme.onSurfaceVariant),
    floatingLabelStyle: text.bodyLarge?.copyWith(color: scheme.primary),
    hintStyle: text.bodyLarge?.copyWith(
      color: scheme.onSurfaceVariant.withValues(alpha: 0.6),
    ),
    helperStyle: text.bodySmall?.copyWith(color: scheme.onSurfaceVariant),
    errorStyle: text.bodySmall?.copyWith(color: scheme.error),
    prefixIconColor: scheme.onSurfaceVariant,
    suffixIconColor: scheme.onSurfaceVariant,
    errorMaxLines: 2,
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

      // Dialog / bottom sheet / divider / list tile follow the scheme roles —
      // pinned here so the authority is explicit and locked by contract test.
      //
      // There is intentionally NO `snackBarTheme`: the one canonical Snackbar
      // authority (`AppSnackBar`) paints each toast from the status-colour
      // foundation, so a global snackbar palette would be a competing
      // authority with no consumers.
      dialogTheme: DialogThemeData(
        backgroundColor: scheme.surfaceContainerHigh,
      ),
      // Bottom sheet — THE single surface/shape/elevation authority. Every
      // modal sheet (canonical family or framework call) inherits the muted
      // `surfaceContainerLow` surface, a top-r20 shape and a flat elevation;
      // call sites must NOT paint a local surface, radius or shadow.
      bottomSheetTheme: BottomSheetThemeData(
        backgroundColor: scheme.surfaceContainerLow,
        surfaceTintColor: Colors.transparent,
        elevation: AppElevation.none,
        modalElevation: AppElevation.none,
        shape: const RoundedRectangleBorder(
          borderRadius: BorderRadius.vertical(
            top: Radius.circular(AppShape.r20),
          ),
        ),
      ),
      dividerTheme: DividerThemeData(color: scheme.outlineVariant),
      listTileTheme: ListTileThemeData(iconColor: scheme.onSurfaceVariant),

      // Card theme — Flutter fallback ONLY, never a semantic authority.
      // Owner decision (card foundation): the canonical default for ordinary
      // card/surface visuals is flat. Non-zero elevation is opt-in per
      // semantic surface via [AppElevation], never inherited from here.
      cardTheme: CardThemeData(
        color: scheme.surface,
        elevation: AppElevation.none,
        shape: const RoundedRectangleBorder(
          borderRadius: AppShape.containerRadius,
        ),
      ),

      // Elevated button theme. Disabled treatment is OWNED HERE (owner
      // decision 2026-10-05): the one disabled-action language is the neutral
      // `surfaceContainerHighest` fill with `onSurfaceVariant` ink — never a
      // faded brand colour. A call site that restates disabled colours forks
      // that language.
      elevatedButtonTheme: ElevatedButtonThemeData(
        style: ElevatedButton.styleFrom(
          backgroundColor: scheme.primary,
          foregroundColor: scheme.onPrimary,
          disabledBackgroundColor: scheme.surfaceContainerHighest,
          disabledForegroundColor: scheme.onSurfaceVariant,
          shape: const RoundedRectangleBorder(
            borderRadius: AppShape.buttonRadius,
          ),
          padding: AppMetrics.buttonPadding,
        ),
      ),

      // Outlined button — CANONICAL NEUTRAL secondary action (scope: button
      // authority): onSurface ink on an outlineVariant border. Call sites may
      // only express VARIANTS (e.g. destructive via context.statusColors),
      // never re-state these defaults. Outlined buttons stay transparent when
      // disabled — only their ink mutes to the canonical disabled role.
      outlinedButtonTheme: OutlinedButtonThemeData(
        style: OutlinedButton.styleFrom(
          foregroundColor: scheme.onSurface,
          disabledForegroundColor: scheme.onSurfaceVariant,
          side: BorderSide(color: scheme.outlineVariant),
          shape: const RoundedRectangleBorder(
            borderRadius: AppShape.buttonRadius,
          ),
          padding: AppMetrics.buttonPadding,
        ),
      ),

      // Text button — tertiary action, brand ink. Disabled ink is the
      // canonical muted role; the fill stays transparent by nature.
      textButtonTheme: TextButtonThemeData(
        style: TextButton.styleFrom(
          foregroundColor: scheme.primary,
          disabledForegroundColor: scheme.onSurfaceVariant,
          shape: const RoundedRectangleBorder(
            borderRadius: AppShape.buttonRadius,
          ),
          padding: AppMetrics.buttonPadding,
        ),
      ),

      // Filled button — same canonical pair as the elevated theme, including
      // the one owned disabled treatment.
      filledButtonTheme: FilledButtonThemeData(
        style: FilledButton.styleFrom(
          backgroundColor: scheme.primary,
          foregroundColor: scheme.onPrimary,
          disabledBackgroundColor: scheme.surfaceContainerHighest,
          disabledForegroundColor: scheme.onSurfaceVariant,
          shape: const RoundedRectangleBorder(
            borderRadius: AppShape.buttonRadius,
          ),
          padding: AppMetrics.buttonPadding,
        ),
      ),

      // Icon-button theme — THE canonical disabled language for icon-only
      // actions (owner decision 2026-10-05 / Icon Action Foundation): a
      // disabled icon action mutes to the neutral `onSurfaceVariant`, never an
      // arbitrary local opacity, a faded brand tone or a status colour. Enabled
      // ink still comes from each site's explicit icon colour or the scheme
      // default, so this authority only owns the disabled state.
      iconButtonTheme: IconButtonThemeData(
        style: ButtonStyle(
          foregroundColor: WidgetStateProperty.resolveWith<Color?>((
            Set<WidgetState> states,
          ) {
            if (states.contains(WidgetState.disabled)) {
              return scheme.onSurfaceVariant;
            }
            return null;
          }),
        ),
      ),

      // Selection-control authority (owner decision 2026-10-05): checkbox,
      // radio and switch share ONE canonical Labuda treatment — brand
      // `primary` for the selected/active state, `outline` for the unselected
      // control and `onSurface @ 0.38` for the disabled state (the M3 disabled
      // ink). Call sites must not restate these colours; a local
      // `activeColor`/`activeTrackColor`/`activeThumbColor` that repeats a
      // theme-owned value is a competing authority.
      checkboxTheme: CheckboxThemeData(
        fillColor: WidgetStateProperty.resolveWith<Color?>((states) {
          if (states.contains(WidgetState.disabled)) {
            return scheme.onSurface.withValues(alpha: 0.38);
          }
          if (states.contains(WidgetState.selected)) return scheme.primary;
          return null;
        }),
        checkColor: WidgetStateProperty.all(scheme.onPrimary),
        shape: const RoundedRectangleBorder(
          borderRadius: BorderRadius.all(Radius.circular(AppShape.r4)),
        ),
        side: BorderSide(color: scheme.outline),
      ),
      radioTheme: RadioThemeData(
        fillColor: WidgetStateProperty.resolveWith<Color?>((states) {
          if (states.contains(WidgetState.disabled)) {
            return scheme.onSurface.withValues(alpha: 0.38);
          }
          if (states.contains(WidgetState.selected)) return scheme.primary;
          return scheme.onSurfaceVariant;
        }),
      ),
      switchTheme: SwitchThemeData(
        thumbColor: WidgetStateProperty.resolveWith<Color?>((states) {
          if (states.contains(WidgetState.disabled)) {
            return scheme.onSurface.withValues(alpha: 0.38);
          }
          if (states.contains(WidgetState.selected)) return scheme.onPrimary;
          return scheme.outline;
        }),
        trackColor: WidgetStateProperty.resolveWith<Color?>((states) {
          if (states.contains(WidgetState.disabled)) {
            return scheme.onSurface.withValues(alpha: 0.12);
          }
          if (states.contains(WidgetState.selected)) return scheme.primary;
          return scheme.surfaceContainerHighest;
        }),
        trackOutlineColor: WidgetStateProperty.resolveWith<Color?>((states) {
          if (states.contains(WidgetState.selected) ||
              states.contains(WidgetState.disabled)) {
            return Colors.transparent;
          }
          return scheme.outline;
        }),
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
      inputDecorationTheme: _inputDecorationTheme(scheme, resolved.textTheme),
      extensions: <ThemeExtension<dynamic>>[
        status,
        AppTypeRoles.fromResolved(resolved.textTheme),
      ],
    );
  }
}
