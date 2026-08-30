// Share Notifier

import 'package:flutter_riverpod/flutter_riverpod.dart';
import '../../domain/domain.dart';
import 'share_state.dart';
import 'share_providers.dart'; // Import shareRepositoryProvider

/// Notifier for share operations
/// Presentation layer - orchestrates use cases, replaces UseCase classes
class ShareNotifier extends Notifier<ShareState> {
  @override
  ShareState build() {
    return const ShareInitial();
  }

  /// Reset state to initial
  void reset() {
    state = const ShareInitial();
  }

  /// Share content via external platform (WhatsApp, Telegram, etc.)
  Future<void> shareViaExternal({
    required ShareTarget target,
    required ShareDestinationType destination,
  }) async {
    state = const ShareLoading();

    final repository = ref.read(shareRepositoryProvider);

    final result = await repository.shareViaExternal(
      target: target,
      destination: destination,
    );

    result.fold(
      (failure) => state = ShareError(failure),
      (shareResult) => state = ShareSuccess(result: shareResult),
    );
  }

  /// Share content as a new Post in feed (Repost)
  /// Returns the created post ID or null if failed
  Future<String?> shareAsPost({
    required ShareTarget target,
    required String authorId,
    String? caption,
  }) async {
    state = const ShareLoading();

    final repository = ref.read(shareRepositoryProvider);

    final result = await repository.shareAsPost(
      target: target,
      authorId: authorId,
      caption: caption,
    );

    String? postId;

    result.fold((failure) => state = ShareError(failure), (createdPostId) {
      postId = createdPostId;
      state = ShareSuccess(
        result: ShareResult.success(ShareDestinationType.shareToFeed),
      );
    });

    return postId;
  }

  /// Send content via LABUDA internal chat
  Future<void> sendToChat({
    required ShareTarget target,
    required String recipientUserId,
    String? message,
  }) async {
    state = const ShareLoading();

    final repository = ref.read(shareRepositoryProvider);

    final result = await repository.sendToChat(
      target: target,
      recipientUserId: recipientUserId,
      message: message,
    );

    result.fold(
      (failure) => state = ShareError(failure),
      (shareResult) => state = ShareSuccess(result: shareResult),
    );
  }
}

/// Provider for ShareNotifier
///
/// Usage in UI:
/// ```dart
/// final shareState = ref.watch(shareNotifierProvider);
/// final notifier = ref.read(shareNotifierProvider.notifier);
///
/// // Share via external
/// await notifier.shareViaExternal(
///   target: shareTarget,
///   destination: ShareDestinationType.whatsapp,
/// );
///
/// // Share as post (Repost)
/// final postId = await notifier.shareAsPost(
///   target: shareTarget,
///   authorId: userId,
///   caption: 'Check this out!',
/// );
/// ```
final shareNotifierProvider = NotifierProvider<ShareNotifier, ShareState>(
  ShareNotifier.new,
);
