import 'package:flutter/material.dart';
import 'package:video_player/video_player.dart';
import 'package:chewie/chewie.dart';
import 'package:visibility_detector/visibility_detector.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:labuda/shared/widgets/app_image.dart';

/// Video Player Widget untuk Media Carousel
///
/// Features:
/// - Chewie video player dengan default controls
/// - Play/pause, seek, mute controls
/// - Duration display
/// - Custom fullscreen button (opens MediaViewerWidget, NOT Chewie fullscreen)
/// - Error handling dan retry
/// - Static loading mat (shimmer is banned on media surfaces)
/// - Visibility-aware: only initializes when [isActive] (the carousel's
///   current page) and pauses when scrolled off-screen — adjacent/off-screen
///   pages render the poster mat without ever calling `initialize()`.
/// - Auto-dispose resources
class CarouselVideoPlayer extends StatefulWidget {
  final String videoUrl;
  final double width;
  final double height;
  final BoxFit fit;
  final VoidCallback? onFullscreenTap;
  final String? posterUrl;

  /// True when this player is the carousel's current page. Inactive pages
  /// stay on the poster mat — no controller, no buffering, no battery drain.
  final bool isActive;

  const CarouselVideoPlayer({
    super.key,
    required this.videoUrl,
    required this.width,
    required this.height,
    this.fit = BoxFit.cover,
    this.onFullscreenTap,
    this.posterUrl,
    this.isActive = true,
  });

  @override
  State<CarouselVideoPlayer> createState() => _CarouselVideoPlayerState();
}

class _CarouselVideoPlayerState extends State<CarouselVideoPlayer> {
  VideoPlayerController? _videoPlayerController;
  ChewieController? _chewieController;
  bool _isInitialized = false;
  bool _hasError = false;
  int _loadGeneration = 0;

  @override
  void initState() {
    super.initState();
    // Lazy init: only the active page pays for a controller. Inactive pages
    // render the poster mat until swiped to (see didUpdateWidget).
    if (widget.isActive) {
      _initializeVideo();
    }
  }

  @override
  void didUpdateWidget(CarouselVideoPlayer oldWidget) {
    super.didUpdateWidget(oldWidget);
    // Re-initialize if video URL changed
    if (oldWidget.videoUrl != widget.videoUrl) {
      _disposeControllers();
      if (widget.isActive) {
        _initializeVideo();
      }
      return;
    }
    // Page became current → initialize now (was poster mat before).
    if (widget.isActive && !oldWidget.isActive) {
      if (!_isInitialized && !_hasError) {
        _initializeVideo();
      }
    }
    // Page left the viewport → stop playback immediately; the controller is
    // kept so swiping back resumes without re-buffering.
    if (!widget.isActive && oldWidget.isActive) {
      _videoPlayerController?.pause();
    }
  }

  void _onVisibilityChanged(VisibilityInfo info) {
    if (info.visibleFraction == 0) {
      _videoPlayerController?.pause();
    }
  }

  Future<void> _initializeVideo() async {
    final generation = ++_loadGeneration;
    try {
      // Validate URL before initializing
      if (widget.videoUrl.isEmpty) {
        throw Exception('Empty video URL');
      }

      // Initialize video controller
      _videoPlayerController = VideoPlayerController.networkUrl(
        Uri.parse(widget.videoUrl),
      );
      await _videoPlayerController!.initialize();

      if (!mounted || generation != _loadGeneration) {
        _videoPlayerController?.dispose();
        return;
      }

      if (mounted) {
        setState(() {
          _chewieController = ChewieController(
            videoPlayerController: _videoPlayerController!,
            autoPlay: false,
            looping: false,
            aspectRatio: _videoPlayerController!.value.aspectRatio,
            // Responsive controls
            materialProgressColors: ChewieProgressColors(
              playedColor: Theme.of(context).colorScheme.primary,
              handleColor: Theme.of(context).colorScheme.primary,
              backgroundColor: Theme.of(context)
                  .colorScheme
                  .onSurfaceVariant
                  .withValues(alpha: 0.3),
              bufferedColor: Theme.of(context)
                  .colorScheme
                  .onSurfaceVariant
                  .withValues(alpha: 0.5),
            ),
            placeholder: _buildLoadingMat(),
            autoInitialize: true,
            errorBuilder: (context, errorMessage) {
              debugPrint('Chewie Error: $errorMessage');
              return _buildErrorState();
            },
            // Use custom controls with fullscreen button
            customControls: _CustomMaterialControls(
              onFullscreenTap: widget.onFullscreenTap,
            ),
            showControls: true,
            allowFullScreen:
                false, // Disable Chewie fullscreen, use MediaViewerWidget instead
            allowMuting: true,
            allowPlaybackSpeedChanging: false,
          );
          _isInitialized = true;
          _hasError = false;
        });
      }
    } catch (e) {
      // Log error for debugging
      debugPrint('CarouselVideoPlayer Error: $e');
      debugPrint('Video URL: ${widget.videoUrl}');
      if (mounted) {
        setState(() {
          _isInitialized = false;
          _hasError = true;
        });
      }
    }
  }

  void _disposeControllers() {
    _loadGeneration++;
    _chewieController?.dispose();
    _chewieController = null;
    _videoPlayerController?.dispose();
    _videoPlayerController = null;
    _isInitialized = false;
    _hasError = false;
  }

  @override
  void dispose() {
    _disposeControllers();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return VisibilityDetector(
      key: Key('carousel_video_${widget.videoUrl}'),
      onVisibilityChanged: _onVisibilityChanged,
      child: SizedBox(
        width: widget.width,
        height: widget.height,
        child: _isInitialized && _chewieController != null && !_hasError
            ? _buildVideoPlayer()
            : _hasError
            ? _buildErrorState()
            : _buildLoadingMat(),
      ),
    );
  }

  Widget _buildVideoPlayer() {
    return SizedBox(
      width: widget.width,
      height: widget.height,
      child: Chewie(controller: _chewieController!),
    );
  }

  Widget _buildLoadingMat() {
    final scheme = Theme.of(context).colorScheme;
    if (widget.posterUrl != null && widget.posterUrl!.isNotEmpty) {
      return Stack(
        fit: StackFit.expand,
        children: [
          AppImage(
            imageUrl: widget.posterUrl,
            fit: BoxFit.contain,
            backgroundColor: scheme.surfaceContainerHighest,
            errorWidget: const SizedBox.shrink(),
          ),
          Center(
            child: Icon(
              Icons.play_circle_fill,
              size: AppIconSize.display,
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      );
    }
    return Container(
      width: widget.width,
      height: widget.height,
      color: scheme.surfaceContainerHighest,
      child: Center(
        child: Icon(
          Icons.video_file_outlined,
          size: AppIconSize.display,
          color: scheme.onSurfaceVariant.withValues(alpha: 0.5),
        ),
      ),
    );
  }

  Widget _buildErrorState() {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      width: widget.width,
      height: widget.height,
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
            const SizedBox(height: 8),
            Text(
              'Video Error',
              style: TextStyle(color: scheme.onPrimary, fontSize: AppType.s14),
            ),
            const SizedBox(height: 16),
            ElevatedButton.icon(
              onPressed: () {
                setState(() {
                  _hasError = false;
                  _isInitialized = false;
                });
                _initializeVideo();
              },
              icon: const Icon(Icons.refresh, size: AppIconSize.inlineGlyph),
              label: const Text('Retry'),
              style: ElevatedButton.styleFrom(
                padding: const EdgeInsets.symmetric(
                  horizontal: AppMetrics.p16,
                  vertical: AppMetrics.p8,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Custom Material Controls with Fullscreen Button
/// Extends Chewie's MaterialControls with custom fullscreen functionality
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
              color: Theme.of(
                context,
              ).colorScheme.scrim.withValues(alpha: 0.5),
              shape: BoxShape.circle,
            ),
            child: IconButton(
              iconSize: 48,
              icon: Icon(
                _controller!.value.isPlaying ? Icons.pause : Icons.play_arrow,
                color: Theme.of(context).colorScheme.onPrimary,
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
            bufferedColor: Theme.of(context)
                .colorScheme
                .onSurfaceVariant
                .withValues(alpha: 0.5),
            backgroundColor: Theme.of(context)
                .colorScheme
                .onSurfaceVariant
                .withValues(alpha: 0.3),
          ),
          padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8, vertical: AppMetrics.p8),
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
          padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8, vertical: AppMetrics.p8),
          child: Row(
            children: [
              // Time display
              Text(
                '${_formatDuration(position)} / ${_formatDuration(duration)}',
                style: TextStyle(
                  color: Theme.of(context).colorScheme.onPrimary,
                  fontSize: AppType.s12,
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
                ),
                onPressed: _showPlaybackSpeedMenu,
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints(),
              ),

              const SizedBox(width: 8),

              // Fullscreen button
              if (widget.onFullscreenTap != null)
                IconButton(
                  icon: Icon(
                    Icons.fullscreen,
                    color: Theme.of(context).colorScheme.onPrimary,
                    size: AppIconSize.action,
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

    showModalBottomSheet(
      context: context,
      backgroundColor: Theme.of(context).colorScheme.scrim,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppShape.r16)),
      ),
      builder: (context) {
        return SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Padding(
                padding: const EdgeInsets.all(AppMetrics.p16),
                child: Text(
                  'Playback Speed',
                  style: TextStyle(
                    color: Theme.of(context).colorScheme.onPrimary,
                    fontSize: AppType.s16,
                    fontWeight: FontWeight.bold,
                  ),
                ),
              ),
              Divider(
                color: Theme.of(context).colorScheme.outlineVariant,
                height: 1,
              ),
              ...[0.5, 0.75, 1.0, 1.25, 1.5, 2.0].map((speed) {
                final isSelected = _controller!.value.playbackSpeed == speed;
                return ListTile(
                  leading: Icon(
                    isSelected ? Icons.check_circle : Icons.circle_outlined,
                    color: isSelected
                        ? Theme.of(context).colorScheme.primary
                        : Theme.of(context).colorScheme.onPrimary.withValues(
                            alpha: 0.7,
                          ),
                    size: AppIconSize.action,
                  ),
                  title: Text(
                    '${speed}x',
                    style: TextStyle(
                      color: isSelected
                          ? Theme.of(context).colorScheme.primary
                          : Theme.of(context).colorScheme.onPrimary,
                      fontWeight: isSelected
                          ? FontWeight.bold
                          : FontWeight.normal,
                    ),
                  ),
                  onTap: () {
                    setState(() {
                      _controller!.setPlaybackSpeed(speed);
                    });
                    Navigator.pop(context);
                  },
                );
              }),
              const SizedBox(height: 8),
            ],
          ),
        );
      },
    );
  }

  String _formatDuration(Duration duration) {
    String twoDigits(int n) => n.toString().padLeft(2, '0');
    final minutes = twoDigits(duration.inMinutes.remainder(60));
    final seconds = twoDigits(duration.inSeconds.remainder(60));
    return '$minutes:$seconds';
  }
}
