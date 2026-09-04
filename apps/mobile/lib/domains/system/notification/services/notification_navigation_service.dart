/// Notification Navigation Service
///
/// Handles all navigation logic for notification taps.
/// Extracted from notification_list_screen to comply with file size limits.
///
/// Responsibilities:
/// - Route notifications to appropriate screens
/// - Handle deep linking
/// - Display modals for system notifications
///
/// Size: < 250 lines (per GUIDELINES)
library;

// Dart
import 'package:labuda/core/interfaces/i_notification_trigger.dart';
import 'package:labuda/core/navigation/navigation_handler.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';

// Flutter
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

class NotificationNavigationService {
  final NavigationHandler _navigationHandler;

  NotificationNavigationService(this._navigationHandler);

  /// Handle notification tap and navigate to appropriate screen
  Future<void> handleNotificationTap(
    BuildContext context,
    NotificationEntity notification,
  ) async {
    switch (notification.type) {
      // Chat notifications
      case NotificationType.chatMessage:
        _navigateToChat(context, notification);
        break;

      // Follow notifications
      case NotificationType.userFollowed:
        _navigateToProfile(context, notification);
        break;

      // Mention notifications
      case NotificationType.contentMentioned:
        _navigateToMention(context, notification);
        break;

      // Comment notifications
      case NotificationType.comment:
      case NotificationType.commentReply:
        _navigateToComment(context, notification);
        break;

      // Like notifications
      case NotificationType.contentLiked:
        _navigateToLikedContent(context, notification);
        break;

      // Seller response notifications
      case NotificationType.sellerResponse:
        _navigateToSellerResponse(context, notification);
        break;

      // Order notifications
      case NotificationType.orderCreated:
      case NotificationType.orderCreatedBuyer:
      case NotificationType.orderPaidBuyer:
      case NotificationType.orderPaid:
      case NotificationType.orderExpired:
      case NotificationType.orderShipped:
      case NotificationType.orderCancelled:
      case NotificationType.orderCancelledTimeout:
      case NotificationType.orderCompleted:
      case NotificationType.orderRefunded:
      case NotificationType.orderPartiallyRefunded:
      case NotificationType.orderDisputeOpen:
      case NotificationType.orderConfirmationExtended:
      case NotificationType.orderOverdueReminderSeller:
      case NotificationType.orderOverdueReminderBuyer:
        _navigateToOrder(context, notification);
        break;

      // Refund / dispute notifications
      case NotificationType.refundOpened:
      case NotificationType.refundApproved:
      case NotificationType.refundRejected:
      case NotificationType.refundEscalated:
      case NotificationType.disputeResolved:
      case NotificationType.disputeOpened:
        _navigateToRefund(context, notification);
        break;

      // Admin dispute timeout notifications — navigate to order for context
      case NotificationType.disputeOverdue:
      case NotificationType.disputeTimeoutEscalation:
        _navigateToOrder(context, notification);
        break;

      // Negotiation notifications — pre-order: navigate to chat room
      case NotificationType.negotiationStarted:
      case NotificationType.negotiationMessageSent:
        _navigateToNegotiationChat(context, notification);
        break;

      // Negotiation outcome — route to chat so buyer can initiate checkout or review.
      // Order does not exist at acceptance/expiry time; navigating to order would fail.
      case NotificationType.negotiationAccepted:
      case NotificationType.negotiationExpired:
        _navigateToNegotiationChat(context, notification);
        break;

      // Negotiation cancellation — B1: navigate to chat room if available
      case NotificationType.negotiationCancelled:
        _navigateToNegotiationChat(context, notification);
        break;

      // Auction notifications
      case NotificationType.auctionBidPlaced:
      case NotificationType.auctionWaitingSettlement:
      case NotificationType.auctionSellerHasWinner:
      case NotificationType.auctionEndedNoWinner:
      case NotificationType.auctionBnrSeller:
      case NotificationType.auctionBnrWinner:
        _navigateToAuction(context, notification);
        break;

      // Withdrawal / payout notifications
      case NotificationType.withdrawalRequested:
      case NotificationType.withdrawalApproved:
      case NotificationType.withdrawalRejected:
      case NotificationType.withdrawalCompleted:
      case NotificationType.withdrawalFailed:
        _navigateToSellerEarnings(context);
        break;

      // Verification / seller subscription notifications
      case NotificationType.verificationDocumentApproved:
      case NotificationType.verificationDocumentRejected:
      case NotificationType.sellerVerificationSubmitted:
      case NotificationType.sellerVerificationApproved:
      case NotificationType.sellerVerificationRejected:
      case NotificationType.sellerVerificationNeedsResubmission:
      case NotificationType.sellerVerificationSuspended:
      case NotificationType.sellerVerificationRevoked:
      case NotificationType.sellerVerificationUnderInvestigation:
      case NotificationType.sellerVerificationRestored:
        _navigateToSellerVerification(context);
        break;
      case NotificationType.sellerSubscriptionExpiring:
      case NotificationType.sellerSubscriptionExpired:
        _navigateToSettings(context);
        break;

      // Seller tier change notifications — B1
      case NotificationType.sellerTierUpgraded:
      case NotificationType.sellerTierDowngraded:
        _navigateToSellerDashboard(context);
        break;

      // Moderation notifications
      case NotificationType.moderationContentRemoved:
      case NotificationType.moderationCommentRemoved:
      case NotificationType.moderationContentRestored:
      case NotificationType.moderationCommentRestored:
        _navigateToModerationTarget(context, notification);
        break;
      case NotificationType.moderationForSaleRemoved:
      case NotificationType.moderationForSaleRestored:
      case NotificationType.moderationAuctionRemoved:
      case NotificationType.moderationAuctionRestored:
      case NotificationType.moderationUserSuspended:
      case NotificationType.moderationUserRestored:
      case NotificationType.moderationWarningIssued:
        // No specific deep-link: the removed/cancelled resource cannot be
        // restored from a detail screen. Navigate to settings as a safe
        // fallback where users can see account status and contact support.
        _navigateToSettings(context);
        break;

      // Support notifications
      case NotificationType.supportTicketCreated:
      case NotificationType.supportTicketResolved:
      case NotificationType.supportTicketClosed:
      case NotificationType.supportTicketWaitingUser:
      case NotificationType.supportTicketUserResponded:
        _navigateToSupportTicket(context, notification);
        break;

      // External product review notifications — owner navigates to their product management
      case NotificationType.externalProductReviewApproved:
      case NotificationType.externalProductReviewRejected:
      case NotificationType.externalProductReviewRequestChanges:
      case NotificationType.externalProductReviewHidden:
        _navigateToExternalProductManagement(context, notification);
        break;

      // Marketing / system notifications
      case NotificationType.promotion:
        _navigateToPromotion(context, notification);
        break;

      case NotificationType.announcement:
        _showAnnouncementModal(context, notification);
        break;

      case NotificationType.systemMaintenance:
        _showMaintenanceModal(context, notification);
        break;
    }
  }

  // ========== Private Navigation Methods ==========

  void _navigateToChat(BuildContext context, NotificationEntity notification) {
    final chatId =
        notification.data?['chatRoomId'] as String? ??
        notification.data?['chatId'] as String?;
    if (chatId != null) {
      _navigationHandler.navigateToChatConversation(chatId);
    }
  }

  void _navigateToProfile(
    BuildContext context,
    NotificationEntity notification,
  ) {
    final userId = notification.data?['userId'] as String?;
    if (userId != null) {
      _navigationHandler.navigateToUserProfile(userId);
    }
  }

  void _navigateToMention(
    BuildContext context,
    NotificationEntity notification,
  ) {
    final targetId = notification.data?['targetId'] as String?;
    final targetType = notification.data?['targetType'] as String?;

    if (targetType == 'content' && targetId != null) {
      _navigationHandler.navigateToContentDetail(targetId);
    } else {
      _navigateToNotifications(context);
    }
  }

  void _navigateToComment(
    BuildContext context,
    NotificationEntity notification,
  ) {
    final targetType = _firstString(notification.data, [
      'targetType',
      'target_type',
    ]);
    final targetId = _firstString(notification.data, ['targetId', 'target_id']);

    if (targetType == null) {
      _navigateToNotifications(context);
      return;
    }

    switch (targetType) {
      case 'content':
        if (targetId != null && targetId.isNotEmpty) {
          _navigationHandler.navigateToContentDetail(targetId);
        } else {
          _navigateToNotifications(context);
        }
        return;
      case 'listing':
        if (targetId != null && targetId.isNotEmpty) {
          _navigationHandler.navigateToForSaleDetail(targetId);
        } else {
          _navigateToNotifications(context);
        }
        return;
      case 'comment':
        final contentId = _firstString(notification.data, [
          'parentContentId',
          'parent_content_id',
          'contentId',
          'content_id',
          'targetContentId',
          'target_content_id',
          'targetId',
          'target_id',
        ]);
        if (contentId != null && contentId.isNotEmpty) {
          _navigationHandler.navigateToContentDetail(contentId);
        } else {
          _navigateToNotifications(context);
        }
        return;
      default:
        _navigateToNotifications(context);
    }
  }

  void _navigateToLikedContent(
    BuildContext context,
    NotificationEntity notification,
  ) {
    final targetType = _firstString(notification.data, [
      'targetType',
      'target_type',
    ]);
    final targetId = _firstString(notification.data, ['targetId', 'target_id']);

    if (targetType == null) {
      _navigateToNotifications(context);
      return;
    }

    switch (targetType) {
      case 'content':
        if (targetId != null && targetId.isNotEmpty) {
          _navigationHandler.navigateToContentDetail(targetId);
        } else {
          _navigateToNotifications(context);
        }
        return;
      case 'listing':
        if (targetId != null && targetId.isNotEmpty) {
          _navigationHandler.navigateToForSaleDetail(targetId);
        } else {
          _navigateToNotifications(context);
        }
        return;
      case 'comment':
        final contentId = _firstString(notification.data, [
          'parentContentId',
          'parent_content_id',
          'contentId',
          'content_id',
          'targetContentId',
          'target_content_id',
          'targetId',
          'target_id',
        ]);
        if (contentId != null && contentId.isNotEmpty) {
          _navigationHandler.navigateToContentDetail(contentId);
        } else {
          _navigateToNotifications(context);
        }
        return;
      default:
        _navigateToNotifications(context);
    }
  }

  void _navigateToSellerResponse(
    BuildContext context,
    NotificationEntity notification,
  ) {
    final targetId = notification.data?['targetId'] as String?;
    if (targetId != null) {
      _navigationHandler.navigateToContentDetail(targetId);
    }
  }

  void _navigateToOrder(BuildContext context, NotificationEntity notification) {
    final orderId = notification.data?['orderId'] as String?;
    if (orderId != null) {
      _navigationHandler.navigateToOrderDetail(orderId);
    }
  }

  void _navigateToRefund(
    BuildContext context,
    NotificationEntity notification,
  ) {
    final orderId = notification.data?['orderId'] as String?;
    if (orderId != null) {
      _navigationHandler.navigateToOrderDetail(orderId);
    }
  }

  void _navigateToAuction(
    BuildContext context,
    NotificationEntity notification,
  ) {
    final auctionId = notification.data?['auctionId'] as String?;
    if (auctionId != null) {
      _navigationHandler.navigateToAuction(auctionId);
    }
  }

  void _navigateToModerationTarget(
    BuildContext context,
    NotificationEntity notification,
  ) {
    final targetId = notification.data?['targetId'] as String?;
    final targetType = notification.data?['targetType'] as String?;
    if (targetId != null && targetType == 'content') {
      _navigationHandler.navigateToContentDetail(targetId);
    } else {
      _navigateToSettings(context);
    }
  }

  /// Navigate to the external product detail screen.
  /// Falls back to seller dashboard when externalProductId is missing or invalid.
  void _navigateToExternalProductManagement(
    BuildContext context,
    NotificationEntity notification,
  ) {
    final productId = notification.data?['externalProductId'] as String?;
    if (productId != null && productId.isNotEmpty) {
      _navigationHandler.navigateToExternalProductDetail(productId);
    } else {
      _navigationHandler.navigateToSellerDashboard();
    }
  }

  void _navigateToPromotion(
    BuildContext context,
    NotificationEntity notification,
  ) {
    final externalProductId = _firstString(notification.data, [
      'externalProductId',
      'external_product_id',
    ]);
    if (externalProductId != null && externalProductId.isNotEmpty) {
      _navigationHandler.navigateToExternalProductDetail(externalProductId);
      return;
    }

    final promotionInstanceId = _firstString(notification.data, [
      'promotionInstanceId',
      'promotion_instance_id',
    ]);
    if (promotionInstanceId != null && promotionInstanceId.isNotEmpty) {
      context.push('${RoutePaths.sellerPromotions}/$promotionInstanceId');
      return;
    }

    final ctaRoute = _firstString(notification.data, ['ctaRoute', 'cta_route']);
    if (ctaRoute != null && ctaRoute.isNotEmpty) {
      context.push(ctaRoute);
      return;
    }

    _navigateToSellerDashboard(context);
  }

  void _navigateToSupportTicket(
    BuildContext context,
    NotificationEntity notification,
  ) {
    final ticketId =
        notification.data?['ticketId'] as String? ??
        notification.data?['ticket_id'] as String?;
    if (ticketId != null && ticketId.isNotEmpty) {
      context.pushNamed(
        RouteNames.supportTicketThread,
        pathParameters: {'ticketId': ticketId},
      );
      return;
    }

    final chatRoomId =
        notification.data?['chatRoomId'] as String? ??
        notification.data?['chatId'] as String?;
    if (chatRoomId != null && chatRoomId.isNotEmpty) {
      _navigationHandler.navigateToChatConversation(chatRoomId);
      return;
    }

    _showFallback(context, 'Support ticket data tidak ditemukan');
  }

  void _navigateToSettings(BuildContext context) {
    _navigationHandler.navigateToSettings();
  }

  void _navigateToSellerVerification(BuildContext context) {
    _navigationHandler.navigateToSellerVerification();
  }

  void _navigateToSellerDashboard(BuildContext context) {
    _navigationHandler.navigateToSellerDashboard();
  }

  void _navigateToNotifications(BuildContext context) {
    _navigationHandler.navigateToNotifications();
  }

  void _navigateToNegotiationChat(
    BuildContext context,
    NotificationEntity notification,
  ) {
    final chatRoomId = notification.data?['chatRoomId'] as String?;
    if (chatRoomId != null) {
      _navigationHandler.navigateToChatConversation(chatRoomId);
    }
  }

  void _navigateToSellerEarnings(BuildContext context) {
    _navigationHandler.navigateToSellerEarnings();
  }

  void _showFallback(BuildContext context, String message) {
    if (!context.mounted) return;

    ScaffoldMessenger.of(context).showSnackBar(
      SnackBar(
        content: Text(message),
        behavior: SnackBarBehavior.floating,
        duration: const Duration(seconds: 3),
      ),
    );
  }

  // ========== Modal Methods ==========

  void _showAnnouncementModal(
    BuildContext context,
    NotificationEntity notification,
  ) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: Row(
          children: [
            Icon(Icons.campaign, color: Theme.of(context).primaryColor),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                notification.title,
                style: const TextStyle(fontSize: 18),
              ),
            ),
          ],
        ),
        content: Text(notification.body),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Tutup'),
          ),
        ],
      ),
    );
  }

  void _showMaintenanceModal(
    BuildContext context,
    NotificationEntity notification,
  ) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: Row(
          children: [
            const Icon(Icons.build, color: Colors.orange),
            const SizedBox(width: 8),
            const Expanded(
              child: Text('Maintenance System', style: TextStyle(fontSize: 18)),
            ),
          ],
        ),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(notification.body),
            if (notification.data?['startTime'] != null ||
                notification.data?['endTime'] != null) ...[
              const SizedBox(height: 16),
              const Divider(),
              const SizedBox(height: 8),
              if (notification.data?['startTime'] != null)
                Text(
                  'Mulai: ${notification.data!['startTime']}',
                  style: const TextStyle(fontSize: 12),
                ),
              if (notification.data?['endTime'] != null)
                Text(
                  'Ends: ${notification.data!['endTime']}',
                  style: const TextStyle(fontSize: 12),
                ),
            ],
          ],
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context),
            child: const Text('Mengerti'),
          ),
        ],
      ),
    );
  }

  String? _firstString(Map<String, dynamic>? data, List<String> keys) {
    if (data == null) return null;
    for (final key in keys) {
      final value = data[key];
      if (value is String && value.isNotEmpty) {
        return value;
      }
    }
    return null;
  }
}
