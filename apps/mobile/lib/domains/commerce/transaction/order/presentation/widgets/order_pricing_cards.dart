part of 'order_widgets_impl.dart';

/// OrderBuyerPricingCard - Display pricing breakdown for buyer
///
/// Renders ONLY canonical backend pricing (no client-side derivation):
/// - subtotal                  (P)
/// - shippingCost              (S)
/// - serviceFeeAmount          (F, buyer-side; "Akan dihitung server" when absent)
/// - totalPayableAmount        (PD + S + F — the buyer's gross payable)
///
/// No derived rows: the seller discount is already folded into the canonical
/// money model (PD = P - D) and coins are not an Order snapshot authority.
class OrderBuyerPricingCard extends StatelessWidget {
  final Order order;

  const OrderBuyerPricingCard({super.key, required this.order});

  @override
  Widget build(BuildContext context) {
    return _CommercePricingCard(order: order);
  }
}

/// Commerce pricing card for product / auction / offer orders
class _CommercePricingCard extends StatelessWidget {
  final Order order;

  const _CommercePricingCard({required this.order});

  @override
  Widget build(BuildContext context) {
    final pricing = order.pricing;
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return OrderSectionCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Rincian Pembayaran',
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 16),
          _PricingRow(
            label: 'Harga Produk',
            value: AppFormatters.formatCurrency(pricing.subtotal),
          ),
          if (pricing.shippingCost > 0)
            _PricingRow(
              // Seller tariff is ALL-IN (shipping + packing) — see
              // checkout_order_summary_section.dart for the label contract.
              label: 'Ongkir + Packing',
              value: AppFormatters.formatCurrency(pricing.shippingCost),
            ),
          if (pricing.serviceFeeAmount != null)
            _PricingRow(
              label: 'Biaya Layanan Pembayaran',
              value: AppFormatters.formatCurrency(pricing.serviceFeeAmount!),
            )
          else
            _PricingRow(
              label: 'Biaya Layanan Pembayaran',
              value: 'Akan dihitung server',
              valueColor: colorScheme.onSurfaceVariant,
            ),
          // NOTE: Discount display removed. Backend does not emit
          // discount_amount, discount_code, or discount_description
          // on the order response. The discount is embedded in the
          // canonical money model (subtotal = P, totalBeforeCoinsAmount = PD+S).
          const Divider(height: 24),
          _PricingRow(
            label: 'Total Pembayaran',
            value: pricing.totalPayableAmount != null
                ? AppFormatters.formatCurrency(pricing.totalPayableAmount!)
                : 'Akan dihitung server',
            isBold: true,
            valueColor: colorScheme.onSurface,
          ),
        ],
      ),
    );
  }
}

/// OrderSellerPricingCard - Display pricing breakdown for seller
///
/// Shows seller commission and earnings
class OrderSellerPricingCard extends StatelessWidget {
  final Order order;

  const OrderSellerPricingCard({super.key, required this.order});

  @override
  Widget build(BuildContext context) {
    final pricing = order.pricing;
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return OrderSectionCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Rincian Pendapatan',
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 16),
          _PricingRow(
            label: 'Subtotal',
            value: AppFormatters.formatCurrency(pricing.subtotal),
          ),
          _PricingRow(
            label: 'Ongkir + Packing',
            value: AppFormatters.formatCurrency(pricing.shippingCost),
          ),
          const Divider(height: 24),
          // FINANCIAL OWNERSHIP BOUNDARY (Wave 3.1B):
          // sellerCommission and sellerEarnings are finance-domain data
          // Access via SellerEarnings/SellerDashboard entities, not Order
          _PricingRow(
            label: 'Pendapatan Bersih',
            value: 'Lihat di Dashboard Penjual',
            valueColor: colorScheme.onSurfaceVariant,
          ),
        ],
      ),
    );
  }
}

/// Pricing row widget for displaying label-value pairs.
///
/// F2 CANONICAL COMPOSITION (Owner decisions: money is never truncated):
/// horizontal `Row` when the single-line label + value provably fit the
/// incoming width, vertical `Column` (label over value) otherwise.
///
/// Width ownership is explicit — the label owns the remainder through
/// `Expanded` (single line, ellipsis backstop; labels are compressible) and
/// the value renders from a bounded `Flexible` slot with `softWrap` and NO
/// ellipsis, so monetary values stay fully visible and can never overflow.
/// The horizontal/vertical decision comes from [LayoutBuilder] constraints +
/// measured single-line text widths — never a hardcoded device breakpoint,
/// never `MediaQuery` width arithmetic, never `IntrinsicWidth`.
bool _fitsOrderLabelValueSingleLine({
  required BuildContext context,
  required double maxWidth,
  required String label,
  required String value,
  required TextStyle? labelStyle,
  required TextStyle? valueStyle,
  required double fixedExtrasWidth,
}) {
  // Unbounded incoming width (never expected inside OrderSectionCard):
  // stack vertically, which is always safe.
  if (!maxWidth.isFinite) {
    return false;
  }
  final TextDirection direction = Directionality.of(context);
  final TextScaler scaler = MediaQuery.textScalerOf(context);

  double singleLineWidth(String text, TextStyle? style) {
    final TextPainter painter = TextPainter(
      text: TextSpan(text: text, style: style),
      textDirection: direction,
      textScaler: scaler,
      maxLines: 1,
    )..layout();
    return painter.width;
  }

  // 2px safety margin: TextPainter measurement vs real Row layout can differ
  // by subpixels. Err toward stacking (safe direction), never toward a
  // hairline overflow.
  const double safetyMargin = 2;
  final double required =
      singleLineWidth(label, labelStyle) +
      fixedExtrasWidth +
      singleLineWidth(value, valueStyle) +
      safetyMargin;
  return required <= maxWidth;
}

/// Pricing row widget for displaying label-value pairs
class _PricingRow extends StatelessWidget {
  final String label;
  final String value;
  final bool isBold;
  final Color? valueColor;

  const _PricingRow({
    required this.label,
    required this.value,
    this.isBold = false,
    this.valueColor,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;
    final TextStyle? labelStyle = theme.textTheme.bodyMedium?.copyWith(
      color: colorScheme.onSurfaceVariant,
      fontWeight: isBold ? FontWeight.w600 : FontWeight.normal,
    );
    final TextStyle? valueStyle = theme.textTheme.bodyMedium?.copyWith(
      color: valueColor ?? (isBold ? null : colorScheme.onSurfaceVariant),
      fontWeight: isBold ? FontWeight.w600 : FontWeight.normal,
    );

    return Padding(
      padding: const EdgeInsets.only(bottom: core.AppMetrics.p8),
      child: LayoutBuilder(
        builder: (BuildContext context, BoxConstraints constraints) {
          // Horizontal gap between label and value in the Row branch.
          const double gapWidth = 12;
          final bool fits = _fitsOrderLabelValueSingleLine(
            context: context,
            maxWidth: constraints.maxWidth,
            label: label,
            value: value,
            labelStyle: labelStyle,
            valueStyle: valueStyle,
            fixedExtrasWidth: gapWidth,
          );
          if (fits) {
            return Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  child: Text(
                    label,
                    style: labelStyle,
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
                const SizedBox(width: gapWidth),
                Flexible(
                  child: Text(
                    value,
                    style: valueStyle,
                    textAlign: TextAlign.end,
                    // NEVER ellipsis here: monetary values must remain fully
                    // visible. softWrap is a layout backstop only (the fit
                    // check above already proved single-line fit); it wraps
                    // instead of overflowing, never truncates.
                    softWrap: true,
                  ),
                ),
              ],
            );
          }
          // Canonical vertical fallback: label over value, both fully visible.
          return Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(label, style: labelStyle, softWrap: true),
              const SizedBox(height: 4),
              Text(value, style: valueStyle, softWrap: true),
            ],
          );
        },
      ),
    );
  }
}

// =============================================================================
// RefundRequestStatusCard - Refund Status Card
// =============================================================================
