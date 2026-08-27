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
import 'package:labuda/core/core.dart';
import 'package:labuda/core/common/types/preparation_time.dart';
import 'package:labuda/domains/commerce/transaction/order/domain/entities/order_status.dart';

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
_StatusDisplay _getOrderStatusDisplay({
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
          backgroundColor: AppColors.coinPrimary,
        );
      case PaymentStatus.processing:
        return _StatusDisplay(
          icon: Icons.pending_outlined,
          label: useCompactLabels ? 'Memproses' : 'Memproses Pembayaran',
          subtitle: 'Pembayaran sedang diverifikasi',
          backgroundColor: AppColors.primaryRed,
        );
      case PaymentStatus.expired:
        return _StatusDisplay(
          icon: Icons.error_outline,
          label: 'Kadaluarsa',
          subtitle: 'Buat pesanan baru untuk melanjutkan',
          backgroundColor: AppColors.statusError,
        );
      case PaymentStatus.failed:
        return _StatusDisplay(
          icon: Icons.error_outline,
          label: 'Gagal',
          subtitle: 'Coba lagi atau gunakan metode lain',
          backgroundColor: AppColors.statusError,
        );
      case PaymentStatus.paid:
        // Fall through to order status
        break;
      case PaymentStatus.refunded:
        return _StatusDisplay(
          icon: Icons.currency_exchange,
          label: 'Dikembalikan',
          subtitle: null,
          backgroundColor: AppColors.statusError,
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
          backgroundColor: AppColors.coinPrimary,
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
        } else if (preparationTimeSnapshot != null &&
            !preparationTimeSnapshot.isImmediate) {
          // NORMAL CASE: Show preparation time estimate with maximum context
          paidSubtitle =
              'Estimasi siap kirim: ${preparationTimeSnapshot.displayName.toLowerCase()} (bisa lebih cepat)';
        } else {
          // DEFAULT: Immediate preparation or no data available
          paidSubtitle = 'Menunggu penjual mengirim barang';
        }

        return _StatusDisplay(
          icon: Icons.check_circle_outline,
          label: useCompactLabels ? 'Dibayar' : 'Pembayaran Diterima',
          subtitle: paidSubtitle,
          backgroundColor: AppColors.successGreen,
        );
      case OrderStatus.shipped:
        return _StatusDisplay(
          icon: Icons.local_shipping_outlined,
          label: 'Dikirim',
          subtitle: 'Dalam perjalanan menuju lokasi Anda',
          backgroundColor: AppColors.primaryRed,
        );
      case OrderStatus.delivered:
        // B4A: Delivered is internal-only. If reached, show as completing.
        return _StatusDisplay(
          icon: Icons.done_all,
          label: 'Selesai',
          subtitle: 'Barang diterima, transaksi diselesaikan',
          backgroundColor: AppColors.successGreen,
        );
      case OrderStatus.completed:
        return _StatusDisplay(
          icon: Icons.done_all,
          label: 'Selesai',
          subtitle: 'Transaksi berhasil diselesaikan',
          backgroundColor: AppColors.successGreen,
        );
      case OrderStatus.cancelled:
        return _StatusDisplay(
          icon: Icons.cancel_outlined,
          label: 'Dibatalkan',
          subtitle: null,
          backgroundColor: AppColors.neutralGray500,
        );
      case OrderStatus.cancelledTimeout:
        return _StatusDisplay(
          icon: Icons.timer_off_outlined,
          label: 'Dibatalkan (Timeout)',
          subtitle: 'Penjual tidak mengirim dalam batas waktu',
          backgroundColor: AppColors.neutralGray500,
        );
      case OrderStatus.refunded:
        return _StatusDisplay(
          icon: Icons.currency_exchange,
          label: 'Dikembalikan',
          subtitle: null,
          backgroundColor: AppColors.statusError,
        );
      case OrderStatus.disputeOpen:
        return _StatusDisplay(
          icon: Icons.gavel_outlined,
          label: 'Dispute',
          subtitle: 'Menunggu resolusi dari admin',
          backgroundColor: AppColors.statusError,
        );
      case OrderStatus.partiallyRefunded:
        return _StatusDisplay(
          icon: Icons.currency_exchange,
          label: 'Refund Sebagian',
          subtitle: null,
          backgroundColor: AppColors.coinPrimary,
        );
      case OrderStatus.expired:
        return _StatusDisplay(
          icon: Icons.error_outline,
          label: 'Kadaluarsa',
          subtitle: 'Buat pesanan baru untuk melanjutkan',
          backgroundColor: AppColors.statusError,
        );
    }
  }

  // Default fallback
  return _StatusDisplay(
    icon: Icons.shopping_bag_outlined,
    label: 'Aktif',
    subtitle: 'Tap untuk melihat detail',
    backgroundColor: AppColors.primaryRed,
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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    // Use shared status display logic for consistency
    final statusDisplay = _getOrderStatusDisplay(
      orderStatus: status,
      paymentStatus: paymentStatus,
      useCompactLabels: false,
      // Pass fulfillment visibility data
      preparationTimeSnapshot: preparationTimeSnapshot,
      isOverdue: isOverdue,
      overdueDays: overdueDays,
    );

    return Container(
      margin: const EdgeInsets.fromLTRB(12, 4, 12, 8),
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
      decoration: BoxDecoration(
        color: statusDisplay.backgroundColor.withValues(
          alpha: isDark ? 0.3 : 0.15,
        ),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: statusDisplay.backgroundColor.withValues(alpha: 0.4),
          width: 1,
        ),
      ),
      child: InkWell(
        onTap: isLoading ? null : onTap,
        borderRadius: BorderRadius.circular(12),
        child: Row(
          children: [
            // Status Icon
            Container(
              width: 32,
              height: 32,
              decoration: BoxDecoration(
                color: statusDisplay.backgroundColor.withValues(alpha: 0.3),
                borderRadius: BorderRadius.circular(8),
              ),
              child: Icon(
                statusDisplay.icon,
                size: 18,
                color: statusDisplay.backgroundColor,
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
                        size: 14,
                        color: statusDisplay.backgroundColor,
                      ),
                      const SizedBox(width: 4),
                      Text(
                        'Pesanan Terkait',
                        style: TextStyle(
                          fontSize: 11,
                          color: statusDisplay.backgroundColor,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 2),
                  Text(
                    statusDisplay.label,
                    style: TextStyle(
                      fontSize: 13,
                      fontWeight: FontWeight.w600,
                      color: isDark
                          ? AppColors.neutralWhite
                          : AppColors.neutralGray900,
                    ),
                  ),
                  if (statusDisplay.subtitle != null) ...[
                    const SizedBox(height: 1),
                    Text(
                      statusDisplay.subtitle!,
                      style: TextStyle(
                        fontSize: 11,
                        color: AppColors.neutralGray600,
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
              Icon(Icons.chevron_right, color: AppColors.neutralGray400),
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
  final Color backgroundColor;

  const _StatusDisplay({
    required this.icon,
    required this.label,
    this.subtitle,
    required this.backgroundColor,
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
    final display = _getOrderStatusDisplay(
      orderStatus: status,
      paymentStatus: paymentStatus,
      useCompactLabels: true,
      // Pass fulfillment visibility data
      preparationTimeSnapshot: preparationTimeSnapshot,
      isOverdue: isOverdue,
      overdueDays: overdueDays,
    );

    if (compact) {
      return Container(
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        decoration: BoxDecoration(
          color: display.backgroundColor.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(6),
          border: Border.all(
            color: display.backgroundColor.withValues(alpha: 0.3),
            width: 1,
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(display.icon, size: 12, color: display.backgroundColor),
            const SizedBox(width: 4),
            Text(
              display.label,
              style: TextStyle(
                fontSize: 11,
                fontWeight: FontWeight.w600,
                color: display.backgroundColor,
              ),
            ),
          ],
        ),
      );
    }

    return Container(
      padding: const EdgeInsets.all(8),
      decoration: BoxDecoration(
        color: display.backgroundColor.withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(8),
      ),
      child: Row(
        children: [
          Icon(display.icon, size: 16, color: display.backgroundColor),
          const SizedBox(width: 8),
          Text(
            display.label,
            style: TextStyle(
              fontSize: 12,
              fontWeight: FontWeight.w500,
              color: display.backgroundColor,
            ),
          ),
        ],
      ),
    );
  }
}
