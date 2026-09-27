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

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: colorScheme.surface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colorScheme.outlineVariant),
      ),
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

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: colorScheme.surface,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: colorScheme.outlineVariant),
      ),
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

    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(
            label,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: colorScheme.onSurfaceVariant,
              fontWeight: isBold ? FontWeight.w600 : FontWeight.normal,
            ),
          ),
          Text(
            value,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: valueColor ?? (isBold ? null : colorScheme.onSurfaceVariant),
              fontWeight: isBold ? FontWeight.w600 : FontWeight.normal,
            ),
          ),
        ],
      ),
    );
  }
}

// =============================================================================
// RefundRequestStatusCard - Refund Status Card
// =============================================================================
