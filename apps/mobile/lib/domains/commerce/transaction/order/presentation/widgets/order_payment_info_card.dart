part of 'order_widgets_impl.dart';

class OrderPaymentInfoCard extends StatelessWidget {
  final Order order;
  final bool isDark;

  const OrderPaymentInfoCard({
    super.key,
    required this.order,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
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
          Row(
            children: [
              Icon(
                Icons.payment_outlined,
                size: 20,
                color: _getPaymentStatusColor(),
              ),
              const SizedBox(width: 8),
              Text(
                'Info Pembayaran',
                style: theme.textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.w600,
                ),
              ),
              const Spacer(),
              _PaymentStatusBadge(status: order.paymentStatus),
            ],
          ),
          const SizedBox(height: 16),
          // Payment method
          _PaymentInfoRow(
            label: 'Metode',
            value: _getPaymentMethodDisplay(order.paymentMethod),
            isDark: isDark,
          ),
          const SizedBox(height: 8),
          // Total amount
          _PaymentInfoRow(
            label: 'Total',
            value: AppFormatters.formatCurrency(order.pricing.total),
            isDark: isDark,
            isBold: true,
            valueColor: core.AppColors.primaryRed,
          ),
          // Payment date (if paid)
          if (order.paidAt != null) ...[
            const SizedBox(height: 8),
            _PaymentInfoRow(
              label: 'Tanggal Bayar',
              value: AppFormatters.formatDateTime(order.paidAt!),
              isDark: isDark,
            ),
          ],
        ],
      ),
    );
  }

  Color _getPaymentStatusColor() {
    switch (order.paymentStatus) {
      case PaymentStatus.paid:
        return core.AppColors.statusSuccess;
      case PaymentStatus.pending:
        return core.AppColors.statusWarning;
      case PaymentStatus.processing:
        return core.AppColors.primaryBlue;
      case PaymentStatus.failed:
        return core.AppColors.statusError;
      case PaymentStatus.expired:
        return Colors.grey;
      case PaymentStatus.refunded:
        return core.AppColors.primaryBlue;
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
  final bool isDark;
  final bool isBold;
  final Color? valueColor;

  const _PaymentInfoRow({
    required this.label,
    required this.value,
    required this.isDark,
    this.isBold = false,
    this.valueColor,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Row(
      mainAxisAlignment: MainAxisAlignment.spaceBetween,
      children: [
        Text(
          label,
          style: theme.textTheme.bodyMedium?.copyWith(color: Colors.grey),
        ),
        Text(
          value,
          style: theme.textTheme.bodyMedium?.copyWith(
            color: valueColor ?? (isDark ? Colors.white : Colors.black87),
            fontWeight: isBold ? FontWeight.w700 : FontWeight.w600,
          ),
        ),
      ],
    );
  }
}

class _PaymentStatusBadge extends StatelessWidget {
  final PaymentStatus status;

  const _PaymentStatusBadge({required this.status});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 4),
      decoration: BoxDecoration(
        color: _getBadgeColor().withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: _getBadgeColor().withValues(alpha: 0.3)),
      ),
      child: Text(
        _getBadgeLabel(),
        style: theme.textTheme.bodySmall?.copyWith(
          color: _getBadgeColor(),
          fontWeight: FontWeight.w600,
          fontSize: 11,
        ),
      ),
    );
  }

  Color _getBadgeColor() {
    switch (status) {
      case PaymentStatus.paid:
        return core.AppColors.statusSuccess;
      case PaymentStatus.pending:
        return core.AppColors.statusWarning;
      case PaymentStatus.processing:
        return core.AppColors.primaryBlue;
      case PaymentStatus.failed:
        return core.AppColors.statusError;
      case PaymentStatus.expired:
        return Colors.grey;
      case PaymentStatus.refunded:
        return core.AppColors.primaryBlue;
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
// ❌ Jangan infer payout/fee dari field lain
// ❌ Jangan tampilkan paymentFee sebelum PAID
// ✅ Semua angka = read-only dari backend
//
// Non-Contest (product / auction / offer) pricing display:
// - baseAmount, shippingFee, platformFee, discountAmount, coinDiscount,
//   taxAmount, paymentFee (tampil HANYA saat PAID), totalAmount
//
// Contest pricing display:
// - registrationFee, platformFee, paymentFee, organizerPayout
// =============================================================================
