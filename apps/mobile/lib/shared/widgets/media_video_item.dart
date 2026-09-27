import 'dart:io';
import 'package:flutter/material.dart';
import 'package:video_player/video_player.dart';

class MediaVideoItem extends StatefulWidget {
  final File video;
  final VoidCallback? onRemove;
  final double height;
  final double width;

  const MediaVideoItem({
    super.key,
    required this.video,
    this.onRemove,
    required this.height,
    required this.width,
  });

  @override
  State<MediaVideoItem> createState() => _MediaVideoItemState();
}

class _MediaVideoItemState extends State<MediaVideoItem> {
  VideoPlayerController? _controller;
  bool _isInitialized = false;
  bool _hasError = false;

  @override
  void initState() {
    super.initState();
    _initializeVideo();
  }

  @override
  void dispose() {
    _controller?.dispose();
    super.dispose();
  }

  Future<void> _initializeVideo() async {
    try {
      _controller = VideoPlayerController.file(widget.video);
      await _controller!.initialize();
      if (mounted) {
        setState(() {
          _isInitialized = true;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _hasError = true;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;

    return Container(
      width: widget.width,
      height: widget.height,
      margin: const EdgeInsets.only(right: 8),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(12),
        color: scheme.surfaceContainerHighest,
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(12),
        child: Stack(
          children: [
            // Video thumbnail or placeholder
            SizedBox(
              width: double.infinity,
              height: double.infinity,
              child: _buildVideoContent(scheme),
            ),

            // Video indicator
            _buildVideoIndicator(scheme),

            // Remove button
            if (widget.onRemove != null) _buildRemoveButton(scheme),
          ],
        ),
      ),
    );
  }

  Widget _buildVideoContent(ColorScheme scheme) {
    if (_hasError) {
      return Container(
        decoration: BoxDecoration(
          color: scheme.surfaceContainerHighest,
          borderRadius: BorderRadius.circular(12),
        ),
        child: Icon(
          Icons.error_outline,
          color: scheme.onSurfaceVariant,
          size: 32,
        ),
      );
    }

    if (!_isInitialized || _controller == null) {
      return Container(
        decoration: BoxDecoration(
          color: scheme.surfaceContainerHighest,
          borderRadius: BorderRadius.circular(12),
        ),
        child: const Center(child: CircularProgressIndicator(strokeWidth: 2)),
      );
    }

    return AspectRatio(
      aspectRatio: _controller!.value.aspectRatio,
      child: VideoPlayer(_controller!),
    );
  }

  Widget _buildVideoIndicator(ColorScheme scheme) {
    return Positioned(
      bottom: 8,
      left: 8,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 3),
        decoration: BoxDecoration(
          color: scheme.scrim.withValues(alpha: 0.7),
          borderRadius: BorderRadius.circular(4),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.play_arrow, color: scheme.onPrimary, size: 12),
            const SizedBox(width: 2),
            Text(
              _isInitialized && _controller != null
                  ? _formatDuration(_controller!.value.duration)
                  : '--:--',
              style: TextStyle(
                color: scheme.onPrimary,
                fontSize: 10,
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildRemoveButton(ColorScheme scheme) {
    return Positioned(
      top: 4,
      right: 4,
      child: GestureDetector(
        onTap: widget.onRemove,
        child: Container(
          width: 24,
          height: 24,
          decoration: BoxDecoration(
            color: scheme.error.withValues(alpha: 0.9),
            shape: BoxShape.circle,
          ),
          child: Icon(Icons.close, color: scheme.onPrimary, size: 16),
        ),
      ),
    );
  }

  String _formatDuration(Duration duration) {
    String twoDigits(int n) => n.toString().padLeft(2, '0');
    final minutes = twoDigits(duration.inMinutes.remainder(60));
    final seconds = twoDigits(duration.inSeconds.remainder(60));
    return '$minutes:$seconds';
  }
}
