import 'package:flutter/material.dart';
import 'package:hishumi/domains/social/content/domain/entities/content.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';
import 'app_image.dart';
import 'carousel_video_player.dart';
import 'carousel_indicators.dart';

/// Shared Media Carousel Widget untuk menampilkan multiple images/videos.
///
/// Koi rules (locked): frame 4:5 portrait, `contain` (tidak pernah crop),
/// [MediaEntity.type] is the render authority — type is never inferred from
/// a URL. Single source: [media] entities (original URLs — detail adalah
/// ruang meneliti, bukan daftar).
///
/// Refactored into modular components:
/// - CarouselVideoPlayer: For video playback
/// - CarouselIndicators: For page indicators and counter
class MediaCarouselWidget extends StatefulWidget {
  /// Canonical media entities in owner order.
  final List<MediaEntity> media;

  final VoidCallback? onImageTap;
  final Function(int)? onImageTapWithIndex;
  final bool showIndicators;
  final BorderRadius? borderRadius;

  /// Frame aspect ratio (default: 4/5 portrait seperti kartu marketplace).
  final double aspectRatio;

  /// Render fit for images (default: contain — koi tidak boleh terpotong).
  final BoxFit fit;

  /// Whether media contains video (optional — defaults to entity-type check).
  final bool? hasVideo;

  const MediaCarouselWidget({
    super.key,
    required this.media,
    this.onImageTap,
    this.onImageTapWithIndex,
    this.showIndicators = true,
    this.borderRadius,
    this.aspectRatio = 4 / 5,
    this.fit = BoxFit.contain,
    this.hasVideo,
  }) : assert(media.length > 0, 'Media must not be empty');

  @override
  State<MediaCarouselWidget> createState() => _MediaCarouselWidgetState();
}

class _MediaCarouselWidgetState extends State<MediaCarouselWidget> {
  late PageController _pageController;
  int _currentIndex = 0;

  @override
  void initState() {
    super.initState();
    _pageController = PageController();
  }

  @override
  void dispose() {
    _pageController.dispose();
    super.dispose();
  }

  /// Check if should show video badge
  /// Uses hasVideo parameter if provided, otherwise entity-type check.
  bool _shouldShowVideoBadge() {
    if (widget.hasVideo != null) {
      return widget.hasVideo!;
    }
    return widget.media.any((m) => m.type == MediaType.video);
  }

  @override
  Widget build(BuildContext context) {
    if (widget.media.isEmpty) return const SizedBox.shrink();

    // Single image case
    if (widget.media.length == 1) {
      return _buildSingleImage();
    }

    // Multiple images case
    return _buildCarousel();
  }

  Widget _buildSingleImage() {
    final item = widget.media.first;
    final isVideo = item.type == MediaType.video;
    final showVideoBadge = _shouldShowVideoBadge();

    // Wrap dengan AspectRatio untuk portrait frame 4:5
    return AspectRatio(
      aspectRatio: widget.aspectRatio,
      child: ClipRRect(
        borderRadius: widget.borderRadius ?? BorderRadius.circular(AppShape.r12),
        child: Stack(
          children: [
            isVideo
                ? _buildMediaItem(item, 0)
                : _buildImageItem(item, 0),

            // Video badge at top-right corner
            if (showVideoBadge)
              Positioned(
                top: 8,
                right: 8,
                child: Container(
                  padding: const EdgeInsets.all(AppMetrics.p8),
                  decoration: BoxDecoration(
                    color: Theme.of(
                      context,
                    ).colorScheme.scrim.withValues(alpha: 0.7),
                    borderRadius: BorderRadius.circular(AppShape.r8),
                  ),
                  child: Icon(
                    Icons.play_circle_filled,
                    color: Theme.of(context).colorScheme.onPrimary,
                    size: AppIconSize.action,
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  /// Build image item — original URL (detail is for scrutiny) with the
  /// widget-level fit (contain default: koi never cropped).
  Widget _buildImageItem(MediaEntity item, int index) {
    return GestureDetector(
      onTap: () => _handleImageTap(index),
      child: AppImage(
        imageUrl: item.originalUrl,
        fit: widget.fit,
        width: double.infinity,
        height: double.infinity,
      ),
    );
  }

  Widget _buildCarousel() {
    final showVideoBadge = _shouldShowVideoBadge();

    return AspectRatio(
      aspectRatio: widget.aspectRatio,
      child: ClipRRect(
        borderRadius: widget.borderRadius ?? BorderRadius.circular(AppShape.r12),
        child: Stack(
          children: [
            // PageView carousel
            PageView.builder(
              controller: _pageController,
              itemCount: widget.media.length,
              onPageChanged: (index) {
                setState(() {
                  _currentIndex = index;
                });
              },
              itemBuilder: (context, index) {
                final item = widget.media[index];
                final isVideo = item.type == MediaType.video;

                // Only build video widget for current and adjacent pages
                // This prevents creating too many video players at once which causes buffer overflow
                if (isVideo && (index - _currentIndex).abs() > 1) {
                  return const SizedBox.shrink();
                }

                // Video render biasa
                if (isVideo) {
                  return _buildMediaItem(item, index);
                }

                // Image render (no limit needed for images)
                return _buildImageItem(item, index);
              },
            ),

            // Video badge at top-right corner
            if (showVideoBadge)
              Positioned(
                top: 8,
                right: 8,
                child: Container(
                  padding: const EdgeInsets.all(AppMetrics.p8),
                  decoration: BoxDecoration(
                    color: Theme.of(
                      context,
                    ).colorScheme.scrim.withValues(alpha: 0.7),
                    borderRadius: BorderRadius.circular(AppShape.r8),
                  ),
                  child: Icon(
                    Icons.play_circle_filled,
                    color: Theme.of(context).colorScheme.onPrimary,
                    size: AppIconSize.action,
                  ),
                ),
              ),

            // Indicators and counter
            CarouselIndicators(
              currentIndex: _currentIndex,
              totalItems: widget.media.length,
              showIndicators: widget.showIndicators,
              showCounter: true,
            ),
          ],
        ),
      ),
    );
  }

  void _handleImageTap(int index) {
    if (widget.onImageTapWithIndex != null) {
      widget.onImageTapWithIndex!(index);
    } else if (widget.onImageTap != null) {
      widget.onImageTap!();
    }
  }

  /// Build media item (video only)
  Widget _buildMediaItem(MediaEntity item, int index) {
    final mediaUrl = item.originalUrl;
    // This method is only used for videos
    // Use LayoutBuilder to get available height from AspectRatio
    return LayoutBuilder(
      builder: (context, constraints) {
        return CarouselVideoPlayer(
          videoUrl: mediaUrl,
          width: constraints.maxWidth,
          height: constraints.maxHeight,
          fit: widget.fit,
          // Visibility-aware: only the current page initializes a
          // controller; adjacent pages stay on the poster mat.
          isActive: index == _currentIndex,
          posterUrl: item.thumbnailUrl != item.originalUrl
              ? item.thumbnailUrl
              : null,
          onFullscreenTap: () => _handleImageTap(
            index,
          ), // Custom fullscreen button opens MediaViewerWidget
        );
      },
    );
  }
}
