import 'dart:ui';
import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_colors.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'stable_network_image.dart';
import 'media_viewer_video_player.dart';

/// Shared Media Viewer Widget untuk fullscreen image/video viewing
///
/// CANONICAL CONTENT MEDIA SURFACE:
/// - [MediaEntity.type] is the render authority. A `MediaType.video` entity
///   renders through [MediaViewerVideoPlayer]; an image entity renders
///   through [StableNetworkImage] (the shared network-media path that
///   projects the reference through `resolveNetworkImageUrl`). A video
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
    return Scaffold(
      backgroundColor: AppColors.dark,
      appBar: AppBar(
        backgroundColor: Colors.transparent,
        foregroundColor: AppColors.light,
        elevation: 0,
        surfaceTintColor: Colors.transparent,
        scrolledUnderElevation: 0,
        title: Text(
          widget.title ?? '${_currentIndex + 1} / ${widget.media.length}',
          style: const TextStyle(
            color: AppColors.light,
            fontSize: 16,
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
        return Center(child: _buildMediaItem(widget.media[index]));
      },
    );
  }

  /// Build media item (image atau video) untuk fullscreen viewer.
  ///
  /// [MediaEntity.type] is the render authority — no extension sniffing, no
  /// fallback decoder for video references.
  Widget _buildMediaItem(MediaEntity media) {
    if (media.type == MediaType.video) {
      return MediaViewerVideoPlayer(videoUrl: media.originalUrl);
    }
    return _buildImage(media.originalUrl);
  }

  /// Image frame: blurred backdrop + zoomable canonical image. Both layers
  /// render through [StableNetworkImage], the shared network-media path.
  Widget _buildImage(String imageUrl) {
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
            child: StableNetworkImage(
              imageUrl: imageUrl,
              fit: BoxFit.cover,
              fallback: const SizedBox.shrink(),
            ),
          ),
        ),
        // Dark overlay
        Container(color: Colors.black.withValues(alpha: 0.1)),
        // Main image centered dengan InteractiveViewer untuk zoom
        InteractiveViewer(
          minScale: 0.5,
          maxScale: 3.0,
          child: StableNetworkImage(
            imageUrl: imageUrl,
            fit: BoxFit.contain,
            fallback: const SizedBox.shrink(),
          ),
        ),
      ],
    );
  }
}
