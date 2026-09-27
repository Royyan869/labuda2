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
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/attachment/entities/share_reference.dart';
import 'package:labuda/shared/widgets/stable_network_image.dart';

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
    final imageUrl = reference.preview.imageUrl;

    return Card(
      margin: EdgeInsets.zero,
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(8),
        child: Padding(
          padding: const EdgeInsets.all(12),
          child: Row(
            children: [
              if (imageUrl != null) _buildThumbnail(imageUrl),
              if (imageUrl != null) const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    if (showTypeBadge)
                      Text(
                        reference.targetType.displayName,
                        style: TextStyle(
                          fontSize: 12,
                          color: AppColors.primaryRed,
                          fontWeight: FontWeight.w600,
                        ),
                      ),
                    const SizedBox(height: 4),
                    Text(
                      reference.preview.title,
                      style: const TextStyle(fontWeight: FontWeight.w600),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                  ],
                ),
              ),
              Icon(Icons.chevron_right, color: AppColors.neutralGray400),
            ],
          ),
        ),
      ),
    );
  }

  /// Thumbnail for the reference preview.
  ///
  /// [ShareTargetType.content] references carry a persisted Content media
  /// reference (`content_media.media_url`) in their cached preview, so they are
  /// projected through the shared network-media path — [StableNetworkImage] /
  /// `resolveNetworkImageUrl` — exactly like every other converged Content media
  /// surface. A persisted storage reference is never handed straight to the
  /// image decoder.
  ///
  /// Commerce (for_sale / auction) and profile references are unchanged.
  Widget _buildThumbnail(String imageUrl) {
    const borderRadius = BorderRadius.all(Radius.circular(8));

    if (reference.targetType == ShareTargetType.content) {
      return ClipRRect(
        borderRadius: borderRadius,
        child: SizedBox(
          width: 60,
          height: 60,
          child: StableNetworkImage(
            imageUrl: imageUrl,
            fit: BoxFit.cover,
            fallback: _buildThumbnailFallback(),
          ),
        ),
      );
    }

    return ClipRRect(
      borderRadius: borderRadius,
      child: Image.network(
        imageUrl,
        width: 60,
        height: 60,
        fit: BoxFit.cover,
        errorBuilder: (context, error, stackTrace) => _buildThumbnailFallback(),
      ),
    );
  }

  Widget _buildThumbnailFallback() {
    return Container(
      width: 60,
      height: 60,
      color: AppColors.neutralGray200,
      child: const Icon(Icons.image_not_supported),
    );
  }
}
