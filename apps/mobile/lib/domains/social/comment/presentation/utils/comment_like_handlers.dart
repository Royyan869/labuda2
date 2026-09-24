/// Comment Like Handlers
///
/// Utility class for handling comment like actions
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/social/comment/domain/entities/comment.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';

/// Handlers for comment like actions
class CommentLikeHandlers {
  final WidgetRef ref;
  final BuildContext context;
  final Comment comment;

  CommentLikeHandlers({
    required this.ref,
    required this.context,
    required this.comment,
  });

  /// Handle like comment
  ///
  /// Industry optimistic UX: 0ms local update + server reconcile.
  Future<void> handleLike(String currentUserId, String currentUserName) async {
    final params = LikeStatsParams(
      targetId: comment.id,
      targetType: LikeTargetType.comment,
      currentUserId: currentUserId,
    );
    final repository = ref.read(likeRepositoryProvider);
    final currentStats = ref.read(likeStatsProvider(params)).asData?.value;
    if (currentStats != null) {
      repository.pushOptimisticLikeStats(
        repository.optimisticToggled(currentStats),
      );
    }

    final notifier = ref.read(likeNotifierProvider.notifier);
    final result = await notifier.toggleLike(
      targetId: comment.id,
      targetType: LikeTargetType.comment,
      userId: currentUserId,
      likerName: currentUserName,
      targetOwnerId: comment.authorId,
    );

    if (result.isSuccess) {
      await repository.refreshLikeStats(
        targetId: comment.id,
        targetType: LikeTargetType.comment,
        currentUserId: currentUserId,
      );
      return;
    }

    // Rollback
    if (currentStats != null) {
      repository.pushOptimisticLikeStats(currentStats);
    }
    if (context.mounted) {
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('Gagal menyukai komentar')));
    }
  }
}
