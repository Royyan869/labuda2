part of 'order_widgets_impl.dart';

class OrderInfoCard extends StatelessWidget {
  final Order order;

  const OrderInfoCard({super.key, required this.order});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return OrderSectionCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            'Informasi Pesanan',
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 16),
          _InfoRow(
            label: 'Order ID',
            value: order.orderNumber ?? order.id.substring(0, 8).toUpperCase(),
            isMonospace: true,
          ),
          const SizedBox(height: 8),
          _InfoRow(
            label: 'Tanggal',
            value: AppFormatters.formatDateTime(order.createdAt),
          ),
          const SizedBox(height: 8),
          _InfoRow(
            label: 'Status',
            value: _getStatusDisplay(order.status),
            valueColor: _getStatusColor(context, order.status, colorScheme),
          ),
          if (order.notes != null && order.notes!.isNotEmpty) ...[
            const SizedBox(height: 8),
            _InfoRow(label: 'Catatan', value: order.notes!),
          ],
        ],
      ),
    );
  }

  String _getStatusDisplay(OrderStatus status) {
    switch (status) {
      case OrderStatus.pending:
        return 'Menunggu Pembayaran';
      case OrderStatus.paid:
        return 'Dikonfirmasi';
      case OrderStatus.shipped:
        return 'Dalam Pengiriman';
      case OrderStatus.delivered:
      case OrderStatus.completed:
        return 'Selesai';
      case OrderStatus.cancelled:
        return 'Dibatalkan';
      case OrderStatus.cancelledTimeout:
        return 'Dibatalkan (Timeout)';
      case OrderStatus.refunded:
        return 'Dikembalikan';
      case OrderStatus.disputeOpen:
        return 'Sedang Dispute';
      case OrderStatus.partiallyRefunded:
        return 'Pengembalian Sebagian';
      case OrderStatus.expired:
        return 'Kedaluwarsa';
    }
  }

  Color _getStatusColor(
    BuildContext context,
    OrderStatus status,
    ColorScheme colorScheme,
  ) {
    switch (status) {
      case OrderStatus.pending:
        return context.statusColors.warning;
      case OrderStatus.paid:
        return colorScheme.secondary;
      case OrderStatus.shipped:
        return context.statusColors.info;
      case OrderStatus.delivered:
      case OrderStatus.completed:
        return context.statusColors.success;
      case OrderStatus.cancelled:
      case OrderStatus.cancelledTimeout:
      case OrderStatus.refunded:
        return context.statusColors.error;
      case OrderStatus.disputeOpen:
        return context.statusColors.warning;
      case OrderStatus.partiallyRefunded:
        return context.statusColors.info;
      case OrderStatus.expired:
        return colorScheme.onSurfaceVariant;
    }
  }
}

/// Info row widget for displaying label-value pairs
class _InfoRow extends StatelessWidget {
  final String label;
  final String value;
  final bool isMonospace;
  final Color? valueColor;

  const _InfoRow({
    required this.label,
    required this.value,
    this.isMonospace = false,
    this.valueColor,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 100,
          child: Text(
            label,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: colorScheme.onSurfaceVariant,
            ),
          ),
        ),
        Expanded(
          child: Text(
            value,
            style: theme.textTheme.bodyMedium?.copyWith(
              color: valueColor ?? colorScheme.onSurface,
              fontFamily: isMonospace ? 'monospace' : null,
              fontWeight: valueColor != null ? FontWeight.w600 : null,
            ),
          ),
        ),
      ],
    );
  }
}

// =============================================================================
// OrderUserInfoCard - Order User Info Card (Seller/Buyer)
// =============================================================================
