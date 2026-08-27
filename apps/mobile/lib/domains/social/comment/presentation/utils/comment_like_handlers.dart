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
  /// Uses canonical Like system to toggle like status for comments.
  /// Optimistic update: UI updates immediately, rolls back on error.
  Future<void> handleLike(String currentUserId, String currentUserName) async {
    final notifier = ref.read(likeNotifierProvider.notifier);

    final result = await notifier.toggleLike(
      targetId: comment.id,
      targetType: LikeTargetType.comment,
      userId: currentUserId,
      likerName: currentUserName,
      targetOwnerId: comment.authorId,
    );

    if (!result.isSuccess && context.mounted) {
      // Show error if toggle failed
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('Gagal menyukai komentar')));
    }
    // If successful, the likeStatsProvider will automatically update
  }
}
