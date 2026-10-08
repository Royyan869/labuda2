part of 'order_widgets_impl.dart';

/// Canonical Order section frame — ONE AUTHORITY for every Order detail
/// section surface.
///
/// Covers: info, items, shipping, payment, buyer/seller pricing, user,
/// overdue and refund sections. They are one semantic family (a titled,
/// static grouped block on the order detail) and previously each restated
/// the same container contract in its own file.
///
/// VISUAL CONTRACT (locked — identical to what every section rendered
/// before convergence, so migration is pixel-neutral):
///   - surface `colorScheme.surface`, flat (no elevation, no shadow)
///   - radius [AppShape.r12]
///   - 1px `colorScheme.outlineVariant` border
///   - internal padding [AppMetrics.p16]
///
/// Alert variants (overdue, refund status) do NOT get their own frame:
/// they pass [backgroundColor]/[borderColor]/[borderWidth]. The tint stays
/// the caller's semantic decision; the geometry stays here.
///
/// NOT for commerce discovery cards ([CommerceMarketplaceCardShell]) and
/// NOT for detail-section blocks of other domains — this authority is
/// Order-scoped by Owner decision.
class OrderSectionCard extends StatelessWidget {
  final Widget child;

  /// Section tint. Defaults to `colorScheme.surface`.
  final Color? backgroundColor;

  /// Border ink. Defaults to `colorScheme.outlineVariant`.
  final Color? borderColor;

  /// Border stroke width. Defaults to 1.
  final double borderWidth;

  /// Outer margin. Defaults to none — the list owns rhythm, except the
  /// two alert banners that historically carry their own bottom margin.
  final EdgeInsetsGeometry? margin;

  const OrderSectionCard({
    super.key,
    required this.child,
    this.backgroundColor,
    this.borderColor,
    this.borderWidth = 1,
    this.margin,
  });

  @override
  Widget build(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Container(
      margin: margin,
      padding: const EdgeInsets.all(core.AppMetrics.p16),
      decoration: BoxDecoration(
        color: backgroundColor ?? colorScheme.surface,
        borderRadius: BorderRadius.circular(core.AppShape.r12),
        border: Border.all(
          color: borderColor ?? colorScheme.outlineVariant,
          width: borderWidth,
        ),
      ),
      child: child,
    );
  }
}
