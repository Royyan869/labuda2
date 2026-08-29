import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:labuda/domains/social/content/presentation/utils/content_like_handlers.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

/// Canonical Content like control shared across feed and detail surfaces.
class ContentLikeAction extends ConsumerStatefulWidget {
  final String contentId;
  final String contentOwnerId;
  final int fallbackLikeCount;
  final bool showLabel;

  const ContentLikeAction({
    super.key,
    required this.contentId,
    required this.contentOwnerId,
    required this.fallbackLikeCount,
    this.showLabel = false,
  });

  @override
  ConsumerState<ContentLikeAction> createState() => _ContentLikeActionState();
}

class _ContentLikeActionState extends ConsumerState<ContentLikeAction> {
  bool _isMutating = false;

  @override
  Widget build(BuildContext context) {
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
                targetId: widget.contentId,
                targetType: LikeTargetType.content,
                currentUserId: currentUserId,
              ),
            ),
          )
        : null;

    final stats = likeStatsAsync?.maybeWhen(
      data: (stats) => stats,
      orElse: () => null,
    );
    final hasStats = stats != null;
    final isLoading = likeStatsAsync?.isLoading == true && !hasStats;
    final likeCount = stats?.totalLikes ?? widget.fallbackLikeCount;
    final isLiked = stats?.isLikedByCurrentUser ?? false;

    final canTap = isAuthenticated && !isLoading && !_isMutating;

    return InkWell(
      onTap: canTap
          ? () async {
              if (_isMutating) return;
              setState(() => _isMutating = true);
              try {
                // ContentLikeHandlers requires a full Content entity. The
                // widget is given only the lightweight feed identifiers
                // (contentId + ownerId), so we synthesize a minimal Content
                // that satisfies the handler's contract (id + authorId) with
                // safe defaults for the remaining fields.
                final syntheticContent = Content(
                  id: widget.contentId,
                  content: '',
                  authorId: widget.contentOwnerId,
                  status: ContentStatus.active,
                  lifecycle: ContentLifecycle.active,
                  authorLifecycle: ContentLifecycle.active,
                  engagement: const ContentEngagement(),
                  moderationInfo: const ContentModerationInfo(),
                  createdAt: DateTime.fromMillisecondsSinceEpoch(0),
                  updatedAt: DateTime.fromMillisecondsSinceEpoch(0),
                );
                final handlers = ContentLikeHandlers(
                  ref: ref,
                  context: context,
                  content: syntheticContent,
                );
                await handlers.handleLike(currentUserId, currentUserName ?? '');
              } finally {
                if (mounted) {
                  setState(() => _isMutating = false);
                }
              }
            }
          : null,
      borderRadius: BorderRadius.circular(8),
      child: Padding(
        padding: const EdgeInsets.symmetric(horizontal: 4, vertical: 6),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            SizedBox(
              width: 16,
              height: 16,
              child: _isMutating
                  ? CircularProgressIndicator(
                      strokeWidth: 2,
                      valueColor: AlwaysStoppedAnimation<Color>(
                        AppColors.primaryRed,
                      ),
                    )
                  : isLoading
                  ? CircularProgressIndicator(
                      strokeWidth: 2,
                      valueColor: AlwaysStoppedAnimation<Color>(
                        isAuthenticated
                            ? AppColors.primaryRed
                            : AppColors.neutralGray400,
                      ),
                    )
                  : Icon(
                      isLiked ? Icons.favorite : Icons.favorite_border,
                      size: 16,
                      color: isLiked
                          ? AppColors.primaryRed
                          : (isAuthenticated
                                ? AppColors.primaryRed
                                : AppColors.neutralGray500),
                    ),
            ),
            const SizedBox(width: 4),
            Text(
              hasStats
                  ? likeCount.toString()
                  : (isLoading ? '...' : likeCount.toString()),
              style: TextStyle(
                fontSize: 12,
                color: isLiked
                    ? AppColors.primaryRed
                    : (isAuthenticated
                          ? AppColors.primaryRed
                          : AppColors.neutralGray500),
                fontWeight: FontWeight.w600,
              ),
            ),
            if (widget.showLabel) ...[
              const SizedBox(width: 4),
              Text(
                'Like',
                style: TextStyle(
                  fontSize: 12,
                  color: isLiked
                      ? AppColors.primaryRed
                      : (isAuthenticated
                            ? AppColors.primaryRed
                            : AppColors.neutralGray500),
                  fontWeight: FontWeight.w500,
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
