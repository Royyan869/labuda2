/// Content Detail Screen - Shows universal content detail with full content
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/domains/system/shared/domain/services/time_format_service.dart';
import 'package:hishumi/shared/shared.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';
import 'package:hishumi/domains/social/content/content.dart';
import 'package:hishumi/domains/social/content/presentation/providers/content_state.dart';
import 'package:hishumi/domains/social/content/presentation/widgets/content_resource_projection_card.dart';
import 'package:hishumi/domains/social/share/share.dart';
import 'package:hishumi/domains/social/content/presentation/widgets/content_engagement_actions.dart';
import 'package:hishumi/domains/user/profile/presentation/providers/user_data_provider.dart';
import 'package:hishumi/domains/system/report/domain/entities/entities.dart';
import 'package:hishumi/shared/widgets/carousel_video_player.dart';

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

    final authState = ref.watch(authControllerProvider);

    return Scaffold(
      appBar: _buildAppBar(context, detailState, authState),
      // This screen has NO bottom action bar, so the BODY is the layer that
      // owns the system bottom inset: `SafeArea` consumes the real, live inset
      // exactly once (Scaffold has already removed the top inset because an
      // AppBar is present), replacing the fixed 100px spacer that used to stand
      // in for the inset and stayed behind when the system bar disappeared.
      body: SafeArea(
        child: detailState.map(
          initial: (_) => const SizedBox.shrink(),
          loading: (_) => const Center(child: CircularProgressIndicator()),
          loaded: (state) => _buildContent(context, state.content),
          error: (state) => _buildError(context, state.message),
        ),
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
        icon: const Icon(Icons.arrow_back, semanticLabel: 'Kembali'),
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
        ref.read(navigationHandlerProvider).navigateToSignIn();
      }
      return;
    }

    // Check if user is trying to report their own content
    if (content.authorId == authState.user.id) {
      if (mounted) {
        AppSnackBar.showError(
          context,
          'Tidak dapat melaporkan konten Anda sendiri',
        );
      }
      return;
    }

    // Open the canonical report destination (content reporting is ENABLED)
    await context.push<bool>(
      RoutePaths.reportLocation(
        targetType: ReportTargetType.content.name,
        targetId: content.id,
        targetTitle: content.content.substring(
          0,
          content.content.length > 100 ? 100 : content.content.length,
        ),
      ),
    );
  }

  Widget _buildContent(BuildContext context, Content content) {
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
          // Engagement — the canonical icon+count producer (like/comment/share)
          SliverPadding(
            padding: const EdgeInsets.symmetric(
              horizontal: AppMetrics.p16,
              vertical: AppMetrics.p12,
            ),
            sliver: SliverToBoxAdapter(
              child: ContentEngagementActions(
                targetId: content.id,
                targetOwnerId: content.authorId,
                likeCount: content.engagement.likeCount,
                commentCount: content.engagement.commentCount,
                onComment: () => _navigateToComments(context),
                onShare: () => _handleShareContent(context, content),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildMediaSection(BuildContext context, Content content) {
    return Stack(
      children: [
        // Main image
        AspectRatio(
          aspectRatio: 4 / 5,
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
              padding: const EdgeInsets.symmetric(
                horizontal: AppMetrics.p12,
                vertical: AppMetrics.p8,
              ),
              decoration: BoxDecoration(
                color: Theme.of(
                  context,
                ).colorScheme.scrim.withValues(alpha: 0.6),
                borderRadius: BorderRadius.circular(AppShape.r16),
              ),
              child: Text(
                '${_currentMediaIndex + 1} / ${content.media.length}',
                style: context.typeRoles.labelMicro.copyWith(
                  color: Theme.of(context).colorScheme.onPrimary,
                  fontWeight: FontWeight.w600,
                ),
              ),
            ),
          ),
      ],
    );
  }

  /// Canonical content media frame for the detail hero: 4:5 contain like every
  /// other surface (koi never cropped).
  ///
  /// [MediaEntity.type] is the render authority: images go through [AppImage]
  /// (original URL — detail is for scrutiny), videos through
  /// [CarouselVideoPlayer]. A video reference is never handed to the image
  /// decoder.
  Widget _buildMediaFrame(BuildContext context, Content content, int index) {
    final media = content.media[index];

    if (media.type == MediaType.video) {
      return LayoutBuilder(
        builder: (context, constraints) => CarouselVideoPlayer(
          videoUrl: media.originalUrl,
          width: constraints.maxWidth,
          height: constraints.maxHeight,
          fit: BoxFit.contain,
          posterUrl: media.thumbnailUrl != media.originalUrl
              ? media.thumbnailUrl
              : null,
          onFullscreenTap: () => _openMediaViewer(context, content, index),
        ),
      );
    }

    return AppImage(
      imageUrl: media.originalUrl,
      blurhash: media.blurhash,
      fit: BoxFit.contain,
      backgroundColor: Theme.of(context).colorScheme.surfaceContainerHighest,
      errorWidget: _buildMediaPlaceholder(),
    );
  }

  /// Error icon shown when the media cannot load. Loading shows the static
  /// mat — the two states are never the same widget.
  Widget _buildMediaPlaceholder() {
    return Builder(
      builder: (context) {
        final scheme = Theme.of(context).colorScheme;
        return Container(
          width: double.infinity,
          height: double.infinity,
          color: scheme.surfaceContainerHighest,
          child: Icon(
            Icons.image,
            size: AppIconSize.display,
            color: scheme.onSurfaceVariant,
          ),
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
          style: Theme.of(context).textTheme.bodyLarge?.copyWith(height: 1.5),
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
                labelStyle: Theme.of(context).textTheme.labelLarge,
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
                            style: context.typeRoles.bodyDense.copyWith(
                              fontWeight: FontWeight.w600,
                              fontStyle: FontStyle.italic,
                              color: scheme.onSurfaceVariant,
                            ),
                          ),
                        )
                      else if (content.authorUsername != null)
                        Flexible(
                          child: Text(
                            '@${content.authorUsername}',
                            style: context.typeRoles.bodyDense.copyWith(
                              fontWeight: FontWeight.w600,
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
                          size: AppIconSize.inlineGlyph,
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
        // Trailing timestamp is compact secondary metadata: flex-bounded so
        // it can never push the author row out of bounds, with the same
        // single-line ellipsis strategy as the username body.
        Flexible(
          child: Text(
            const TimeFormatService().formatTimeAgo(content.createdAt),
            style: context.typeRoles.labelMicro.copyWith(
              color: scheme.onSurfaceVariant,
            ),
            maxLines: 1,
            overflow: TextOverflow.ellipsis,
          ),
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
        return AddressLocationView(
          location: location.displayLocation,
          mode: AddressLocationMode.compact,
          icon: Icons.location_on,
          iconSize: AppIconSize.inlineGlyph,
          spacing: 4,
          style: context.typeRoles.bodyDense.copyWith(
            color: scheme.onSurfaceVariant,
          ),
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
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p16,
        vertical: AppMetrics.p12,
      ),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHigh,
        border: Border(bottom: BorderSide(color: scheme.outlineVariant)),
      ),
      child: Row(
        children: [
          Container(
            padding: const EdgeInsets.all(AppMetrics.p8),
            decoration: BoxDecoration(
              color: scheme.onSurfaceVariant.withValues(alpha: 0.2),
              shape: BoxShape.circle,
            ),
            child: Icon(
              Icons.visibility_off_outlined,
              size: AppIconSize.inlineGlyph,
              color: scheme.onSurfaceVariant,
            ),
          ),
          const SizedBox(width: 8),
          Text(
            'Tidak tersedia',
            style: context.typeRoles.bodyDense.copyWith(
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
              size: AppIconSize.display,
              color: scheme.outline,
            ),
            const SizedBox(height: 16),
            Text(
              'Konten dihapus',
              style: context.typeRoles.titleProminent.copyWith(
                fontWeight: FontWeight.w500,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              'Konten ini sudah tidak tersedia.',
              style: context.typeRoles.bodyDense.copyWith(
                color: scheme.onSurfaceVariant,
              ),
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
            size: AppIconSize.display,
            color: scheme.error,
          ),
          const SizedBox(height: 16),
          Text(
            'Failed to load content',
            style: context.typeRoles.titleProminent.copyWith(
              fontWeight: FontWeight.w500,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            message,
            style: context.typeRoles.bodyDense.copyWith(
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
          padding: EdgeInsets.only(left: AppMetrics.p8),
          child: Icon(
            Icons.verified,
            size: AppIconSize.inlineGlyph,
            color: context.statusColors.info,
          ),
        );
      },
      loading: () => const SizedBox.shrink(),
      error: (_, _) => const SizedBox.shrink(),
    );
  }
}
