// Flutter
import 'package:flutter/material.dart';

// External
import 'package:flutter_riverpod/flutter_riverpod.dart';

// Internal
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';

/// Chat Badge Widget
///
/// Displays unread conversations count badge.
/// Auto-updates dari stream provider.
///
/// Consistent design with ShortlistBadgeWidget and NotificationBadgeWidget.
class ChatBadgeWidget extends ConsumerWidget {
  final Widget child;

  const ChatBadgeWidget({super.key, required this.child});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Get current user ID
    final authState = ref.watch(authControllerProvider);
    final userId = authState is AuthStateAuthenticated
        ? authState.user.id
        : null;

    if (userId == null) {
      return child;
    }

    // Watch total unread count from chat provider
    final unreadCount = ref.watch(totalUnreadCountProvider);

    // Return child without badge if count is 0
    if (unreadCount == 0) {
      return child;
    }

    return Stack(
      clipBehavior: Clip.none,
      children: [
        child,
        Positioned(
          right: -6,
          top: -6,
          child: Container(
            padding: EdgeInsets.symmetric(
              horizontal: unreadCount > 99
                  ? 3
                  : unreadCount > 9
                  ? 4
                  : 4,
              vertical: 2,
            ),
            decoration: BoxDecoration(
              color: Colors.red[600],
              borderRadius: BorderRadius.circular(10),
              boxShadow: [
                BoxShadow(
                  color: Colors.black.withValues(alpha: 0.2),
                  blurRadius: 3,
                  offset: const Offset(0, 1),
                ),
              ],
            ),
            constraints: const BoxConstraints(minWidth: 16, minHeight: 16),
            child: Text(
              unreadCount > 99 ? '99+' : unreadCount.toString(),
              style: const TextStyle(
                color: Colors.white,
                fontSize: 9,
                fontWeight: FontWeight.w600,
                height: 1.1,
              ),
              textAlign: TextAlign.center,
            ),
          ),
        ),
      ],
    );
  }
}
