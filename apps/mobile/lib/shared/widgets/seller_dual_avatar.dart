import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/models/seller_identity_data.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/widgets/stable_network_image.dart';

/// Pure seller dual-avatar primitive.
///
/// No provider access and no global state. The personal overlay renders
/// through [ProfileAvatar] with initials fallback.
class SellerDualAvatar extends StatelessWidget {
  final SellerIdentityData identity;
  final double size;
  final String? storeImageReloadToken;
  final VoidCallback? onTap;

  const SellerDualAvatar({
    super.key,
    required this.identity,
    this.size = 80,
    this.storeImageReloadToken,
    this.onTap,
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
                    color: Colors.black.withValues(alpha: 0.2),
                    blurRadius: 8,
                    offset: const Offset(0, 2),
                  ),
                ],
              ),
              child: ClipOval(
                child: hasStoreImage
                    ? StableNetworkImage(
                        imageUrl: identity.normalizedStoreImageUrl,
                        reloadToken: storeImageReloadToken,
                        fit: BoxFit.cover,
                        fallback: storePlaceholder,
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
                initials: UserInitialsHelper.fromName(identity.normalizedUsername),
                showShadow: false,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildStorePlaceholder(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    return Container(
      color: isDark ? AppColors.darkGray700 : AppColors.neutralGray200,
      child: Icon(
        Icons.storefront,
        size: size * 0.4,
        color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
      ),
    );
  }
}
