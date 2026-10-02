import 'package:flutter/material.dart';
import 'package:labuda/core/media/media_upload_orchestrator.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// The tile extent, named ONCE.
///
/// The strip's height IS the tile's height, and this used to be five separate
/// `72`s (the strip, the video tile, the image tile, the failed tile and the
/// `width` beside each `height`), so nothing stopped a thumbnail from growing
/// while the strip that must contain it stayed behind.
///
/// Where this name belongs: media sizing is the foundation's NEXT phase —
/// `AppMetrics` owns the STEPS between things, and the census says so out loud
/// (`contentDimension` is not a migration target for the spacing ladder). The
/// final home is therefore a media-size role beside `AppIconSize`, not a private
/// const. Until that ladder exists, holding the number in ONE place is the
/// honest intermediate: re-inline a number here and the geometry census counts
/// it again, immediately.
const double _tileExtent = 72;

/// Pending media preview for deferred-upload composers — ONE widget for comment
/// and chat.
///
/// Every file carries its own lifecycle, so the batch tells the truth file by
/// file: which one is uploading (and how far), which already made it to S3, and
/// which failed. A failure on file 3 of 5 is visible as file 3's problem, and
/// the retry re-runs only the files that have no URL/asset yet.
///
/// The files are local until they upload, so removing one before it starts
/// costs nothing and abandoning the draft leaves no orphan object behind.
class PendingMediaStrip extends StatelessWidget {
  final List<MediaPendingItem> items;

  /// Removing a file is only offered while it has not started uploading: an
  /// in-flight PUT must not have the ground removed from under it.
  final void Function(int index) onRemove;

  /// Retries the failed files. Nothing is uploaded twice — the loop skips items
  /// that already hold a URL or asset id.
  final VoidCallback? onRetry;

  const PendingMediaStrip({
    super.key,
    required this.items,
    required this.onRemove,
    this.onRetry,
  });

  @override
  Widget build(BuildContext context) {
    if (items.isEmpty) return const SizedBox.shrink();
    final scheme = Theme.of(context).colorScheme;
    return SizedBox(
      height: _tileExtent,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        itemCount: items.length,
        separatorBuilder: (_, _) => const SizedBox(width: AppMetrics.p8),
        itemBuilder: (context, i) {
          final item = items[i];
          final file = item.file;
          final isVideo = item.isVideo;
          final uploading = item.phase == MediaUploadPhase.uploading;
          final failed = item.phase == MediaUploadPhase.failed;
          final done = item.phase == MediaUploadPhase.uploaded;
          return Stack(
            children: [
              ClipRRect(
                borderRadius: BorderRadius.circular(AppShape.r8),
                child: isVideo
                    ? Container(
                        width: _tileExtent,
                        height: _tileExtent,
                        color: scheme.scrim,
                        child: Icon(Icons.videocam, color: scheme.onPrimary),
                      )
                    : Image.file(
                        file,
                        width: _tileExtent,
                        height: _tileExtent,
                        fit: BoxFit.cover,
                        errorBuilder: (_, _, _) =>
                            Icon(Icons.broken_image, color: scheme.onSurfaceVariant),
                      ),
              ),
              if (!uploading)
                Positioned(
                  top: 2,
                  right: 2,
                  child: GestureDetector(
                    onTap: () => onRemove(i),
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
                    size: AppIconSize.header,
                    color: scheme.onPrimary.withValues(alpha: 0.7),
                  ),
                ),
              // Per-file state — the batch never hides which file is in flight.
              if (uploading) ...[
                Positioned(
                  left: 0,
                  right: 0,
                  bottom: 0,
                  child: ClipRRect(
                    borderRadius: BorderRadius.only(
                      bottomLeft: Radius.circular(AppShape.r8),
                      bottomRight: Radius.circular(AppShape.r8),
                    ),
                    child: LinearProgressIndicator(
                      // Indeterminate until the PUT reports its first byte count.
                      value: item.progress <= 0 ? null : item.progress,
                      minHeight: 4,
                    ),
                  ),
                ),
                if (item.progress > 0)
                  Positioned(
                    right: 4,
                    bottom: 6,
                    child: Text(
                      '${(item.progress * 100).round()}%',
                      style: TextStyle(
                        fontSize: AppType.s12,
                        color: scheme.onPrimary,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
              ] else if (failed)
                GestureDetector(
                  onTap: onRetry,
                  child: Container(
                    width: _tileExtent,
                    height: _tileExtent,
                    decoration: BoxDecoration(
                      color: scheme.error.withValues(alpha: 0.72),
                      borderRadius: BorderRadius.circular(AppShape.r8),
                    ),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Icon(
                          Icons.error_outline,
                          size: AppIconSize.action,
                          color: scheme.onError,
                        ),
                        Text(
                          'Coba lagi',
                          style: TextStyle(
                            fontSize: AppType.s12,
                            color: scheme.onError,
                          ),
                        ),
                      ],
                    ),
                  ),
                )
              else if (done)
                Positioned(
                  right: 4,
                  bottom: 4,
                  child: Icon(
                    Icons.check_circle,
                    size: AppIconSize.inlineGlyph,
                    color: scheme.primary,
                  ),
                ),
            ],
          );
        },
      ),
    );
  }
}
