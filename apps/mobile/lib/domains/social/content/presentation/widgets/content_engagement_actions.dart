import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/social/like/domain/entities/like.dart';
import 'package:hishumi/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:hishumi/domains/social/content/presentation/utils/content_like_handlers.dart';

/// THE canonical content-engagement action producer — like / comment / share.
///
/// ONE business concept ("engage with a piece of content") produced once for
/// every surface that shows it: the home feed card and the content detail
/// surface. Behavior — optimistic like, server reconcile, rollback and the
/// email-verification gate — is owned by [ContentLikeHandlers]; this widget
/// binds the target identity, the fallback counts and the two navigation
/// callbacks. It owns no surface, colour or shape: the icons read the theme
/// (`colorScheme`, `AppIconSize`) and the tap area is the standard
/// [InkWell]/[Semantics] button contract.
///
/// Every icon-only action exposes an accessibility name (Owner decision B);
/// the visible tooltip is intentionally omitted — the count beside a glyph is
/// already self-describing and a tooltip would be redundant UI noise.
class ContentEngagementActions extends ConsumerWidget {
  /// Content identity the like toggle targets.
  final String targetId;

  /// Owner of the content (like notification recipient).
  final String targetOwnerId;

  /// Server fallback like count, shown until live like stats resolve.
  final int likeCount;

  /// Comment count (hidden when zero).
  final int commentCount;

  final VoidCallback onComment;
  final VoidCallback onShare;

  const ContentEngagementActions({
    super.key,
    required this.targetId,
    required this.targetOwnerId,
    required this.onComment,
    required this.onShare,
    this.likeCount = 0,
    this.commentCount = 0,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final scheme = Theme.of(context).colorScheme;
    final authState = ref.watch(authControllerProvider);
    final currentUserId = authState is AuthStateAuthenticated
        ? authState.user.id
        : null;
    final currentUserName = authState is AuthStateAuthenticated
        ? authState.user.username
        : null;
    final isAuthenticated = currentUserId != null && currentUserId.isNotEmpty;

    final likeStatsAsync = isAuthenticated
        ? ref.watch(
            likeStatsProvider(
              LikeStatsParams(
                targetId: targetId,
                targetType: LikeTargetType.content,
                currentUserId: currentUserId,
              ),
            ),
          )
        : null;
    final stats = likeStatsAsync?.maybeWhen(data: (s) => s, orElse: () => null);
    final resolvedLikeCount = stats?.totalLikes ?? likeCount;
    final isLiked = stats?.isLikedByCurrentUser ?? false;

    // Brand ink while the like is actionable (authenticated), muted for a
    // guest. One rule for feed and detail.
    final likeColor = isLiked || isAuthenticated
        ? scheme.primary
        : scheme.onSurfaceVariant;

    void onLikeTap() {
      if (!isAuthenticated) return;
      ContentLikeHandlers(
        ref: ref,
        context: context,
        targetId: targetId,
        targetOwnerId: targetOwnerId,
      ).handleLike(currentUserId, currentUserName ?? '');
    }

    return Row(
      children: [
        _EngagementAction(
          icon: isLiked ? Icons.favorite : Icons.favorite_border,
          label: isLiked ? 'Batal suka' : 'Suka',
          count: resolvedLikeCount,
          color: likeColor,
          onTap: isAuthenticated ? onLikeTap : null,
        ),
        const SizedBox(width: 16),
        _EngagementAction(
          icon: Icons.chat_bubble_outline,
          label: 'Komentar',
          count: commentCount,
          color: scheme.primary,
          onTap: onComment,
        ),
        const SizedBox(width: 16),
        _EngagementAction(
          icon: Icons.share_outlined,
          label: 'Bagikan',
          count: 0,
          color: scheme.onSurfaceVariant,
          onTap: onShare,
        ),
      ],
    );
  }
}

/// One icon-only engagement action. The accessible name lives on the button
/// node itself (the glyph is excluded so the name is announced exactly once).
class _EngagementAction extends StatelessWidget {
  final IconData icon;
  final String label;
  final int count;
  final Color color;
  final VoidCallback? onTap;

  const _EngagementAction({
    required this.icon,
    required this.label,
    required this.count,
    required this.color,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: count > 0 ? '$label, $count' : label,
      onTap: onTap,
      child: ExcludeSemantics(
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(AppShape.r8),
          child: Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: AppMetrics.p4,
              vertical: AppMetrics.p8,
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(icon, size: AppIconSize.inlineGlyph, color: color),
                if (count > 0) ...[
                  const SizedBox(width: 4),
                  Text(
                    '$count',
                    style: context.typeRoles.labelMicro.copyWith(
                      color: color,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ],
              ],
            ),
          ),
        ),
      ),
    );
  }
}
