import 'package:flutter/material.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/domain/entities/resource_projection.dart';
import 'package:hishumi/shared/widgets/app_image.dart';

/// Canonical pre-send commerce chip.
///
/// ONE AUTHORITY for every composer that holds a not-yet-sent product
/// attachment — the comment composer, the chat composer, and every chat
/// entry point (For Sale detail, Auction detail, checkout). The row shows
/// photo + title + price and a remove affordance only.
///
/// Display data here is a SNAPSHOT from the picker (identity + display
/// hints), never commerce authority: price and availability are re-resolved
/// by the server at send into the viewer-aware projection. That is why the
/// chip carries no status badge — a pre-send snapshot must never pose as a
/// lifecycle state.
///
/// LAYOUT lives here; the ONLY action is [onRemove], which closes the
/// attachment. Sending is the composer's send icon — a pre-send chip never
/// carries its own send CTA.
class PendingCommerceChip extends StatelessWidget {
  const PendingCommerceChip({
    super.key,
    required this.title,
    this.imageUrl,
    this.price,
    this.caption,
    this.onRemove,
  });

  /// Display title of the attached product.
  final String title;

  /// Display photo (picker snapshot, may be null).
  final String? imageUrl;

  /// Display price in rupiah (picker snapshot, may be null for auctions
  /// whose price is a live bid — the server projection owns the number).
  final int? price;

  /// Small lead line above the title (e.g. 'Lampiran produk').
  final String? caption;

  /// Closes the attachment. When null, no close button renders.
  final VoidCallback? onRemove;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      padding: const EdgeInsets.all(AppMetrics.p12),
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHigh,
        borderRadius: BorderRadius.circular(AppShape.r10),
        border: Border.all(
          color: scheme.primary.withValues(alpha: 0.3),
          width: 1,
        ),
      ),
      child: Row(
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(AppShape.r6),
            child: imageUrl != null
                ? AppImage(
                    imageUrl: imageUrl,
                    width: 45,
                    height: 45,
                    fit: BoxFit.cover,
                    errorWidget: _placeholder(context),
                  )
                : _placeholder(context),
          ),
          const SizedBox(width: 10),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              mainAxisSize: MainAxisSize.min,
              children: [
                if (caption != null) ...[
                  Text(
                    caption!,
                    style: Theme.of(context).textTheme.labelSmall?.copyWith(
                      color: scheme.primary,
                      fontWeight: FontWeight.w600,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
                Text(
                  title,
                  style: Theme.of(
                    context,
                  ).textTheme.bodyMedium?.copyWith(fontWeight: FontWeight.w600),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
                if (price != null) ...[
                  const SizedBox(height: 2),
                  Text(
                    'Rp ${formatGroupedAmount(price!)}',
                    style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                      color: scheme.primary,
                      fontWeight: FontWeight.bold,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ],
            ),
          ),
          if (onRemove != null)
            IconButton(
              tooltip: 'Hapus lampiran',
              onPressed: onRemove,
              icon: const Icon(Icons.close, size: AppIconSize.action),
              constraints: const BoxConstraints(),
              padding: EdgeInsets.zero,
            ),
        ],
      ),
    );
  }

  Widget _placeholder(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      width: 45,
      height: 45,
      decoration: BoxDecoration(
        color: scheme.surfaceContainerHighest,
        borderRadius: BorderRadius.circular(AppShape.r6),
      ),
      child: Icon(
        Icons.image_not_supported,
        size: AppIconSize.inlineGlyph,
        color: scheme.onSurfaceVariant,
      ),
    );
  }
}
