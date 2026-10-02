import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/social/content/domain/entities/content.dart';
import 'package:labuda/shared/widgets/app_image.dart';

/// Canonical feed media mosaic — the single authority for rendering a feed
/// card's media list.
///
/// Layout by count (Facebook-style frame, koi-adapted):
/// - 1 → full 4:5 frame
/// - 2 → two squares side by side
/// - 3 → one big left + two stacked right
/// - 4+ → 2x2 grid, last tile overlays "+N"
///
/// Koi rules (locked):
/// - `contain`, never `cover` — no fin, tail, or pattern is ever cropped.
/// - [MediaEntity.type] is the render authority: video tiles show a play
///   badge and never reach the image decoder.
/// - Backend CloudFront thumbnail is rendered as-is via [AppImage].
/// - Taps bubble to the enclosing card (navigation owns routing).
class FeedMediaMosaic extends StatelessWidget {
  final List<MediaEntity> media;

  const FeedMediaMosaic({super.key, required this.media});

  @override
  Widget build(BuildContext context) {
    if (media.isEmpty) return const SizedBox.shrink();
    final scheme = Theme.of(context).colorScheme;

    Widget frame = switch (media.length) {
      1 => AspectRatio(
          aspectRatio: 4 / 5,
          child: _tile(context, 0),
        ),
      2 => LayoutBuilder(
          builder: (context, constraints) => SizedBox(
            height: constraints.maxWidth / 2,
            child: Row(
              children: [
                Expanded(child: _tile(context, 0)),
                const SizedBox(width: 2),
                Expanded(child: _tile(context, 1)),
              ],
            ),
          ),
        ),
      3 => LayoutBuilder(
          builder: (context, constraints) => SizedBox(
            height: constraints.maxWidth * 0.75,
            child: Row(
              children: [
                Expanded(flex: 2, child: _tile(context, 0)),
                const SizedBox(width: 2),
                Expanded(
                  child: Column(
                    children: [
                      Expanded(child: _tile(context, 1)),
                      const SizedBox(height: 2),
                      Expanded(child: _tile(context, 2)),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
      _ => LayoutBuilder(
          builder: (context, constraints) => SizedBox(
            height: constraints.maxWidth,
            child: Column(
              children: [
                Expanded(
                  child: Row(
                    children: [
                      Expanded(child: _tile(context, 0)),
                      const SizedBox(width: 2),
                      Expanded(child: _tile(context, 1)),
                    ],
                  ),
                ),
                const SizedBox(height: 2),
                Expanded(
                  child: Row(
                    children: [
                      Expanded(child: _tile(context, 2)),
                      const SizedBox(width: 2),
                      Expanded(child: _tile(context, 3, overflow: media.length - 4)),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ),
    };

    if (media.length <= 1) return frame;

    return Stack(
      children: [
        frame,
        Positioned(
          right: 8,
          top: 8,
          child: Container(
            padding: const EdgeInsets.symmetric(
              horizontal: AppMetrics.p8,
              vertical: AppMetrics.p4,
            ),
            decoration: BoxDecoration(
              color: scheme.scrim.withValues(alpha: 0.6),
              borderRadius: BorderRadius.circular(AppShape.pill),
            ),
            child: Text(
              '${media.length} media',
              style: TextStyle(
                color: scheme.onPrimary,
                fontSize: AppType.s12,
                fontWeight: FontWeight.w600,
              ),
            ),
          ),
        ),
      ],
    );
  }

  Widget _tile(BuildContext context, int index, {int overflow = 0}) {
    final scheme = Theme.of(context).colorScheme;
    final item = media[index];
    // Video tiles render the poster frame with a play badge; absent poster
    // falls back to the dark play tile (never the image decoder on mp4).
    final content = item.type == MediaType.video
        ? ColoredBox(
            color: scheme.surfaceContainerHighest,
            child: Stack(
              fit: StackFit.expand,
              children: [
                AppImage(
                  imageUrl: item.thumbnailUrl,
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
            ),
          )
        : AppImage(
            imageUrl: item.thumbnailUrl,
            blurhash: item.blurhash,
            fit: BoxFit.contain,
            backgroundColor: scheme.surfaceContainerHighest,
            errorWidget: ColoredBox(
              color: scheme.surfaceContainerHighest,
              child: Icon(
                Icons.broken_image_outlined,
                color: scheme.onSurfaceVariant,
              ),
            ),
          );

    if (overflow <= 0) return content;

    return Stack(
      fit: StackFit.expand,
      children: [
        content,
        Container(
          color: scheme.scrim.withValues(alpha: 0.55),
          alignment: Alignment.center,
          child: Text(
            '+$overflow',
            style: TextStyle(
              color: scheme.onPrimary,
              fontSize: AppType.s24,
              fontWeight: FontWeight.w800,
            ),
          ),
        ),
      ],
    );
  }
}
