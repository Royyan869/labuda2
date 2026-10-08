import 'dart:ui';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:video_player/video_player.dart';
import 'package:chewie/chewie.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:labuda/shared/widgets/app_bottom_sheet_actions.dart';
import 'package:labuda/shared/widgets/app_image.dart';

/// Fullscreen Video Player Widget untuk Media Viewer
///
/// Features:
/// - Chewie video player dengan rich controls
/// - Fullscreen support dengan orientation lock
/// - Progress bar, seek, duration display
/// - Error handling dengan retry functionality
/// - Static loading mat (shimmer is banned on media surfaces)
/// - Auto-dispose resources
class MediaViewerVideoPlayer extends StatefulWidget {
  final String videoUrl;
  final String? posterUrl;

  /// True when this page is the viewer's current page. Sibling pages render
  /// the poster backdrop without initializing a controller.
  final bool isActive;

  const MediaViewerVideoPlayer({
    super.key,
    required this.videoUrl,
    this.posterUrl,
    this.isActive = true,
  });

  @override
  State<MediaViewerVideoPlayer> createState() => _MediaViewerVideoPlayerState();
}

class _MediaViewerVideoPlayerState extends State<MediaViewerVideoPlayer> {
  VideoPlayerController? _videoPlayerController;
  ChewieController? _chewieController;
  bool _isInitialized = false;
  bool _hasError = false;

  @override
  void initState() {
    super.initState();
    // Lazy init: sibling pages stay on the poster backdrop (zero video
    // bytes) until swiped to — see didUpdateWidget.
    if (widget.isActive) {
      _initializeVideo();
    }
    // Force landscape for fullscreen video
    SystemChrome.setPreferredOrientations([
      DeviceOrientation.landscapeLeft,
      DeviceOrientation.landscapeRight,
      DeviceOrientation.portraitUp,
    ]);
  }

  @override
  void didUpdateWidget(MediaViewerVideoPlayer oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.videoUrl != widget.videoUrl) {
      _chewieController?.dispose();
      _chewieController = null;
      _videoPlayerController?.dispose();
      _videoPlayerController = null;
      _isInitialized = false;
      _hasError = false;
      if (widget.isActive) {
        _initializeVideo();
      }
      return;
    }
    if (widget.isActive && !oldWidget.isActive) {
      if (!_isInitialized && !_hasError) {
        _initializeVideo();
      }
    }
    if (!widget.isActive && oldWidget.isActive) {
      _videoPlayerController?.pause();
    }
  }

  Future<void> _initializeVideo() async {
    try {
      _videoPlayerController = VideoPlayerController.networkUrl(
        Uri.parse(widget.videoUrl),
      );

      await _videoPlayerController!.initialize();

      if (mounted) {
        setState(() {
          _chewieController = ChewieController(
            videoPlayerController: _videoPlayerController!,
            autoPlay: true,
            looping: true,
            aspectRatio: _videoPlayerController!.value.aspectRatio,
            // Fullscreen optimized controls
            materialProgressColors: ChewieProgressColors(
              playedColor: Theme.of(context).colorScheme.primary,
              handleColor: Theme.of(context).colorScheme.primary,
              backgroundColor: Theme.of(
                context,
              ).colorScheme.onSurfaceVariant.withValues(alpha: 0.3),
              bufferedColor: Theme.of(
                context,
              ).colorScheme.onSurfaceVariant.withValues(alpha: 0.5),
            ),
            placeholder: _buildLoadingMat(),
            autoInitialize: true,
            errorBuilder: (context, errorMessage) {
              return _buildErrorState();
            },
            // Use custom controls (same design as carousel, but without fullscreen button)
            customControls: const _CustomMaterialControls(
              onFullscreenTap: null, // No fullscreen button in fullscreen view
            ),
            showControls: true,
            allowFullScreen:
                false, // Disable Chewie fullscreen - already in fullscreen view
            allowMuting: true,
            allowPlaybackSpeedChanging: false,
            // Hide system UI for immersive fullscreen
            systemOverlaysAfterFullScreen: [],
            deviceOrientationsAfterFullScreen: [
              DeviceOrientation.portraitUp,
              DeviceOrientation.landscapeLeft,
              DeviceOrientation.landscapeRight,
            ],
          );
          _isInitialized = true;
          _hasError = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _isInitialized = false;
          _hasError = true;
        });
      }
    }
  }

  @override
  void dispose() {
    // Restore orientation on dispose
    SystemChrome.setPreferredOrientations([
      DeviceOrientation.portraitUp,
      DeviceOrientation.landscapeLeft,
      DeviceOrientation.landscapeRight,
    ]);
    _chewieController?.dispose();
    _videoPlayerController?.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      width: double.infinity,
      height: double.infinity,
      color: Theme.of(context).colorScheme.scrim,
      child: Stack(
        fit: StackFit.expand,
        children: [
          // Poster backdrop (cached photo, zero video bytes) behind player.
          if (widget.posterUrl != null && widget.posterUrl!.isNotEmpty)
            _buildPosterBackdrop(),
          if (_isInitialized && _chewieController != null && !_hasError)
            SizedBox(
              width: double.infinity,
              height: double.infinity,
              child: Chewie(controller: _chewieController!),
            )
          else if (_hasError)
            _buildErrorState()
          else if (widget.posterUrl == null || widget.posterUrl!.isEmpty)
            _buildLoadingMat(),
        ],
      ),
    );
  }

  Widget _buildPosterBackdrop() {
    return Stack(
      fit: StackFit.expand,
      children: [
        ClipRect(
          child: ImageFiltered(
            imageFilter: ImageFilter.blur(
              sigmaX: 20,
              sigmaY: 20,
              tileMode: TileMode.decal,
            ),
            child: AppImage(
              imageUrl: widget.posterUrl,
              fit: BoxFit.cover,
              errorWidget: const SizedBox.shrink(),
            ),
          ),
        ),
        Container(
          color: Theme.of(context).colorScheme.scrim.withValues(alpha: 0.35),
        ),
      ],
    );
  }

  Widget _buildLoadingMat() {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      width: double.infinity,
      height: double.infinity,
      color: scheme.scrim,
      child: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.video_file_outlined,
              size: AppIconSize.display,
              color: scheme.onPrimary.withValues(alpha: 0.5),
            ),
            const SizedBox(height: 16),
            SizedBox(
              width: 200,
              child: LinearProgressIndicator(
                backgroundColor: scheme.onPrimary.withValues(alpha: 0.24),
                valueColor: AlwaysStoppedAnimation<Color>(scheme.primary),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildErrorState() {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      width: double.infinity,
      height: double.infinity,
      color: scheme.scrim,
      child: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.error_outline,
              size: AppIconSize.display,
              color: scheme.onPrimary.withValues(alpha: 0.7),
            ),
            const SizedBox(height: 16),
            Text(
              'Video Failed to Load',
              style: context.typeRoles.titleProminent.copyWith(
                color: scheme.onPrimary,
                fontWeight: FontWeight.w500,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              'Unable to play this video',
              style: context.typeRoles.bodyDense.copyWith(
                color: scheme.onPrimary.withValues(alpha: 0.7),
              ),
            ),
            const SizedBox(height: 24),
            ElevatedButton.icon(
              onPressed: () {
                setState(() {
                  _hasError = false;
                  _isInitialized = false;
                });
                _initializeVideo();
              },
              icon: const Icon(Icons.refresh, size: AppIconSize.action),
              label: const Text('Retry'),
            ),
          ],
        ),
      ),
    );
  }
}

/// Custom Material Controls with Fullscreen Button
/// Same design as carousel controls for consistency
class _CustomMaterialControls extends StatefulWidget {
  final VoidCallback? onFullscreenTap;

  const _CustomMaterialControls({this.onFullscreenTap});

  @override
  State<_CustomMaterialControls> createState() =>
      _CustomMaterialControlsState();
}

class _CustomMaterialControlsState extends State<_CustomMaterialControls> {
  VideoPlayerController? _controller;
  final bool _hideControls = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final chewieController = ChewieController.of(context);
    _controller = chewieController.videoPlayerController;
    // _chewieController = chewieController; // Unused
  }

  @override
  Widget build(BuildContext context) {
    return Stack(
      children: [
        // Base controls layer - tap to toggle play/pause
        GestureDetector(
          onTap: _togglePlayPause,
          child: Container(color: Colors.transparent),
        ),

        // Controls overlay
        if (!_hideControls)
          Container(
            decoration: BoxDecoration(
              gradient: LinearGradient(
                begin: Alignment.topCenter,
                end: Alignment.bottomCenter,
                colors: [
                  Theme.of(context).colorScheme.scrim.withValues(alpha: 0.0),
                  Theme.of(context).colorScheme.scrim.withValues(alpha: 0.7),
                ],
                stops: const [0.5, 1.0],
              ),
            ),
            child: Column(
              mainAxisAlignment: MainAxisAlignment.end,
              children: [
                // Progress bar
                _buildProgressBar(),

                // Bottom controls row: duration, volume, fullscreen
                _buildBottomControls(),
              ],
            ),
          ),

        // Center play/pause button
        if (!_hideControls) Center(child: _buildCenterPlayButton()),
      ],
    );
  }

  void _togglePlayPause() {
    if (_controller == null) return;

    setState(() {
      if (_controller!.value.isPlaying) {
        _controller!.pause();
      } else {
        _controller!.play();
      }
    });
  }

  Widget _buildCenterPlayButton() {
    if (_controller == null) return const SizedBox.shrink();

    return AnimatedBuilder(
      animation: _controller!,
      builder: (context, child) {
        return AnimatedOpacity(
          opacity: _controller!.value.isPlaying ? 0.0 : 1.0,
          duration: AppMotion.settled,
          child: Container(
            decoration: BoxDecoration(
              color: Theme.of(context).colorScheme.scrim.withValues(alpha: 0.5),
              shape: BoxShape.circle,
            ),
            child: IconButton(
              iconSize: AppIconSize.display,
              icon: Icon(
                _controller!.value.isPlaying ? Icons.pause : Icons.play_arrow,
                color: Theme.of(context).colorScheme.onPrimary,
                semanticLabel: _controller!.value.isPlaying ? 'Jeda' : 'Putar',
              ),
              onPressed: _togglePlayPause,
            ),
          ),
        );
      },
    );
  }

  Widget _buildProgressBar() {
    if (_controller == null) return const SizedBox.shrink();

    return AnimatedBuilder(
      animation: _controller!,
      builder: (context, child) {
        // final duration = _controller!.value.duration; // Unused
        // final position = _controller!.value.position; // Unused

        return VideoProgressIndicator(
          _controller!,
          allowScrubbing: true,
          colors: VideoProgressColors(
            playedColor: Theme.of(context).colorScheme.primary,
            bufferedColor: Theme.of(
              context,
            ).colorScheme.onSurfaceVariant.withValues(alpha: 0.5),
            backgroundColor: Theme.of(
              context,
            ).colorScheme.onSurfaceVariant.withValues(alpha: 0.3),
          ),
          padding: const EdgeInsets.symmetric(
            horizontal: AppMetrics.p8,
            vertical: AppMetrics.p8,
          ),
        );
      },
    );
  }

  Widget _buildBottomControls() {
    if (_controller == null) return const SizedBox.shrink();

    return AnimatedBuilder(
      animation: _controller!,
      builder: (context, child) {
        final duration = _controller!.value.duration;
        final position = _controller!.value.position;

        return Padding(
          padding: const EdgeInsets.symmetric(
            horizontal: AppMetrics.p8,
            vertical: AppMetrics.p8,
          ),
          child: Row(
            children: [
              // Time display
              Text(
                '${_formatDuration(position)} / ${_formatDuration(duration)}',
                style: context.typeRoles.labelMicro.copyWith(
                  color: Theme.of(context).colorScheme.onPrimary,
                ),
              ),
              const Spacer(),

              // Volume/Mute button
              IconButton(
                icon: Icon(
                  _controller!.value.volume > 0
                      ? Icons.volume_up
                      : Icons.volume_off,
                  color: Theme.of(context).colorScheme.onPrimary,
                  size: AppIconSize.action,
                  semanticLabel: _controller!.value.volume > 0
                      ? 'Bisukan'
                      : 'Nyalakan suara',
                ),
                onPressed: () {
                  setState(() {
                    _controller!.setVolume(
                      _controller!.value.volume > 0 ? 0.0 : 1.0,
                    );
                  });
                },
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints(),
              ),

              const SizedBox(width: 8),

              // Playback speed button
              IconButton(
                icon: Icon(
                  Icons.speed,
                  color: Theme.of(context).colorScheme.onPrimary,
                  size: AppIconSize.action,
                 semanticLabel: 'Kecepatan putar',
                 ),
                onPressed: _showPlaybackSpeedMenu,
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints(),
              ),

              const SizedBox(width: 8),

              // Fullscreen button (only show if callback provided)
              if (widget.onFullscreenTap != null)
                IconButton(
                  icon: Icon(
                    Icons.fullscreen,
                    color: Theme.of(context).colorScheme.onPrimary,
                    size: AppIconSize.action,
                   semanticLabel: 'Layar penuh',
                   ),
                  onPressed: widget.onFullscreenTap,
                  padding: EdgeInsets.zero,
                  constraints: const BoxConstraints(),
                ),
            ],
          ),
        );
      },
    );
  }

  void _showPlaybackSpeedMenu() {
    if (_controller == null) return;

    // Canonical action sheet — no bespoke sheet, no local surface/shape.
    final current = _controller!.value.playbackSpeed;
    AppBottomSheetActions.showActions<double>(
      context: context,
      title: 'Playback Speed',
      showCancel: false,
      actions: const [0.5, 0.75, 1.0, 1.25, 1.5, 2.0]
          .map(
            (speed) => BottomSheetAction<double>(
              title: '${speed}x',
              selected: current == speed,
              onPressed: () {
                setState(() => _controller!.setPlaybackSpeed(speed));
                Navigator.pop(context);
              },
            ),
          )
          .toList(),
    );
  }

  String _formatDuration(Duration duration) {
    String twoDigits(int n) => n.toString().padLeft(2, '0');
    final minutes = twoDigits(duration.inMinutes.remainder(60));
    final seconds = twoDigits(duration.inSeconds.remainder(60));
    return '$minutes:$seconds';
  }
}
