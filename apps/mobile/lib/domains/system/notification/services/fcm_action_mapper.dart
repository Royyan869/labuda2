import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/utils/notification_navigation_handler.dart';
import 'in_app_banner_service.dart';

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
      // Follow request - View profile
      case 'follower':
      case 'new_follower': // Backend alias
      case 'request':
        return [
          BannerAction(
            label: 'Lihat',
            icon: Icons.person,
            onTap: () => _navigate(type, data),
          ),
        ];

      // Like - View post
      case 'like':
      case 'new_like': // Backend alias
        return [
          BannerAction(
            label: 'Lihat Post',
            icon: Icons.visibility,
            onTap: () => _navigate(type, data),
          ),
        ];

      // Comment - View & Reply
      case 'comment':
      case 'new_comment': // Backend alias
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

      // Chat message - Reply
      case 'chat_message':
      case 'new_chat_message': // Backend alias
        return [
          BannerAction(
            label: 'Balas',
            icon: Icons.reply,
            color: Colors.blue,
            onTap: () => _navigate(type, data),
          ),
        ];

      // Auction notifications
      case 'auction_bid_placed':
      case 'auction_ending':
        return [
          BannerAction(
            label: 'Lihat Lelang',
            icon: Icons.gavel,
            color: Colors.orange,
            onTap: () => _navigate(type, data),
          ),
        ];

      // Auction won
      case 'auction_won':
        return [
          BannerAction(
            label: 'Bayar Sekarang',
            icon: Icons.payment,
            color: Colors.green,
            onTap: () => _navigate(type, data),
          ),
        ];

      // Order notifications
      case 'order_created':
      case 'order.created':
      case 'order.created.buyer':
      case 'order_confirmed':
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
      case 'order_shipped':
      case 'order.shipped':
        return [
          BannerAction(
            label: 'Lacak Paket',
            icon: Icons.local_shipping,
            color: Colors.blue,
            onTap: () => _navigate(type, data),
          ),
        ];

      // Order delivered
      case 'order_delivered':
        return [
          BannerAction(
            label: 'Konfirmasi Terima',
            icon: Icons.check_circle,
            color: Colors.green,
            onTap: () => _navigate(type, data),
          ),
        ];

      // Refund notifications
      case 'refund_requested':
      case 'refund.opened':
      case 'refund_approved':
      case 'refund.approved':
      case 'refund_rejected':
      case 'refund.rejected':
      case 'refund.escalated':
      case 'refund_processed':
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

  /// Navigate using NotificationNavigationHandler
  /// FIX: Fetch fresh context from global navigatorKey at navigation time
  void _navigate(String type, Map<String, dynamic> data) {
    // Use global navigatorKey from app_router.dart
    final freshContext = navigatorKey.currentContext;

    if (freshContext != null && freshContext.mounted) {
      NotificationNavigationHandler.navigate(
        context: freshContext,
        type: type,
        data: data,
      );
    }
  }
}
