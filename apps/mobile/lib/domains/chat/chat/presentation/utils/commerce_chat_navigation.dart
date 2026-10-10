import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/chat/chat/presentation/models/pending_commerce_attachment.dart';
import 'package:hishumi/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:hishumi/shared/shared.dart';

/// Opens (or creates) the canonical commerce chat room for [sellerId] and
/// navigates into it with a pending product attachment.
///
/// **GUEST BOUNDARY:** unauthenticated users are routed to the canonical
/// sign-in flow (Model B — affordance visible, auth boundary redirects to
/// login).
///
/// **COMMERCE CONTEXT:** [attachment] is carried as route extra and delivered
/// to [ChatDetailScreen] as the canonical pending composer attachment. It is
/// sent ONLY through the composer send icon (message-level resourceOccurrence:
/// `direct_commerce_insert_chat`) — there is no send CTA on the chip.
Future<void> openCommerceChat({
  required BuildContext context,
  required WidgetRef ref,
  required PendingCommerceAttachment attachment,
  required String sellerId,

  /// Optional composer DRAFT delivered to Chat as PRE-FILLED textarea content.
  ///
  /// Supplied ONLY by an entry point that already knows why the user is opening
  /// Chat — today that is the Checkout uncovered-shipping shortcut, which asks
  /// the seller about shipping. It is never auto-sent: the composer send icon
  /// remains the one send authority. Every other entry leaves this null so no
  /// shipping question is ever autofilled.
  String? draftMessage,

  /// Truthful failure signal for the CALLER's surface.
  ///
  /// Room resolution can fail for real business reasons — the other
  /// participant blocked this user, a messaging restriction, or a transport
  /// failure. Without this hook the room silently fails to open and the CTA
  /// that opened it becomes a lying affordance on the surface the buyer is
  /// looking at. Callers pass their own surface-appropriate copy; when omitted,
  /// behaviour is unchanged (the failure stays on the canonical chat list
  /// state).
  void Function(String error)? onFailure,
}) async {
  final router = GoRouter.of(context);
  final authState = ref.read(authControllerProvider);
  if (authState is! AuthStateAuthenticated) {
    // Canonical guest interception: affordance is visible on the detail
    // surface; the auth boundary redirects to the canonical login route.
    router.push(RoutePaths.signIn);
    return;
  }

  final currentUserId = authState.user.id;
  if (currentUserId == sellerId) {
    AppSnackBar.showWarning(
      context,
      'Anda tidak dapat membuka chat untuk produk Anda sendiri',
    );
    return;
  }

  final chat = await ref
      .read(chatListProvider.notifier)
      .getOrCreateChat(userId: currentUserId, otherUserId: sellerId);

  if (chat == null) {
    onFailure?.call(ref.read(chatListProvider).error ?? 'Gagal membuka chat');
    return;
  }

  final uri = Uri(path: '/chat/${chat.id}');
  final extra = <String, dynamic>{'pendingCommerce': attachment};
  if (draftMessage != null && draftMessage.isNotEmpty) {
    extra['draftMessage'] = draftMessage;
  }
  router.push(uri.toString(), extra: extra);
}
