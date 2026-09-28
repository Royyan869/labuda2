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
import 'package:labuda/shared/widgets/carousel_video_player.dart';


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
          // Content section — avatar/username/time + text first (canonical)
          SliverPadding(
            padding: const EdgeInsets.all(AppMetrics.p16),
            sliver: SliverToBoxAdapter(
              child: _buildContentSection(context, content),
            ),
          ),
          // Media — below identity+text (canonical)
          if (content.media.isNotEmpty)
            SliverToBoxAdapter(child: _buildMediaSection(context, content)),
          // Linked items (canonical resource projection only)
          if (content.resourceProjection != null)
            SliverPadding(
              padding: const EdgeInsets.all(AppMetrics.p16),
              sliver: SliverToBoxAdapter(
                child: _buildResourceProjection(context, content),
              ),
            ),
          // Engagement — icon+count only (canonical, no labels)
          SliverPadding(
            padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16, vertical: AppMetrics.p12),
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
                child: _buildMediaFrame(context, content, index),
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
              padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p6),
              decoration: BoxDecoration(
                color: Theme.of(context).colorScheme.scrim.withValues(alpha: 0.6),
                borderRadius: BorderRadius.circular(AppShape.r16),
              ),
              child: Text(
                '${_currentMediaIndex + 1} / ${content.media.length}',
                style: TextStyle(
                  color: Theme.of(context).colorScheme.onPrimary,
                  fontSize: AppType.s12,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
          ),
      ],
    );
  }

  /// Canonical content media frame for the detail hero.
  ///
  /// [MediaEntity.type] is the render authority: images go through [AppImage]
  /// (CloudFront URL as-is, cached), videos through [CarouselVideoPlayer].
  /// A video reference is never handed to the image decoder.
  Widget _buildMediaFrame(BuildContext context, Content content, int index) {
    final media = content.media[index];

    if (media.type == MediaType.video) {
      return LayoutBuilder(
        builder: (context, constraints) => CarouselVideoPlayer(
          videoUrl: media.originalUrl,
          width: constraints.maxWidth,
          height: constraints.maxHeight,
          fit: BoxFit.cover,
          onFullscreenTap: () => _openMediaViewer(context, content, index),
        ),
      );
    }

    return AppImage(
      imageUrl: media.originalUrl,
      fit: BoxFit.cover,
      backgroundColor: Theme.of(context).colorScheme.surfaceContainerHighest,
      errorWidget: _buildMediaPlaceholder(),
    );
  }

  /// Error icon shown when the media cannot load. Loading shows the AppImage
  /// shimmer — the two states are never the same widget.
  Widget _buildMediaPlaceholder() {
    return Builder(
      builder: (context) {
        final scheme = Theme.of(context).colorScheme;
        return Container(
          width: double.infinity,
          height: 300,
          color: scheme.surfaceContainerHighest,
          child: Icon(Icons.image, size: 64, color: scheme.onSurfaceVariant),
        );
      },
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
          ).textTheme.bodyLarge?.copyWith(fontSize: AppType.s16, height: 1.5),
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
                labelStyle: const TextStyle(fontSize: AppType.s12),
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
      ],
    );
  }

  Widget _buildAuthorInfo(BuildContext context, Content content) {
    final scheme = Theme.of(context).colorScheme;
    final authorDegraded = content.authorLifecycle.isDegraded;
    final authorPlaceholder = _authorRedactionLabel(content.authorLifecycle);
    final showAvatar = !authorDegraded && content.authorAvatarUrl != null;
    final authState = ref.watch(authControllerProvider);
    final isOwner =
        authState is AuthStateAuthenticated &&
        authState.user.id == content.authorId;
    final visibilityIcon = isOwner
        ? _visibilityIcon(content.settings.visibility)
        : null;

    return Row(
      children: [
        Expanded(
          child: InkWell(
            onTap: authorDegraded
                ? null
                : () => _navigateToAuthorProfile(context, content),
            borderRadius: BorderRadius.circular(AppShape.r8),
            child: Row(
              children: [
                ProfileAvatar(
                  userId: content.authorId,
                  size: 40,
                  imageUrl: showAvatar ? content.authorAvatarUrl : null,
                ),
                const SizedBox(width: 10),
                Expanded(
                  child: Row(
                    children: [
                      if (authorDegraded)
                        Flexible(
                          child: Text(
                            authorPlaceholder,
                            style: TextStyle(
                              fontWeight: FontWeight.w600,
                              fontSize: AppType.s14,
                              fontStyle: FontStyle.italic,
                              color: scheme.onSurfaceVariant,
                            ),
                          ),
                        )
                      else if (content.authorUsername != null)
                        Flexible(
                          child: Text(
                            '@${content.authorUsername}',
                            style: const TextStyle(
                              fontWeight: FontWeight.w600,
                              fontSize: AppType.s14,
                            ),
                          ),
                        ),
                      if (!authorDegraded)
                        _ContentAuthorVerificationBadge(
                          authorId: content.authorId,
                        ),
                      if (visibilityIcon != null) ...[
                        const SizedBox(width: 6),
                        Icon(
                          visibilityIcon,
                          size: 14,
                          color: scheme.onSurfaceVariant,
                        ),
                      ],
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
        const SizedBox(width: 8),
        Text(
          _formatTime(content.createdAt),
          style: TextStyle(fontSize: AppType.s12, color: scheme.onSurfaceVariant),
        ),
      ],
    );
  }

  IconData? _visibilityIcon(ContentVisibility visibility) {
    switch (visibility) {
      case ContentVisibility.public:
        return Icons.public;
      case ContentVisibility.followersOnly:
        return Icons.people_outline;
      case ContentVisibility.private:
        return Icons.lock_outline;
    }
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
    return Builder(
      builder: (context) {
        final scheme = Theme.of(context).colorScheme;
        return Row(
          children: [
            Icon(
              Icons.location_on,
              size: 16,
              color: scheme.onSurfaceVariant,
            ),
            const SizedBox(width: 4),
            Text(
              location.displayLocation,
              style: TextStyle(fontSize: AppType.s14, color: scheme.onSurfaceVariant),
            ),
          ],
        );
      },
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
    final scheme = Theme.of(context).colorScheme;
    return Row(
      children: [
        _buildLikeEngagementItem(
          context,
          content: content,
          likeStatsAsync: likeStatsAsync,
          currentUserId: currentUserId,
          currentUserName: currentUserName,
        ),
        const SizedBox(width: 16),
        InkWell(
          onTap: () => _navigateToComments(context),
          borderRadius: BorderRadius.circular(AppShape.r8),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p4, vertical: AppMetrics.p6),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  Icons.chat_bubble_outline,
                  size: 16,
                  color: Theme.of(context).colorScheme.primary,
                ),
                if (content.engagement.commentCount > 0) ...[
                  const SizedBox(width: 4),
                  Text(
                    '${content.engagement.commentCount}',
                    style: TextStyle(
                      fontSize: AppType.s12,
                      color: Theme.of(context).colorScheme.primary,
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                ],
              ],
            ),
          ),
        ),
        const SizedBox(width: 16),
        InkWell(
          onTap: () => _handleShareContent(context, content),
          borderRadius: BorderRadius.circular(AppShape.r8),
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p4, vertical: AppMetrics.p6),
            child: Icon(
              Icons.share_outlined,
              size: 16,
              color: scheme.onSurfaceVariant,
            ),
          ),
        ),
      ],
    );
  }

  Widget _buildLikeEngagementItem(
    BuildContext context, {
    required Content content,
    AsyncValue<LikeStats>? likeStatsAsync,
    String? currentUserId,
    String? currentUserName,
  }) {
    Widget buildLikeRow({
      required IconData icon,
      required int count,
      required bool isActive,
      VoidCallback? onTap,
    }) {
      final scheme = Theme.of(context).colorScheme;
      final color = isActive
          ? scheme.primary
          : (onTap != null ? scheme.primary : scheme.onSurfaceVariant);
      return InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppShape.r8),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p4, vertical: AppMetrics.p6),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(icon, size: 16, color: color),
              if (count > 0) ...[
                const SizedBox(width: 4),
                Text(
                  '$count',
                  style: TextStyle(
                    fontSize: AppType.s12,
                    color: color,
                    fontWeight: FontWeight.w600,
                  ),
                ),
              ],
            ],
          ),
        ),
      );
    }

    if (currentUserId == null || currentUserId.isEmpty) {
      return buildLikeRow(
        icon: Icons.favorite_border,
        count: content.engagement.likeCount,
        isActive: false,
      );
    }

    return likeStatsAsync?.when(
          data: (stats) => buildLikeRow(
            icon: stats.isLikedByCurrentUser
                ? Icons.favorite
                : Icons.favorite_border,
            count: stats.totalLikes,
            isActive: stats.isLikedByCurrentUser,
            onTap: () => _handleContentLike(
              context,
              content,
              currentUserId,
              currentUserName ?? '',
            ),
          ),
          loading: () => buildLikeRow(
            icon: Icons.favorite_border,
            count: content.engagement.likeCount,
            isActive: false,
          ),
          error: (_, _) => buildLikeRow(
            icon: Icons.favorite_border,
            count: content.engagement.likeCount,
            isActive: false,
          ),
        ) ??
        buildLikeRow(
          icon: Icons.favorite_border,
          count: content.engagement.likeCount,
          isActive: false,
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

  /// Open the canonical MediaViewerWidget fullscreen for the tapped media item.
  ///
  /// The Content media entities are handed over verbatim so the viewer renders
  /// by [MediaEntity.type] — image through [AppImage], video through
  /// [MediaViewerVideoPlayer] — instead of sniffing the file extension of a
  /// flattened URL list.
  void _openMediaViewer(
    BuildContext context,
    Content content,
    int initialIndex,
  ) {
    if (content.media.isEmpty) return;
    showDialog(
      context: context,
      barrierColor: Theme.of(context).colorScheme.scrim.withValues(alpha: 0.87),
      builder: (_) => MediaViewerWidget(
        media: content.media,
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
    final scheme = Theme.of(context).colorScheme;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16, vertical: AppMetrics.p12),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHigh,
        border: Border(
          bottom: BorderSide(
            color: scheme.outlineVariant,
          ),
        ),
      ),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(AppMetrics.p6),
            decoration: BoxDecoration(
              color: scheme.onSurfaceVariant.withValues(alpha: 0.2),
              shape: BoxShape.circle,
            ),
            child: Icon(
              Icons.visibility_off_outlined,
              size: 16,
              color: scheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(width: 8),
          Text(
            'Tidak tersedia',
            style: TextStyle(
              fontSize: AppType.s13,
              fontWeight: FontWeight.w600,
              color: scheme.onSurface,
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
    final scheme = Theme.of(context).colorScheme;
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.remove_circle_outline,
              size: 64,
              color: scheme.outline,
            ),
            const SizedBox(height: 16),
            const Text(
              'Konten dihapus',
              style: TextStyle(fontSize: AppType.s18, fontWeight: FontWeight.w500),
            ),
            const SizedBox(height: 8),
            Text(
              'Konten ini sudah tidak tersedia.',
              style: TextStyle(fontSize: AppType.s14, color: scheme.onSurfaceVariant),
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
    final scheme = Theme.of(context).colorScheme;
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.error_outline,
            size: 64,
            color: scheme.error,
          ),
          const SizedBox(height: 16),
          const Text(
            'Failed to load content',
            style: TextStyle(fontSize: AppType.s18, fontWeight: FontWeight.w500),
          ),
          const SizedBox(height: 8),
          Text(
            message,
            style: TextStyle(
              fontSize: AppType.s14,
              color: scheme.onSurfaceVariant,
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

  String _formatTime(DateTime dateTime) {
    final now = DateTime.now();
    final diff = now.difference(dateTime);
    if (diff.inMinutes < 1) return 'baru saja';
    if (diff.inMinutes < 60) return '${diff.inMinutes}m';
    if (diff.inHours < 24) return '${diff.inHours}j';
    if (diff.inDays < 7) return '${diff.inDays}h';
    return '${dateTime.day}/${dateTime.month}/${dateTime.year}';
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
        return Padding(
          padding: EdgeInsets.only(left: AppMetrics.p6),
          child: Icon(Icons.verified, size: 16, color: context.statusColors.info),
        );
      },
      loading: () => const SizedBox.shrink(),
      error: (_, _) => const SizedBox.shrink(),
    );
  }
}
