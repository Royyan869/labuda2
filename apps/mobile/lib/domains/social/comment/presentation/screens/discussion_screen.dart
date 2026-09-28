/// Discussion Screen - Full screen comment list and composer
///
/// CONTRACT ALIGNMENT V1:
/// - Full screen discussion surface (NOT bottom sheet, NOT inline expansion)
/// - Reply max depth = 1 (top-level comments can be replied, replies cannot be replied)
/// - Seller responses get special visual treatment
///
/// DISCUSSION SURFACE V1:
/// - User can view all comments for a content
/// - User can create new comments
/// - User can reply to top-level comments only
/// - Sellers can attach a commerce resource to their comments
/// - Tap author navigates to profile
/// - Tap commerce reference navigates to For Sale/Auction detail
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/app_snackbar.dart';
import 'package:labuda/domains/social/comment/domain/entities/comment.dart';
import 'package:labuda/domains/social/comment/presentation/comment_widgets.dart';
import 'package:labuda/domains/social/comment/presentation/providers/comment_notifier.dart'
    show commentProvider;
import 'package:labuda/domains/social/comment/presentation/providers/comment_state.dart';
import 'package:labuda/domains/social/content/content.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:labuda/domains/social/comment/presentation/utils/comment_like_handlers.dart';

/// Discussion Screen - Full screen comment surface
///
/// Displays all comments for a content and allows creating new comments.
/// This is the canonical V1 discussion surface.
class DiscussionScreen extends ConsumerStatefulWidget {
  /// The content ID to show comments for
  final String contentId;

  /// Optional content title for the app bar
  final String? contentTitle;

  const DiscussionScreen({
    super.key,
    required this.contentId,
    this.contentTitle,
  });

  @override
  ConsumerState<DiscussionScreen> createState() => _DiscussionScreenState();
}

class _DiscussionScreenState extends ConsumerState<DiscussionScreen> {
  final ScrollController _scrollController = ScrollController();
  bool _isLoadingMore = false;
  bool _canAttachCommerceResource = false;
  Comment? _replyingToComment; // Track which comment is being replied to

  @override
  void initState() {
    super.initState();
    // Load comments when screen initializes
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref
          .read(commentProvider.notifier)
          .loadComments(
            targetId: widget.contentId,
            targetType: CommentTargetType.content,
          );
      _refreshAttachmentGuard();
    });

    // Set up scroll listener for pagination
    _scrollController.addListener(_onScroll);
  }

  @override
  void dispose() {
    _scrollController.dispose();
    super.dispose();
  }

  Future<void> _refreshAttachmentGuard() async {
    try {
      final repository = ref.read(contentRepositoryProvider);
      final result = await repository.getContentById(widget.contentId);

      if (!mounted) return;

      setState(() {
        _canAttachCommerceResource = result.fold(
          (error) => false,
          (content) => content.canReceiveForSaleResponses,
        );
      });
    } catch (_) {
      if (!mounted) return;
      setState(() => _canAttachCommerceResource = false);
    }
  }

  void _onScroll() {
    if (_scrollController.position.pixels >=
        _scrollController.position.maxScrollExtent * 0.8) {
      _loadMoreComments();
    }
  }

  Future<void> _loadMoreComments() async {
    if (_isLoadingMore) return;

    // BATCH 3E — Defense-in-depth exhaustion guard. The notifier also
    // short-circuits when state.hasMore is false (so the same page is
    // not refetched indefinitely), but checking here avoids spinning up
    // the inline loader for a request we know will be a no-op.
    final commentState = ref.read(commentProvider);
    if (!commentState.hasMore) return;

    setState(() {
      _isLoadingMore = true;
    });

    // BATCH 3E — Page argument intentionally omitted; the notifier owns
    // the pagination cursor (state.nextCursor) and advances it on success.
    // The pre-Batch-3E call passed neither page nor a guard, which made
    // every scroll past 80% refetch page 1 and append duplicates.
    await ref
        .read(commentProvider.notifier)
        .loadComments(
          targetId: widget.contentId,
          targetType: CommentTargetType.content,
          limit: 20,
          loadMore: true,
        );

    if (mounted) {
      setState(() {
        _isLoadingMore = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final commentState = ref.watch(commentProvider);
    final comments = commentState.comments
        .where((c) => c.contentId == widget.contentId)
        .toList();

    return Scaffold(
      appBar: _buildAppBar(context),
      body: Column(
        children: [
          // Comments list
          Expanded(child: _buildCommentsList(context, commentState, comments)),
          // Comment composer
          _buildCommentComposer(context),
        ],
      ),
    );
  }

  PreferredSizeWidget _buildAppBar(BuildContext context) {
    return AppBar(
      title: Text(_getAppBarTitle()),
      backgroundColor: Theme.of(context).scaffoldBackgroundColor,
      elevation: AppElevation.none,
      leading: IconButton(
        icon: const Icon(Icons.arrow_back),
        onPressed: () => context.pop(),
      ),
      actions: [
        // Refresh button
        IconButton(
          icon: const Icon(Icons.refresh),
          onPressed: () {
            ref
                .read(commentProvider.notifier)
                .loadComments(
                  targetId: widget.contentId,
                  targetType: CommentTargetType.content,
                );
          },
        ),
      ],
    );
  }

  String _getAppBarTitle() {
    if (widget.contentTitle != null && widget.contentTitle!.isNotEmpty) {
      return 'Komentar • ${widget.contentTitle}';
    }
    return 'Komentar';
  }

  Widget _buildCommentsList(
    BuildContext context,
    CommentState state,
    List<Comment> comments,
  ) {
    if (state.isLoading && comments.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }

    if (state.error != null && comments.isEmpty) {
      return _buildErrorState(context, state.error!);
    }

    if (comments.isEmpty) {
      return _buildEmptyState(context);
    }

    // Group comments: top-level with their replies
    final groupedComments = _groupCommentsWithReplies(comments);
    final flatList = _flattenCommentGroups(groupedComments);

    // GROUPED: top-level comments with their replies
    return _CommentsBatchWidget(
      flatList: flatList,
      scrollController: _scrollController,
      isLoadingMore: _isLoadingMore,
      onFixedPriceSaleTap: (fixedPriceSaleId) {
        context.push(
          RoutePaths.forSaleDetail.replaceFirst(
            ':fixedPriceSaleId',
            fixedPriceSaleId,
          ),
        );
      },
      onAuctionTap: (auctionId) {
        context.push(RoutePaths.auctionDetail(auctionId));
      },
      onAuthorTap: (userId) {
        context.push('/user/$userId');
      },
      onReply: (comment) => _startReply(context, comment),
      onRefresh: () async {
        await ref
            .read(commentProvider.notifier)
            .loadComments(
              targetId: widget.contentId,
              targetType: CommentTargetType.content,
            );
      },
    );
  }

  /// Group comments with their replies for 1-level threading
  List<_CommentGroup> _groupCommentsWithReplies(List<Comment> comments) {
    final Map<String, _CommentGroup> groups = {};
    final List<_CommentGroup> topLevelGroups = [];

    // First pass: create groups for all top-level comments
    for (final comment in comments) {
      if (comment.isTopLevel) {
        final group = _CommentGroup(parent: comment, replies: []);
        groups[comment.id] = group;
        topLevelGroups.add(group);
      }
    }

    // Second pass: add replies to their parent groups
    for (final comment in comments) {
      if (comment.isReply && comment.parentId != null) {
        final parentGroup = groups[comment.parentId];
        if (parentGroup != null) {
          parentGroup.replies.add(comment);
        }
      }
    }

    return topLevelGroups;
  }

  /// Flatten grouped comments into a list with markers for replies
  List<_CommentItem> _flattenCommentGroups(List<_CommentGroup> groups) {
    final result = <_CommentItem>[];
    for (final group in groups) {
      result.add(_CommentItem(comment: group.parent, isReply: false));
      for (final reply in group.replies) {
        result.add(_CommentItem(comment: reply, isReply: true));
      }
    }
    return result;
  }

  /// Start replying to a comment
  void _startReply(BuildContext context, Comment comment) {
    setState(() {
      _replyingToComment = comment;
    });
    // Scroll to bottom to show the input
    Future.delayed(AppMotion.settled, () {
      _scrollController.animateTo(
        _scrollController.position.maxScrollExtent,
        duration: AppMotion.settled,
        curve: Curves.easeOut,
      );
    });
  }

  /// Cancel reply mode
  void _cancelReply() {
    setState(() {
      _replyingToComment = null;
    });
  }

  Future<bool> _handleCommentSubmit(
    String body,
    ResourceIdentity? resource,
    List<String> mediaUrls,
  ) async {
    final trimmed = body.trim();
    final hasMedia = mediaUrls.isNotEmpty;
    if (trimmed.isEmpty && resource == null && !hasMedia) return false;

    final isReplying = _replyingToComment != null;

    final result = resource != null
        ? await ref
              .read(commentProvider.notifier)
              .createCommerceReferenceComment(
                contentId: widget.contentId,
                resourceType: resource.resourceType.wireValue,
                resourceId: resource.resourceId,
                body: trimmed.isEmpty ? null : trimmed,
              )
        : await ref
              .read(commentProvider.notifier)
              .createComment(
                targetId: widget.contentId,
                targetType: CommentTargetType.content,
                content: trimmed.isEmpty && hasMedia ? '' : body,
                parentId: isReplying ? _replyingToComment!.id : null,
                mediaUrls: mediaUrls,
              );

    if (!mounted) return result.isSuccess;

    if (result.isSuccess) {
      if (isReplying) _cancelReply();
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text(
            resource != null
                ? 'Respons Penjual berhasil dikirim'
                : isReplying
                ? 'Balasan berhasil dikirim'
                : 'Komentar berhasil dikirim',
          ),
          duration: const Duration(seconds: 2),
        ),
      );
      return true;
    }

    if (result.errorCode == 'EMAIL_VERIFICATION_REQUIRED') {        AppSnackBar.showError(
          context,
          'Verifikasi email kamu diperlukan sebelum menulis komentar.',
          duration: const Duration(seconds: 3),
        );
      return false;
    }      AppSnackBar.showError(
        context,
        result.error ?? 'Gagal mengirim komentar',
        duration: const Duration(seconds: 3),
      );
    return false;
  }

  Widget _buildEmptyState(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.comment_outlined,
            size: 64,
            color: scheme.outline,
          ),
          const SizedBox(height: 16),
          Text(
            'Belum ada komentar',
            style: TextStyle(
              fontSize: AppType.s18,
              fontWeight: FontWeight.w500,
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Jadilah yang pertama berkomentar!',
            style: TextStyle(fontSize: AppType.s14, color: scheme.onSurfaceVariant),
          ),
        ],
      ),
    );
  }

  Widget _buildErrorState(BuildContext context, String error) {
    final scheme = Theme.of(context).colorScheme;
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.error_outline, size: 64, color: scheme.error),
          const SizedBox(height: 16),
          Text(
            'Gagal memuat komentar',
            style: TextStyle(
              fontSize: AppType.s18,
              fontWeight: FontWeight.w500,
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            error,
            style: TextStyle(fontSize: AppType.s14, color: scheme.onSurfaceVariant),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 16),
          ElevatedButton(
            onPressed: () {
              ref
                  .read(commentProvider.notifier)
                  .loadComments(
                    targetId: widget.contentId,
                    targetType: CommentTargetType.content,
                  );
            },
            child: const Text('Coba Lagi'),
          ),
        ],
      ),
    );
  }

  Widget _buildCommentComposer(BuildContext context) {    final authState = ref.watch(authControllerProvider);

    // Check if user is a seller - use PermissionHelper
    final isSeller =
        authState is AuthStateAuthenticated &&
        PermissionHelper.canAccessSellerFeatures(authState.user);

    // Reply mode - disable seller features when replying
    final isReplying = _replyingToComment != null;
    final replyHintText = isReplying
        ? 'Membalas @${_replyingToComment!.authorUsername}...'
        : 'Tulis komentar...';

    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        // Reply indicator bar
        if (isReplying)
          Builder(
            builder: (context) {
              final scheme = Theme.of(context).colorScheme;
              return Container(
                padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16, vertical: AppMetrics.p8),
                decoration: BoxDecoration(
                  color: scheme.primary.withValues(alpha: 0.1),
                  border: Border(
                    top: BorderSide(color: scheme.outlineVariant, width: 1),
                  ),
                ),
                child: Row(
                  children: [
                    Icon(Icons.reply, size: 16, color: scheme.primary),
                    const SizedBox(width: 8),
                    Expanded(
                      child: Text(
                        'Membalas @${_replyingToComment!.authorUsername}',
                        style: TextStyle(
                          fontSize: AppType.s13,
                          color: scheme.primary,
                          fontWeight: FontWeight.w500,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    IconButton(
                      icon: const Icon(Icons.close, size: 18),
                      onPressed: _cancelReply,
                      padding: EdgeInsets.zero,
                      constraints: const BoxConstraints(),
                    ),
                  ],
                ),
              );
            },
          ),
        CommentInputWithCommerceReference(
          hintText: replyHintText,
          isSeller:
              isSeller &&
              !isReplying &&
              _canAttachCommerceResource, // Disable seller features when content is not active
          onSubmit: (body, resource) async =>
              _handleCommentSubmit(body, resource, const []),
          onSubmitWithMedia: (body, resource, mediaUrls) async =>
              _handleCommentSubmit(body, resource, mediaUrls),
        ),
      ],
    );
  }
}

/// Helper class for grouping comments with replies
class _CommentGroup {
  final Comment parent;
  final List<Comment> replies;

  _CommentGroup({required this.parent, required this.replies});
}

/// Helper class for flattening comment list
class _CommentItem {
  final Comment comment;
  final bool isReply;

  _CommentItem({required this.comment, required this.isReply});
}

/// Comments list widget.
///
/// Commerce attachments render from the canonical `resource_projection`
/// envelope carried by each comment — no client-side batch preview fetching.
class _CommentsBatchWidget extends ConsumerWidget {
  final List<_CommentItem> flatList;
  final ScrollController scrollController;
  final bool isLoadingMore;
  final Function(String) onFixedPriceSaleTap;
  final Function(String) onAuctionTap;
  final Function(String) onAuthorTap;
  final Function(Comment) onReply;
  final Future<void> Function() onRefresh;

  const _CommentsBatchWidget({
    required this.flatList,
    required this.scrollController,
    required this.isLoadingMore,
    required this.onFixedPriceSaleTap,
    required this.onAuctionTap,
    required this.onAuthorTap,
    required this.onReply,
    required this.onRefresh,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Auth state for comment like functionality
    final authState = ref.watch(authControllerProvider);
    final currentUserId = authState is AuthStateAuthenticated
        ? authState.user.id
        : null;
    final currentUserName = authState is AuthStateAuthenticated
        ? authState.user.username
        : null;

    return RefreshIndicator(
      onRefresh: onRefresh,
      child: ListView.separated(
        controller: scrollController,
        padding: const EdgeInsets.symmetric(vertical: AppMetrics.p8),
        itemCount: flatList.length + (isLoadingMore ? 1 : 0),
        separatorBuilder: (context, index) => const Divider(height: 1),
        itemBuilder: (context, index) {
          if (index >= flatList.length) {
            return const Center(
              child: Padding(
                padding: EdgeInsets.all(AppMetrics.p16),
                child: CircularProgressIndicator(),
              ),
            );
          }

          final item = flatList[index];
          if (item.isReply) {
            return _buildReplyItem(
              context,
              ref,
              item.comment,
              currentUserId,
              currentUserName,
            );
          }

          final comment = item.comment;

          return CommentCard(
            comment: comment,
            userName: '@${comment.authorUsername}',
            userUsername: comment.authorUsername,
            userAvatar: comment.authorAvatarUrl,
            userId: comment.authorId,
            currentUserId: currentUserId,
            currentUserName: currentUserName,
            onFixedPriceSaleTap: onFixedPriceSaleTap,
            onAuctionTap: onAuctionTap,
            onAuthorTap: onAuthorTap,
            onReply: () => onReply(comment),
            onEdit: () => _showEditDialog(context, ref, comment),
            onDelete: () => _showDeleteConfirm(context, ref, comment),
          );
        },
      ),
    );
  }

  Future<void> _showEditDialog(
    BuildContext context,
    WidgetRef ref,
    Comment comment,
  ) async {
    final controller = TextEditingController(text: comment.body ?? '');
    final result = await showDialog<String>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Edit Komentar'),
        content: TextField(
          controller: controller,
          maxLines: 4,
          maxLength: 2000,
          decoration: const InputDecoration(hintText: 'Tulis komentar...'),
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx),
            child: const Text('Batal'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, controller.text.trim()),
            child: const Text('Simpan'),
          ),
        ],
      ),
    );
    controller.dispose();
    if (result == null || result == comment.body) return;
    if (result.isEmpty) {
      if (context.mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(content: Text('Komentar tidak boleh kosong')),
        );
      }
      return;
    }
    final res = await ref
        .read(commentProvider.notifier)
        .updateComment(commentId: comment.id, body: result);
    if (!context.mounted) return;      if (res.isSuccess) {
        AppSnackBar.showSuccess(context, 'Komentar diperbarui');
      } else {
        AppSnackBar.showError(context, res.error ?? 'Gagal mengedit');
      }
  }

  Future<void> _showDeleteConfirm(
    BuildContext context,
    WidgetRef ref,
    Comment comment,
  ) async {
    final confirm = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('Hapus Komentar'),
        content: const Text('Yakin ingin menghapus komentar ini?'),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(ctx, false),
            child: const Text('Batal'),
          ),
          FilledButton(
            onPressed: () => Navigator.pop(ctx, true),
            style: FilledButton.styleFrom(backgroundColor: context.statusColors.error),
            child: const Text('Hapus'),
          ),
        ],
      ),
    );
    if (confirm != true) return;
    final res = await ref
        .read(commentProvider.notifier)
        .deleteComment(comment.id);
    if (!context.mounted) return;      if (res.isSuccess) {
        AppSnackBar.showSuccess(context, 'Komentar dihapus');
      } else {
        AppSnackBar.showError(context, res.error ?? 'Gagal menghapus');
      }
  }

  Widget _buildReplyItem(
    BuildContext context,
    WidgetRef ref,
    Comment comment,
    String? currentUserId,
    String? currentUserName,
  ) {
    final scheme = Theme.of(context).colorScheme;
    // E3.1 — Reply renderer mirrors the CommentCard header redaction
    // rules. The reply has no tap target / no avatar / no badge, so the
    // only gate needed is the username label. Comment body remains
    // visible per E3.1 doctrine (author-block governance only).
    final authorRedacted = comment.authorLifecycle.isDegraded;
    final authorLabel = authorRedacted
        ? comment.authorLifecycle.publicRedactionLabel
        : '@${comment.authorUsername}';

    // Watch like stats for this reply (replies are comments — same canonical Like system)
    final likeStatsAsync = (currentUserId != null && currentUserId.isNotEmpty)
        ? ref.watch(
            likeStatsProvider(
              LikeStatsParams(
                targetId: comment.id,
                targetType: LikeTargetType.comment,
                currentUserId: currentUserId,
              ),
            ),
          )
        : null;

    return Container(
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16, vertical: AppMetrics.p12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                Icons.reply,
                size: 16,
                color: scheme.onSurfaceVariant,
              ),
              const SizedBox(width: 8),
              Text(
                authorLabel,
                style: TextStyle(
                  fontWeight: FontWeight.w600,
                  fontSize: AppType.s14,
                  fontStyle: authorRedacted
                      ? FontStyle.italic
                      : FontStyle.normal,
                  color: authorRedacted ? scheme.onSurfaceVariant : scheme.onSurface,
                ),
              ),
              const SizedBox(width: 8),
              Text(
                _formatDate(comment.createdAt),
                style: TextStyle(
                  fontSize: AppType.s12,
                  color: scheme.onSurfaceVariant,
                ),
              ),
            ],
          ),
          if (comment.body != null && comment.body!.isNotEmpty)
            Padding(
              padding: const EdgeInsets.only(left: AppMetrics.p24, top: AppMetrics.p8),
              child: Text(comment.body!, style: const TextStyle(fontSize: AppType.s14)),
            ),
          // Like button for replies — same canonical Comment Like system
          if (currentUserId != null && currentUserId.isNotEmpty)
            _buildReplyLikeSection(
              context,
              ref,
              likeStatsAsync,
              comment,
              currentUserId,
              currentUserName,
            ),
        ],
      ),
    );
  }

  Widget _buildReplyLikeSection(
    BuildContext context,
    WidgetRef ref,
    AsyncValue<LikeStats>? likeStatsAsync,
    Comment comment,
    String currentUserId,
    String? currentUserName,
  ) {
    return likeStatsAsync?.when(
          data: (stats) => _ReplyLikeButton(
            likeCount: stats.totalLikes,
            isLiked: stats.isLikedByCurrentUser,
            onTap: () {
              final handlers = CommentLikeHandlers(
                ref: ref,
                context: context,
                comment: comment,
              );
              handlers.handleLike(currentUserId, currentUserName ?? '');
            },
          ),
          loading: () => const _ReplyLikeButton(
            likeCount: null,
            isLiked: false,
            onTap: null,
          ),
          error: (_, _) => _ReplyLikeButton(
            likeCount: 0,
            isLiked: false,
            onTap: () {
              final handlers = CommentLikeHandlers(
                ref: ref,
                context: context,
                comment: comment,
              );
              handlers.handleLike(currentUserId, currentUserName ?? '');
            },
          ),
        ) ??
        const SizedBox.shrink();
  }

  String _formatDate(DateTime dateTime) {
    final now = DateTime.now();
    final diff = now.difference(dateTime);

    if (diff.inMinutes < 1) {
      return 'Baru saja';
    } else if (diff.inHours < 1) {
      return '${diff.inMinutes}m yang lalu';
    } else if (diff.inDays < 1) {
      return '${diff.inHours}j yang lalu';
    } else if (diff.inDays < 7) {
      return '${diff.inDays}h yang lalu';
    } else {
      return '${dateTime.day}/${dateTime.month}/${dateTime.year}';
    }
  }
}

/// Like button widget for reply comments
///
/// Uses the same canonical Comment Like system as CommentCard.
/// target_type=comment, target_id=reply.comment.id
class _ReplyLikeButton extends StatelessWidget {
  final int? likeCount;
  final bool isLiked;
  final VoidCallback? onTap;

  const _ReplyLikeButton({
    required this.likeCount,
    required this.isLiked,
    required this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return InkWell(
      onTap: onTap,
      borderRadius: BorderRadius.circular(AppShape.r4),
      child: Padding(
        padding: const EdgeInsets.only(left: AppMetrics.p24, top: AppMetrics.p4),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              isLiked ? Icons.favorite : Icons.favorite_border,
              size: 14,
              color: isLiked ? scheme.error : scheme.onSurfaceVariant,
            ),
            if (likeCount != null) ...[
              const SizedBox(width: 4),
              Text(
                likeCount! > 0 ? '$likeCount' : '',
                style: TextStyle(
                  fontSize: AppType.s12,
                  color: scheme.onSurfaceVariant,
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
