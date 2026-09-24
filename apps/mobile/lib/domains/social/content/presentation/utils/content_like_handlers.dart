/// Content Like Handlers
///
/// Utility class for handling content like actions
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';

/// Handlers for content like actions
class ContentLikeHandlers {
  final WidgetRef ref;
  final BuildContext context;
  final Content content;

  ContentLikeHandlers({
    required this.ref,
    required this.context,
    required this.content,
  });

  /// Handle like content (post or request).
  ///
  /// Industry optimistic UX: 0ms local update + server reconcile.
  /// D2 HARD GATE (design scope v2): no client-side email-verification
  /// preflight — every authenticated user is already verified. The backend
  /// stays authoritative; its EMAIL_VERIFICATION_REQUIRED rejection still
  /// surfaces the canonical gate below (defense-in-depth handler).
  Future<void> handleLike(String currentUserId, String currentUserName) async {
    final params = LikeStatsParams(
      targetId: content.id,
      targetType: LikeTargetType.content,
      currentUserId: currentUserId,
    );
    final repository = ref.read(likeRepositoryProvider);
    final currentStats = ref.read(likeStatsProvider(params)).asData?.value;
    LikeStats? optimistic;
    if (currentStats != null) {
      optimistic = repository.optimisticToggled(currentStats);
      repository.pushOptimisticLikeStats(optimistic);
    }

    final notifier = ref.read(likeNotifierProvider.notifier);
    final result = await notifier.toggleLike(
      targetId: content.id,
      targetType: LikeTargetType.content,
      userId: currentUserId,
      likerName: currentUserName,
      targetOwnerId: content.authorId,
    );

    if (result.isSuccess) {
      await repository.refreshLikeStats(
        targetId: content.id,
        targetType: LikeTargetType.content,
        currentUserId: currentUserId,
      );
      return;
    }

    // Rollback on failure
    if (currentStats != null) {
      repository.pushOptimisticLikeStats(currentStats);
    }
    if (!context.mounted) return;
    if (result.errorCode == 'EMAIL_VERIFICATION_REQUIRED') {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(
          content: Text(
            'Verifikasi email kamu diperlukan sebelum menyukai konten.',
          ),
          backgroundColor: AppColors.statusError,
        ),
      );
      return;
    }
    ScaffoldMessenger.of(
      context,
    ).showSnackBar(const SnackBar(content: Text('Gagal menyukai konten')));
  }
}
