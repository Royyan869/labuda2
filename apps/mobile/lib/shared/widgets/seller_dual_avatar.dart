import 'package:flutter/material.dart';
import 'package:hishumi/shared/models/seller_identity_data.dart';
import 'package:hishumi/shared/widgets/app_image.dart';
import 'package:hishumi/shared/widgets/profile_avatar.dart';

/// CANONICAL dual avatar for sellers — store/farm image as the main circle
/// (storefront icon when no photo) with the personal [ProfileAvatar] overlaid
/// bottom-right. Non-sellers never render this widget: the single-surface
/// gate lives in [SellerIdentityView] (`identity.isSeller`).
class SellerDualAvatar extends StatelessWidget {
  final SellerIdentityData identity;
  final double size;
  final String? storeImageReloadToken;
  final VoidCallback? onTap;

  /// DECODE / CACHE IDENTITY for BOTH circles (store + personal overlay) —
  /// deliberately separate from [size].
  ///
  /// Null (the normal case) derives the decode target from [size], valid only
  /// while [size] is static. Callers that animate [size] (Profile header
  /// collapse) must pin this to a fixed value so the image provider key never
  /// moves with the animation — otherwise every frame re-instates the `Image`
  /// state and the photo flashes through its placeholder. [size] keeps
  /// driving the layout; this only drives the decode/cache key.
  final int? cacheWidth;

  const SellerDualAvatar({
    super.key,
    required this.identity,
    this.size = 80,
    this.storeImageReloadToken,
    this.onTap,
    this.cacheWidth,
  });

  @override
  Widget build(BuildContext context) {
    final avatar = _buildAvatar(context);
    return avatar;
  }

  Widget _buildAvatar(BuildContext context) {
    final hasStoreImage =
        identity.isSeller && identity.normalizedStoreImageUrl != null;

    final storePlaceholder = _buildStorePlaceholder(context);
    final personalSize = size * 0.4;

    // Decode targets: pinned to the caller's fixed cache width when supplied
    // (animated-size surfaces), otherwise derived from the static [size].
    final int? pinnedCacheWidth = cacheWidth;
    final storeCacheWidth = pinnedCacheWidth ?? (size * 2).round();
    final personalCacheWidth =
        pinnedCacheWidth == null ? null : (pinnedCacheWidth * 0.4).round();

    return GestureDetector(
      onTap: onTap,
      child: SizedBox(
        width: size,
        height: size,
        child: Stack(
          children: [
            Container(
              width: size,
              height: size,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                boxShadow: [
                  BoxShadow(
                    color: Theme.of(
                      context,
                    ).colorScheme.shadow.withValues(alpha: 0.2),
                    blurRadius: 8,
                    offset: const Offset(0, 2),
                  ),
                ],
              ),
              child: ClipOval(
                child: hasStoreImage
                    ? AppImage(
                        key: ValueKey(
                          'store:$storeImageReloadToken:${identity.normalizedStoreImageUrl}',
                        ),
                        imageUrl: identity.normalizedStoreImageUrl,
                        fit: BoxFit.cover,
                        isCircle: true,
                        width: size,
                        height: size,
                        cacheWidth: storeCacheWidth,
                        backgroundColor: Theme.of(
                          context,
                        ).colorScheme.surfaceContainerHighest,
                        errorWidget: storePlaceholder,
                      )
                    : storePlaceholder,
              ),
            ),
            Positioned(
              right: 0,
              bottom: 0,
              child: ProfileAvatar(
                userId: identity.userId,
                size: personalSize,
                imageUrl: identity.normalizedAvatarUrl,
                showShadow: false,
                cacheWidth: personalCacheWidth,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildStorePlaceholder(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Container(
      color: scheme.surfaceContainerHighest,
      child: Icon(
        Icons.storefront,
        size: size * 0.4,
        color: scheme.onSurfaceVariant,
      ),
    );
  }
}
