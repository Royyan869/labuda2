/// Content Like Handlers
///
/// Utility class for handling content like actions
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:labuda/domains/user/identity/authentication/authentication.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/widgets/blocked_action_gate.dart';

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
  /// Preflight: backend route `POST /api/v1/likes/toggle` is gated by
  /// `RequireInteractionAuthority` (email-verified). If the local auth
  /// state already reports `!emailVerified`, surface the canonical
  /// blocked-action gate instead of burning a guaranteed 403. Backend
  /// remains the authoritative gate.
  Future<void> handleLike(String currentUserId, String currentUserName) async {
    final authState = ref.read(authControllerProvider);
    if (authState is AuthStateAuthenticated && !authState.emailVerified) {
      await showBlockedActionGate(
        context,
        actionDescription: 'menyukai konten',
      );
      return;
    }

    final notifier = ref.read(likeNotifierProvider.notifier);

    // BACKEND ALIGNMENT V1: Use 'content' for all content types (post and request)
    final result = await notifier.toggleLike(
      targetId: content.id,
      targetType: LikeTargetType.content,
      userId: currentUserId,
      likerName: currentUserName,
      targetOwnerId: content.authorId,
    );

    if (!result.isSuccess && context.mounted) {
      // If the backend rejected with EMAIL_VERIFICATION_REQUIRED despite our
      // preflight (e.g. verification status changed mid-session), still surface
      // the canonical gate rather than a generic snackbar.
      if (result.errorCode == 'EMAIL_VERIFICATION_REQUIRED') {
        await showBlockedActionGate(
          context,
          actionDescription: 'menyukai konten',
        );
        return;
      }
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(const SnackBar(content: Text('Gagal menyukai konten')));
    }
    // If successful, the likeStatsProvider will automatically update
  }
}
