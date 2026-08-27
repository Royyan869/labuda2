import 'dart:async';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';

/// Notification Navigation Handler
///
/// Centralized navigation logic for all notification types.
/// Used by:
/// - FCM Service (push notifications)
/// - Local Notification Service
/// - In-App Notification List
///
/// Supports all NotificationType enum values from i_notification_trigger.dart
class NotificationNavigationHandler {
  /// Navigate based on notification type and data
  ///
  /// [context] - BuildContext for navigation
  /// [type] - Notification type string (e.g., 'chat_message', 'order_created')
  /// [data] - Map containing navigation parameters (IDs, etc.)
  ///
  /// Returns true if navigation succeeded, false otherwise
  static bool navigate({
    required BuildContext context,
    required String type,
    required Map<String, dynamic> data,
  }) {
    if (!context.mounted) {
      return false;
    }

    try {
      return _navigateByType(context, type, data);
    } catch (e) {
      _showError(context, 'Failed to open notification');
      return false;
    }
  }

  /// Internal navigation dispatcher
  static bool _navigateByType(
    BuildContext context,
    String type,
    Map<String, dynamic> data,
  ) {
    switch (type) {
      // ========================================
      // Chat Notifications
      // ========================================
      case 'chat_message':
      case 'new_chat_message': // Backend alias
        return _navigateToChat(context, data);

      // ========================================
      // Social/Engagement Notifications
      // ========================================
      // BATCH N1: Removed legacy 'follower' and 'like' aliases
      // Canonical types: user.followed, content.liked, comment, comment.reply, seller.response
      case 'user.followed': // Backend sends snake_case
        return _navigateToProfile(context, data);

      case 'comment':
      case 'comment_reply': // Backend sends snake_case
        return _navigateToContentByTarget(context, data);

      case 'content.liked': // Canonical: backend sends this for like notifications
        return _navigateToContentByTarget(context, data);

      case 'mention':
        return _navigateToMention(context, data);

      case 'seller.response': // Seller responded to a request - navigate to request
        return _navigateToContent(context, data);

      case 'verification.document.approved':
      case 'verification.document.rejected':
      case 'seller.verification.submitted':
      case 'seller.verification.approved':
      case 'seller.verification.rejected':
      case 'seller.verification.needs_resubmission':
      case 'seller.verification.suspended':
      case 'seller.verification.revoked':
      case 'seller.verification.under_investigation':
      case 'seller.verification.restored':
        return _navigateToSellerVerification(context);

      case 'seller.subscription.expiring':
      case 'seller.subscription.expired':
        return _navigateToSettings(context);

      case 'seller.tier.upgraded':
      case 'seller.tier.downgraded':
        return _navigateToSellerDashboard(context);

      // ========================================
      // Order Notifications
      // ========================================
      case 'order_created':
      case 'order.created': // Backend sends dot notation
      case 'order.created.buyer': // Buyer confirmation for order created
      case 'order_confirmed':
      case 'order.paid': // Backend sends dot notation
      case 'order.paid.buyer': // Buyer confirmation for payment
      case 'order_shipped':
      case 'order.shipped': // Backend sends dot notation
      case 'order_delivered':
      case 'order_cancelled':
      case 'order.cancelled': // Backend sends dot notation
      case 'order.completed': // Backend sends dot notation
      case 'order.expired': // Backend sends dot notation
      case 'order.refunded': // Backend sends dot notation
      case 'order.partially_refunded': // Backend sends dot notation
      case 'order.dispute_open': // Dispute opened (pre-release)
      case 'order.cancelled_timeout': // Seller non-shipment timeout
      case 'order.confirmation_extended': // Buyer extended confirmation
        return _navigateToOrder(context, data);

      // ========================================
      // Refund Notifications
      // ========================================
      case 'refund.opened': // Canonical backend dot notation
      case 'refund.approved': // Canonical backend dot notation
      case 'refund.rejected': // Canonical backend dot notation
      case 'refund.escalated': // Canonical backend dot notation
      case 'refund_requested': // Legacy alias kept
      case 'refund_approved': // Legacy alias kept
      case 'refund_rejected': // Legacy alias kept
      case 'refund_processed': // Legacy alias kept
        return _navigateToOrder(context, data);

      // ========================================
      // Dispute Notifications
      // ========================================
      case 'dispute.opened': // Post-release dispute opened (D1B)
      case 'dispute.resolved': // Dispute resolved
      case 'dispute.overdue': // Dispute overdue — admin fanout (G1)
      case 'dispute.timeout_escalation': // Dispute timeout escalated — admin fanout (G1)
        return _navigateToOrder(context, data);

      // ========================================
      // Negotiation Notifications
      // ========================================
      case 'negotiation.started':
      case 'negotiation.message_sent':
      case 'negotiation.accepted':
      case 'negotiation.expired':
      case 'negotiation.cancelled':
        return _navigateToNegotiationChat(context, data);

      // ========================================
      // Listing/Catalog Notifications
      // ========================================
      // BATCH N1: Removed auction notification types (auction_bid_placed, auction_ending, auction_won)
      // Backend emits auction events but doesn't handle them for notifications - ghost code removed
      case 'promotion':
        return _navigateToPromotion(context, data);

      // ========================================
      // Support Ticket Notifications (Chat with Admin)
      // ========================================
      case 'support_ticket_created':
      case 'support.ticket.created':
      case 'support_ticket_claimed':
      case 'support.ticket.claimed':
      case 'support_ticket_resolved':
      case 'support.ticket.resolved':
      case 'support_ticket_closed':
      case 'support.ticket.closed':
      case 'support_ticket_user_responded':
      case 'support.ticket.user_responded':
      case 'support.ticket_waiting_user':
        return _navigateToSupportTicket(context, data);

      // ========================================
      // System Notifications (no navigation - shown as modal)
      // ========================================
      case 'announcement':
      case 'system_maintenance':
        // These are handled as modals in NotificationListScreen
        // No navigation needed from FCM banner tap
        return true; // Not an error, just different handling

      default:
        _showError(context, 'Tipe notifikasi tidak dikenal: $type');
        return false;
    }
  }

  // ============================================================================
  // PRIVATE NAVIGATION METHODS
  // ============================================================================

  /// Show error message to user
  static void _showError(BuildContext context, String message) {
    if (!context.mounted) return;

    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(message),
        backgroundColor: Colors.red,
        behavior: SnackBarBehavior.floating,
        duration: const Duration(seconds: 4),
      ),
    );
  }

  /// Navigate to Chat Detail
  static bool _navigateToChat(BuildContext context, Map<String, dynamic> data) {
    final chatId = data['chatRoomId'] as String? ?? data['chatId'] as String?;

    if (chatId == null || chatId.isEmpty) {
      _showError(context, 'Chat ID tidak ditemukan');
      return false;
    }

    try {
      final appRouter = AppRouter();

      // Build stack: First go to home, then push target
      // This ensures back button returns to home, not exit app
      appRouter.navigateToHome(); // Step 1: Ensure home in stack

      // Step 2: Wait for home to load, then push target
      Timer(const Duration(milliseconds: 600), () {
        appRouter.navigateToChatConversation(chatId);
      });

      return true;
    } catch (e) {
      _showError(context, 'Gagal membuka chat');
      return false;
    }
  }

  /// Navigate to support ticket thread when ticketId is available.
  /// Falls back to the chat room when the backend only gives chatRoomId.
  static bool _navigateToSupportTicket(
    BuildContext context,
    Map<String, dynamic> data,
  ) {
    final ticketId =
        data['ticketId'] as String? ?? data['ticket_id'] as String?;
    if (ticketId != null && ticketId.isNotEmpty) {
      try {
        final appRouter = AppRouter();
        appRouter.navigateTo(
          RouteNames.supportTicketThread,
          parameters: {'ticketId': ticketId},
        );
        return true;
      } catch (e) {
        _showError(context, 'Gagal membuka tiket bantuan');
        return false;
      }
    }

    final chatRoomId =
        data['chatRoomId'] as String? ?? data['chatId'] as String?;
    if (chatRoomId != null && chatRoomId.isNotEmpty) {
      return _navigateToChat(context, data);
    }

    _showError(context, 'Data tiket bantuan tidak ditemukan');
    return false;
  }

  /// Navigate to Negotiation Chat Detail
  static bool _navigateToNegotiationChat(
    BuildContext context,
    Map<String, dynamic> data,
  ) {
    final chatRoomId = data['chatRoomId'] as String?;

    if (chatRoomId == null || chatRoomId.isEmpty) {
      _showError(context, 'Chat room ID tidak ditemukan');
      return false;
    }

    try {
      final appRouter = AppRouter();

      appRouter.navigateToHome();
      Timer(const Duration(milliseconds: 600), () {
        appRouter.navigateToChatConversation(chatRoomId);
      });

      return true;
    } catch (e) {
      _showError(context, 'Gagal membuka chat');
      return false;
    }
  }

  /// Navigate to Profile
  static bool _navigateToProfile(
    BuildContext context,
    Map<String, dynamic> data,
  ) {
    final userId = data['userId'] as String?;

    if (userId == null || userId.isEmpty) {
      _showError(context, 'User ID tidak ditemukan');
      return false;
    }

    try {
      final appRouter = AppRouter();

      appRouter.navigateToHome();
      Timer(const Duration(milliseconds: 600), () {
        appRouter.navigateToUserProfile(userId);
      });

      return true;
    } catch (e) {
      _showError(context, 'Gagal membuka profil');
      return false;
    }
  }

  /// Navigate to content by targetId and targetType
  /// Used for comment, like notifications
  static bool _navigateToContentByTarget(
    BuildContext context,
    Map<String, dynamic> data,
  ) {
    final targetId = _firstString(data, ['targetId', 'target_id']);
    final rawTargetType = _firstString(data, ['targetType', 'target_type']);
    final commentContentId = _firstString(data, [
      'parentContentId',
      'parent_content_id',
      'contentId',
      'content_id',
      'targetContentId',
      'target_content_id',
      'targetId',
      'target_id',
    ]);

    if (rawTargetType == null || rawTargetType.isEmpty) {
      return _navigateToNotifications(context);
    }
    if (rawTargetType != 'comment' && (targetId == null || targetId.isEmpty)) {
      return _navigateToNotifications(context);
    }

    try {
      final appRouter = AppRouter();

      appRouter.navigateToHome();
      Timer(const Duration(milliseconds: 600), () {
        switch (rawTargetType) {
          case 'content':
            if (targetId != null && targetId.isNotEmpty) {
              appRouter.navigateToContentDetail(targetId);
            } else {
              _navigateToNotifications(context);
            }
            break;
          case 'listing':
            if (targetId != null && targetId.isNotEmpty) {
              appRouter.navigateToForSaleDetail(targetId);
            } else {
              _navigateToNotifications(context);
            }
            break;
          case 'comment':
            if (commentContentId != null && commentContentId.isNotEmpty) {
              appRouter.navigateToContentDetail(commentContentId);
            } else {
              _navigateToNotifications(context);
            }
            break;
          // BATCH N1: Removed 'auction' case - likes/comments don't support auction targets
          default:
            _navigateToNotifications(context);
            break;
        }
      });

      return true;
    } catch (e) {
      _showError(context, 'Gagal membuka konten');
      return false;
    }
  }

  /// Navigate to mention - can be in content, comment, or chat
  static bool _navigateToMention(
    BuildContext context,
    Map<String, dynamic> data,
  ) {
    try {
      final appRouter = AppRouter();

      appRouter.navigateToHome();
      Timer(const Duration(milliseconds: 600), () {
        final chatId = data['chatId'] as String?;
        if (chatId != null && chatId.isNotEmpty) {
          appRouter.navigateToChatConversation(chatId);
          return;
        }

        final contentId =
            data['contentId'] as String? ?? data['content_id'] as String?;
        if (contentId != null && contentId.isNotEmpty) {
          appRouter.navigateToContentDetail(contentId);
          return;
        }

        _navigateToContentByTarget(context, data);
      });

      return true;
    } catch (e) {
      _showError(context, 'Gagal membuka mention');
      return false;
    }
  }

  /// Navigate to Content Detail (content)
  static bool _navigateToContent(
    BuildContext context,
    Map<String, dynamic> data,
  ) {
    final contentId =
        data['contentId'] as String? ?? data['content_id'] as String?;

    if (contentId == null || contentId.isEmpty) {
      _showError(context, 'Content ID tidak ditemukan');
      return false;
    }

    try {
      final appRouter = AppRouter();

      appRouter.navigateToHome();
      Timer(const Duration(milliseconds: 600), () {
        appRouter.navigateToContentDetail(contentId);
      });

      return true;
    } catch (e) {
      _showError(context, 'Gagal membuka konten');
      return false;
    }
  }

  /// Navigate to seller verification screen.
  static bool _navigateToSellerVerification(BuildContext context) {
    try {
      AppRouter().navigateToSellerVerification();
      return true;
    } catch (e) {
      _showError(context, 'Gagal membuka verifikasi penjual');
      return false;
    }
  }

  /// Navigate to seller dashboard screen.
  static bool _navigateToSellerDashboard(BuildContext context) {
    try {
      AppRouter().navigateToSellerDashboard();
      return true;
    } catch (e) {
      _showError(context, 'Gagal membuka dashboard penjual');
      return false;
    }
  }

  /// Navigate to notifications list.
  static bool _navigateToNotifications(BuildContext context) {
    try {
      AppRouter().navigateToNotifications();
      return true;
    } catch (e) {
      _showError(context, 'Gagal membuka notifikasi');
      return false;
    }
  }

  static bool _navigateToSettings(BuildContext context) {
    try {
      AppRouter().navigateToSettings();
      return true;
    } catch (e) {
      _showError(context, 'Gagal membuka pengaturan');
      return false;
    }
  }

  /// Navigate to promotion management, preferring the most specific route
  /// available in the notification payload.
  static bool _navigateToPromotion(
    BuildContext context,
    Map<String, dynamic> data,
  ) {
    final externalProductId = _firstString(data, [
      'externalProductId',
      'external_product_id',
    ]);
    if (externalProductId != null) {
      try {
        AppRouter().navigateToExternalProductDetail(externalProductId);
        return true;
      } catch (e) {
        _showError(context, 'Gagal membuka external product');
        return false;
      }
    }

    final promotionInstanceId = _firstString(data, [
      'promotionInstanceId',
      'promotion_instance_id',
    ]);
    if (promotionInstanceId != null) {
      try {
        final navigatorCtx = navigatorKey.currentContext;
        if (navigatorCtx != null) {
          GoRouter.of(
            navigatorCtx,
          ).push('/seller/promotions/$promotionInstanceId');
          return true;
        }
      } catch (e) {
        _showError(context, 'Gagal membuka promotion');
        return false;
      }
    }

    final ctaRoute = _firstString(data, ['ctaRoute', 'cta_route']);
    if (ctaRoute != null) {
      try {
        final navigatorCtx = navigatorKey.currentContext;
        if (navigatorCtx != null) {
          GoRouter.of(navigatorCtx).push(ctaRoute);
          return true;
        }
      } catch (e) {
        _showError(context, 'Gagal membuka promotion');
        return false;
      }
    }

    return _navigateToSellerDashboard(context);
  }

  /// Navigate to Order Detail
  static bool _navigateToOrder(
    BuildContext context,
    Map<String, dynamic> data,
  ) {
    final orderId = data['orderId'] as String?;

    if (orderId == null || orderId.isEmpty) {
      _showError(context, 'Order ID tidak ditemukan');
      return false;
    }

    try {
      final appRouter = AppRouter();

      appRouter.navigateToHome();
      Timer(const Duration(milliseconds: 600), () {
        appRouter.navigateToOrderDetail(orderId);
      });

      return true;
    } catch (e) {
      _showError(context, 'Gagal membuka order');
      return false;
    }
  }

  static String? _firstString(Map<String, dynamic> data, List<String> keys) {
    for (final key in keys) {
      final value = data[key];
      if (value is String && value.isNotEmpty) {
        return value;
      }
    }
    return null;
  }
}
