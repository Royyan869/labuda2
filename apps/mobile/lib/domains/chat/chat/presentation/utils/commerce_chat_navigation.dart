import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/chat/chat/presentation/providers/chat_providers.dart';
import 'package:labuda/shared/shared.dart';

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
    AppSnackBar.showWarning(context, 'Silakan masuk untuk membuka chat');
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

  // Commerce reference creation is now handled via chat context at room
  // creation (getOrCreateChat context) or via message resourceOccurrence.
  // The legacy createCommerceReference endpoint no longer exists in
  // ChatApiDatasource — navigate directly without a referenceId.
  final queryParameters = <String, String>{};
  if (autoOpenNegotiation) {
    queryParameters['action'] = 'negotiate';
  }

  final uri = Uri(
    path: '/chat/${chat.id}',
    queryParameters: queryParameters.isEmpty ? null : queryParameters,
  );
  router.push(uri.toString());
}
