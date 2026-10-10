/// Chat Order Status Banner
///
/// Displays order status within chat for commerce continuity.
/// Shows when a chat has a linked order and provides navigation to order detail.
///
/// **COMMERCE CONTINUITY:**
/// - Users can see order status without leaving chat
/// - Clear entry point to order detail screen
/// - Honest status visibility (no fake or client-authoritative states)
library;

import 'package:flutter/material.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/core/common/types/preparation_time.dart';
import 'package:hishumi/domains/commerce/transaction/order/domain/entities/order_status.dart';

// =============================================================================
// SHARED STATUS DISPLAY LOGIC
// =============================================================================

/// Get status display configuration for order/payment status
///
/// Consolidated status display logic used by both ChatOrderStatusBanner
/// and OrderStatusMiniWidget. This ensures consistent labels across all
/// chat order status displays.
///
/// **FULFILLMENT VISIBILITY:** Shows preparation time estimates for paid orders
/// instead of generic "Menunggu penjual mengirim barang" message.
/// This helps buyers understand when to expect their order to be shipped.
_StatusDisplay _getOrderStatusDisplay(BuildContext context, {
  required ColorScheme scheme,
  required OrderStatus? orderStatus,
  required PaymentStatus? paymentStatus,
  required bool useCompactLabels,
  // ═══════════════════════════════════════════════════════════════════════
  // FULFILLMENT VISIBILITY PARAMETERS
  // ═══════════════════════════════════════════════════════════════════════
  // These parameters enable showing preparation time estimates for paid orders
  PreparationTime? preparationTimeSnapshot,
  bool? isOverdue,
  int? overdueDays,
}) {
  // Priority: Payment status for pending/expired/failed states
  if (paymentStatus != null) {
    switch (paymentStatus) {
      case PaymentStatus.pending:
        return _StatusDisplay(
          icon: Icons.payments_outlined,
          label: useCompactLabels ? 'Menunggu Bayar' : 'Menunggu Pembayaran',
          subtitle: 'Segera selesaikan pembayaran',
          tone: AppColors.coinPrimary,
        );
      case PaymentStatus.processing:
        return _StatusDisplay(
          icon: Icons.pending_outlined,
          label: useCompactLabels ? 'Memproses' : 'Memproses Pembayaran',
          subtitle: 'Pembayaran sedang diverifikasi',
          tone: scheme.primary,
        );
      case PaymentStatus.expired:
        return _StatusDisplay(
          icon: Icons.error_outline,
          label: 'Kadaluarsa',
          subtitle: 'Buat pesanan baru untuk melanjutkan',
          tone: context.statusColors.error,
        );
      case PaymentStatus.failed:
        return _StatusDisplay(
          icon: Icons.error_outline,
          label: 'Gagal',
          subtitle: 'Coba lagi atau gunakan metode lain',
          tone: context.statusColors.error,
        );
      case PaymentStatus.paid:
        // Fall through to order status
        break;
      case PaymentStatus.refunded:
        return _StatusDisplay(
          icon: Icons.currency_exchange,
          label: 'Dikembalikan',
          subtitle: null,
          tone: context.statusColors.error,
        );
    }
  }

  // Order status display
  if (orderStatus != null) {
    switch (orderStatus) {
      case OrderStatus.pending:
        return _StatusDisplay(
          icon: Icons.payments_outlined,
          label: useCompactLabels ? 'Menunggu Bayar' : 'Menunggu Pembayaran',
          subtitle: 'Segera selesaikan pembayaran',
          tone: AppColors.coinPrimary,
        );
      case OrderStatus.paid:
        // ═══════════════════════════════════════════════════════════════════════
        // FULFILLMENT VISIBILITY: Show preparation time estimate
        // ═══════════════════════════════════════════════════════════════════════
        // Replace generic "Menunggu penjual mengirim barang" with specific
        // preparation time estimates or overdue information
        // IMPORTANT: Always indicate this is maximum time (upper bound)
        String? paidSubtitle;
        if (isOverdue == true && overdueDays != null && overdueDays > 0) {
          // OVERDUE CASE: Show how many days overdue
          paidSubtitle =
              'Terlambat $overdueDays ${overdueDays == 1 ? 'hari' : 'hari'} dari estimasi';
        } else if (preparationTimeSnapshot != null) {
          // NORMAL CASE: Show preparation time estimate with maximum context
          paidSubtitle =
              'Estimasi siap kirim: ${preparationTimeSnapshot.displayName.toLowerCase()} (bisa lebih cepat)';
        } else {
          // DEFAULT: no snapshot available
          paidSubtitle = 'Menunggu penjual mengirim barang';
        }

        return _StatusDisplay(
          icon: Icons.check_circle_outline,
          label: useCompactLabels ? 'Dibayar' : 'Pembayaran Diterima',
          subtitle: paidSubtitle,
          tone: context.statusColors.success,
        );
      case OrderStatus.shipped:
        return _StatusDisplay(
          icon: Icons.local_shipping_outlined,
          label: 'Dikirim',
          subtitle: 'Dalam perjalanan menuju lokasi Anda',
          tone: scheme.primary,
        );
      case OrderStatus.delivered:
        // B4A: Delivered is internal-only. If reached, show as completing.
        return _StatusDisplay(
          icon: Icons.done_all,
          label: 'Selesai',
          subtitle: 'Barang diterima, transaksi diselesaikan',
          tone: context.statusColors.success,
        );
      case OrderStatus.completed:
        return _StatusDisplay(
          icon: Icons.done_all,
          label: 'Selesai',
          subtitle: 'Transaksi berhasil diselesaikan',
          tone: context.statusColors.success,
        );
      case OrderStatus.cancelled:
        return _StatusDisplay(
          icon: Icons.cancel_outlined,
          label: 'Dibatalkan',
          subtitle: null,
          tone: scheme.onSurfaceVariant,
        );
      case OrderStatus.cancelledTimeout:
        return _StatusDisplay(
          icon: Icons.timer_off_outlined,
          label: 'Dibatalkan (Timeout)',
          subtitle: 'Penjual tidak mengirim dalam batas waktu',
          tone: scheme.onSurfaceVariant,
        );
      case OrderStatus.refunded:
        return _StatusDisplay(
          icon: Icons.currency_exchange,
          label: 'Dikembalikan',
          subtitle: null,
          tone: context.statusColors.error,
        );
      case OrderStatus.disputeOpen:
        return _StatusDisplay(
          icon: Icons.gavel_outlined,
          label: 'Dispute',
          subtitle: 'Menunggu resolusi dari admin',
          tone: context.statusColors.error,
        );
      case OrderStatus.partiallyRefunded:
        return _StatusDisplay(
          icon: Icons.currency_exchange,
          label: 'Refund Sebagian',
          subtitle: null,
          tone: AppColors.coinPrimary,
        );
      case OrderStatus.expired:
        return _StatusDisplay(
          icon: Icons.error_outline,
          label: 'Kadaluarsa',
          subtitle: 'Buat pesanan baru untuk melanjutkan',
          tone: context.statusColors.error,
        );
    }
  }

  // Default fallback
  return _StatusDisplay(
    icon: Icons.shopping_bag_outlined,
    label: 'Aktif',
    subtitle: 'Tap untuk melihat detail',
    tone: scheme.primary,
  );
}

/// Chat Order Status Banner
///
/// Shows compact order status in chat with navigation to order detail.
///
/// **FULFILLMENT VISIBILITY:** Accepts preparation time data to show
/// shipping estimates instead of generic "waiting for seller" messages.
class ChatOrderStatusBanner extends StatelessWidget {
  final String orderId;
  final OrderStatus? status;
  final PaymentStatus? paymentStatus;
  final VoidCallback onTap;
  final bool isLoading;

  // ═══════════════════════════════════════════════════════════════════════
  // FULFILLMENT VISIBILITY PARAMETERS
  // ═══════════════════════════════════════════════════════════════════════
  // Optional preparation time data to show shipping estimates in paid status
  final PreparationTime? preparationTimeSnapshot;
  final bool? isOverdue;
  final int? overdueDays;

  const ChatOrderStatusBanner({
    super.key,
    required this.orderId,
    this.status,
    this.paymentStatus,
    required this.onTap,
    this.isLoading = false,
    // ═══════════════════════════════════════════════════════════════════════
    // FULFILLMENT VISIBILITY: Optional preparation time data
    // ═══════════════════════════════════════════════════════════════════════
    this.preparationTimeSnapshot,
    this.isOverdue,
    this.overdueDays,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    // Use shared status display logic for consistency
    final statusDisplay = _getOrderStatusDisplay(context, 
      scheme: Theme.of(context).colorScheme,
      orderStatus: status,
      paymentStatus: paymentStatus,
      useCompactLabels: false,
      // Pass fulfillment visibility data
      preparationTimeSnapshot: preparationTimeSnapshot,
      isOverdue: isOverdue,
      overdueDays: overdueDays,
    );

    return Container(
      margin: const EdgeInsets.fromLTRB(AppMetrics.p12, AppMetrics.p4, AppMetrics.p12, AppMetrics.p8),
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p12),
      decoration: BoxDecoration(
        color: statusDisplay.tone.withValues(alpha: 0.2),
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(
          color: statusDisplay.tone.withValues(alpha: 0.4),
          width: 1,
        ),
      ),
      child: InkWell(
        onTap: isLoading ? null : onTap,
        borderRadius: BorderRadius.circular(AppShape.r12),
        child: Row(
          children: [
            // Status Icon
            Container(
              width: 32,
              height: 32,
              decoration: BoxDecoration(
                color: statusDisplay.tone.withValues(alpha: 0.3),
                borderRadius: BorderRadius.circular(AppShape.r8),
              ),
              child: Icon(
                statusDisplay.icon,
                size: AppIconSize.action,
                color: statusDisplay.tone,
              ),
            ),
            const SizedBox(width: 12),
            // Status Info
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      Icon(
                        Icons.shopping_bag_outlined,
                        size: AppIconSize.inlineGlyph,
                        color: statusDisplay.tone,
                      ),
                      const SizedBox(width: 4),
                      Text(
                        'Pesanan Terkait',
                        style: context.typeRoles.labelMicro.copyWith(
                          color: scheme.onSurface,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 2),
                  Text(
                    statusDisplay.label,
                    style: context.typeRoles.bodyDense.copyWith(
                      fontWeight: FontWeight.w600,
                      color: scheme.onSurface,
                    ),
                  ),
                  if (statusDisplay.subtitle != null) ...[
                    const SizedBox(height: 1),
                    Text(
                      statusDisplay.subtitle!,
                      style: context.typeRoles.labelMicro.copyWith(
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                  ],
                ],
              ),
            ),
            // Loading indicator or chevron
            if (isLoading)
              const SizedBox(
                width: 16,
                height: 16,
                child: CircularProgressIndicator(strokeWidth: 2),
              )
            else
              Icon(
                Icons.chevron_right,
                color: scheme.onSurfaceVariant,
              ),
          ],
        ),
      ),
    );
  }
}

/// Status display configuration
class _StatusDisplay {
  final IconData icon;
  final String label;
  final String? subtitle;
  /// Status TONE for this order state (painted at low alpha behind the row).
  /// Named `tone`, not `backgroundColor`: a background is a surface role, and
  /// this is the status vocabulary — see the ink-as-fill rule in
  /// `test/support/theme_authority_gate.dart`.
  final Color tone;

  const _StatusDisplay({
    required this.icon,
    required this.label,
    this.subtitle,
    required this.tone,
  });
}

/// Compact Order Status Mini Widget
///
/// A smaller version for inline display in message bubbles or commerce attachments.
/// Uses shared status display logic for consistency.
///
/// **FULFILLMENT VISIBILITY:** Accepts preparation time data to show
/// shipping estimates instead of generic messages.
class OrderStatusMiniWidget extends StatelessWidget {
  final OrderStatus? status;
  final PaymentStatus? paymentStatus;
  final bool compact;

  // ═══════════════════════════════════════════════════════════════════════
  // FULFILLMENT VISIBILITY PARAMETERS
  // ═══════════════════════════════════════════════════════════════════════
  // Optional preparation time data to show shipping estimates in paid status
  final PreparationTime? preparationTimeSnapshot;
  final bool? isOverdue;
  final int? overdueDays;

  const OrderStatusMiniWidget({
    super.key,
    this.status,
    this.paymentStatus,
    this.compact = true,
    // ═══════════════════════════════════════════════════════════════════════
    // FULFILLMENT VISIBILITY: Optional preparation time data
    // ═══════════════════════════════════════════════════════════════════════
    this.preparationTimeSnapshot,
    this.isOverdue,
    this.overdueDays,
  });

  @override
  Widget build(BuildContext context) {
    // Use shared status display logic for consistency
    final display = _getOrderStatusDisplay(context, 
      scheme: Theme.of(context).colorScheme,
      orderStatus: status,
      paymentStatus: paymentStatus,
      useCompactLabels: true,
      // Pass fulfillment visibility data
      preparationTimeSnapshot: preparationTimeSnapshot,
      isOverdue: isOverdue,
      overdueDays: overdueDays,
    );

    final scheme = Theme.of(context).colorScheme;
    if (compact) {
      return Container(
        padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8, vertical: AppMetrics.p4),
        decoration: BoxDecoration(
          color: display.tone.withValues(alpha: 0.12),
          borderRadius: BorderRadius.circular(AppShape.r6),
          border: Border.all(
            color: display.tone.withValues(alpha: 0.3),
            width: 1,
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(display.icon, size: AppIconSize.inlineGlyph, color: display.tone),
            const SizedBox(width: 4),
            Text(
              display.label,
              style: context.typeRoles.labelMicro.copyWith(
                fontWeight: FontWeight.w600,
                color: scheme.onSurface,
              ),
            ),
          ],
        ),
      );
    }

    return Container(
      padding: const EdgeInsets.all(AppMetrics.p8),
      decoration: BoxDecoration(
        color: display.tone.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(AppShape.r8),
      ),
      child: Row(
        children: [
          Icon(display.icon, size: AppIconSize.inlineGlyph, color: display.tone),
          const SizedBox(width: 8),
          Text(
            display.label,
            style: context.typeRoles.labelMicro.copyWith(
              fontWeight: FontWeight.w500,
              color: scheme.onSurface,
            ),
          ),
        ],
      ),
    );
  }
}
