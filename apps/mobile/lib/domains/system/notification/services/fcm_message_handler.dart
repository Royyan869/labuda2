import 'dart:async';
import 'package:firebase_messaging/firebase_messaging.dart';
import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/core/utils/notification_navigation_handler.dart';
import 'fcm_action_mapper.dart';
import 'in_app_banner_service.dart';

/// FCM Message Handler
///
/// Handles Firebase Cloud Messaging message processing:
/// - Foreground messages
/// - Background messages
/// - Message opened (tap handling)
/// - Initial message (app opened from terminated)
///
/// Extracted from fcm_service.dart for better modularity.
/// Size: < 200 lines (per GUIDELINES)
class FCMMessageHandler {
  final FirebaseMessaging _messaging;
  final InAppBannerService _inAppBannerService;

  StreamSubscription<RemoteMessage>? _messageSubscription;
  StreamSubscription<RemoteMessage>? _messageOpenedSubscription;

  FCMMessageHandler({
    required FirebaseMessaging messaging,
    required InAppBannerService inAppBannerService,
  }) : _messaging = messaging,
       _inAppBannerService = inAppBannerService;

  /// Setup message listeners
  Future<void> setupListeners() async {
    // Foreground message handler
    _messageSubscription = FirebaseMessaging.onMessage.listen(
      _handleForegroundMessage,
      onError: (error) {
        // Log foreground errors
      },
    );

    // Background message handler (static method required by Firebase)
    FirebaseMessaging.onBackgroundMessage(_backgroundMessageHandler);

    // Message opened handler (app in background)
    _messageOpenedSubscription = FirebaseMessaging.onMessageOpenedApp.listen(
      _handleMessageTap,
      onError: (error) {
        // Log message opened errors
      },
    );

    // Check initial message (app opened from terminated state)
    final initialMessage = await _messaging.getInitialMessage();
    if (initialMessage != null) {
      _handleMessageTap(initialMessage);
    }
  }

  /// Handle foreground message - show in-app banner
  Future<void> _handleForegroundMessage(RemoteMessage message) async {
    if (message.notification == null) return;

    // Get overlay from global navigatorKey
    final navigatorOverlay = navigatorKey.currentState?.overlay;
    final navigatorContext = navigatorKey.currentContext;

    if (navigatorOverlay == null) return;

    final notificationType = message.data['type'] as String?;

    // ============================================================
    // UX IMPROVEMENT: Suppress banner if already in relevant screen
    // ============================================================
    if (FCMMessageHandler.shouldSuppressBanner(
      navigatorContext,
      notificationType,
      message.data,
    )) {
      return;
    }

    // Get action buttons based on notification type
    final actionMapper = FCMActionMapper();
    final actions = actionMapper.getActionsForType(
      notificationType,
      message.data,
    );

    // Show in-app banner
    _inAppBannerService.show(
      overlay: navigatorOverlay,
      title: message.notification!.title ?? 'Notification',
      body: message.notification!.body ?? '',
      avatarUrl: message.data['avatarUrl'] as String?,
      actions: actions,
      // Only provide onTap if no actions
      // FIX: Fetch fresh context at tap time, not at banner display time
      onTap: (actions == null || actions.isEmpty)
          ? () {
              final freshContext = navigatorKey.currentContext;
              if (freshContext != null && freshContext.mounted) {
                _navigateFromMessage(message, freshContext);
              }
            }
          : null,
    );
  }

  /// Check if banner should be suppressed (user already in relevant screen)
  @visibleForTesting
  static bool shouldSuppressBanner(
    BuildContext? context,
    String? notificationType,
    Map<String, dynamic> data,
  ) {
    if (context == null || notificationType == null) return false;

    // Get current route name
    final currentRoute = ModalRoute.of(context)?.settings.name;

    switch (notificationType) {
      // Suppress chat notifications if already in the same chat
      case 'chat_message':
      case 'new_chat_message': // Backend alias
      case 'support.ticket.created':
      case 'support.ticket.resolved':
      case 'support.ticket.closed':
      case 'support_ticket_created':
      case 'support_ticket_resolved':
      case 'support_ticket_closed':
      case 'support.ticket_waiting_user':
      case 'support.ticket.user_responded':
        final chatId =
            data['chatId'] as String? ??
            data['chatRoomId'] as String? ??
            data['ticketId'] as String? ??
            data['ticket_id'] as String?;
        if (chatId == null) return false;

        // Check if currently in chat detail screen
        // Route format: '/chat/:chatId' or contains chatId
        final isInChat =
            currentRoute != null &&
            (currentRoute.contains('/chat') ||
                currentRoute.contains('/support/tickets') ||
                currentRoute.contains('supportTicketThread') ||
                currentRoute.contains(chatId));

        if (isInChat) {
          return true;
        }
        break;

      // Suppress order notifications if in order detail screen
      case 'order_created':
      case 'order.created':
      case 'order.created.buyer':
      case 'order_confirmed':
      case 'order.paid':
      case 'order.paid.buyer':
      case 'order_shipped':
      case 'order_delivered':
      case 'order.shipped':
      case 'order.completed':
      case 'order_cancelled':
      case 'order.cancelled':
      case 'order.cancelled_timeout':
      case 'order_expired':
      case 'order.expired':
      case 'order_refunded':
      case 'order.refunded':
      case 'order_partially_refunded':
      case 'order.partially_refunded':
      case 'order_dispute_open':
      case 'order.dispute_open':
      case 'order_confirmation_extended':
      case 'order.confirmation_extended':
      case 'refund_requested':
      case 'refund.opened':
      case 'refund_approved':
      case 'refund.approved':
      case 'refund_rejected':
      case 'refund.rejected':
      case 'refund.escalated':
      case 'refund_processed':
        final orderId = data['orderId'] as String?;
        if (orderId == null) return false;

        final isInOrder =
            currentRoute != null &&
            (currentRoute.contains('/order') || currentRoute.contains(orderId));

        if (isInOrder) {
          return true;
        }
        break;

      // Add more cases as needed
      default:
        return false;
    }

    return false;
  }

  /// Handle message tap - navigate to appropriate screen
  void _handleMessageTap(RemoteMessage message) {
    final context = navigatorKey.currentContext;

    if (context != null && context.mounted) {
      _navigateFromMessage(message, context);
    }
  }

  /// Navigate from message data
  void _navigateFromMessage(RemoteMessage message, BuildContext? context) {
    final type = message.data['type'] as String?;
    if (type != null && context != null && context.mounted) {
      NotificationNavigationHandler.navigate(
        context: context,
        type: type,
        data: message.data,
      );
    }
  }

  /// Cleanup - cancel subscriptions
  void dispose() {
    _messageSubscription?.cancel();
    _messageOpenedSubscription?.cancel();
    _messageSubscription = null;
    _messageOpenedSubscription = null;
  }
}

/// Background message handler (must be top-level function)
@pragma('vm:entry-point')
Future<void> _backgroundMessageHandler(RemoteMessage message) async {
  // Background messages are automatically displayed by Firebase
  // No additional handling needed
}
