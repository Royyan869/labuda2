import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/shared/shared.dart';

/// Opens (or creates) the canonical commerce chat room for [sellerId] and
/// navigates into it with a pending product reference.
///
/// **GUEST BOUNDARY:** unauthenticated users are routed to the canonical
/// sign-in flow (Model B — affordance visible, auth boundary redirects to
/// login).
///
/// **COMMERCE CONTEXT:** [reference] is carried as route extra and delivered
/// to [ChatDetailScreen] as a pending card — the user explicitly sends it
/// through the canonical chat send flow (message-level objectReference),
/// which persists server-side and reloads as a real message.
Future<void> openCommerceChat({
  required BuildContext context,
  required WidgetRef ref,
  required ShareReference reference,
  required String sellerId,
  bool autoOpenNegotiation = false,
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

  final normalizedReference = reference.asChatReference();
  if (normalizedReference == null) {
    AppSnackBar.showError(
      context,
      'Context chat commerce tidak valid untuk produk ini',
    );
    return;
  }

  final chat = await ref
      .read(chatListProvider.notifier)
      .getOrCreateChat(userId: currentUserId, otherUserId: sellerId);

  if (chat == null) return;

  final uri = Uri(path: '/chat/${chat.id}');
  router.push(
    uri.toString(),
    extra: <String, dynamic>{
      'pendingReference': normalizedReference,
      'autoOpenNegotiation': autoOpenNegotiation,
    },
  );
}
