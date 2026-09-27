import 'package:flutter/material.dart';
import 'package:labuda/core/media/media_upload_config.dart';
import 'package:labuda/core/media/media_upload_orchestrator.dart';

/// Shared commerce media grid — foto+video, dipakai for_sale, auction, komentar, chat.
///
/// Immediate upload: pick → S3 → List<String> URLs → parent setState.
/// Foto tampil Image.network, video tampil icon overlay (storageKey mp4).
class MediaGridUploader extends StatelessWidget {
  final List<String> mediaUrls;
  final void Function(String url) onMediaAdded;
  final void Function(int index) onMediaRemoved;
  final MediaUploadConfig config;
  final String emptyHint;

  const MediaGridUploader({
    super.key,
    required this.mediaUrls,
    required this.onMediaAdded,
    required this.onMediaRemoved,
    this.config = MediaUploadConfig.forCommerce,
    this.emptyHint = 'Tap untuk upload foto/video',
  });

  void _openPicker(BuildContext context) {
    MediaUploadOrchestrator.showPicker(
      context: context,
      config: config,
      currentCount: mediaUrls.length,
      onUploaded: (urls) async {
        for (final u in urls) onMediaAdded(u);
      },
    );
  }

  bool _isVideoUrl(String url) {
    final l = url.toLowerCase();
    return l.endsWith('.mp4') || l.endsWith('.mov') || l.endsWith('.webm') || l.contains('/videos/');
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    if (mediaUrls.isEmpty) {
      return GestureDetector(
        onTap: () => _openPicker(context),
        child: Container(
          height: 150,
          decoration: BoxDecoration(
            color: scheme.surfaceContainerHighest,
            borderRadius: BorderRadius.circular(12),
            border: Border.all(color: scheme.outlineVariant),
          ),
          child: Center(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(
                  Icons.add_photo_alternate,
                  size: 40,
                  color: scheme.onSurfaceVariant,
                ),
                SizedBox(height: 8),
                Text(
                  'Tap untuk upload foto/video',
                  style: TextStyle(color: scheme.onSurface),
                ),
                Text(
                  '(Minimal 1 media)',
                  style: TextStyle(
                    fontSize: 12,
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
        ),
      );
    }
    return GridView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      gridDelegate: const SliverGridDelegateWithFixedCrossAxisCount(
        crossAxisCount: 3,
        crossAxisSpacing: 8,
        mainAxisSpacing: 8,
      ),
      itemCount: mediaUrls.length + 1,
      itemBuilder: (context, index) {
        if (index < mediaUrls.length) {
          final url = mediaUrls[index];
          final isVideo = _isVideoUrl(url);
          return Stack(
            fit: StackFit.expand,
            children: [
              ClipRRect(
                borderRadius: BorderRadius.circular(8),
                child: isVideo
                    ? Container(
                        color: scheme.scrim,
                        child: Icon(
                          Icons.videocam,
                          color: scheme.onPrimary,
                          size: 32,
                        ),
                      )
                    : Image.network(url, fit: BoxFit.cover, errorBuilder: (_, __, ___) => const Icon(Icons.broken_image)),
              ),
              Positioned(
                top: 4,
                right: 4,
                child: GestureDetector(
                  onTap: () => onMediaRemoved(index),
                  child: Container(
                    padding: const EdgeInsets.all(4),
                    decoration: BoxDecoration(
                      color: scheme.scrim.withValues(alpha: 0.54),
                      shape: BoxShape.circle,
                    ),
                    child: Icon(
                      Icons.close,
                      size: 16,
                      color: scheme.onPrimary,
                    ),
                  ),
                ),
              ),
              if (isVideo)
                Center(
                  child: Icon(
                    Icons.play_circle_fill,
                    size: 28,
                    color: scheme.onPrimary.withValues(alpha: 0.7),
                  ),
                ),
            ],
          );
        }
        return GestureDetector(
          onTap: () => _openPicker(context),
          child: Container(
            decoration: BoxDecoration(
              color: scheme.surfaceContainerHighest,
              borderRadius: BorderRadius.circular(8),
              border: Border.all(color: scheme.outlineVariant),
            ),
            child: Icon(Icons.add, size: 32, color: scheme.onSurfaceVariant),
          ),
        );
      },
    );
  }
}

/// Compact variant for komentar/chat (max 5, row preview)
class CompactMediaStrip extends StatelessWidget {
  final List<String> mediaUrls;
  final void Function(String url) onMediaAdded;
  final void Function(int index) onMediaRemoved;
  final MediaUploadConfig config;

  const CompactMediaStrip({
    super.key,
    required this.mediaUrls,
    required this.onMediaAdded,
    required this.onMediaRemoved,
    this.config = MediaUploadConfig.forChat,
  });

  void _openPicker(BuildContext context) {
    MediaUploadOrchestrator.showPicker(
      context: context,
      config: config,
      currentCount: mediaUrls.length,
      onUploaded: (urls) async {
        for (final u in urls) onMediaAdded(u);
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (mediaUrls.isNotEmpty)
          SizedBox(
            height: 72,
            child: ListView.separated(
              scrollDirection: Axis.horizontal,
              itemCount: mediaUrls.length,
              separatorBuilder: (_, __) => const SizedBox(width: 8),
              itemBuilder: (ctx, i) {
                final url = mediaUrls[i];
                final isVideo = url.toLowerCase().contains('/videos/') || url.endsWith('.mp4');
                return Stack(
                  children: [
                    ClipRRect(
                      borderRadius: BorderRadius.circular(8),
                      child: isVideo
                          ? Container(
                              width: 72,
                              height: 72,
                              color: scheme.scrim,
                              child: Icon(
                                Icons.videocam,
                                color: scheme.onPrimary,
                              ),
                            )
                          : Image.network(url, width: 72, height: 72, fit: BoxFit.cover, errorBuilder: (_, __, ___) => const Icon(Icons.broken_image)),
                    ),
                    Positioned(
                      top: 2, right: 2,
                      child: GestureDetector(
                        onTap: () => onMediaRemoved(i),
                        child: Container(
                          padding: const EdgeInsets.all(2),
                          decoration: BoxDecoration(
                            color: scheme.scrim.withValues(alpha: 0.54),
                            shape: BoxShape.circle,
                          ),
                          child: Icon(
                            Icons.close,
                            size: 12,
                            color: scheme.onPrimary,
                          ),
                        ),
                      ),
                    ),
                  ],
                );
              },
            ),
          ),
        if (mediaUrls.isNotEmpty) const SizedBox(height: 8),
        OutlinedButton.icon(
          onPressed: mediaUrls.length >= config.maxTotal ? null : () => _openPicker(context),
          icon: const Icon(Icons.add_photo_alternate, size: 18),
          label: Text(mediaUrls.isEmpty ? 'Tambah foto/video' : 'Tambah lagi (${mediaUrls.length}/${config.maxTotal})'),
        ),
      ],
    );
  }
}
