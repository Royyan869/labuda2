/// Order Confirmation Section
///
/// Displays confirmation/verification information for shipped/delivered orders.
/// This section helps users understand the current state and available actions.
library;

import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart' as core;
import 'package:labuda/core/src/theme/app_theme.dart';
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
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    // Only show for shipped/delivered orders
    if (order.status != OrderStatus.shipped &&
        order.status != OrderStatus.delivered) {
      return const SizedBox.shrink();
    }

    return Container(
      padding: const EdgeInsets.all(core.AppMetrics.p16),
      margin: const EdgeInsets.only(bottom: core.AppMetrics.p16),
      decoration: BoxDecoration(
        color: colorScheme.surface,
        borderRadius: BorderRadius.circular(core.AppShape.r12),
        border: Border.all(
          color: _getBorderColorForStatus(context, order.status, colorScheme),
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
                  color: _getIconBgColorForStatus(context, 
                    order.status,
                    colorScheme,
                  ),
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  _getIconForStatus(order.status),
                  color: _getIconColorForStatus(context, order.status, colorScheme),
                  size: AppIconSize.action,
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
                        color: colorScheme.onSurfaceVariant,
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
            ),
          ],

          // Delivered date info (if available)
          if (order.deliveredAt != null) ...[
            const SizedBox(height: 8),
            _InfoRow(
              label: 'Tanggal Terkirim',
              value: AppFormatters.formatDateTime(order.deliveredAt!),
            ),
          ],

          // SHIPPING CONFIRMATION TRUTH: Shipping reference with honest label
          if (order.shippingInfo.trackingNumber != null &&
              order.shippingInfo.trackingNumber!.isNotEmpty) ...[
            const SizedBox(height: 8),
            _HonestShippingReferenceRow(
              shipping: order.shippingInfo,
            ),
          ],

          // SHIPPING CONFIRMATION TRUTH: Shipping note from seller
          if (order.shippingInfo.shippingNote != null &&
              order.shippingInfo.shippingNote!.isNotEmpty) ...[
            const SizedBox(height: 8),
            _ShippingNoteSection(
              note: order.shippingInfo.shippingNote!,
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
              padding: const EdgeInsets.all(core.AppMetrics.p12),
              decoration: BoxDecoration(
                color: context.statusColors.info.withValues(alpha: 0.1),
                borderRadius: BorderRadius.circular(core.AppShape.r8),
                border: Border.all(
                  color: context.statusColors.info.withValues(alpha: 0.3),
                ),
              ),
              child: Row(
                children: [
                  Icon(
                    Icons.info_outline,
                    color: context.statusColors.info,
                    size: AppIconSize.action,
                  ),
                  const SizedBox(width: 8),
                  Expanded(
                    child: Text(
                      'Jika barang sudah diterima dan sesuai, tap "Terima Barang" untuk menyelesaikan pesanan.',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: context.statusColors.info,
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

  Color _getBorderColorForStatus(BuildContext context, OrderStatus status, ColorScheme colorScheme) {
    switch (status) {
      case OrderStatus.shipped:
        return context.statusColors.info.withValues(alpha: 0.5);
      case OrderStatus.delivered:
        return colorScheme.primary.withValues(alpha: 0.5);
      default:
        return colorScheme.outlineVariant;
    }
  }

  Color _getIconBgColorForStatus(BuildContext context, OrderStatus status, ColorScheme colorScheme) {
    switch (status) {
      case OrderStatus.shipped:
        return context.statusColors.info.withValues(alpha: 0.1);
      case OrderStatus.delivered:
        return colorScheme.primary.withValues(alpha: 0.1);
      default:
        return colorScheme.onSurfaceVariant.withValues(alpha: 0.1);
    }
  }

  Color _getIconColorForStatus(BuildContext context, OrderStatus status, ColorScheme colorScheme) {
    switch (status) {
      case OrderStatus.shipped:
        return context.statusColors.info;
      case OrderStatus.delivered:
        return colorScheme.primary;
      default:
        return colorScheme.onSurfaceVariant;
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
  final bool isMonospace = false;

  const _InfoRow({
    required this.label,
    required this.value,
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
            style: theme.textTheme.bodySmall?.copyWith(
              color: colorScheme.onSurfaceVariant,
            ),
          ),
        ),
        Expanded(
          child: Text(
            value,
            style: theme.textTheme.bodySmall?.copyWith(
              color: colorScheme.onSurface,
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

  const _HonestShippingReferenceRow({required this.shipping});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;
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
        Icon(getIcon(), size: AppIconSize.inlineGlyph, color: colorScheme.onSurfaceVariant),
        const SizedBox(width: 8),
        SizedBox(
          width: 92,
          child: Text(
            getLabel(),
            style: theme.textTheme.bodySmall?.copyWith(
              color: colorScheme.onSurfaceVariant,
            ),
          ),
        ),
        Expanded(
          child: Text(
            reference,
            style: theme.textTheme.bodySmall?.copyWith(
              color: colorScheme.onSurface,
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

  const _ShippingNoteSection({required this.note});

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Container(
      padding: const EdgeInsets.all(core.AppMetrics.p12),
      decoration: BoxDecoration(
        color: colorScheme.secondaryContainer,
        borderRadius: BorderRadius.circular(core.AppShape.r8),
        border: Border.all(
          color: colorScheme.secondary.withValues(alpha: 0.4),
        ),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Icon(Icons.info_outline, size: AppIconSize.inlineGlyph, color: colorScheme.secondary),
          const SizedBox(width: 8),
          Expanded(
            child: Text(
              note,
              style: theme.textTheme.bodySmall?.copyWith(
                color: colorScheme.onSurface,
                fontStyle: FontStyle.italic,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
