part of 'order_widgets_impl.dart';

/// Shipping proof data class for ship order action
///
/// SHIPPING CONFIRMATION TRUTH:
/// - shippingReference: REQUIRED - resi, phone/WA, or other reference
/// - referenceType: Optional hint for UI ("tracking" | "phone" | "other")
/// - note: Optional shipping note
class ShippingProofData {
  final String shippingReference;
  final String? referenceType;
  final String? note;

  const ShippingProofData({
    required this.shippingReference,
    this.referenceType,
    this.note,
  });

  // Backward compatibility constructor
  ShippingProofData.withLegacyTracking({
    required String trackingNumber,
    this.note,
  }) : shippingReference = trackingNumber,
       referenceType = null;

  Map<String, dynamic> toJson() => {
    'shippingReference': shippingReference,
    'referenceType': referenceType,
    'note': note,
  };
}

// =============================================================================
// OVERDUE INDICATOR WIDGET - Seller Awareness
// =============================================================================

/// OrderOverdueIndicator - Shows overdue status badge on order cards
///
/// This widget provides seller awareness for overdue orders.
/// It displays a badge indicating the overdue tier when an order is past
/// the ready_to_ship_by deadline.
///
/// BUSINESS RULES:
/// - Only shows for paid orders that are overdue
/// - Does NOT show for shipped, completed, cancelled orders
/// - Badge color and text based on overdue tier
class OrderOverdueIndicator extends StatelessWidget {
  final Order order;

  const OrderOverdueIndicator({super.key, required this.order});

  @override
  Widget build(BuildContext context) {
    // Only show for paid overdue orders
    if (order.status != OrderStatus.paid) {
      return const SizedBox.shrink();
    }
    if (order.isOverdue != true || order.overdueTier == null) {
      return const SizedBox.shrink();
    }

    final tier = order.overdueTier!;
    final daysOverdue = order.overdueDays ?? 0;

    Color getBadgeColor() {
      if (tier == 'critical_overdue') return core.AppColors.statusError;
      if (tier == 'severely_overdue') return core.AppColors.statusError;
      return core.AppColors.statusWarning;
    }

    String getBadgeLabel() {
      if (tier == 'critical_overdue') return 'Sangat Terlambat';
      if (tier == 'severely_overdue') return 'Terlambat';
      return 'Lewat Estimasi';
    }

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        color: getBadgeColor().withValues(alpha: 0.1),
        borderRadius: BorderRadius.circular(4),
        border: Border.all(
          color: getBadgeColor().withValues(alpha: 0.3),
          width: 1,
        ),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(Icons.warning_amber_rounded, size: 12, color: getBadgeColor()),
          const SizedBox(width: 4),
          Text(
            getBadgeLabel(),
            style: TextStyle(
              fontSize: 11,
              fontWeight: FontWeight.w600,
              color: getBadgeColor(),
            ),
          ),
          if (daysOverdue > 0) ...[
            const SizedBox(width: 4),
            Text(
              '($daysOverdue hari)',
              style: TextStyle(
                fontSize: 10,
                color: getBadgeColor().withValues(alpha: 0.8),
              ),
            ),
          ],
        ],
      ),
    );
  }
}

/// OrderOverdueInfoCard - Detailed overdue information for seller order detail
///
/// Shows comprehensive overdue information including:
/// - Overdue tier badge
/// - Days overdue count
/// - Ready to ship by deadline
/// - Warning message for seller
class OrderOverdueInfoCard extends StatelessWidget {
  final Order order;

  const OrderOverdueInfoCard({super.key, required this.order});

  @override
  Widget build(BuildContext context) {
    // Only show for paid overdue orders
    if (order.status != OrderStatus.paid) {
      return const SizedBox.shrink();
    }
    if (order.isOverdue != true || order.overdueTier == null) {
      return const SizedBox.shrink();
    }

    final tier = order.overdueTier!;
    final daysOverdue = order.overdueDays ?? 0;
    final isDark = Theme.of(context).brightness == Brightness.dark;

    Color getBadgeColor() {
      if (tier == 'critical_overdue') return core.AppColors.statusError;
      if (tier == 'severely_overdue') return core.AppColors.statusError;
      return core.AppColors.statusWarning;
    }

    String getBadgeLabel() {
      if (tier == 'critical_overdue') return 'Sangat Terlambat';
      if (tier == 'severely_overdue') return 'Terlambat';
      return 'Melewati Estimasi';
    }

    String getWarningMessage() {
      if (tier == 'critical_overdue') {
        return 'Pesanan ini sangat terlambat. Segera kirim pesanan atau hubungi pembeli.';
      } else if (tier == 'severely_overdue') {
        return 'Pesanan ini terlambat. Mohon segera kirim pesanan.';
      } else {
        return 'Pesanan ini melewati estimasi siap kirim.';
      }
    }

    return Container(
      margin: const EdgeInsets.only(bottom: 16),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: isDark
            ? core.AppColors.statusError.withValues(alpha: 0.08)
            : core.AppColors.statusError.withValues(alpha: 0.05),
        borderRadius: BorderRadius.circular(12),
        border: Border.all(
          color: isDark
              ? core.AppColors.statusError.withValues(alpha: 0.3)
              : core.AppColors.statusError.withValues(alpha: 0.2),
        ),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header with badge
          Row(
            children: [
              Container(
                padding: const EdgeInsets.all(8),
                decoration: BoxDecoration(
                  color: getBadgeColor().withValues(alpha: 0.15),
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  Icons.warning_amber_rounded,
                  size: 18,
                  color: getBadgeColor(),
                ),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Pesanan Lewat Waktu',
                      style: TextStyle(
                        fontSize: 14,
                        fontWeight: FontWeight.w600,
                        color: isDark
                            ? core.AppColors.neutralWhite
                            : core.AppColors.neutralGray900,
                      ),
                    ),
                    Text(
                      getBadgeLabel(),
                      style: TextStyle(
                        fontSize: 12,
                        color: getBadgeColor(),
                        fontWeight: FontWeight.w500,
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),

          const SizedBox(height: 12),

          // Warning message
          Container(
            padding: const EdgeInsets.all(12),
            decoration: BoxDecoration(
              color: isDark
                  ? core.AppColors.darkGray700.withValues(alpha: 0.5)
                  : core.AppColors.neutralGray100,
              borderRadius: BorderRadius.circular(8),
            ),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Icon(Icons.info_outline, size: 16, color: getBadgeColor()),
                const SizedBox(width: 8),
                Expanded(
                  child: Text(
                    getWarningMessage(),
                    style: TextStyle(
                      fontSize: 12,
                      color: isDark
                          ? core.AppColors.neutralGray300
                          : core.AppColors.neutralGray700,
                    ),
                  ),
                ),
              ],
            ),
          ),

          // Ready to ship by deadline
          if (order.readyToShipBy != null) ...[
            const SizedBox(height: 12),
            Row(
              children: [
                Icon(
                  Icons.event,
                  size: 14,
                  color: isDark
                      ? core.AppColors.neutralGray400
                      : core.AppColors.neutralGray600,
                ),
                const SizedBox(width: 6),
                Text(
                  'Target siap kirim: ${_formatDate(order.readyToShipBy!)}',
                  style: TextStyle(
                    fontSize: 11,
                    color: isDark
                        ? core.AppColors.neutralGray400
                        : core.AppColors.neutralGray600,
                  ),
                ),
                if (daysOverdue > 0) ...[
                  const Spacer(),
                  Text(
                    'Telat $daysOverdue hari',
                    style: TextStyle(
                      fontSize: 11,
                      fontWeight: FontWeight.w600,
                      color: getBadgeColor(),
                    ),
                  ),
                ],
              ],
            ),
          ],
        ],
      ),
    );
  }

  String _formatDate(DateTime date) {
    return '${date.day}/${date.month}/${date.year}';
  }
}
