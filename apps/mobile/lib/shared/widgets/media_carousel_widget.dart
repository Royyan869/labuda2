import 'package:flutter/material.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/shared/utils/media_extensions.dart';
import 'app_image.dart';
import 'carousel_video_player.dart';
import 'carousel_indicators.dart';

/// Shared Media Carousel Widget untuk menampilkan multiple images/videos
///
/// Features:
/// - Instagram-style portrait frame (4:5 aspect ratio)
/// - Carousel dengan smooth swipe navigation dan indicators
/// - Image tap untuk fullscreen view / Video play controls
/// - Video playback dengan play/pause controls
/// - Auto-detection untuk image vs video berdasarkan file extension
/// - Loading dan error handling untuk both images dan videos
///
/// Design:
/// - Frame portrait 4:5 (konsisten untuk semua foto)
/// - Media ditampilkan dengan BoxFit.cover (full screen, crop jika perlu)
/// - Multiple photos: Swipe dengan page indicators
///
/// Refactored into modular components:
/// - CarouselVideoPlayer: For video playback
/// - CarouselIndicators: For page indicators and counter
class MediaCarouselWidget extends StatefulWidget {
  /// Media URLs as string list (for backward compatibility)
  final List<String>? mediaUrls;

  /// Media entities (new canonical approach)
  final List<MediaEntity>? media;

  final VoidCallback? onImageTap;
  final Function(int)? onImageTapWithIndex;
  final bool showIndicators;
  final BorderRadius? borderRadius;

  /// Aspect ratio untuk portrait frame (default: 4/5 = 0.8 seperti Instagram)
  final double aspectRatio;

  /// Whether media contains video (optional - will auto-detect from URLs if null)
  final bool? hasVideo;

  const MediaCarouselWidget({
    super.key,
    this.mediaUrls,
    this.media,
    this.onImageTap,
    this.onImageTapWithIndex,
    this.showIndicators = true,
    this.borderRadius,
    this.aspectRatio = 4 / 5, // Portrait frame 4:5
    this.hasVideo,

    /// Asserts that at least one media source is provided
  }) : assert(
         mediaUrls != null || media != null,
         'Either mediaUrls or media must be provided',
       );

  /// Get the actual list of URLs to display
  List<String> get displayUrls => media?.urls ?? mediaUrls ?? const [];

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
  /// Uses hasVideo parameter if provided, otherwise auto-detects from URLs
  bool _shouldShowVideoBadge() {
    if (widget.hasVideo != null) {
      return widget.hasVideo!;
    }
    // Auto-detect: check if any URL is a video
    return widget.displayUrls.any((url) => _isVideoUrl(url));
  }

  /// Deteksi media type berdasarkan URL extension
  bool _isVideoUrl(String url) {
    final videoExtensions = [
      '.mp4',
      '.mov',
      '.avi',
      '.mkv',
      '.wmv',
      '.flv',
      '.webm',
      '.m4v',
    ];
    final lowerUrl = url.toLowerCase();
    return videoExtensions.any((ext) => lowerUrl.contains(ext));
  }

  @override
  Widget build(BuildContext context) {
    if (widget.displayUrls.isEmpty) return const SizedBox.shrink();

    // Single image case
    if (widget.displayUrls.length == 1) {
      return _buildSingleImage();
    }

    // Multiple images case
    return _buildCarousel();
  }

  Widget _buildSingleImage() {
    final mediaUrl = widget.displayUrls.first;
    final isVideo = _isVideoUrl(mediaUrl);
    final showVideoBadge = _shouldShowVideoBadge();

    // Wrap dengan AspectRatio untuk portrait frame 4:5
    return AspectRatio(
      aspectRatio: widget.aspectRatio,
      child: ClipRRect(
        borderRadius: widget.borderRadius ?? BorderRadius.circular(12),
        child: Stack(
          children: [
            isVideo
                ? _buildMediaItem(mediaUrl, 0)
                : _buildImageItem(mediaUrl, 0),

            // Video badge at top-right corner
            if (showVideoBadge)
              Positioned(
                top: 8,
                right: 8,
                child: Container(
                  padding: const EdgeInsets.all(6),
                  decoration: BoxDecoration(
                    color: Colors.black.withValues(alpha: 0.7),
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: const Icon(
                    Icons.play_circle_filled,
                    color: Colors.white,
                    size: 20,
                  ),
                ),
              ),
          ],
        ),
      ),
    );
  }

  /// Build image item
  Widget _buildImageItem(String imageUrl, int index) {
    return GestureDetector(
      onTap: () => _handleImageTap(index),
      child: AppImage(
        imageUrl: imageUrl,
        fit: BoxFit.cover,
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
        borderRadius: widget.borderRadius ?? BorderRadius.circular(12),
        child: Stack(
          children: [
            // PageView carousel
            PageView.builder(
              controller: _pageController,
              itemCount: widget.displayUrls.length,
              onPageChanged: (index) {
                setState(() {
                  _currentIndex = index;
                });
              },
              itemBuilder: (context, index) {
                final mediaUrl = widget.displayUrls[index];
                final isVideo = _isVideoUrl(mediaUrl);

                // Only build video widget for current and adjacent pages
                // This prevents creating too many video players at once which causes buffer overflow
                if (isVideo && (index - _currentIndex).abs() > 1) {
                  return const SizedBox.shrink();
                }

                // Video render biasa
                if (isVideo) {
                  return _buildMediaItem(mediaUrl, index);
                }

                // Image render (no limit needed for images)
                return _buildImageItem(mediaUrl, index);
              },
            ),

            // Video badge at top-right corner
            if (showVideoBadge)
              Positioned(
                top: 8,
                right: 8,
                child: Container(
                  padding: const EdgeInsets.all(6),
                  decoration: BoxDecoration(
                    color: Colors.black.withValues(alpha: 0.7),
                    borderRadius: BorderRadius.circular(8),
                  ),
                  child: const Icon(
                    Icons.play_circle_filled,
                    color: Colors.white,
                    size: 20,
                  ),
                ),
              ),

            // Indicators and counter
            CarouselIndicators(
              currentIndex: _currentIndex,
              totalItems: widget.displayUrls.length,
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
  Widget _buildMediaItem(String mediaUrl, int index) {
    // This method is only used for videos
    // Use LayoutBuilder to get available height from AspectRatio
    return LayoutBuilder(
      builder: (context, constraints) {
        return CarouselVideoPlayer(
          videoUrl: mediaUrl,
          width: constraints.maxWidth,
          height: constraints.maxHeight,
          fit: BoxFit.cover,
          onFullscreenTap: () => _handleImageTap(
            index,
          ), // Custom fullscreen button opens MediaViewerWidget
        );
      },
    );
  }
}
