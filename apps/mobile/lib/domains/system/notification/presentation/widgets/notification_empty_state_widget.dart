import 'package:flutter/material.dart';
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

    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            // Empty illustration
            Container(
              width: 160,
              height: 160,
              decoration: BoxDecoration(
                color: Colors.grey[100],
                shape: BoxShape.circle,
              ),
              child: Stack(
                alignment: Alignment.center,
                children: [
                  Icon(filter.icon, size: 80, color: Colors.grey[300]),
                  Positioned(
                    right: 35,
                    top: 35,
                    child: Container(
                      width: 32,
                      height: 32,
                      decoration: BoxDecoration(
                        color: Colors.white,
                        shape: BoxShape.circle,
                        border: Border.all(color: Colors.grey[200]!, width: 2),
                      ),
                      child: Icon(
                        Icons.check,
                        size: 20,
                        color: Colors.grey[400],
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
              style: const TextStyle(
                fontSize: 20,
                fontWeight: FontWeight.w600,
                color: Colors.grey,
                letterSpacing: -0.5,
              ),
            ),
            const SizedBox(height: 12),

            // Description
            Text(
              description,
              textAlign: TextAlign.center,
              style: TextStyle(
                fontSize: 15,
                color: Colors.grey[600],
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
                    color: Colors.green,
                  ),
                  _InfoChip(
                    icon: Icons.chat_bubble_outline,
                    label: 'Chat',
                    color: Colors.blue,
                  ),
                  _InfoChip(
                    icon: Icons.gavel_outlined,
                    label: 'Lelang',
                    color: Colors.amber,
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
  final MaterialColor color;

  const _InfoChip({
    required this.icon,
    required this.label,
    required this.color,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
      decoration: BoxDecoration(
        color: color[50],
        borderRadius: BorderRadius.circular(16),
        border: Border.all(color: color[100]!, width: 1),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 16, color: color[700]),
          const SizedBox(width: 6),
          Text(
            label,
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w500,
              color: color[800],
            ),
          ),
        ],
      ),
    );
  }
}
