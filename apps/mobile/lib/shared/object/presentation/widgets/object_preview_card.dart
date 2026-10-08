/// Reference Attachment Card — transport-snapshot display shell
///
/// Renders the identity a shared reference already carries on the message wire
/// (`ShareReference` → `ObjectPreview` snapshot: resource type, title,
/// thumbnail). It is the display fallback for communication rows whose
/// server-resolved `ResourceProjection` is absent — rows persisted before the
/// canonical projection authority existed, or any row the server did not
/// project.
///
/// HARD RULES (one truth, one authority):
/// - NO live fetch. This shell never resolves the resource through a detail
///   endpoint; one card must never become an N+1 per message page.
/// - NO derived state. Availability, lifecycle and money are Commerce truth
///   and come only from the canonical `ResourceProjection` envelope, rendered
///   by the per-surface projection cards. This shell therefore renders no
///   availability badge and no money.
/// - Navigation is identity-only: the target surface answers with truth (and
///   stays fail-closed when the resource is gone).
library;

import 'package:flutter/material.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/shared/widgets/app_image.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Displays the cached transport preview of a shared reference.
///
/// Used only when no canonical projection envelope is available for the row.
class ObjectPreviewCard extends StatelessWidget {
  /// ShareReference containing the transport snapshot
  final ShareReference reference;

  /// Callback when card is tapped
  final VoidCallback? onTap;

  /// Whether to show the type badge
  final bool showTypeBadge;

  const ObjectPreviewCard({
    super.key,
    required this.reference,
    this.onTap,
    this.showTypeBadge = true,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final imageUrl = reference.preview.imageUrl;

    return Card(
      margin: EdgeInsets.zero,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppShape.r8),
        child: Padding(
          padding: const EdgeInsets.all(AppMetrics.p12),
          child: Row(
            children: [
              if (imageUrl != null) _buildThumbnail(context, imageUrl),
              if (imageUrl != null) const SizedBox(width: AppMetrics.p12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (showTypeBadge)
                      Text(
                        reference.targetType.displayName,
                        // Role, not size: a caption/badge is `labelSmall` on the
                        // documented mapping (AppTheme). The w600 and the brand
                        // colour are the call site's own decisions, so they
                        // stay here instead of being baked into the role.
                        style: Theme.of(context).textTheme.labelSmall?.copyWith(
                          color: scheme.primary,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    const SizedBox(height: AppMetrics.p4),
                    Text(
                      reference.preview.title,
                      // Ambient role + call-site weight: identical pixels to
                      // the old raw literal, without restating a TextStyle.
                      style: DefaultTextStyle.of(
                        context,
                      ).style.copyWith(fontWeight: FontWeight.w600),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ],
                ),
              ),
              Icon(Icons.chevron_right, color: scheme.onSurfaceVariant),
            ],
          ),
        ),
      ),
    );
  }

  /// Thumbnail for the reference preview. Every target type renders the cached
  /// preview URL through [AppImage] as-is — one widget, no per-type decoder.
  Widget _buildThumbnail(BuildContext context, String imageUrl) {
    const borderRadius = BorderRadius.all(Radius.circular(AppShape.r8));

    return ClipRRect(
      borderRadius: borderRadius,
      child: SizedBox(
        width: 60,
        height: 60,
        child: AppImage(
          imageUrl: imageUrl,
          fit: BoxFit.cover,
          backgroundColor: Theme.of(
            context,
          ).colorScheme.surfaceContainerHighest,
          errorWidget: _buildThumbnailFallback(context),
        ),
      ),
    );
  }

  Widget _buildThumbnailFallback(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      width: 60,
      height: 60,
      color: scheme.surfaceContainerHighest,
      child: Icon(Icons.image_not_supported, color: scheme.onSurfaceVariant),
    );
  }
}
