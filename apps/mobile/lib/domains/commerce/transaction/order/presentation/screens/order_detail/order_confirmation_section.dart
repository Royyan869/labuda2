/// Order Confirmation Section
///
/// Displays confirmation/verification information for shipped/delivered orders.
/// This section helps users understand the current state and available actions.
library;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/domains/commerce/transaction/order/order.dart';
import 'package:labuda/shared/utils/app_formatters.dart';

class OrderConfirmationSection extends StatelessWidget {
  final Order order;
  final bool isBuyer;
  final String? currentUserId;

  const OrderConfirmationSection({
    super.key,
    required this.order,
    required this.isBuyer,
    this.currentUserId,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final theme = Theme.of(context);

    // Only show for shipped/delivered orders
    if (order.status != OrderStatus.shipped &&
        order.status != OrderStatus.delivered) {
      return const SizedBox.shrink();
    }

    return Container(
      padding: const EdgeInsets.all(16),
      margin: const EdgeInsets.only(bottom: 16),
      decoration: BoxDecoration(
        color: isDark ? const Color(0xFF1E1E1E) : Colors.white,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: _getBorderColorForStatus(order.status),
          width: 1.5,
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header with icon and title
          Row(
            children: [
              Container(
                width: 40,
                height: 40,
                decoration: BoxDecoration(
                  color: _getIconBgColorForStatus(order.status),
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  _getIconForStatus(order.status),
                  color: _getIconColorForStatus(order.status),
                  size: 20,
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      _getTitleForStatus(order.status),
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    Text(
                      _getSubtitleForStatus(order.status),
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: Colors.grey,
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),

          // Shipped date info (if available)
          if (order.shippedAt != null) ...[
            const SizedBox(height: 12),
            _InfoRow(
              label: 'Tanggal Dikirim',
              value: AppFormatters.formatDateTime(order.shippedAt!),
              isDark: isDark,
            ),
          ],

          // Delivered date info (if available)
          if (order.deliveredAt != null) ...[
            const SizedBox(height: 8),
            _InfoRow(
              label: 'Tanggal Terkirim',
              value: AppFormatters.formatDateTime(order.deliveredAt!),
              isDark: isDark,
            ),
          ],

          // SHIPPING CONFIRMATION TRUTH: Shipping reference with honest label
          if (order.shippingInfo.trackingNumber != null &&
              order.shippingInfo.trackingNumber!.isNotEmpty) ...[
            const SizedBox(height: 8),
            _HonestShippingReferenceRow(
              shipping: order.shippingInfo,
              isDark: isDark,
            ),
          ],

          // SHIPPING CONFIRMATION TRUTH: Shipping note from seller
          if (order.shippingInfo.shippingNote != null &&
              order.shippingInfo.shippingNote!.isNotEmpty) ...[
            const SizedBox(height: 8),
            _ShippingNoteSection(
              note: order.shippingInfo.shippingNote!,
              isDark: isDark,
            ),
          ],

          // ===== AUTO-RELEASE COUNTDOWN TIMER =====
          // Shows escrow auto-release countdown for shipped/delivered orders
          // This is a critical piece of information for both buyers and sellers
          AutoReleaseCountdownWidget(
            autoReleaseAt: order.buyerConfirmDeadline,
            status: order.status,
            isBuyer: isBuyer,
          ),

          // B4A: Buyer action reminder shown on SHIPPED (buyer accepts directly)
          if (isBuyer && order.status == OrderStatus.shipped) ...[
            const SizedBox(height: 12),
            Container(
              padding: const EdgeInsets.all(12),
              decoration: BoxDecoration(
                color: core.AppColors.statusInfo.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(8),
                border: Border.all(
                  color: core.AppColors.statusInfo.withValues(alpha: 0.3),
                ),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.info_outline,
                    color: core.AppColors.statusInfo,
                    size: 20,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'Jika barang sudah diterima dan sesuai, tap "Terima Barang" untuk menyelesaikan pesanan.',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: core.AppColors.statusInfo,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ],
        ],
      ),
    );
  }

  Color _getBorderColorForStatus(OrderStatus status) {
    switch (status) {
      case OrderStatus.shipped:
        return core.AppColors.statusInfo.withValues(alpha: 0.5);
      case OrderStatus.delivered:
        return core.AppColors.primaryRed.withValues(alpha: 0.5);
      default:
        return const Color(0xFFE0E0E0);
    }
  }

  Color _getIconBgColorForStatus(OrderStatus status) {
    switch (status) {
      case OrderStatus.shipped:
        return core.AppColors.statusInfo.withValues(alpha: 0.1);
      case OrderStatus.delivered:
        return core.AppColors.primaryRed.withValues(alpha: 0.1);
      default:
        return Colors.grey.withValues(alpha: 0.1);
    }
  }

  Color _getIconColorForStatus(OrderStatus status) {
    switch (status) {
      case OrderStatus.shipped:
        return core.AppColors.statusInfo;
      case OrderStatus.delivered:
        return core.AppColors.primaryRed;
      default:
        return Colors.grey;
    }
  }

  IconData _getIconForStatus(OrderStatus status) {
    switch (status) {
      case OrderStatus.shipped:
        return Icons.local_shipping_outlined;
      case OrderStatus.delivered:
        return Icons.inbox_outlined;
      default:
        return Icons.info_outline;
    }
  }

  String _getTitleForStatus(OrderStatus status) {
    switch (status) {
      case OrderStatus.shipped:
        return 'Pesanan Dikirim';
      case OrderStatus.delivered:
        return 'Pesanan Dalam Perjalanan';
      default:
        return 'Status Pesanan';
    }
  }

  String _getSubtitleForStatus(OrderStatus status) {
    switch (status) {
      case OrderStatus.shipped:
        return 'Pesanan Anda sedang dalam perjalanan';
      case OrderStatus.delivered:
        return 'Silakan periksa kondisi barang saat diterima';
      default:
        return '';
    }
  }
}

/// Info row widget for displaying label-value pairs
class _InfoRow extends StatelessWidget {
  final String label;
  final String value;
  final bool isDark;
  final bool isMonospace = false;

  const _InfoRow({
    required this.label,
    required this.value,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        SizedBox(
          width: 100,
          child: Text(
            label,
            style: theme.textTheme.bodySmall?.copyWith(color: Colors.grey),
          ),
        ),
        Expanded(
          child: Text(
            value,
            style: theme.textTheme.bodySmall?.copyWith(
              color: isDark ? Colors.white : Colors.black87,
              fontFamily: isMonospace ? 'monospace' : null,
            ),
          ),
        ),
      ],
    );
  }
}

/// SHIPPING CONFIRMATION TRUTH: Honest shipping reference row
///
/// Displays shipping reference with honest labeling based on reference type
class _HonestShippingReferenceRow extends StatelessWidget {
  final ShippingInfo shipping;
  final bool isDark;

  const _HonestShippingReferenceRow({
    required this.shipping,
    required this.isDark,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final referenceType = shipping.referenceType ?? 'tracking';
    final reference = shipping.trackingNumber!;

    // Get honest label and icon based on reference type
    String getLabel() {
      switch (referenceType) {
        case 'phone':
          return 'No. HP / WA';
        case 'other':
          return 'Referensi';
        case 'tracking':
        default:
          return 'Nomor Resi';
      }
    }

    IconData getIcon() {
      switch (referenceType) {
        case 'phone':
          return Icons.phone_outlined;
        case 'other':
          return Icons.description_outlined;
        case 'tracking':
        default:
          return Icons.receipt_long_outlined;
      }
    }

    return Row(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Icon(getIcon(), size: 16, color: Colors.grey),
        const SizedBox(width: 8),
        SizedBox(
          width: 92,
          child: Text(
            getLabel(),
            style: theme.textTheme.bodySmall?.copyWith(color: Colors.grey),
          ),
        ),
        Expanded(
          child: Text(
            reference,
            style: theme.textTheme.bodySmall?.copyWith(
              color: isDark ? Colors.white : Colors.black87,
              fontFamily: 'monospace',
            ),
          ),
        ),
      ],
    );
  }
}

/// SHIPPING CONFIRMATION TRUTH: Shipping note section
///
/// Displays seller's shipping note to provide buyer context
class _ShippingNoteSection extends StatelessWidget {
  final String note;
  final bool isDark;

  const _ShippingNoteSection({required this.note, required this.isDark});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);

    return Container(
      padding: const EdgeInsets.all(10),
      decoration: BoxDecoration(
        color: isDark
            ? const Color(0xFF2A2A2A)
            : core.AppColors.primaryBlue.withValues(alpha: 0.05),
        borderRadius: BorderRadius.circular(8),
        border: Border.all(
          color: isDark
              ? const Color(0xFF333333)
              : core.AppColors.primaryBlue.withValues(alpha: 0.2),
        ),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.info_outline, size: 14, color: core.AppColors.primaryBlue),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              note,
              style: theme.textTheme.bodySmall?.copyWith(
                color: isDark ? Colors.white70 : Colors.black87,
                fontStyle: FontStyle.italic,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
