part of 'order_widgets_impl.dart';

/// OrderBuyerPricingCard - Display pricing breakdown for buyer
///
/// This widget displays the pricing breakdown for the buyer with:
/// - Non-Contest: baseAmount, shippingFee, serviceFee, discount, coinDiscount,
///   totalAmount
/// - Contest: registrationFee, serviceFee, payout
class OrderBuyerPricingCard extends StatelessWidget {
  final Order order;
  final bool isDark;

  const OrderBuyerPricingCard({
    super.key,
    required this.order,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    return _CommercePricingCard(order: order, isDark: isDark);
  }
}

/// Commerce pricing card for Non-Contest orders (product/auction/offer)
class _CommercePricingCard extends StatelessWidget {
  final Order order;
  final bool isDark;

  const _CommercePricingCard({required this.order, required this.isDark});

  @override
  Widget build(BuildContext context) {
    final pricing = order.pricing;
    final theme = Theme.of(context);

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? const Color(0xFF1E1E1E) : Colors.white,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? const Color(0xFF333333) : const Color(0xFFE0E0E0),
        ),
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
              label: 'Biaya Pengiriman',
              value: AppFormatters.formatCurrency(pricing.shippingCost),
            ),
          if (pricing.serviceFeeAmount != null)
            _PricingRow(
              label: 'Biaya Layanan Pembayaran',
              value: AppFormatters.formatCurrency(pricing.serviceFeeAmount!),
            )
          else
            const _PricingRow(
              label: 'Biaya Layanan Pembayaran',
              value: 'Akan dihitung server',
              valueColor: Colors.grey,
            ),
          // DISCOUNT HONESTY: Show discount with code and description
          // - Shows discount code used (e.g., "HEMAT10")
          // - Shows discount description if available (e.g., "10% off")
          // - Shows actual discount amount from backend
          // - All data comes from backend - no fake calculations
          if (pricing.discount > 0)
            _DiscountRow(
              code: pricing.discountCode,
              description: pricing.discountDescription,
              amount: pricing.discount,
            ),
          const Divider(height: 24),
          _PricingRow(
            label: 'Total Pembayaran',
            value: AppFormatters.formatCurrency(
              pricing.totalPayableAmount ?? pricing.total,
            ),
            isBold: true,
            valueColor: isDark ? Colors.white : const Color(0xFF1E1E1E),
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
  final bool isDark;

  const OrderSellerPricingCard({
    super.key,
    required this.order,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    final pricing = order.pricing;
    final theme = Theme.of(context);

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark ? const Color(0xFF1E1E1E) : Colors.white,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark ? const Color(0xFF333333) : const Color(0xFFE0E0E0),
        ),
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
            label: 'Biaya Pengiriman',
            value: AppFormatters.formatCurrency(pricing.shippingCost),
          ),
          const Divider(height: 24),
          // FINANCIAL OWNERSHIP BOUNDARY (Wave 3.1B):
          // sellerCommission and sellerEarnings are finance-domain data
          // Access via SellerEarnings/SellerDashboard entities, not Order
          const _PricingRow(
            label: 'Pendapatan Bersih',
            value: 'Lihat di Dashboard Penjual',
            valueColor: Colors.grey,
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

    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Row(
        mainAxisAlignment: MainAxisAlignment.spaceBetween,
        children: [
          Text(
            label,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: Colors.grey,
              fontWeight: isBold ? FontWeight.w600 : FontWeight.normal,
            ),
          ),
          Text(
            value,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: valueColor ?? (isBold ? null : Colors.grey),
              fontWeight: isBold ? FontWeight.w600 : FontWeight.normal,
            ),
          ),
        ],
      ),
    );
  }
}

/// DISCOUNT HONESTY: Discount row widget for displaying applied discount
///
/// Shows honest discount information from backend:
/// - Discount code used (e.g., "HEMAT10")
/// - Discount description (e.g., "10% off", "Free shipping")
/// - Actual discount amount from backend
///
/// IMPORTANT: Does NOT invent savings or show fake "X% OFF" badges
class _DiscountRow extends StatelessWidget {
  final String? code;
  final String? description;
  final double amount;

  const _DiscountRow({
    required this.code,
    required this.description,
    required this.amount,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              Text(
                'Diskon',
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: Colors.green,
                  fontWeight: FontWeight.w500,
                ),
              ),
              Text(
                '-${AppFormatters.formatCurrency(amount)}',
                style: theme.textTheme.bodyMedium?.copyWith(
                  color: Colors.green,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ],
          ),
          // HONESTY: Show discount code and description from backend
          // This helps user understand which discount was applied
          if (code != null || description != null)
            Padding(
              padding: const EdgeInsets.only(left: 0, top: 4),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  if (code != null)
                    Text(
                      'Kode: $code',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: Colors.grey[600],
                        fontSize: 11,
                      ),
                    ),
                  if (description != null)
                    Text(
                      description!,
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: Colors.grey[600],
                        fontSize: 11,
                      ),
                    ),
                ],
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
