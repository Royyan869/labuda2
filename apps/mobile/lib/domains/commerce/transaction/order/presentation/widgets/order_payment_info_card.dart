part of 'order_widgets_impl.dart';

class OrderPaymentInfoCard extends StatelessWidget {
  final Order order;

  const OrderPaymentInfoCard({super.key, required this.order});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Container(
      padding: const EdgeInsets.all(core.AppMetrics.p16),
      decoration: BoxDecoration(
        color: colorScheme.surface,
        borderRadius: BorderRadius.circular(core.AppShape.r12),
        border: Border.all(color: colorScheme.outlineVariant),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.payment_outlined,
                size: 20,
                color: _getPaymentStatusColor(context, colorScheme),
              ),
              const SizedBox(width: 8),
              Text(
                'Info Pembayaran',
                style: theme.textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.w600,
                ),
              ),
              const Spacer(),
              _PaymentStatusBadge(
                status: order.paymentStatus,
                colorScheme: colorScheme,
              ),
            ],
          ),
          const SizedBox(height: 16),
          // Payment method
          _PaymentInfoRow(
            label: 'Metode',
            value: _getPaymentMethodDisplay(order.paymentMethod),
          ),
          const SizedBox(height: 8),
          // Total amount
          _PaymentInfoRow(
            label: 'Total',
            value: order.pricing.totalPayableAmount != null
                ? AppFormatters.formatCurrency(order.pricing.totalPayableAmount!)
                : '—',
            isBold: true,
            valueColor: colorScheme.primary,
          ),
          // Payment date (if paid)
          if (order.paidAt != null) ...[
            const SizedBox(height: 8),
            _PaymentInfoRow(
              label: 'Tanggal Bayar',
              value: AppFormatters.formatDateTime(order.paidAt!),
            ),
          ],
        ],
      ),
    );
  }

  Color _getPaymentStatusColor(BuildContext context, ColorScheme colorScheme) {
    switch (order.paymentStatus) {
      case PaymentStatus.paid:
        return context.statusColors.success;
      case PaymentStatus.pending:
        return context.statusColors.warning;
      case PaymentStatus.processing:
        return colorScheme.secondary;
      case PaymentStatus.failed:
        return context.statusColors.error;
      case PaymentStatus.expired:
        return colorScheme.onSurfaceVariant;
      case PaymentStatus.refunded:
        return colorScheme.secondary;
    }
  }

  String _getPaymentMethodDisplay(PaymentMethodType method) {
    // Convert payment method enum to display name
    // Using canonical PaymentMethodType from core/common/types/payment_types.dart
    return method.displayName;
  }
}

class _PaymentInfoRow extends StatelessWidget {
  final String label;
  final String value;
  final bool isBold;
  final Color? valueColor;

  const _PaymentInfoRow({
    required this.label,
    required this.value,
    this.isBold = false,
    this.valueColor,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(
          label,
          style: theme.textTheme.bodyMedium?.copyWith(
            color: colorScheme.onSurfaceVariant,
          ),
        ),
        Text(
          value,
          style: theme.textTheme.bodyMedium?.copyWith(
            color: valueColor ?? colorScheme.onSurface,
            fontWeight: isBold ? FontWeight.w700 : FontWeight.w600,
          ),
        ),
      ],
    );
  }
}

class _PaymentStatusBadge extends StatelessWidget {
  final PaymentStatus status;
  final ColorScheme colorScheme;

  const _PaymentStatusBadge({
    required this.status,
    required this.colorScheme,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: core.AppMetrics.p12, vertical: core.AppMetrics.p4),
      decoration: BoxDecoration(
        color: _getBadgeColor(context, ).withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(core.AppShape.r12),
        border: Border.all(color: _getBadgeColor(context, ).withValues(alpha: 0.3)),
      ),
      child: Text(
        _getBadgeLabel(),
        style: theme.textTheme.bodySmall?.copyWith(
          color: _getBadgeColor(context, ),
          fontWeight: FontWeight.w600,
          fontSize: core.AppType.s11,
        ),
      ),
    );
  }

  Color _getBadgeColor(BuildContext context) {
    switch (status) {
      case PaymentStatus.paid:
        return context.statusColors.success;
      case PaymentStatus.pending:
        return context.statusColors.warning;
      case PaymentStatus.processing:
        return colorScheme.secondary;
      case PaymentStatus.failed:
        return context.statusColors.error;
      case PaymentStatus.expired:
        return colorScheme.onSurfaceVariant;
      case PaymentStatus.refunded:
        return colorScheme.secondary;
    }
  }

  String _getBadgeLabel() {
    switch (status) {
      case PaymentStatus.paid:
        return 'LUNAS';
      case PaymentStatus.pending:
        return 'BELUM';
      case PaymentStatus.processing:
        return 'DIPROSES';
      case PaymentStatus.failed:
        return 'GAGAL';
      case PaymentStatus.expired:
        return 'KADALUARSA';
      case PaymentStatus.refunded:
        return 'DIKEMBALALIKAN';
    }
  }
}

// =============================================================================
// ORDER PRICING DISPLAY WIDGETS
// =============================================================================
// CLIENT UNFREEZE — PRICING ONLY (FINAL)
//
// ATURAN EMAS (WAJIB):
// ❌ Jangan hitung ulang apa pun di client
// ❌ Jangan infer fee dari field lain
// ❌ Jangan tampilkan serviceFeeAmount sebelum tersedia dari backend
// ✅ Semua angka = read-only dari backend
//
// Canonical pricing display (subtotal = P, shippingCost = S,
// serviceFeeAmount = F, totalPayableAmount = PD + S + F):
// - subtotal, shippingCost, serviceFeeAmount, totalPayableAmount
//
// The legacy fields (baseAmount, shippingFee, platformFee, discountAmount,
// coinDiscount, taxAmount, paymentFee) and the "Contest" pricing variant were
// never part of the canonical Order contract and were purged.
// =============================================================================
