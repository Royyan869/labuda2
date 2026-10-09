import 'package:flutter/material.dart';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:labuda/domains/system/notification/presentation/providers/unread_count_provider.dart';
import 'package:labuda/shared/widgets/count_badge.dart';

/// Unread-notifications badge for the app bar.
///
/// DOMAIN SEAM only: [unreadCountProvider] for [userId]. Renders the LAST
/// BACKEND-CONFIRMED count (`AsyncValue.value`): a failed refresh keeps the
/// previous confirmed count visible (error is never presented as a
/// confirmed `0`); before any confirmed read (initial loading or initial
/// failure) the plain icon shows — no fabricated number. The badge LAYOUT
/// belongs to the one renderer [CountBadgeOverlay]; this file used to carry
/// its own copy of the whole Stack, and that copy is dead.
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
    final async = ref.watch(unreadCountProvider(userId));
    // Last backend-confirmed count; null while none exists yet (initial
    // loading / initial failure). A backend-confirmed 0 flows through as 0
    // and the overlay renders the plain icon (count <= 0).
    final count = async.value;
    if (count == null) return child;
    return CountBadgeOverlay(count: count, child: child);
  }
}
