import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'in_app_banner_service.dart';
import 'notification_navigation_service.dart';

/// FCM Action Mapper
///
/// Maps notification types to action buttons for in-app banners.
/// Extracted from fcm_service.dart for better modularity.
///
/// Responsibilities:
/// - Define action buttons per notification type
/// - Handle action button taps
/// - Navigate to appropriate screens
///
/// Size: < 200 lines (per GUIDELINES)
class FCMActionMapper {
  FCMActionMapper();

  /// Get action buttons for notification type
  ///
  /// Returns appropriate action buttons based on notification type.
  /// Returns null for notifications that should use default onTap.
  List<BannerAction>? getActionsForType(
    String? type,
    Map<String, dynamic> data,
  ) {
    if (type == null) return null;

    switch (type) {
      // Follow - View profile (canonical wire type)
      case 'user.followed':
        return [
          BannerAction(
            label: 'Lihat',
            icon: Icons.person,
            onTap: () => _navigate(type, data),
          ),
        ];

      // Like - View post (canonical wire type)
      case 'content.liked':
        return [
          BannerAction(
            label: 'Lihat Post',
            icon: Icons.visibility,
            onTap: () => _navigate(type, data),
          ),
        ];

      // Comment - View & Reply (canonical wire types)
      case 'comment':
      case 'comment_reply':
        return [
          BannerAction(
            label: 'Lihat',
            icon: Icons.visibility,
            onTap: () => _navigate(type, data),
          ),
        ];

      // Mention - View
      case 'content.mentioned':
        return [
          BannerAction(
            label: 'Lihat',
            icon: Icons.visibility,
            onTap: () => _navigate(type, data),
          ),
        ];

      // Chat message - Reply (canonical wire type)
      case 'chat_message':
        return [
          BannerAction(
            label: 'Balas',
            icon: Icons.reply,
            onTap: () => _navigate(type, data),
          ),
        ];

      // Auction notifications (canonical wire types)
      case 'auction.bid.placed':
      case 'auction.waiting_settlement':
      case 'auction.seller_has_winner':
      case 'auction.ended_no_winner':
      case 'auction.bnr_seller':
        return [
          BannerAction(
            label: 'Lihat Lelang',
            icon: Icons.gavel,
            tone: BannerTone.warning,
            onTap: () => _navigate(type, data),
          ),
        ];

      // Auction won
      case 'auction.bnr_winner':
        return [
          BannerAction(
            label: 'Bayar Sekarang',
            icon: Icons.payment,
            tone: BannerTone.success,
            onTap: () => _navigate(type, data),
          ),
        ];

      // Order notifications (canonical wire types)
      case 'order.created':
      case 'order.created.buyer':
      case 'order.paid':
      case 'order.paid.buyer':
        return [
          BannerAction(
            label: 'Lihat Order',
            icon: Icons.receipt_long,
            onTap: () => _navigate(type, data),
          ),
        ];

      // Order shipped
      case 'order.shipped':
        return [
          BannerAction(
            label: 'Lacak Paket',
            icon: Icons.local_shipping,
            onTap: () => _navigate(type, data),
          ),
        ];

      // Order delivered / completed
      case 'order.completed':
        return [
          BannerAction(
            label: 'Konfirmasi Terima',
            icon: Icons.check_circle,
            tone: BannerTone.success,
            onTap: () => _navigate(type, data),
          ),
        ];

      // Refund / dispute notifications (canonical wire types)
      case 'refund.opened':
      case 'refund.approved':
      case 'refund.rejected':
      case 'refund.escalated':
      case 'order.refunded':
      case 'order.partially_refunded':
      case 'dispute.opened':
      case 'dispute.resolved':
        return [
          BannerAction(
            label: 'Lihat Detail',
            icon: Icons.info_outline,
            onTap: () => _navigate(type, data),
          ),
        ];

      // Default - no actions (use default onTap)
      default:
        return null;
    }
  }

  /// Navigate through the ONE notification destination decision.
  /// Fetch a fresh context from the global navigatorKey at tap time.
  void _navigate(String type, Map<String, dynamic> data) {
    // Use global navigatorKey from app_router.dart
    final freshContext = navigatorKey.currentContext;

    if (freshContext != null && freshContext.mounted) {
      NotificationNavigationService.canonical().handleNotificationPayload(
        freshContext,
        type: type,
        data: data,
      );
    }
  }
}
