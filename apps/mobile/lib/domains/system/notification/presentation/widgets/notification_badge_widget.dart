import 'package:flutter/material.dart';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:labuda/domains/system/notification/presentation/providers/unread_count_provider.dart';
import 'package:labuda/shared/widgets/count_badge.dart';

/// Unread-notifications badge for the app bar.
///
/// DOMAIN SEAM only: [unreadCountProvider] for [userId]. Loading and error
/// render the plain icon — never a wrong count. The badge LAYOUT belongs to
/// the one renderer [CountBadgeOverlay]; this file used to carry its own copy
/// of the whole Stack, and that copy is dead.
class NotificationBadgeWidget extends ConsumerWidget {
  const NotificationBadgeWidget({
    super.key,
    required this.userId,
    required this.child,
  });

  final String userId;
  final Widget child;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return ref.watch(unreadCountProvider(userId)).when(
      data: (count) => CountBadgeOverlay(count: count, child: child),
      loading: () => child,
      error: (error, stackTrace) => child,
    );
  }
}
