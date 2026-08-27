/// Comment Card Widget
///
/// CONTRACT ALIGNMENT V1:
/// - Displays a comment with optional fixed-price sale attachment
/// - Fixed-price sale reference is seller response only, NOT a binding offer
/// - Uses canonical Comment entity from comment domain
/// - INTEGRATION PASS V1: Author username is tappable to navigate to profile
/// - SELLER RESPONSE V1: Listing reference comments get "Respons Penjual" badge
/// - LIKE SYSTEM V1: Uses canonical Like system for comment likes
/// - REPLY SYSTEM V1: Reply max depth = 1 (only top-level comments can be replied)
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/comment/domain/entities/comment.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:labuda/domains/social/comment/presentation/utils/comment_like_handlers.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/shared/object/object_preview.dart' as obj;
import 'package:labuda/shared/object/presentation/widgets/object_preview_card.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/system/report/presentation/dialogs/report_submission_dialog.dart';

/// Comment Card Widget
///
/// Displays a comment with optional fixed-price sale attachment.
/// Handles both normal comments and fixed-price sale reference comments (seller responses).
/// Includes like functionality using canonical Like system.
class CommentCard extends ConsumerWidget {
  /// The comment to display
  final Comment comment;

  /// User information (now embedded in Comment entity via backend enrichment)
  final String userName;
  final String? userAvatar;
  final String? userUsername;
  final String? userId;

  /// Current user info for like functionality
  final String? currentUserId;
  final String? currentUserName;

  /// Callback when user taps on the fixed-price sale attachment
  final Function(String fixedPriceSaleId)? onFixedPriceSaleTap;

  /// Callback when user taps on author name/username
  final Function(String userId)? onAuthorTap;

  /// Callback when user taps on reply button (only for top-level comments)
  final VoidCallback? onReply;

  /// Pre-resolved live preview data (from batch provider)
  /// If provided, will be used directly without calling objectPreviewProvider
  final obj.ObjectPreview? preResolved;

  const CommentCard({
    super.key,
    required this.comment,
    required this.userName,
    this.userAvatar,
    this.userUsername,
    this.userId,
    this.currentUserId,
    this.currentUserName,
    this.onFixedPriceSaleTap,
    this.onAuthorTap,
    this.onReply,
    this.preResolved,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final isSellerResponse = comment.isCommerceReference;

    // Watch like stats for this comment (only if authenticated)
    final likeStatsAsync = (currentUserId != null && currentUserId!.isNotEmpty)
        ? ref.watch(
            likeStatsProvider(
              LikeStatsParams(
                targetId: comment.id,
                targetType: LikeTargetType.comment,
                currentUserId: currentUserId!,
              ),
            ),
          )
        : null;

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      decoration: isSellerResponse
          ? BoxDecoration(
              color: isDark
                  ? AppColors.primaryRed.withValues(alpha: 0.05)
                  : AppColors.primaryRed.withValues(alpha: 0.03),
              border: Border(
                left: BorderSide(
                  color: AppColors.primaryRed.withValues(alpha: 0.3),
                  width: 3,
                ),
              ),
            )
          : null,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Header: User info + timestamp + seller response badge
          _buildHeader(context, isDark),
          const SizedBox(height: 8),
          // Body text
          if (comment.body != null && comment.body!.isNotEmpty)
            _buildBody(context, isDark),
          // Fixed-price sale attachment (ShareReference)
          if (comment.reference != null &&
              (comment.reference!.targetType == ShareTargetType.forSale ||
                  comment.reference!.targetType ==
                      ShareTargetType.auction)) ...[
            if (comment.body != null && comment.body!.isNotEmpty)
              const SizedBox(height: 8),
            ObjectPreviewCard(
              reference: comment.reference!,
              onTap: comment.reference!.targetType == ShareTargetType.forSale
                  ? () => onFixedPriceSaleTap?.call(comment.reference!.targetId)
                  : null,
              showTypeBadge: false, // Don't show type badge in comments
              preResolved: preResolved, // Use pre-resolved data if available
            ),
          ],
          // Like button (shown only for authenticated users)
          if (currentUserId != null && currentUserId!.isNotEmpty)
            _buildLikeSection(context, ref, likeStatsAsync),
          // Reply button (only for top-level comments)
          if (comment.isTopLevel && onReply != null) _buildReplyButton(context),
        ],
      ),
    );
  }

  Widget _buildLikeSection(
    BuildContext context,
    WidgetRef ref,
    AsyncValue<LikeStats>? likeStatsAsync,
  ) {
    return likeStatsAsync?.when(
          data: (stats) => _LikeButton(
            likeCount: stats.totalLikes,
            isLiked: stats.isLikedByCurrentUser,
            onTap: () {
              final handlers = CommentLikeHandlers(
                ref: ref,
                context: context,
                comment: comment,
              );
              handlers.handleLike(currentUserId!, currentUserName ?? '');
            },
          ),
          loading: () =>
              const _LikeButton(likeCount: null, isLiked: false, onTap: null),
          error: (_, _) => _LikeButton(
            likeCount: 0,
            isLiked: false,
            onTap: () {
              final handlers = CommentLikeHandlers(
                ref: ref,
                context: context,
                comment: comment,
              );
              handlers.handleLike(currentUserId!, currentUserName ?? '');
            },
          ),
        ) ??
        const SizedBox.shrink();
  }

  Widget _buildReplyButton(BuildContext context) {
    return InkWell(
      onTap: onReply,
      borderRadius: BorderRadius.circular(4),
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 8),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.reply, size: 16, color: AppColors.neutralGray600),
            const SizedBox(width: 4),
            Text(
              'Balas',
              style: TextStyle(fontSize: 13, color: AppColors.neutralGray600),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildHeader(BuildContext context, bool isDark) {
    final isSellerResponse = comment.isCommerceReference;

    // E3.1 — Comment-author lifecycle redaction. Independent from the
    // comment body (which always remains visible per E3.1 doctrine): when
    // the author is unavailable/removed the header switches to a
    // placeholder label, the avatar drops the NetworkImage, the "Respons
    // Penjual" badge is suppressed (a redacted seller must not retain
    // seller-authority signal), and tap-to-profile is disabled. Active /
    // null / unknown lifecycle falls through to current behavior.
    final authorRedacted = comment.authorLifecycle.isDegraded;
    final authorPlaceholder = _authorRedactionLabel(comment.authorLifecycle);

    final displayName = authorRedacted ? authorPlaceholder : userName;

    // Create tappable area for author info if userId is provided
    final authorSection = Row(
      children: [
        // Avatar — degraded author always renders a neutral fallback
        // icon (no NetworkImage, no initials from a redacted name).
        CircleAvatar(
          radius: 16,
          backgroundColor: authorRedacted
              ? AppColors.neutralGray200
              : AppColors.primaryRed.withValues(alpha: 0.1),
          backgroundImage: (!authorRedacted && userAvatar != null)
              ? NetworkImage(userAvatar!)
              : null,
          child: authorRedacted
              ? const Icon(
                  Icons.person_off_outlined,
                  size: 18,
                  color: AppColors.neutralGray500,
                )
              : (userAvatar == null
                    ? Text(
                        UserInitialsHelper.fromName(userName),
                        style: const TextStyle(
                          color: AppColors.primaryRed,
                          fontWeight: FontWeight.bold,
                          fontSize: 14,
                        ),
                      )
                    : null),
        ),
        const SizedBox(width: 12),
        // Username + badges + timestamp
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Row(
                children: [
                  Flexible(
                    child: Text(
                      displayName,
                      style: TextStyle(
                        fontWeight: FontWeight.w600,
                        fontSize: 14,
                        fontStyle: authorRedacted
                            ? FontStyle.italic
                            : FontStyle.normal,
                        color: authorRedacted
                            ? AppColors.neutralGray500
                            : (isDark
                                  ? AppColors.neutralWhite
                                  : AppColors.neutralGray900),
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ),
                  // Seller Response Badge — suppressed when the author
                  // identity is degraded; the badge is a seller-authority
                  // signal and must not affirm a redacted account.
                  if (isSellerResponse && !authorRedacted) ...[
                    const SizedBox(width: 8),
                    Container(
                      padding: const EdgeInsets.symmetric(
                        horizontal: 6,
                        vertical: 2,
                      ),
                      decoration: BoxDecoration(
                        color: AppColors.primaryRed.withValues(alpha: 0.1),
                        borderRadius: BorderRadius.circular(4),
                      ),
                      child: Text(
                        'Respons Penjual',
                        style: TextStyle(
                          fontSize: 10,
                          fontWeight: FontWeight.w600,
                          color: AppColors.primaryRed,
                        ),
                      ),
                    ),
                  ],
                ],
              ),
              // Secondary @-handle row: suppressed for degraded authors
              // so we never surface the canonical handle alongside the
              // redaction placeholder.
              if (!authorRedacted &&
                  userUsername != null &&
                  userUsername!.isNotEmpty &&
                  userUsername != userName)
                Text(
                  '@${userUsername!}',
                  style: TextStyle(
                    fontSize: 12,
                    color: AppColors.neutralGray600,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              Text(
                _formatTimestamp(comment.createdAt),
                style: TextStyle(fontSize: 12, color: AppColors.neutralGray600),
              ),
            ],
          ),
        ),
        // Report button (for non-authors)
        if (userId != null && currentUserId != null && userId != currentUserId)
          PopupMoreOptionsButton(
            contentType: PopupMoreOptionsContentType.content,
            isCreator: false,
            isDeleting: false,
            iconSize: 16,
            onReport: () => _handleReportComment(context),
          ),
      ],
    );

    // Wrap in InkWell if userId and onAuthorTap are provided AND the
    // author is not redacted — never navigate to a tombstoned profile.
    if (!authorRedacted && userId != null && onAuthorTap != null) {
      return InkWell(
        onTap: () => onAuthorTap!(userId!),
        borderRadius: BorderRadius.circular(8),
        child: authorSection,
      );
    }

    return authorSection;
  }

  /// Comment author redaction label.
  /// Delegates to the canonical [ContentLifecycleParse.publicRedactionLabel].
  String _authorRedactionLabel(ContentLifecycle authorLifecycle) =>
      authorLifecycle.publicRedactionLabel;

  Future<void> _handleReportComment(BuildContext context) async {
    if (currentUserId == null || currentUserId!.isEmpty) {
      if (context.mounted) {
        AppSnackBar.showError(context, 'Please login to report comments');
      }
      return;
    }

    // Check if user is trying to report their own comment
    if (userId == currentUserId) {
      if (context.mounted) {
        AppSnackBar.showError(context, 'Cannot report your own comment');
      }
      return;
    }

    // Show report submission dialog (comment reporting is ENABLED)
    await ReportSubmissionDialog.show(
      context,
      targetId: comment.id,
      targetType: ReportTargetType.comment,
      targetTitle: comment.body?.substring(0, 100) ?? 'Comment',
    );
  }

  Widget _buildBody(BuildContext context, bool isDark) {
    return Text(
      comment.body ?? '',
      style: TextStyle(
        fontSize: 14,
        color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
        height: 1.4,
      ),
    );
  }

  String _formatTimestamp(DateTime timestamp) {
    final now = DateTime.now();
    final difference = now.difference(timestamp);

    if (difference.inMinutes < 1) {
      return 'Baru saja';
    } else if (difference.inMinutes < 60) {
      return '${difference.inMinutes}m lalu';
    } else if (difference.inHours < 24) {
      return '${difference.inHours}j lalu';
    } else if (difference.inDays < 7) {
      return '${difference.inDays} hari lalu';
    } else {
      return '${timestamp.day}/${timestamp.month}/${timestamp.year}';
    }
  }
}

/// Like button widget for comments
class _LikeButton extends StatelessWidget {
  final int? likeCount;
  final bool isLiked;
  final VoidCallback? onTap;

  const _LikeButton({
    required this.likeCount,
    required this.isLiked,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(4),
      child: Padding(
        padding: const EdgeInsets.symmetric(vertical: 8),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              isLiked ? Icons.favorite : Icons.favorite_border,
              size: 16,
              color: isLiked ? Colors.red : AppColors.neutralGray600,
            ),
            if (likeCount != null) ...[
              const SizedBox(width: 4),
              Text(
                likeCount! > 0 ? '$likeCount' : '',
                style: TextStyle(fontSize: 13, color: AppColors.neutralGray600),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
