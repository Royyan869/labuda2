import 'dart:ui';
import 'package:flutter/material.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/core/core.dart';
import 'app_image.dart';
import 'media_viewer_video_player.dart';

/// Shared Media Viewer Widget untuk fullscreen image/video viewing
///
/// CANONICAL CONTENT MEDIA SURFACE:
/// - [MediaEntity.type] is the render authority. A `MediaType.video` entity
///   renders through [MediaViewerVideoPlayer]; an image entity renders
///   through [AppImage] (CloudFront URL as-is, cached). A video
///   reference is never handed to the image decoder, and the render decision
///   is never inferred from a file extension.
///
/// Features:
/// - Instagram-style fullscreen viewer dengan black background
/// - Swipe navigation untuk multiple images/videos
/// - Pinch to zoom untuk images dengan InteractiveViewer
/// - Video playback controls dengan play/pause dan progress bar
/// - Media counter di AppBar
/// - Smooth page transitions
/// - Error handling dan retry untuk video loading
class MediaViewerWidget extends StatefulWidget {
  /// Canonical media entities. The list is the render authority — order and
  /// type come straight from the persisted Content media projection.
  final List<MediaEntity> media;

  final int initialIndex;
  final String? title;

  const MediaViewerWidget({
    super.key,
    required this.media,
    this.initialIndex = 0,
    this.title,
  });

  @override
  State<MediaViewerWidget> createState() => _MediaViewerWidgetState();
}

class _MediaViewerWidgetState extends State<MediaViewerWidget> {
  late PageController _pageController;
  late int _currentIndex;

  @override
  void initState() {
    super.initState();
    _currentIndex = widget.initialIndex;
    _pageController = PageController(initialPage: widget.initialIndex);
  }

  @override
  void dispose() {
    _pageController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    // Immersive media room: fixed dark via the scheme scrim role in both
    // modes (photo-bound, not a theme surface) — identical pixels.
    final scheme = Theme.of(context).colorScheme;
    return Scaffold(
      backgroundColor: scheme.scrim,
      appBar: AppBar(
        backgroundColor: Colors.transparent,
        foregroundColor: scheme.onPrimary,
        elevation: AppElevation.none,
        surfaceTintColor: Colors.transparent,
        scrolledUnderElevation: 0,
        title: Text(
          widget.title ?? '${_currentIndex + 1} / ${widget.media.length}',
          style: TextStyle(
            color: scheme.onPrimary,
            fontSize: AppType.s16,
            fontWeight: FontWeight.w500,
          ),
        ),
        centerTitle: true,
      ),
      body: SafeArea(
        child: Stack(
          children: [
            // Main PageView for media
            _buildMediaPageView(),

            // Bottom page indicators - disabled for cleaner fullscreen view
            // if (widget.media.length > 1)
            //   Positioned(
            //     bottom: 16,
            //     left: 0,
            //     right: 0,
            //     child: MediaViewerIndicators(
            //       currentIndex: _currentIndex,
            //       totalItems: widget.media.length,
            //     ),
            //   ),
          ],
        ),
      ),
    );
  }

  Widget _buildMediaPageView() {
    return PageView.builder(
      controller: _pageController,
      itemCount: widget.media.length,
      onPageChanged: (index) {
        setState(() {
          _currentIndex = index;
        });
      },
      itemBuilder: (context, index) {
        return Center(
          child: _buildMediaItem(widget.media[index], index == _currentIndex),
        );
      },
    );
  }

  /// Build media item (image atau video) untuk fullscreen viewer.
  ///
  /// [MediaEntity.type] is the render authority — no extension sniffing, no
  /// fallback decoder for video references.
  Widget _buildMediaItem(MediaEntity media, bool isActive) {
    if (media.type == MediaType.video) {
      return MediaViewerVideoPlayer(
        videoUrl: media.originalUrl,
        posterUrl: media.thumbnailUrl != media.originalUrl
            ? media.thumbnailUrl
            : null,
        // Visibility-aware: swiping to a sibling pauses this player and
        // only the current page holds an initialized controller.
        isActive: isActive,
      );
    }
    return _buildImage(media);
  }

  /// Image frame: blurhash first (instant), blurred backdrop + zoomable
  /// canonical image after. All layers render through [AppImage]; the shared
  /// HTTP cache serves the repeated layers.
  Widget _buildImage(MediaEntity media) {
    return Stack(
      fit: StackFit.expand,
      children: [
        // Background blur layer
        ClipRect(
          child: ImageFiltered(
            imageFilter: ImageFilter.blur(
              sigmaX: 20,
              sigmaY: 20,
              tileMode: TileMode.decal,
            ),
            child: AppImage(
              imageUrl: media.originalUrl,
              blurhash: media.blurhash,
              fit: BoxFit.cover,
              errorWidget: const SizedBox.shrink(),
            ),
          ),
        ),
        // Dark overlay
        Container(
          color: Theme.of(context).colorScheme.scrim.withValues(alpha: 0.1),
        ),
        // Main image centered dengan InteractiveViewer untuk zoom
        InteractiveViewer(
          minScale: 0.5,
          maxScale: 3.0,
          child: AppImage(
            imageUrl: media.originalUrl,
            blurhash: media.blurhash,
            fit: BoxFit.contain,
            errorWidget: const SizedBox.shrink(),
          ),
        ),
      ],
    );
  }
}
