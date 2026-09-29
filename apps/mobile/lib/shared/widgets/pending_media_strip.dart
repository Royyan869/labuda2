import 'package:flutter/material.dart';
import 'package:labuda/core/media/media_upload_orchestrator.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

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
      height: 72,
      child: ListView.separated(
        scrollDirection: Axis.horizontal,
        itemCount: items.length,
        separatorBuilder: (_, _) => const SizedBox(width: 8),
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
                        width: 72,
                        height: 72,
                        color: scheme.scrim,
                        child: Icon(Icons.videocam, color: scheme.onPrimary),
                      )
                    : Image.file(
                        file,
                        width: 72,
                        height: 72,
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
                      padding: const EdgeInsets.all(AppMetrics.p2),
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
              if (isVideo)
                Center(
                  child: Icon(
                    Icons.play_circle_fill,
                    size: 24,
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
                        fontSize: AppType.s10,
                        color: scheme.onPrimary,
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                  ),
              ] else if (failed)
                GestureDetector(
                  onTap: onRetry,
                  child: Container(
                    width: 72,
                    height: 72,
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
                          size: 20,
                          color: scheme.onError,
                        ),
                        Text(
                          'Coba lagi',
                          style: TextStyle(
                            fontSize: AppType.s10,
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
                    size: 16,
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
