import 'package:flutter/material.dart';

import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:hishumi/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:hishumi/domains/user/identity/authentication/authentication.dart';
import 'package:hishumi/shared/widgets/count_badge.dart';

/// Unread-conversations badge for the app bar.
///
/// DOMAIN SEAM only: guests never fetch the badge (plain icon), and the count
/// comes from the chat domain's canonical provider. The badge LAYOUT is not
/// this widget's to know — it is rendered by the one authority
/// [CountBadgeOverlay]. This file used to carry its own copy of the whole
/// Stack; the copy is dead.
class ChatBadgeWidget extends ConsumerWidget {
  const ChatBadgeWidget({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authState = ref.watch(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      return child;
    }

    return CountBadgeOverlay(
      count: ref.watch(totalUnreadCountProvider),
      child: child,
    );
  }
}
