/// Notification Badge Widget
///
/// Displays unread count badge.
/// Auto-updates dari stream provider.
///
/// Size: < 150 lines (per GUIDELINES)
library;

// Dart
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/system/notification/presentation/providers/unread_count_provider.dart';

// Flutter
import 'package:flutter/material.dart';

class NotificationBadgeWidget extends ConsumerWidget {
  final String userId;
  final Widget child;

  const NotificationBadgeWidget({
    super.key,
    required this.userId,
    required this.child,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final unreadCountAsync = ref.watch(unreadCountProvider(userId));

    return unreadCountAsync.when(
      data: (count) {
        if (count == 0) {
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
                  horizontal: count > 99
                      ? 3
                      : count > 9
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
                constraints: BoxConstraints(
                  minWidth: count > 9 ? 18 : 16,
                  minHeight: 16,
                ),
                child: Text(
                  count > 99 ? '99+' : count.toString(),
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
      },
      loading: () => child,
      error: (error, stackTrace) => child,
    );
  }
}
