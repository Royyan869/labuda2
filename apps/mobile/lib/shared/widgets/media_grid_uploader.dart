import 'package:flutter/material.dart';
import 'package:labuda/core/media/media_upload_config.dart';
import 'package:labuda/core/media/media_upload_orchestrator.dart';
import 'package:labuda/shared/widgets/app_image.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Shared commerce media grid — foto+video, dipakai for_sale, auction, komentar, chat.
///
/// Immediate upload: pick → S3 → `List<String>` URLs → parent setState.
/// Foto tampil AppImage, video tampil icon overlay (storageKey mp4).
class MediaGridUploader extends StatelessWidget {
  final List<String> mediaUrls;
  final void Function(String url) onMediaAdded;
  final void Function(int index) onMediaRemoved;
  final void Function(int oldIndex, int newIndex)? onMediaReordered;
  final MediaUploadConfig config;
  final String emptyHint;

  const MediaGridUploader({
    super.key,
    required this.mediaUrls,
    required this.onMediaAdded,
    required this.onMediaRemoved,
    this.onMediaReordered,
    this.config = MediaUploadConfig.forCommerce,
    this.emptyHint = 'Tap untuk upload foto/video',
  });

  void _openPicker(BuildContext context) {
    MediaUploadOrchestrator.showPicker(
      context: context,
      config: config,
      current: MediaUploadOrchestrator.countsOfUrls(mediaUrls),
      onUploaded: (urls) async {
        for (final u in urls) {
          onMediaAdded(u);
        }
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    if (mediaUrls.isEmpty) {
      return GestureDetector(
        onTap: () => _openPicker(context),
        child: Container(
          padding: const EdgeInsets.symmetric(vertical: AppMetrics.p32),
          decoration: BoxDecoration(
            color: scheme.surfaceContainerHighest,
            borderRadius: BorderRadius.circular(AppShape.r12),
            border: Border.all(color: scheme.outlineVariant),
          ),
          child: Center(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(
                  Icons.add_photo_alternate,
                  size: AppIconSize.display,
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
                    fontSize: AppType.s12,
                    color: scheme.onSurfaceVariant,
                  ),
                ),
              ],
            ),
          ),
        ),
      );
    }
    return ReorderableListView.builder(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      buildDefaultDragHandles: true,
      itemCount: mediaUrls.length,
      onReorder: (oldIndex, newIndex) {
        if (onMediaReordered == null) return;
        var target = newIndex;
        if (oldIndex < target) target--;
        onMediaReordered!(oldIndex, target);
      },
      itemBuilder: (context, index) {
        if (index < mediaUrls.length) {
          final url = mediaUrls[index];
          final isVideo = MediaUploadOrchestrator.isVideoUrl(url);
           return SizedBox(
             key: ValueKey('media-$url-$index'),
             height: AppContentSize.thumbnail,
             child: Stack(
             fit: StackFit.expand,
             children: [
              ClipRRect(
                borderRadius: BorderRadius.circular(AppShape.r8),
                child: isVideo
                    ? Container(
                        color: scheme.scrim,
                        child: Icon(
                          Icons.videocam,
                          color: scheme.onPrimary,
                          size: AppIconSize.emphasis,
                        ),
                      )
                    : AppImage(imageUrl: url, fit: BoxFit.cover, errorWidget: const Icon(Icons.broken_image)),
              ),
              Positioned(
                top: 4,
                right: 4,
                child: GestureDetector(
                  onTap: () => onMediaRemoved(index),
                  child: Container(
                    padding: const EdgeInsets.all(AppMetrics.p4),
                    decoration: BoxDecoration(
                      color: scheme.scrim.withValues(alpha: 0.54),
                      shape: BoxShape.circle,
                    ),
                    child: Icon(
                      Icons.close,
                      size: AppIconSize.inlineGlyph,
                      color: scheme.onPrimary,
                    ),
                  ),
                ),
              ),
              if (isVideo)
                Center(
                  child: Icon(
                    Icons.play_circle_fill,
                    size: AppIconSize.emphasis,
                    color: scheme.onPrimary.withValues(alpha: 0.7),
                  ),
                ),
             ],
           ),
           );
         }
          return const SizedBox.shrink(key: ValueKey('media-add-disabled'));
        },
      );
  }
}
