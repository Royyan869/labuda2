import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/chat/chat/data/chat_providers.dart'
    show chatApiDatasourceProvider;
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

  final api = ref.read(chatApiDatasourceProvider);
  final referenceResult = await api.createCommerceReference(chat.id, {
    'target_type': normalizedReference.targetType.wireValue,
    'target_id': normalizedReference.targetId,
    'preview': {
      'title': normalizedReference.preview.title,
      if (normalizedReference.preview.imageUrl != null)
        'imageUrl': normalizedReference.preview.imageUrl,
      'isAvailable': normalizedReference.preview.isAvailable,
      'isSold': normalizedReference.preview.isSold,
      'isClosed': normalizedReference.preview.isClosed,
      'isDeleted': normalizedReference.preview.isDeleted,
    },
  });

  final referenceId = referenceResult.fold<String?>(
    (_) => null,
    (data) => data.id,
  );

  final queryParameters = <String, String>{};
  if (referenceId != null) {
    queryParameters['referenceId'] = referenceId;
  }
  if (autoOpenNegotiation) {
    queryParameters['action'] = 'negotiate';
  }

  final uri = Uri(
    path: '/chat/${chat.id}',
    queryParameters: queryParameters.isEmpty ? null : queryParameters,
  );
  router.push(uri.toString());
}
