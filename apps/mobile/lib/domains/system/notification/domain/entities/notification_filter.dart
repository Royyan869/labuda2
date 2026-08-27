import 'package:flutter/material.dart';
import 'package:labuda/core/interfaces/i_notification_trigger.dart';

/// Notification Filter Enum
///
/// Defines filter categories for notification list.
/// Maps notification types to user-friendly tabs.
enum NotificationFilter {
  all('All'),
  order('Order'),
  dispute('Dispute'),
  payout('Payout'),
  support('Support');

  const NotificationFilter(this.label);
  final String label;

  /// Get display label
  String get displayLabel {
    switch (this) {
      case NotificationFilter.all:
        return 'Semua';
      case NotificationFilter.order:
        return 'Pesanan';
      case NotificationFilter.dispute:
        return 'Sengketa';
      case NotificationFilter.payout:
        return 'Pembayaran';
      case NotificationFilter.support:
        return 'Bantuan';
    }
  }

  /// Get icon for this filter
  IconData get icon {
    switch (this) {
      case NotificationFilter.all:
        return Icons.notifications;
      case NotificationFilter.order:
        return Icons.shopping_cart_outlined;
      case NotificationFilter.dispute:
        return Icons.warning_outlined;
      case NotificationFilter.payout:
        return Icons.account_balance_wallet_outlined;
      case NotificationFilter.support:
        return Icons.support_agent_outlined;
    }
  }

  /// Check if notification type matches this filter
  bool matches(NotificationType type) {
    switch (this) {
      case NotificationFilter.all:
        return true;

      case NotificationFilter.order:
        return type.name.contains('order') ||
            type.name.contains('payment') ||
            type.name.contains('guarantee') ||
            type.name.contains('refund');

      case NotificationFilter.dispute:
        return type.name.contains('dispute') ||
            type.name.contains('moderation');

      case NotificationFilter.payout:
        return type.name.contains('payout') ||
            type.name.contains('wallet') ||
            type.name.contains('withdrawal');

      case NotificationFilter.support:
        return type.name.contains('support') || type.name.contains('ticket');
    }
  }
}
