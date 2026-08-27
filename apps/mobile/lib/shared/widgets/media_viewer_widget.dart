import 'dart:ui';
import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_colors.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/shared/utils/media_extensions.dart';
import 'app_image.dart';
import 'media_viewer_video_player.dart';
import 'media_viewer_utils.dart';

/// Shared Media Viewer Widget untuk fullscreen image/video viewing
///
/// Features:
/// - Instagram-style fullscreen viewer dengan black background
/// - Swipe navigation untuk multiple images/videos
/// - Pinch to zoom untuk images dengan InteractiveViewer
/// - Video playback controls dengan play/pause dan progress bar
/// - Auto-detection untuk image vs video berdasarkan file extension
/// - Navigation arrows untuk easier navigation
/// - Media counter di AppBar
/// - Smooth page transitions
/// - Error handling dan retry untuk video loading
///
/// Refactored into modular components:
/// - MediaViewerVideoPlayer: For video playback
/// - MediaViewerNavigation: For navigation arrows and indicators
class MediaViewerWidget extends StatefulWidget {
  /// Media URLs as string list (for backward compatibility)
  final List<String>? mediaUrls;

  /// Media entities (new canonical approach)
  final List<MediaEntity>? media;

  final int initialIndex;
  final String? title;

  const MediaViewerWidget({
    super.key,
    this.mediaUrls,
    this.media,
    this.initialIndex = 0,
    this.title,

    /// Asserts that at least one media source is provided
  }) : assert(
         mediaUrls != null || media != null,
         'Either mediaUrls or media must be provided',
       );

  /// Get the actual list of URLs to display
  List<String> get displayUrls => media?.urls ?? mediaUrls ?? const [];

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
          widget.title ?? '${_currentIndex + 1} / ${widget.displayUrls.length}',
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
            // if (widget.displayUrls.length > 1)
            //   Positioned(
            //     bottom: 16,
            //     left: 0,
            //     right: 0,
            //     child: MediaViewerIndicators(
            //       currentIndex: _currentIndex,
            //       totalItems: widget.displayUrls.length,
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
      itemCount: widget.displayUrls.length,
      onPageChanged: (index) {
        setState(() {
          _currentIndex = index;
        });
      },
      itemBuilder: (context, index) {
        return Center(child: _buildMediaItem(widget.displayUrls[index]));
      },
    );
  }

  /// Build media item (image atau video) untuk fullscreen viewer
  Widget _buildMediaItem(String mediaUrl) {
    final isVideo = MediaViewerUtils.isVideoUrl(mediaUrl);

    if (isVideo) {
      return MediaViewerVideoPlayer(videoUrl: mediaUrl);
    } else {
      // Image dengan blur background (Instagram style)
      return Stack(
        fit: StackFit.expand,
        children: [
          // Background blur layer menggunakan ImageFiltered
          ClipRect(
            child: ImageFiltered(
              imageFilter: ImageFilter.blur(
                sigmaX: 20,
                sigmaY: 20,
                tileMode: TileMode.decal,
              ),
              child: AppImage(
                imageUrl: mediaUrl,
                fit: BoxFit.cover,
                width: double.infinity,
                height: double.infinity,
              ),
            ),
          ),
          // Dark overlay
          Container(color: Colors.black.withValues(alpha: 0.1)),
          // Main image centered dengan InteractiveViewer untuk zoom
          Center(
            child: InteractiveViewer(
              minScale: 0.5,
              maxScale: 3.0,
              child: AppImage(
                imageUrl: mediaUrl,
                fit: BoxFit.contain,
                width: double.infinity,
                height: double.infinity,
              ),
            ),
          ),
        ],
      );
    }
  }

  // /// Static method untuk show media viewer
  // /// Unused - use MediaViewerUtils.showMediaViewer directly
  // static void show(
  //   BuildContext context, {
  //   required List<String> mediaUrls,
  //   int initialIndex = 0,
  //   String? title,
  // }) {
  //   MediaViewerUtils.showMediaViewer(
  //     context,
  //     mediaUrls: mediaUrls,
  //     initialIndex: initialIndex,
  //     title: title,
  //     mediaViewerBuilder: ({required mediaUrls, initialIndex = 0, title}) =>
  //         MediaViewerWidget(
  //           mediaUrls: mediaUrls,
  //           initialIndex: initialIndex,
  //           title: title,
  //         ),
  //   );
  // }
}
