/// Content Detail Screen - Shows universal content detail with full content
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';
import 'package:labuda/domains/social/content/content.dart';
import 'package:labuda/domains/social/content/presentation/providers/content_state.dart';
import 'package:labuda/domains/social/content/presentation/widgets/content_resource_projection_card.dart';
import 'package:labuda/domains/social/share/share.dart';
import 'package:labuda/domains/social/like/domain/entities/like.dart';
import 'package:labuda/domains/social/like/presentation/providers/like_notifier.dart';
import 'package:labuda/domains/social/content/presentation/utils/content_like_handlers.dart';
import 'package:labuda/domains/user/profile/presentation/providers/user_data_provider.dart';
import 'package:labuda/domains/system/report/domain/entities/entities.dart';
import 'package:labuda/domains/system/report/presentation/dialogs/report_submission_dialog.dart';

/// Content Detail Screen
class ContentDetailScreen extends ConsumerStatefulWidget {
  final String contentId;

  const ContentDetailScreen({super.key, required this.contentId});

  @override
  ConsumerState<ContentDetailScreen> createState() =>
      _ContentDetailScreenState();
}

class _ContentDetailScreenState extends ConsumerState<ContentDetailScreen> {
  int _currentMediaIndex = 0;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      ref.read(contentDetailProvider.notifier).fetchContent(widget.contentId);
    });
  }

  @override
  Widget build(BuildContext context) {
    final detailState = ref.watch(contentDetailProvider);

    // Get current user info for like functionality
    final authState = ref.watch(authControllerProvider);
    final currentUserId = authState is AuthStateAuthenticated
        ? authState.user.id
        : null;
    final currentUserName = authState is AuthStateAuthenticated
        ? authState.user.username
        : null;

    // Watch like stats for this content (only if authenticated)
    AsyncValue<LikeStats>? likeStatsAsync;
    if (currentUserId == null || currentUserId.isEmpty) {
      likeStatsAsync = null;
    } else {
      likeStatsAsync = ref.watch(
        likeStatsProvider(
          LikeStatsParams(
            targetId: widget.contentId,
            // BACKEND ALIGNMENT V1: Use "content" for all universal content rows
            targetType: LikeTargetType.content,
            currentUserId: currentUserId,
          ),
        ),
      );
    }

    return Scaffold(
      appBar: _buildAppBar(context, detailState, authState),
      body: detailState.map(
        initial: (_) => const SizedBox.shrink(),
        loading: (_) => const Center(child: CircularProgressIndicator()),
        loaded: (state) => _buildContent(
          context,
          state.content,
          likeStatsAsync: likeStatsAsync,
          currentUserId: currentUserId,
          currentUserName: currentUserName,
        ),
        error: (state) => _buildError(context, state.message),
      ),
    );
  }

  PreferredSizeWidget _buildAppBar(
    BuildContext context,
    ContentDetailState detailState,
    AuthState authState,
  ) {
    return AppBar(
      leading: IconButton(
        icon: const Icon(Icons.arrow_back),
        onPressed: () => context.pop(),
      ),
      title: const Text('Content Detail'),
      actions: [
        // Report button (for non-creators)
        if (detailState is ContentDetailLoaded &&
            authState is AuthStateAuthenticated)
          Builder(
            builder: (context) {
              final content = detailState.content;
              final isCreator = content.authorId == authState.user.id;

              if (!isCreator) {
                return PopupMoreOptionsButton(
                  contentType: PopupMoreOptionsContentType.content,
                  isCreator: false,
                  isDeleting: false,
                  onReport: () => _handleReportContent(context, content),
                );
              }
              return const SizedBox.shrink();
            },
          ),
      ],
    );
  }

  Future<void> _handleReportContent(
    BuildContext context,
    Content content,
  ) async {
    final authState = ref.read(authControllerProvider);
    if (authState is! AuthStateAuthenticated) {
      if (mounted) {
        AppSnackBar.showError(context, 'Please login to report content');
      }
      return;
    }

    // Check if user is trying to report their own content
    if (content.authorId == authState.user.id) {
      if (mounted) {
        AppSnackBar.showError(context, 'Cannot report your own content');
      }
      return;
    }

    // Show report submission dialog (content reporting is ENABLED)
    await ReportSubmissionDialog.show(
      context,
      targetId: content.id,
      targetType: ReportTargetType.content,
      targetTitle: content.content.substring(
        0,
        content.content.length > 100 ? 100 : content.content.length,
      ),
    );
  }

  Widget _buildContent(
    BuildContext context,
    Content content, {
    AsyncValue<LikeStats>? likeStatsAsync,
    String? currentUserId,
    String? currentUserName,
  }) {
    // D1 — governance lifecycle gate. Detail surface preserves architectural
    // truth (HTTP 404 for removed) but defends in depth against any future
    // path where lifecycle=removed reaches the screen.
    if (content.lifecycle.isRemoved) {
      return _buildRemovedTombstone(context);
    }
    final isUnavailable = content.lifecycle.isUnavailable;
    return RefreshIndicator(
      onRefresh: () async {
        await ref
            .read(contentDetailProvider.notifier)
            .fetchContent(widget.contentId);
      },
      child: CustomScrollView(
        slivers: [
          if (isUnavailable)
            SliverToBoxAdapter(child: _buildUnavailableBanner(context)),
          // Media section
          if (content.media.isNotEmpty)
            SliverToBoxAdapter(child: _buildMediaSection(context, content)),
          // Content section
          SliverPadding(
            padding: const EdgeInsets.all(16),
            sliver: SliverToBoxAdapter(
              child: _buildContentSection(context, content),
            ),
          ),
          // Linked items (canonical resource projection only)
          if (content.resourceProjection != null)
            SliverPadding(
              padding: const EdgeInsets.symmetric(horizontal: 16),
              sliver: SliverToBoxAdapter(
                child: _buildResourceProjection(context, content),
              ),
            ),
          // Engagement section
          SliverPadding(
            padding: const EdgeInsets.all(16),
            sliver: SliverToBoxAdapter(
              child: _buildEngagementSection(
                context,
                content,
                likeStatsAsync: likeStatsAsync,
                currentUserId: currentUserId,
                currentUserName: currentUserName,
              ),
            ),
          ),
          // Bottom spacing
          const SliverToBoxAdapter(child: SizedBox(height: 100)),
        ],
      ),
    );
  }

  Widget _buildMediaSection(BuildContext context, Content content) {
    return Stack(
      children: [
        // Main image
        SizedBox(
          width: double.infinity,
          height: 300,
          child: PageView.builder(
            itemCount: content.media.length,
            onPageChanged: (index) {
              setState(() {
                _currentMediaIndex = index;
              });
            },
            itemBuilder: (context, index) {
              return GestureDetector(
                onTap: () => _openMediaViewer(context, content, index),
                child: Image.network(
                  content.media[index].originalUrl,
                  width: double.infinity,
                  height: 300,
                  fit: BoxFit.cover,
                  errorBuilder: (context, error, stackTrace) => Container(
                    width: double.infinity,
                    height: 300,
                    color: AppColors.neutralGray200,
                    child: const Icon(
                      Icons.image_not_supported,
                      size: 64,
                      color: AppColors.neutralGray400,
                    ),
                  ),
                  loadingBuilder: (context, child, loadingProgress) {
                    if (loadingProgress == null) return child;
                    return Container(
                      width: double.infinity,
                      height: 300,
                      color: AppColors.neutralGray100,
                      child: const Center(child: CircularProgressIndicator()),
                    );
                  },
                ),
              );
            },
          ),
        ),

        // Image count indicator
        if (content.media.length > 1)
          Positioned(
            top: 16,
            right: 16,
            child: Container(
              padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
              decoration: BoxDecoration(
                color: Colors.black.withValues(alpha: 0.6),
                borderRadius: BorderRadius.circular(16),
              ),
              child: Text(
                '${_currentMediaIndex + 1} / ${content.media.length}',
                style: const TextStyle(
                  color: Colors.white,
                  fontSize: 12,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
          ),
      ],
    );
  }

  Widget _buildContentSection(BuildContext context, Content content) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Author info
        _buildAuthorInfo(context, content),
        const SizedBox(height: 16),

        // Content text
        Text(
          content.content,
          style: Theme.of(
            context,
          ).textTheme.bodyLarge?.copyWith(fontSize: 16, height: 1.5),
        ),

        // Tags
        if (content.tags.isNotEmpty) ...[
          const SizedBox(height: 16),
          Wrap(
            spacing: 8,
            runSpacing: 8,
            children: content.tags.map((tag) {
              return Chip(
                label: Text('#$tag'),
                labelStyle: const TextStyle(fontSize: 12),
                padding: EdgeInsets.zero,
                materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
              );
            }).toList(),
          ),
        ],

        // Location
        if (content.location != null) ...[
          const SizedBox(height: 16),
          _buildLocation(content.location!),
        ],

        // Timestamp
        const SizedBox(height: 16),
        Text(
          'Posted on ${_formatDate(content.createdAt)}',
          style: TextStyle(fontSize: 12, color: AppColors.neutralGray500),
        ),
      ],
    );
  }



  Widget _buildAuthorInfo(BuildContext context, Content content) {
    // E6 — Author identity lifecycle redaction. Independent from content
    // lifecycle: an active post by a degraded author still shows the body
    // (handled by _buildContent), but the author identity here renders as
    // an italic placeholder, neutral avatar, verification badge suppressed,
    // tap disabled. Chat shortcut also disabled — chat tools belong to
    // identity that the platform still asserts is interactive.
    final authorDegraded = content.authorLifecycle.isDegraded;
    final authorPlaceholder = _authorRedactionLabel(content.authorLifecycle);
    final showAvatar = !authorDegraded && content.authorAvatarUrl != null;

    return Row(
      children: [
        // Tappable author info area (tap disabled on degraded)
        Expanded(
          child: InkWell(
            onTap: authorDegraded
                ? null
                : () => _navigateToAuthorProfile(context, content),
            borderRadius: BorderRadius.circular(8),
            child: Row(
              children: [
                CircleAvatar(
                  radius: 24,
                  backgroundColor: authorDegraded
                      ? AppColors.neutralGray200
                      : null,
                  backgroundImage: showAvatar
                      ? NetworkImage(content.authorAvatarUrl!)
                      : null,
                  child: showAvatar
                      ? null
                      : Icon(
                          Icons.person,
                          size: 24,
                          color: authorDegraded
                              ? AppColors.neutralGray400
                              : null,
                        ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      // OWNER TRUTH: public identity is @username only.
                      // Legacy bold-name + lighter-handle pair collapses to a
                      // single @username line; the verification badge stays.
                      // E6 — On degraded author lifecycle, render an italic
                      // placeholder label and suppress the verification
                      // badge so a tombstoned identity does not surface
                      // trust signals.
                      Row(
                        children: [
                          if (authorDegraded)
                            Flexible(
                              child: Text(
                                authorPlaceholder,
                                style: const TextStyle(
                                  fontWeight: FontWeight.w600,
                                  fontSize: 16,
                                  fontStyle: FontStyle.italic,
                                  color: AppColors.neutralGray500,
                                ),
                              ),
                            )
                          else if (content.authorUsername != null)
                            Flexible(
                              child: Text(
                                '@${content.authorUsername}',
                                style: const TextStyle(
                                  fontWeight: FontWeight.w600,
                                  fontSize: 16,
                                ),
                              ),
                            ),
                          // VERIFICATION UI: Show verification badge for
                          // content author. Suppressed when the author
                          // lifecycle is degraded.
                          if (!authorDegraded)
                            _ContentAuthorVerificationBadge(
                              authorId: content.authorId,
                            ),
                        ],
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      ],
    );
  }

  /// Content-detail author redaction label.
  /// Delegates to the canonical [ContentLifecycleParse.publicRedactionLabel].
  String _authorRedactionLabel(ContentLifecycle authorLifecycle) =>
      authorLifecycle.publicRedactionLabel;

  /// Navigate to author's profile when author info is tapped
  void _navigateToAuthorProfile(BuildContext context, Content content) {
    if (content.authorId.isNotEmpty) {
      context.push('/user/${content.authorId}');
    }
  }

  Widget _buildLocation(ContentLocation location) {
    return Row(
      children: [
        const Icon(
          Icons.location_on,
          size: 16,
          color: AppColors.neutralGray500,
        ),
        const SizedBox(width: 4),
        Text(
          location.displayLocation,
          style: TextStyle(fontSize: 14, color: AppColors.neutralGray600),
        ),
      ],
    );
  }

  Widget _buildResourceProjection(BuildContext context, Content content) {
    if (content.resourceProjection == null) {
      return const SizedBox.shrink();
    }

    return ContentResourceProjectionCard(
      resourceProjection: content.resourceProjection!,
      onTap: () => _navigateToResourceProjection(context, content),
    );
  }

  Widget _buildEngagementSection(
    BuildContext context,
    Content content, {
    AsyncValue<LikeStats>? likeStatsAsync,
    String? currentUserId,
    String? currentUserName,
  }) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        const Text(
          'Engagement',
          style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
        ),
        const SizedBox(height: 12),
        Row(
          children: [
            const SizedBox(width: 24),
            // Like button - now tappable using canonical Like system
            _buildLikeEngagementItem(
              context,
              content: content,
              likeStatsAsync: likeStatsAsync,
              currentUserId: currentUserId,
              currentUserName: currentUserName,
            ),
            const SizedBox(width: 24),
            // Comments - now tappable to open DiscussionScreen
            InkWell(
              onTap: () => _navigateToComments(context),
              borderRadius: BorderRadius.circular(8),
              child: _buildEngagementItem(
                context,
                icon: Icons.comment_outlined,
                label: 'Comments',
                value: content.engagement.commentCount.toString(),
                isTappable: true,
              ),
            ),
            const SizedBox(width: 24),
            // SHARE CONTRACT V1: Share button - opens ShareBottomSheet
            InkWell(
              onTap: () => _handleShareContent(context, content),
              borderRadius: BorderRadius.circular(8),
              child: _buildEngagementItem(
                context,
                icon: Icons.share_outlined,
                label: 'Share',
                value: '',
                isTappable: true,
              ),
            ),
          ],
        ),
      ],
    );
  }

  /// Build like engagement item with canonical Like system integration
  Widget _buildLikeEngagementItem(
    BuildContext context, {
    required Content content,
    AsyncValue<LikeStats>? likeStatsAsync,
    String? currentUserId,
    String? currentUserName,
  }) {
    // If user is not authenticated, show static like count (not tappable)
    if (currentUserId == null || currentUserId.isEmpty) {
      return _buildEngagementItem(
        context,
        icon: Icons.favorite_border,
        label: 'Likes',
        value: content.engagement.likeCount.toString(),
      );
    }

    // Show tappable like button with live state from canonical Like system
    return likeStatsAsync?.when(
          data: (stats) => InkWell(
            onTap: () => _handleContentLike(
              context,
              content,
              currentUserId,
              currentUserName ?? '',
            ),
            borderRadius: BorderRadius.circular(8),
            child: _buildEngagementItem(
              context,
              icon: stats.isLikedByCurrentUser
                  ? Icons.favorite
                  : Icons.favorite_border,
              label: 'Likes',
              value: stats.totalLikes.toString(),
              isTappable: true,
              isActive: stats.isLikedByCurrentUser,
            ),
          ),
          loading: () => _buildEngagementItem(
            context,
            icon: Icons.favorite_border,
            label: 'Likes',
            value: content.engagement.likeCount.toString(),
          ),
          error: (_, _) => _buildEngagementItem(
            context,
            icon: Icons.favorite_border,
            label: 'Likes',
            value: content.engagement.likeCount.toString(),
          ),
        ) ??
        _buildEngagementItem(
          context,
          icon: Icons.favorite_border,
          label: 'Likes',
          value: content.engagement.likeCount.toString(),
        );
  }

  /// Handle content like action using canonical Like system
  void _handleContentLike(
    BuildContext context,
    Content content,
    String currentUserId,
    String currentUserName,
  ) {
    final handlers = ContentLikeHandlers(
      ref: ref,
      context: context,
      content: content,
    );
    handlers.handleLike(currentUserId, currentUserName);
  }

  /// Handle share action - opens ShareBottomSheet for content
  /// Share flow uses the canonical content share target.
  void _handleShareContent(BuildContext context, Content content) {
    final shareTarget = ShareTarget(
      id: content.id,
      type: ExternalShareType.post,
      title: content.authorUsername != null ? '@${content.authorUsername}' : '',
      description: content.content,
      imageUrl: content.media.isNotEmpty
          ? content.media.first.originalUrl
          : null,
      publicShareUrl: null, // Will use default public share URL generation
    );

    ShareBottomSheet.show(
      context: context,
      target: shareTarget,
      canSharePost: true,
    );
  }

  Widget _buildEngagementItem(
    BuildContext context, {
    required IconData icon,
    required String label,
    required String value,
    bool isTappable = false,
    bool isActive = false,
  }) {
    // Determine color based on state
    Color getColor() {
      if (isActive) return Colors.red; // Liked state
      if (isTappable) return AppColors.primaryRed;
      return AppColors.neutralGray500;
    }

    final color = getColor();

    return Row(
      children: [
        Icon(icon, size: 18, color: color),
        const SizedBox(width: 6),
        Text(
          value,
          style: TextStyle(fontWeight: FontWeight.w600, color: color),
        ),
        const SizedBox(width: 4),
        Text(label, style: TextStyle(fontSize: 12, color: color)),
        if (isTappable && !isActive) ...[
          const SizedBox(width: 4),
          Icon(Icons.chevron_right, size: 14, color: AppColors.primaryRed),
        ],
      ],
    );
  }

  /// Open canonical MediaViewerWidget fullscreen for the tapped media item.
  /// Uses the shared MediaViewerWidget (same as Feed and Commerce Detail).
  void _openMediaViewer(
    BuildContext context,
    Content content,
    int initialIndex,
  ) {
    final mediaUrls = content.media.map((m) => m.originalUrl).toList();
    if (mediaUrls.isEmpty) return;
    showDialog(
      context: context,
      barrierColor: Colors.black87,
      builder: (_) => MediaViewerWidget(
        mediaUrls: mediaUrls,
        initialIndex: initialIndex,
        title: content.authorUsername != null
            ? '@${content.authorUsername}'
            : null,
      ),
    );
  }

  /// Navigate to DiscussionScreen for this content
  void _navigateToComments(BuildContext context) {
    context.push('/comment/content/${widget.contentId}?title=content');
  }

  /// D1 — UNAVAILABLE banner. Mirror of the feed banner pattern at
  /// feed_renderers.dart _buildUnavailableBanner. Rendered when
  /// content.lifecycle == unavailable so the user sees the governance
  /// state at the top of the detail surface.
  Widget _buildUnavailableBanner(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
      decoration: BoxDecoration(
        color: isDark
            ? AppColors.neutralGray800.withValues(alpha: 0.5)
            : AppColors.neutralGray100,
        border: Border(
          bottom: BorderSide(
            color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
          ),
        ),
      ),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(6),
            decoration: BoxDecoration(
              color: AppColors.neutralGray400.withValues(alpha: 0.2),
              shape: BoxShape.circle,
            ),
            child: const Icon(
              Icons.visibility_off_outlined,
              size: 16,
              color: AppColors.neutralGray500,
            ),
          ),
          const SizedBox(width: 8),
          Text(
            'Tidak tersedia',
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray700,
            ),
          ),
        ],
      ),
    );
  }

  /// D1 — REMOVED tombstone. Defense-in-depth visual for the rare case
  /// where lifecycle=removed reaches the screen (existing backend
  /// architectural truth returns 404 for deleted/hidden, so this is a
  /// belt-and-suspenders state — never observed today, never crashes
  /// tomorrow).
  Widget _buildRemovedTombstone(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(
              Icons.remove_circle_outline,
              size: 64,
              color: AppColors.neutralGray400,
            ),
            const SizedBox(height: 16),
            const Text(
              'Konten dihapus',
              style: TextStyle(fontSize: 18, fontWeight: FontWeight.w500),
            ),
            const SizedBox(height: 8),
            const Text(
              'Konten ini sudah tidak tersedia.',
              style: TextStyle(fontSize: 14, color: AppColors.neutralGray500),
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 16),
            ElevatedButton(
              onPressed: () => Navigator.of(context).maybePop(),
              child: const Text('Kembali'),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildError(BuildContext context, String message) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          const Icon(
            Icons.error_outline,
            size: 64,
            color: AppColors.statusError,
          ),
          const SizedBox(height: 16),
          const Text(
            'Failed to load content',
            style: TextStyle(fontSize: 18, fontWeight: FontWeight.w500),
          ),
          const SizedBox(height: 8),
          Text(
            message,
            style: const TextStyle(
              fontSize: 14,
              color: AppColors.neutralGray500,
            ),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 16),
          ElevatedButton(
            onPressed: () {
              ref
                  .read(contentDetailProvider.notifier)
                  .fetchContent(widget.contentId);
            },
            child: const Text('Try Again'),
          ),
        ],
      ),
    );
  }

  String _formatDate(DateTime dateTime) {
    return '${dateTime.day}/${dateTime.month}/${dateTime.year} at ${dateTime.hour.toString().padLeft(2, '0')}:${dateTime.minute.toString().padLeft(2, '0')}';
  }

  void _navigateToResourceProjection(BuildContext context, Content content) {
    final resourceProjection = content.resourceProjection;
    if (resourceProjection != null && resourceProjection.isLive) {
      context.push(resourceProjection.canonicalPath);
    }
  }
}

/// Content Author Verification Badge Widget
///
/// Shows compact verification level badge for content/post author
class _ContentAuthorVerificationBadge extends ConsumerWidget {
  final String authorId;

  const _ContentAuthorVerificationBadge({required this.authorId});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Fetch author's verification data
    final userAsync = ref.watch(userDataProvider(authorId));

    return userAsync.when(
      data: (user) {
        if (user == null) return const SizedBox.shrink();
        final isVerified =
            user.isEmailVerified ||
            (user.isPhoneVerified ?? false) ||
            (user.isIdVerified ?? false) ||
            (user.isFarmVerified ?? false);
        if (!isVerified) return const SizedBox.shrink();
        return const Padding(
          padding: EdgeInsets.only(left: 6),
          child: Icon(Icons.verified, size: 16, color: AppColors.statusInfo),
        );
      },
      loading: () => const SizedBox.shrink(),
      error: (_, _) => const SizedBox.shrink(),
    );
  }
}
