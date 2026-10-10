import 'package:flutter/material.dart';
import 'package:hishumi/core/core.dart';

/// THE canonical Bottom Action Bar authority.
///
/// Every persistent bottom action area in Labuda is rendered here and nowhere
/// else — transaction CTAs (Beli Sekarang, Pasang Bid, Klaim, Buat Pesanan)
/// and form actions (Simpan Profil, Simpan Diskon, Simpan Ongkir,
/// Simpan Alamat) share this ONE foundation. There is exactly ONE visual
/// model (surface chrome + top divider, single padding contract, single
/// action gap, single button height, single loading presentation).
///
/// Callers choose CONTENT (which actions, which labels, which callbacks,
/// which state); they never choose a surface, separator, shadow, padding,
/// Safe Area handling, keyboard handling, or button height.
///
/// Foundation consumed:
/// - surface/separator: `colorScheme.surface` / `colorScheme.outlineVariant`
///   (the divider-theme role — deliberately flat, like the AppBar chrome;
///   no custom shadow, so there is no second elevation authority)
/// - geometry: [AppMetrics] (gutter `p16`, vertical `p12`, gaps `p12`/`p8`)
/// - buttons: [AppContentSize.control] height; fill/border/ink owned by the
///   button themes (a `null` callback disables via the theme, never here)
/// - destructive tone: `context.statusColors.error` (outlined actions only)
/// - typography: icon-action captions ride `context.typeRoles.labelMicro`
///
/// Placement contract: mount via `Scaffold.bottomNavigationBar`. The bar owns
/// the bottom system inset (`SafeArea(top: false)`, so a status-bar-sized
/// system inset is never added above the bar) AND the keyboard inset: the
/// framework does NOT lift `bottomNavigationBar` above the keyboard (only the
/// body is resized — see `Scaffold._ScaffoldLayout.performLayout`), so the
/// foundation pads `MediaQuery.viewInsetsOf(context).bottom` itself. No
/// caller may compute insets for the bar. This same mechanism covers embedded
/// footers (e.g. a dialog footer), where no Scaffold lifts anything.
///
/// Inside a lifted `AppBottomSheetBase` sheet ([embeddedInLiftedSheet]) the
/// presenter already owns BOTH insets: the base lifts the sheet above the
/// keyboard and spends the system bottom inset exactly once as its own
/// spacer. The bar then reserves NEITHER again — a second system-bottom
/// reservation would grow the bar by the inset and double the gap below the
/// CTA (BOTTOMSHEET-03: padat/normal).
class BottomActionBar extends StatelessWidget {
  /// Info content rendered INSIDE the chrome, above the actions (e.g. a
  /// checkout total row, a disabled reason, an unavailability banner).
  /// Content only — no chrome, no Safe Area, no insets inside.
  final Widget? header;

  /// Leading icon+label affordances (e.g. Chat, Nego). Rendered before the
  /// buttons in row layout; unused in stacked layout.
  final List<BottomBarIconAction> leading;

  /// Optional secondary action: outlined, left of [primary] with equal flex.
  final BottomBarAction? secondary;

  /// The main CTA: filled brand button. Null only when the bar carries no
  /// primary (e.g. a support-only fallback); at least one of [primary],
  /// [secondary], [stacked], [leading], [header], [footer] must be present.
  final BottomBarAction? primary;

  /// Extra full-width outlined actions. When non-empty the bar uses the
  /// stacked layout: [primary] full-width on top, then each entry below
  /// (the order-decision presentation). [secondary] and [leading] are
  /// ignored in stacked layout.
  final List<BottomBarAction> stacked;

  /// Tertiary content rendered below the actions (e.g. support links).
  /// Content only.
  final Widget? footer;

  /// Set when the bar is embedded inside an [AppBottomSheetBase] sheet whose
  /// presenter already owns both insets (the base lifts the whole sheet above
  /// `viewInsets.bottom` and spends the system bottom inset exactly once as
  /// its own spacer). The bar then must NOT rise a second time or its buttons
  /// float one keyboard-height above the keyboard, and it adds NO second
  /// system-bottom reservation — the base spacer alone keeps the CTA clear of
  /// the navigation area. Defaults to `false`: every other placement
  /// (`Scaffold.bottomNavigationBar`, body-embedded footers, unlifted sheets)
  /// keeps the self-lift and its own SafeArea system-bottom handling, exactly
  /// as before (BOTTOMSHEET-03).
  final bool embeddedInLiftedSheet;

  const BottomActionBar({
    super.key,
    this.header,
    this.leading = const [],
    this.secondary,
    this.primary,
    this.stacked = const [],
    this.footer,
    this.embeddedInLiftedSheet = false,
  });

  /// Component-local spinner policy: ONE name for the pair, so the box and
  /// the stroke cannot drift apart at the call site (same agreement as the
  /// time-picker wheels in the geometry foundation).
  static const double _spinnerExtent = 20;
  static const double _spinnerStroke = 2;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    // Owned here, not by callers: the framework leaves bottomNavigationBar
    // under the keyboard, so the bar rises by its own inset padding — unless
    // it is embedded in a sheet whose presenter already owns that movement,
    // in which case a second lift would double it (see [embeddedInLiftedSheet]).
    final keyboardInset = embeddedInLiftedSheet
        ? 0.0
        : MediaQuery.viewInsetsOf(context).bottom;

    return Container(
      decoration: BoxDecoration(
        color: scheme.surface,
        border: Border(top: BorderSide(color: scheme.outlineVariant)),
      ),
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p16,
        vertical: AppMetrics.p12,
      ),
      child: SafeArea(
        top: false,
        // Same context boundary as the keyboard lift: embedded in a lifted
        // sheet, the base's spacer is the ONE system-bottom reservation, so
        // the bar adds none (the inset would otherwise be spent twice — bar
        // strip + spacer). Everywhere else the bar IS the bottom surface and
        // keeps owning the inset itself.
        bottom: !embeddedInLiftedSheet,
        child: Padding(
          padding: EdgeInsets.only(bottom: keyboardInset),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (header != null) ...[
                header!,
                const SizedBox(height: AppMetrics.p8),
              ],
              if (stacked.isNotEmpty)
                _StackedActions(
                  primary: primary,
                  stacked: stacked,
                  spinnerExtent: _spinnerExtent,
                  spinnerStroke: _spinnerStroke,
                )
              else if (primary != null ||
                  secondary != null ||
                  leading.isNotEmpty)
                _ActionRow(
                  leading: leading,
                  secondary: secondary,
                  primary: primary,
                  spinnerExtent: _spinnerExtent,
                  spinnerStroke: _spinnerStroke,
                ),
              if (footer != null) ...[
                const SizedBox(height: AppMetrics.p8),
                footer!,
              ],
            ],
          ),
        ),
      ),
    );
  }
}

/// One button in the bar. `onPressed: null` disables via the button theme.
/// `isLoading` replaces the label with the canonical spinner and blocks
/// input. `isDestructive` paints the error tone on outlined (secondary /
/// stacked) actions; the filled primary is always brand.
class BottomBarAction {
  final String label;
  final VoidCallback? onPressed;
  final bool isLoading;
  final bool isDestructive;
  final IconData? icon;

  const BottomBarAction({
    required this.label,
    required this.onPressed,
    this.isLoading = false,
    this.isDestructive = false,
    this.icon,
  });
}

/// One icon+label affordance leading the row (Chat, Nego, …).
class BottomBarIconAction {
  final IconData icon;
  final String label;
  final VoidCallback onPressed;

  const BottomBarIconAction({
    required this.icon,
    required this.label,
    required this.onPressed,
  });
}

/// Row layout: [icon affordances] [secondary?] [primary?] — the single
/// canonical row for every bar with at most two buttons.
class _ActionRow extends StatelessWidget {
  final List<BottomBarIconAction> leading;
  final BottomBarAction? secondary;
  final BottomBarAction? primary;
  final double spinnerExtent;
  final double spinnerStroke;

  const _ActionRow({
    required this.leading,
    required this.secondary,
    required this.primary,
    required this.spinnerExtent,
    required this.spinnerStroke,
  });

  @override
  Widget build(BuildContext context) {
    final hasButtons = secondary != null || primary != null;
    return Row(
      children: [
        for (final action in leading)
          _IconAffordance(action: action, spinnerExtent: spinnerExtent),
        if (leading.isNotEmpty && hasButtons)
          const SizedBox(width: AppMetrics.p12),
        if (secondary != null)
          Expanded(
            child: _OutlinedBarButton(
              action: secondary!,
              spinnerExtent: spinnerExtent,
              spinnerStroke: spinnerStroke,
            ),
          ),
        if (secondary != null && primary != null)
          const SizedBox(width: AppMetrics.p12),
        if (primary != null)
          Expanded(
            child: _FilledBarButton(
              action: primary!,
              spinnerExtent: spinnerExtent,
              spinnerStroke: spinnerStroke,
            ),
          ),
      ],
    );
  }
}

/// Stacked layout: full-width primary on top, then full-width outlined
/// actions (the backend-driven order presentation).
class _StackedActions extends StatelessWidget {
  final BottomBarAction? primary;
  final List<BottomBarAction> stacked;
  final double spinnerExtent;
  final double spinnerStroke;

  const _StackedActions({
    required this.primary,
    required this.stacked,
    required this.spinnerExtent,
    required this.spinnerStroke,
  });

  @override
  Widget build(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (primary != null)
          _FilledBarButton(
            action: primary!,
            spinnerExtent: spinnerExtent,
            spinnerStroke: spinnerStroke,
          ),
        for (var i = 0; i < stacked.length; i++) ...[
          if (primary != null || i > 0) const SizedBox(height: AppMetrics.p12),
          _OutlinedBarButton(
            action: stacked[i],
            spinnerExtent: spinnerExtent,
            spinnerStroke: spinnerStroke,
          ),
        ],
      ],
    );
  }
}

/// Icon above micro-label — the one affordance shape for leading actions.
class _IconAffordance extends StatelessWidget {
  final BottomBarIconAction action;
  final double spinnerExtent;

  const _IconAffordance({required this.action, required this.spinnerExtent});

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: action.onPressed,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            action.icon,
            color: Theme.of(context).colorScheme.onSurfaceVariant,
            size: AppIconSize.action,
          ),
          const SizedBox(height: 2),
          Text(action.label, style: context.typeRoles.labelMicro),
        ],
      ),
    );
  }
}

/// Filled brand CTA. Fill/ink/disabled tokens belong to the elevated-button
/// theme — never restated here.
class _FilledBarButton extends StatelessWidget {
  final BottomBarAction action;
  final double spinnerExtent;
  final double spinnerStroke;

  const _FilledBarButton({
    required this.action,
    required this.spinnerExtent,
    required this.spinnerStroke,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final loading = action.isLoading;
    final child = loading
        ? SizedBox(
            width: spinnerExtent,
            height: spinnerExtent,
            child: CircularProgressIndicator(
              strokeWidth: spinnerStroke,
              // Loading renders on the disabled fill (the theme owns the
              // neutral disabled surface), so the spinner rides the disabled
              // content role — never the brand onPrimary.
              valueColor: AlwaysStoppedAnimation<Color>(
                scheme.onSurfaceVariant,
              ),
            ),
          )
        : action.icon == null
        ? Text(
            action.label,
            style: Theme.of(
              context,
            ).textTheme.labelLarge?.copyWith(fontWeight: FontWeight.w600),
          )
        : null;
    return SizedBox(
      height: AppContentSize.control,
      width: double.infinity,
      child: child != null
          ? ElevatedButton(
              onPressed: loading ? null : action.onPressed,
              child: child,
            )
          : ElevatedButton.icon(
              onPressed: loading ? null : action.onPressed,
              icon: Icon(action.icon, size: AppIconSize.action),
              label: Text(
                action.label,
                style: Theme.of(
                  context,
                ).textTheme.labelLarge?.copyWith(fontWeight: FontWeight.w600),
              ),
            ),
    );
  }
}

/// Outlined secondary CTA. Neutral by default (the outlined-button theme);
/// the error tone only when the domain marks the action destructive.
class _OutlinedBarButton extends StatelessWidget {
  final BottomBarAction action;
  final double spinnerExtent;
  final double spinnerStroke;

  const _OutlinedBarButton({
    required this.action,
    required this.spinnerExtent,
    required this.spinnerStroke,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final loading = action.isLoading;
    final destructive = action.isDestructive;
    final child = loading
        ? SizedBox(
            width: spinnerExtent,
            height: spinnerExtent,
            child: CircularProgressIndicator(
              strokeWidth: spinnerStroke,
              // Same disabled-content rule as the filled bar button.
              valueColor: AlwaysStoppedAnimation<Color>(
                scheme.onSurfaceVariant,
              ),
            ),
          )
        : null;
    final style = destructive
        ? OutlinedButton.styleFrom(
            foregroundColor: context.statusColors.error,
            side: BorderSide(
              color: context.statusColors.error.withValues(alpha: 0.3),
            ),
          )
        : null;
    final label = Text(
      action.label,
      style: Theme.of(
        context,
      ).textTheme.labelLarge?.copyWith(fontWeight: FontWeight.w500),
    );
    return SizedBox(
      height: AppContentSize.control,
      width: double.infinity,
      child: child != null
          ? OutlinedButton(onPressed: null, style: style, child: child)
          : action.icon == null
          ? OutlinedButton(
              onPressed: action.onPressed,
              style: style,
              child: label,
            )
          : OutlinedButton.icon(
              onPressed: action.onPressed,
              style: style,
              icon: Icon(action.icon, size: AppIconSize.action),
              label: label,
            ),
    );
  }
}
