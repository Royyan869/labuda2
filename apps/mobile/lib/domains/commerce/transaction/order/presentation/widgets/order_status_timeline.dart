part of 'order_widgets_impl.dart';

class SellerActionRequiredBanner extends StatelessWidget {
  final Order order;
  final VoidCallback onTapReview;

  const SellerActionRequiredBanner({
    super.key,
    required this.order,
    required this.onTapReview,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      margin: const EdgeInsets.only(bottom: 16),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: core.AppColors.statusWarning.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: core.AppColors.statusWarning.withValues(alpha: 0.3),
        ),
      ),
      child: Row(
        children: [
          Container(
            width: 40,
            height: 40,
            decoration: BoxDecoration(
              color: core.AppColors.statusWarning.withValues(alpha: 0.2),
              shape: BoxShape.circle,
            ),
            child: Icon(
              Icons.notification_important,
              color: core.AppColors.statusWarning,
              size: 20,
            ),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  'Tindakan Diperlukan',
                  style: theme.textTheme.titleSmall?.copyWith(
                    fontWeight: FontWeight.w600,
                    color: core.AppColors.statusWarning,
                  ),
                ),
                const SizedBox(height: 4),
                Text(
                  _getActionMessage(),
                  style: theme.textTheme.bodySmall?.copyWith(
                    color: Colors.grey,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  String _getActionMessage() {
    switch (order.status) {
      case OrderStatus.pending:
        return 'Pesanan baru - Terima atau tolak pesanan ini';
      default:
        return 'Mohon periksa pesanan ini';
    }
  }
}

// =============================================================================
// OrderStatusTimeline - Order Status Timeline Widget
// =============================================================================

/// Timeline step data class
class _TimelineStep {
  final String label;
  final String? sublabel;
  final IconData icon;
  final bool isActive;
  final bool isCompleted;
  final DateTime? timestamp;

  const _TimelineStep({
    required this.label,
    this.sublabel,
    required this.icon,
    required this.isActive,
    required this.isCompleted,
    this.timestamp,
  });
}

class OrderStatusTimeline extends StatelessWidget {
  final Order order;
  final bool isDark;

  const OrderStatusTimeline({
    super.key,
    required this.order,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final steps = _buildTimelineSteps();

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
            'Status Pesanan',
            style: theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w600,
            ),
          ),
          const SizedBox(height: 16),
          ...steps.map(
            (step) => _TimelineStepTile(
              step: step,
              isDark: isDark,
              isLast: steps.last == step,
            ),
          ),
        ],
      ),
    );
  }

  /// Build timeline steps based on order status
  List<_TimelineStep> _buildTimelineSteps() {
    final steps = <_TimelineStep>[];

    // B4A: Timeline progression is now:
    // pending -> paid -> shipped -> completed (selesai)
    // No separate "Barang Diterima" step — acceptance is implicit in completion.
    // cancelled/refunded/expired are terminal states

    final currentStatus = order.status;

    // Determine which steps are active/completed
    final isPending = currentStatus == OrderStatus.pending;
    final isPaid = currentStatus == OrderStatus.paid;
    final isShipped = currentStatus == OrderStatus.shipped;
    final isDelivered = currentStatus == OrderStatus.delivered;
    final isCompleted = currentStatus == OrderStatus.completed;
    final isCancelled = currentStatus == OrderStatus.cancelled;
    final isCancelledTimeout = currentStatus == OrderStatus.cancelledTimeout;
    final isRefunded = currentStatus == OrderStatus.refunded;
    final isExpired = currentStatus == OrderStatus.expired;

    // Special handling for terminal states (cancelled/cancelledTimeout/refunded/expired)
    if (isCancelled || isCancelledTimeout || isRefunded || isExpired) {
      steps.add(
        _TimelineStep(
          label: _getStatusLabel(currentStatus),
          sublabel: _getStatusSublabel(currentStatus),
          icon: (isCancelled || isCancelledTimeout)
              ? Icons.cancel
              : (isRefunded ? Icons.currency_exchange : Icons.timer_off),
          isActive: true,
          isCompleted: false,
          timestamp: order.cancelledAt,
        ),
      );
      return steps;
    }

    // Normal progression steps
    steps.add(
      _TimelineStep(
        label: 'Pesanan Dibuat',
        sublabel: 'Menunggu konfirmasi penjual',
        icon: Icons.shopping_cart_outlined,
        isActive: isPending,
        isCompleted: !isPending,
        timestamp: order.createdAt,
      ),
    );

    if (isPaid || isShipped || isDelivered || isCompleted) {
      steps.add(
        _TimelineStep(
          label: 'Pembayaran Berhasil',
          sublabel: order.confirmedAt != null
              ? AppFormatters.formatDateTime(order.confirmedAt!)
              : null,
          icon: Icons.check_circle_outline,
          isActive: false,
          isCompleted: true,
          timestamp: order.confirmedAt,
        ),
      );
    }

    // B4A: Shipped step — active while awaiting buyer acceptance,
    // completed once buyer accepts (→ completed).
    if (isShipped || isDelivered || isCompleted) {
      steps.add(
        _TimelineStep(
          label: 'Dalam Pengiriman',
          sublabel: order.shippedAt != null
              ? AppFormatters.formatDateTime(order.shippedAt!)
              : null,
          icon: Icons.local_shipping_outlined,
          isActive: isShipped,
          isCompleted: isDelivered || isCompleted,
          timestamp: order.shippedAt,
        ),
      );
    }

    // B4A: "Selesai" = buyer accepted + escrow released. No separate delivered step.
    if (isDelivered || isCompleted) {
      steps.add(
        _TimelineStep(
          label: 'Selesai',
          sublabel: order.completedAt != null
              ? AppFormatters.formatDateTime(order.completedAt!)
              : (order.deliveredAt != null
                    ? AppFormatters.formatDateTime(order.deliveredAt!)
                    : null),
          icon: Icons.done_all,
          isActive: false,
          isCompleted: true,
          timestamp: order.completedAt ?? order.deliveredAt,
        ),
      );
    }

    return steps;
  }

  String _getStatusLabel(OrderStatus status) {
    switch (status) {
      case OrderStatus.pending:
        return 'Menunggu Konfirmasi';
      case OrderStatus.paid:
        return 'Pembayaran Berhasil'; // P11 aligned: state='paid' → label reflects payment success
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
        return 'Kedaluwarsa'; // O1: Added expired status label
    }
  }

  String? _getStatusSublabel(OrderStatus status) {
    switch (status) {
      case OrderStatus.cancelled:
        final reason = order.cancelReason;
        return reason != null && reason.isNotEmpty
            ? 'Alasan: $reason'
            : 'Pesanan telah dibatalkan';
      case OrderStatus.cancelledTimeout:
        return 'Penjual tidak mengirim dalam batas waktu';
      case OrderStatus.refunded:
        return 'Pengembalian dana sedang diproses';
      case OrderStatus.expired:
        return 'Waktu pembayaran habis'; // O1: Added expired sublabel
      default:
        return null;
    }
  }
}

/// Timeline step tile widget
class _TimelineStepTile extends StatelessWidget {
  final _TimelineStep step;
  final bool isDark;
  final bool isLast;

  const _TimelineStepTile({
    required this.step,
    required this.isDark,
    required this.isLast,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    // Determine colors based on state
    Color iconColor;
    Color iconBgColor;
    Color lineColor;

    if (step.isActive) {
      iconColor = core.AppColors.primaryRed;
      iconBgColor = core.AppColors.primaryRed.withValues(alpha: 0.1);
      lineColor = core.AppColors.primaryRed;
    } else if (step.isCompleted) {
      iconColor = core.AppColors.statusSuccess;
      iconBgColor = core.AppColors.statusSuccess.withValues(alpha: 0.1);
      lineColor = core.AppColors.statusSuccess;
    } else {
      iconColor = isDark ? Colors.grey.shade600 : Colors.grey.shade400;
      iconBgColor = isDark
          ? Colors.grey.shade800.withValues(alpha: 0.3)
          : Colors.grey.shade200;
      lineColor = isDark ? Colors.grey.shade700 : Colors.grey.shade300;
    }

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Icon column
        Column(
          children: [
            Container(
              width: 40,
              height: 40,
              decoration: BoxDecoration(
                color: iconBgColor,
                shape: BoxShape.circle,
              ),
              child: Icon(step.icon, color: iconColor, size: 20),
            ),
            if (!isLast)
              Container(
                width: 2,
                height: 40,
                color: lineColor.withValues(alpha: 0.5),
              ),
          ],
        ),
        const SizedBox(width: 12),
        // Content column
        Expanded(
          child: Padding(
            padding: EdgeInsets.only(bottom: isLast ? 0 : 8),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  step.label,
                  style: theme.textTheme.bodyMedium?.copyWith(
                    fontWeight: step.isActive
                        ? FontWeight.w600
                        : FontWeight.normal,
                    color: step.isActive
                        ? iconColor
                        : (isDark ? Colors.white : Colors.black87),
                  ),
                ),
                if (step.sublabel != null)
                  Text(
                    step.sublabel!,
                    style: theme.textTheme.bodySmall?.copyWith(
                      color: Colors.grey,
                    ),
                  ),
              ],
            ),
          ),
        ),
      ],
    );
  }
}
