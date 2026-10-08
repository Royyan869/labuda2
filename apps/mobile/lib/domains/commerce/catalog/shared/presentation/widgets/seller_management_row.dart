import 'package:flutter/material.dart';
import 'package:labuda/core/src/theme/app_theme.dart';
import 'package:labuda/shared/widgets/app_image.dart';

/// Canonical seller-management row — ONE AUTHORITY for every seller-owned
/// listing row (auction management, for-sale management, and anything after
/// them).
///
/// Both screens used to keep their own copy, and the copies had drifted: a
/// `Card` with default margin and 88px thumbnail in one, a `Container` with
/// its own bottom margin and 80px thumbnail in the other.
///
/// VISUAL CONTRACT (locked):
///   - frame: `colorScheme.surface`, flat, radius [AppShape.r12],
///     1px `outlineVariant` border, internal padding [AppMetrics.p12]
///   - rhythm: bottom margin [AppMetrics.p12] — the row owns its gap, so
///     parent lists must NOT add separators around it
///   - thumbnail: 80x80, radius [AppShape.r8], `BoxFit.cover`
///   - interaction: whole-row `InkWell` (may be null = display-only)
///
/// LAYOUT: header row [thumbnail + title + trailing] with an optional
/// full-width [body] below it (price/meta rows). Status chips, badges,
/// popups and actions stay with the owning screen — the row only owns the
/// frame, the thumbnail and the rhythm.
///
/// NOT for buyer discovery grids ([CommerceMarketplaceCardShell]) and NOT
/// for the balance hero ([CoinBalanceCard] stays an explicit exception).
class SellerManagementRow extends StatelessWidget {
  /// Thumbnail slot — use [SellerManagementThumbnail].
  final Widget leading;

  /// Title block: title + status chips. Owned by the calling screen.
  final Widget title;

  /// Trailing slot: popup menu or management actions. Nullable.
  final Widget? trailing;

  /// Optional full-width content under the header row (meta/price rows).
  final Widget? body;

  /// Whole-row tap. Null renders a display-only row (no affordance).
  final VoidCallback? onTap;

  const SellerManagementRow({
    super.key,
    required this.leading,
    required this.title,
    this.trailing,
    this.body,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      margin: const EdgeInsets.only(bottom: AppMetrics.p12),
      decoration: BoxDecoration(
        color: scheme.surface,
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: scheme.outlineVariant),
      ),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(AppShape.r12),
        child: Padding(
          padding: const EdgeInsets.all(AppMetrics.p12),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  leading,
                  const SizedBox(width: 12),
                  Expanded(child: title),
                  if (trailing != null) ...[trailing!],
                ],
              ),
              if (body != null) ...[
                const SizedBox(height: AppMetrics.p12),
                body!,
              ],
            ],
          ),
        ),
      ),
    );
  }
}

/// Canonical management-row thumbnail: 80x80 cover image with a built-in
/// fallback. Callers map their own media model onto [imageUrl]; a null or
/// failing URL renders the placeholder — never an empty box.
class SellerManagementThumbnail extends StatelessWidget {
  final String? imageUrl;

  const SellerManagementThumbnail({super.key, required this.imageUrl});

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final imageUrl = this.imageUrl;
    return ClipRRect(
      borderRadius: BorderRadius.circular(AppShape.r8),
      child: SizedBox(
        width: 80,
        height: 80,
        child: switch (imageUrl) {
          final String url when url.isNotEmpty => AppImage(
            imageUrl: url,
            width: 80,
            height: 80,
            fit: BoxFit.cover,
            backgroundColor: scheme.surfaceContainerHighest,
            errorWidget: _fallback(scheme),
          ),
          _ => _fallback(scheme),
        },
      ),
    );
  }

  Widget _fallback(ColorScheme scheme) {
    return Container(
      color: scheme.surfaceContainerHighest,
      child: Icon(
        Icons.image_not_supported,
        size: AppIconSize.header,
        color: scheme.onSurfaceVariant,
      ),
    );
  }
}
