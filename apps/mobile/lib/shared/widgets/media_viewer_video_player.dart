import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:video_player/video_player.dart';
import 'package:chewie/chewie.dart';
import 'package:shimmer/shimmer.dart';
import 'package:labuda/core/src/theme/app_colors.dart';

/// Fullscreen Video Player Widget untuk Media Viewer
///
/// Features:
/// - Chewie video player dengan rich controls
/// - Fullscreen support dengan orientation lock
/// - Progress bar, seek, duration display
/// - Error handling dengan retry functionality
/// - Shimmer loading state
/// - Auto-dispose resources
class MediaViewerVideoPlayer extends StatefulWidget {
  final String videoUrl;

  const MediaViewerVideoPlayer({super.key, required this.videoUrl});

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
    _initializeVideo();
    // Force landscape for fullscreen video
    SystemChrome.setPreferredOrientations([
      DeviceOrientation.landscapeLeft,
      DeviceOrientation.landscapeRight,
      DeviceOrientation.portraitUp,
    ]);
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
              playedColor: AppColors.primaryRed,
              handleColor: AppColors.primaryRed,
              backgroundColor: AppColors.neutralGray400.withValues(alpha: 0.3),
              bufferedColor: AppColors.neutralGray400.withValues(alpha: 0.5),
            ),
            placeholder: _buildShimmerPlaceholder(),
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
      color: AppColors.dark,
      child: _isInitialized && _chewieController != null && !_hasError
          ? SizedBox(
              width: double.infinity,
              height: double.infinity,
              child: Chewie(controller: _chewieController!),
            )
          : _hasError
          ? _buildErrorState()
          : _buildShimmerPlaceholder(),
    );
  }

  Widget _buildShimmerPlaceholder() {
    return Shimmer.fromColors(
      baseColor: AppColors.darkGray600,
      highlightColor: AppColors.darkGray500,
      child: Container(
        width: double.infinity,
        height: double.infinity,
        color: AppColors.darkGray600,
        child: const Center(
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(Icons.video_file_outlined, size: 80, color: Colors.white38),
              SizedBox(height: 16),
              Text(
                'Loading Video...',
                style: TextStyle(
                  color: AppColors.light,
                  fontSize: 18,
                  fontWeight: FontWeight.w500,
                ),
              ),
              SizedBox(height: 12),
              SizedBox(
                width: 200,
                child: LinearProgressIndicator(
                  backgroundColor: Colors.white24,
                  valueColor: AlwaysStoppedAnimation<Color>(
                    AppColors.primaryRed,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildErrorState() {
    return Container(
      width: double.infinity,
      height: double.infinity,
      color: AppColors.dark,
      child: Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Icon(Icons.error_outline, size: 80, color: Colors.white70),
            const SizedBox(height: 16),
            const Text(
              'Video Failed to Load',
              style: TextStyle(
                color: AppColors.light,
                fontSize: 18,
                fontWeight: FontWeight.w500,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              'Unable to play this video',
              style: TextStyle(
                color: AppColors.light.withValues(alpha: 0.7),
                fontSize: 14,
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
              icon: const Icon(Icons.refresh, size: 20),
              label: const Text('Retry'),
              style: ElevatedButton.styleFrom(
                backgroundColor: AppColors.primaryRed,
                foregroundColor: AppColors.light,
                padding: const EdgeInsets.symmetric(
                  horizontal: 24,
                  vertical: 12,
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
                  Colors.black.withValues(alpha: 0.0),
                  Colors.black.withValues(alpha: 0.7),
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
          duration: const Duration(milliseconds: 300),
          child: Container(
            decoration: BoxDecoration(
              color: Colors.black.withValues(alpha: 0.5),
              shape: BoxShape.circle,
            ),
            child: IconButton(
              iconSize: 48,
              icon: Icon(
                _controller!.value.isPlaying ? Icons.pause : Icons.play_arrow,
                color: Colors.white,
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
            playedColor: AppColors.primaryRed,
            bufferedColor: AppColors.neutralGray400.withValues(alpha: 0.5),
            backgroundColor: AppColors.neutralGray400.withValues(alpha: 0.3),
          ),
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 8),
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
          padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 8),
          child: Row(
            children: [
              // Time display
              Text(
                '${_formatDuration(position)} / ${_formatDuration(duration)}',
                style: const TextStyle(color: Colors.white, fontSize: 12),
              ),
              const Spacer(),

              // Volume/Mute button
              IconButton(
                icon: Icon(
                  _controller!.value.volume > 0
                      ? Icons.volume_up
                      : Icons.volume_off,
                  color: Colors.white,
                  size: 20,
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
                icon: const Icon(Icons.speed, color: Colors.white, size: 20),
                onPressed: _showPlaybackSpeedMenu,
                padding: EdgeInsets.zero,
                constraints: const BoxConstraints(),
              ),

              const SizedBox(width: 8),

              // Fullscreen button (only show if callback provided)
              if (widget.onFullscreenTap != null)
                IconButton(
                  icon: const Icon(
                    Icons.fullscreen,
                    color: Colors.white,
                    size: 20,
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
      backgroundColor: AppColors.darkGray700,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (context) {
        return SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Padding(
                padding: const EdgeInsets.all(16),
                child: Text(
                  'Playback Speed',
                  style: const TextStyle(
                    color: Colors.white,
                    fontSize: 16,
                    fontWeight: FontWeight.bold,
                  ),
                ),
              ),
              const Divider(color: AppColors.neutralGray600, height: 1),
              ...[0.5, 0.75, 1.0, 1.25, 1.5, 2.0].map((speed) {
                final isSelected = _controller!.value.playbackSpeed == speed;
                return ListTile(
                  leading: Icon(
                    isSelected ? Icons.check_circle : Icons.circle_outlined,
                    color: isSelected ? AppColors.primaryRed : Colors.white70,
                    size: 20,
                  ),
                  title: Text(
                    '${speed}x',
                    style: TextStyle(
                      color: isSelected ? AppColors.primaryRed : Colors.white,
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
