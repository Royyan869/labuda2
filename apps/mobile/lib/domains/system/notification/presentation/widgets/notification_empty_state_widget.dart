import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_filter.dart';

/// Notification Empty State Widget
///
/// Professional empty state dengan illustration.
/// Shows contextual message based on active filter.
///
/// Size: < 150 lines (per GUIDELINES)
class NotificationEmptyStateWidget extends StatelessWidget {
  final NotificationFilter filter;

  const NotificationEmptyStateWidget({
    super.key,
    this.filter = NotificationFilter.all,
  });

  @override
  Widget build(BuildContext context) {
    final (title, description) = _getEmptyStateMessage();
    final scheme = Theme.of(context).colorScheme;

    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p32),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            // Empty illustration
            Container(
              width: 160,
              height: 160,
              decoration: BoxDecoration(
                color: scheme.surfaceContainerHighest,
                shape: BoxShape.circle,
              ),
              child: Stack(
                alignment: Alignment.center,
                children: [
                  Icon(filter.icon, size: 80, color: scheme.outlineVariant),
                  Positioned(
                    right: 35,
                    top: 35,
                    child: Container(
                      width: 32,
                      height: 32,
                      decoration: BoxDecoration(
                        color: scheme.surface,
                        shape: BoxShape.circle,
                        border: Border.all(color: scheme.outlineVariant, width: 2),
                      ),
                      child: Icon(
                        Icons.check,
                        size: 20,
                        color: scheme.onSurfaceVariant,
                      ),
                    ),
                  ),
                ],
              ),
            ),
            const SizedBox(height: 32),

            // Title
            Text(
              title,
              style: TextStyle(
                fontSize: AppType.s20,
                fontWeight: FontWeight.w600,
                color: scheme.onSurface,
                letterSpacing: -0.5,
              ),
            ),
            const SizedBox(height: 12),

            // Description
            Text(
              description,
              textAlign: TextAlign.center,
              style: TextStyle(
                fontSize: AppType.s15,
                color: scheme.onSurfaceVariant,
                height: 1.5,
              ),
            ),
            const SizedBox(height: 8),

            // Info badges
            if (filter == NotificationFilter.all)
              Wrap(
                alignment: WrapAlignment.center,
                spacing: 8,
                runSpacing: 8,
                children: [
                  _InfoChip(
                    icon: Icons.shopping_bag_outlined,
                    label: 'Pesanan',
                    color: context.statusColors.success,
                  ),
                  _InfoChip(
                    icon: Icons.chat_bubble_outline,
                    label: 'Chat',
                    color: scheme.secondary,
                  ),
                  _InfoChip(
                    icon: Icons.gavel_outlined,
                    label: 'Lelang',
                    color: context.statusColors.warning,
                  ),
                ],
              ),
          ],
        ),
      ),
    );
  }

  /// Get contextual empty state message based on filter
  (String, String) _getEmptyStateMessage() {
    switch (filter) {
      case NotificationFilter.all:
        return (
          'Belum ada notifikasi',
          'Anda akan menerima notifikasi untuk\naktivitas penting di sini',
        );
      case NotificationFilter.order:
        return (
          'Belum ada notifikasi pesanan',
          'Notifikasi untuk pesanan Anda akan muncul di sini',
        );
      case NotificationFilter.dispute:
        return (
          'Belum ada notifikasi sengketa',
          'Notifikasi untuk sengketa dan banding akan muncul di sini',
        );
      case NotificationFilter.payout:
        return (
          'Belum ada notifikasi pembayaran',
          'Notifikasi untuk pembayaran dan penarikan akan muncul di sini',
        );
      case NotificationFilter.support:
        return (
          'Belum ada notifikasi bantuan',
          'Notifikasi untuk tiket bantuan dan keamanan akan muncul di sini',
        );
    }
  }
}

/// Info chip widget
class _InfoChip extends StatelessWidget {
  final IconData icon;
  final String label;
  final Color color;

  const _InfoChip({
    required this.icon,
    required this.label,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p6),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.12),
        borderRadius: BorderRadius.circular(AppShape.r16),
        border: Border.all(color: color.withValues(alpha: 0.3), width: 1),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 16, color: color),
          const SizedBox(width: 6),
          Text(
            label,
            style: TextStyle(
              fontSize: AppType.s13,
              fontWeight: FontWeight.w500,
              color: color,
            ),
          ),
        ],
      ),
    );
  }
}
