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
import 'package:labuda/core/core.dart' show AppRouter;
import 'package:labuda/core/interfaces/i_notification_trigger.dart';
import 'package:labuda/core/navigation/navigation_handler.dart';
import 'package:labuda/core/src/router/route_paths.dart';
import 'package:labuda/domains/system/notification/domain/entities/notification_entity.dart';

// Flutter
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';

class NotificationNavigationService {
  final NavigationHandler _navigationHandler;

  NotificationNavigationService(this._navigationHandler);

  /// Canonical instance for the push / local-notification surfaces. Those run
  /// from FCM callbacks (no Riverpod container and no widget ref), so they use
  /// the same thin [AppRouter] forwarding layer the providers use.
  NotificationNavigationService.canonical() : _navigationHandler = AppRouter();

  /// Handle an in-app notification tap and navigate to the destination.
  Future<void> handleNotificationTap(
    BuildContext context,
    NotificationEntity notification,
  ) => _dispatch(
    context,
    type: notification.type,
    data: notification.data,
    title: notification.title,
    body: notification.body,
  );

  /// Handle a push / local-notification tap.
  ///
  /// FCM payloads carry the SAME canonical wire `type` value that the backend
  /// stores in the notification row (`NotificationType.value`), so a push tap
  /// resolves through the SAME destination decision as the in-app list. This
  /// service is the app's only notification→destination decision table.
  Future<void> handleNotificationPayload(
    BuildContext context, {
    required String type,
    required Map<String, dynamic> data,
    String title = '',
    String body = '',
  }) async {
    final resolved = NotificationType.tryFromString(type);
    if (resolved == null) {
      _onUnknownType(type);
      return;
    }
    await _dispatch(context, type: resolved, data: data, title: title, body: body);
  }

  /// THE single destination decision for every notification surface.
  Future<void> _dispatch(
    BuildContext context, {
    required NotificationType type,
    Map<String, dynamic>? data,
    String title = '',
    String body = '',
  }) async {
    switch (type) {
      // Chat notifications
      case NotificationType.chatMessage:
        _navigateToChat(context, data);
        break;

      // Follow notifications
      case NotificationType.userFollowed:
        _navigateToProfile(context, data);
        break;

      // Mention notifications
      case NotificationType.contentMentioned:
        _navigateToMention(context, data);
        break;

      // Comment notifications
      case NotificationType.comment:
      case NotificationType.commentReply:
        _navigateToComment(context, data);
        break;

      // Like notifications
      case NotificationType.contentLiked:
        _navigateToLikedContent(context, data);
        break;

      // Seller response notifications
      case NotificationType.sellerResponse:
        _navigateToSellerResponse(context, data);
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
        _navigateToOrder(context, data);
        break;

      // Refund / dispute notifications
      case NotificationType.refundOpened:
      case NotificationType.refundApproved:
      case NotificationType.refundRejected:
      case NotificationType.refundEscalated:
      case NotificationType.disputeResolved:
      case NotificationType.disputeOpened:
        _navigateToRefund(context, data);
        break;

      // Admin dispute timeout notifications — navigate to order for context
      case NotificationType.disputeOverdue:
      case NotificationType.disputeTimeoutEscalation:
        _navigateToOrder(context, data);
        break;

      // Negotiation notifications — pre-order: navigate to chat room
      case NotificationType.negotiationStarted:
      case NotificationType.negotiationMessageSent:
        _navigateToNegotiationChat(context, data);
        break;

      // Negotiation outcome — route to chat so buyer can initiate checkout or review.
      // Order does not exist at acceptance/expiry time; navigating to order would fail.
      case NotificationType.negotiationAccepted:
      case NotificationType.negotiationExpired:
        _navigateToNegotiationChat(context, data);
        break;

      // Negotiation cancellation — B1: navigate to chat room if available
      case NotificationType.negotiationCancelled:
        _navigateToNegotiationChat(context, data);
        break;

      // Auction notifications
      case NotificationType.auctionBidPlaced:
      case NotificationType.auctionWaitingSettlement:
      case NotificationType.auctionSellerHasWinner:
      case NotificationType.auctionEndedNoWinner:
      case NotificationType.auctionBnrSeller:
      case NotificationType.auctionBnrWinner:
        _navigateToAuction(context, data);
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
        _navigateToModerationTarget(context, data);
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
        _navigateToSupportTicket(context, data);
        break;

      // External product review notifications — owner navigates to their product management
      case NotificationType.externalProductReviewApproved:
      case NotificationType.externalProductReviewRejected:
      case NotificationType.externalProductReviewRequestChanges:
      case NotificationType.externalProductReviewHidden:
        _navigateToExternalProductManagement(context, data);
        break;

      // Marketing / system notifications
      case NotificationType.promotion:
        _navigateToPromotion(context, data);
        break;

      case NotificationType.announcement:
        _showAnnouncementModal(context, title: title, body: body);
        break;

      case NotificationType.systemMaintenance:
        _showMaintenanceModal(
        context,
        title: title,
        body: body,
        data: data,
      );
        break;
    }
  }

  /// A wire type outside the canonical catalog is not a Labuda notification
  /// type: stay on the current screen (no invented destination) and log it so
  /// the contract gap stays visible.
  void _onUnknownType(String type) {
    debugPrint('[NotificationNavigation] unknown notification type: $type');
  }

  // ========== Private Navigation Methods ==========

  void _navigateToChat(BuildContext context, Map<String, dynamic>? data) {
    final chatId =
        data?['chatRoomId'] as String? ??
        data?['chatId'] as String?;
    if (chatId != null) {
      _navigationHandler.navigateToChatConversation(chatId);
    }
  }

  void _navigateToProfile(
    BuildContext context,
    Map<String, dynamic>? data,
  ) {
    final userId = data?['userId'] as String?;
    if (userId != null) {
      _navigationHandler.navigateToUserProfile(userId);
    }
  }

  void _navigateToMention(
    BuildContext context,
    Map<String, dynamic>? data,
  ) {
    final targetId = data?['targetId'] as String?;
    final targetType = data?['targetType'] as String?;

    if (targetType == 'content' && targetId != null) {
      _navigationHandler.navigateToContentDetail(targetId);
    } else {
      _navigateToNotifications(context);
    }
  }

  void _navigateToComment(
    BuildContext context,
    Map<String, dynamic>? data,
  ) {
    final targetType = _firstString(data, [
      'targetType',
      'target_type',
    ]);
    final targetId = _firstString(data, ['targetId', 'target_id']);

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
      case 'forSale':
        if (targetId != null && targetId.isNotEmpty) {
          _navigationHandler.navigateToForSaleDetail(targetId);
        } else {
          _navigateToNotifications(context);
        }
        return;
      case 'comment':
        final contentId = _firstString(data, [
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
    Map<String, dynamic>? data,
  ) {
    final targetType = _firstString(data, [
      'targetType',
      'target_type',
    ]);
    final targetId = _firstString(data, ['targetId', 'target_id']);

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
      case 'forSale':
        if (targetId != null && targetId.isNotEmpty) {
          _navigationHandler.navigateToForSaleDetail(targetId);
        } else {
          _navigateToNotifications(context);
        }
        return;
      case 'comment':
        final contentId = _firstString(data, [
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
    Map<String, dynamic>? data,
  ) {
    final targetId = data?['targetId'] as String?;
    if (targetId != null) {
      _navigationHandler.navigateToContentDetail(targetId);
    }
  }

  void _navigateToOrder(BuildContext context, Map<String, dynamic>? data) {
    final orderId = data?['orderId'] as String?;
    if (orderId != null) {
      _navigationHandler.navigateToOrderDetail(orderId);
    }
  }

  void _navigateToRefund(
    BuildContext context,
    Map<String, dynamic>? data,
  ) {
    final orderId = data?['orderId'] as String?;
    if (orderId != null) {
      _navigationHandler.navigateToOrderDetail(orderId);
    }
  }

  void _navigateToAuction(
    BuildContext context,
    Map<String, dynamic>? data,
  ) {
    final auctionId = data?['auctionId'] as String?;
    if (auctionId != null) {
      _navigationHandler.navigateToAuction(auctionId);
    }
  }

  void _navigateToModerationTarget(
    BuildContext context,
    Map<String, dynamic>? data,
  ) {
    final targetId = data?['targetId'] as String?;
    final targetType = data?['targetType'] as String?;
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
    Map<String, dynamic>? data,
  ) {
    final productId = data?['externalProductId'] as String?;
    if (productId != null && productId.isNotEmpty) {
      _navigationHandler.navigateToExternalProductDetail(productId);
    } else {
      _navigationHandler.navigateToSellerDashboard();
    }
  }

  void _navigateToPromotion(
    BuildContext context,
    Map<String, dynamic>? data,
  ) {
    final externalProductId = _firstString(data, [
      'externalProductId',
      'external_product_id',
    ]);
    if (externalProductId != null && externalProductId.isNotEmpty) {
      _navigationHandler.navigateToExternalProductDetail(externalProductId);
      return;
    }

    // Canonical promotion surface: a contract id opens the contract analytics
    // screen; otherwise the seller lands on the promotion contract list.
    // The legacy /seller/promotions/:id detail route is purged.
    final contractId = _firstString(data, [
      'contractId',
      'contract_id',
    ]);
    if (contractId != null && contractId.isNotEmpty) {
      context.push(
        RoutePaths.sellerCanonicalPromotionAnalyticsPath(contractId),
      );
      return;
    }

    final ctaRoute = _firstString(data, ['ctaRoute', 'cta_route']);
    if (ctaRoute != null && ctaRoute.isNotEmpty) {
      context.push(ctaRoute);
      return;
    }

    context.push(RoutePaths.sellerCanonicalPromotions);
  }

  void _navigateToSupportTicket(
    BuildContext context,
    Map<String, dynamic>? data,
  ) {
    final ticketId =
        data?['ticketId'] as String? ??
        data?['ticket_id'] as String?;
    if (ticketId != null && ticketId.isNotEmpty) {
      context.pushNamed(
        RouteNames.supportTicketThread,
        pathParameters: {'ticketId': ticketId},
      );
      return;
    }

    final chatRoomId =
        data?['chatRoomId'] as String? ??
        data?['chatId'] as String?;
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
    Map<String, dynamic>? data,
  ) {
    final chatRoomId = data?['chatRoomId'] as String?;
    if (chatRoomId != null) {
      _navigationHandler.navigateToChatConversation(chatRoomId);
    }
  }

  void _navigateToSellerEarnings(BuildContext context) {
    _navigationHandler.navigateToSellerEarnings();
  }

  void _showFallback(BuildContext context, String message) {
    if (!context.mounted) return;

    AppSnackBar.showInfo(context, message);
  }

  // ========== Modal Methods ==========

  void _showAnnouncementModal(
    BuildContext context, {
    required String title,
    required String body,
  }) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: Row(
          children: [
            Icon(Icons.campaign, color: Theme.of(context).colorScheme.primary),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                title,
                style: context.typeRoles.titleSection,
              ),
            ),
          ],
        ),
        content: Text(body),
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
    BuildContext context, {
    required String title,
    required String body,
    required Map<String, dynamic>? data,
  }) {
    showDialog(
      context: context,
      builder: (context) => AlertDialog(
        title: Row(
          children: [
            Icon(Icons.build, color: context.statusColors.warning),
            const SizedBox(width: 8),
            Expanded(
              child: Text(
                'Maintenance System',
                style: context.typeRoles.titleSection,
              ),
            ),
          ],
        ),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(body),
            if (data?['startTime'] != null ||
                data?['endTime'] != null) ...[
              const SizedBox(height: 16),
              const Divider(),
              const SizedBox(height: 8),
              if (data?['startTime'] != null)
                Text(
                  'Mulai: ${data?['startTime']}',
                  style: context.typeRoles.labelMicro,
                ),
              if (data?['endTime'] != null)
                Text(
                  'Ends: ${data?['endTime']}',
                  style: context.typeRoles.labelMicro,
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
