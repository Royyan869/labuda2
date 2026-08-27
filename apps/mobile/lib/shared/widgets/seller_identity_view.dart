import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/models/seller_identity_data.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/widgets/seller_dual_avatar.dart';

enum SellerIdentityViewVariant { profile, drawer, detail }

/// Shared seller identity composite for profile, drawer, and detail surfaces.
class SellerIdentityView extends StatelessWidget {
  final SellerIdentityData identity;
  final SellerIdentityViewVariant variant;
  final double? size;
  final VoidCallback? onTap;

  const SellerIdentityView({
    super.key,
    required this.identity,
    required this.variant,
    this.size,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final avatarSize = size ?? _defaultAvatarSize();
    switch (variant) {
      case SellerIdentityViewVariant.profile:
        return _buildProfile(context, avatarSize: avatarSize);
      case SellerIdentityViewVariant.drawer:
        return _buildAvatar(context, size: avatarSize);
      case SellerIdentityViewVariant.detail:
        return _buildDetail(context, avatarSize: avatarSize);
    }
  }

  Widget _buildProfile(BuildContext context, {required double avatarSize}) {
    final handle = identity.displayHandle;
    final storeName = identity.normalizedStoreName;

    if (handle == null && storeName == null) {
      return const SizedBox.shrink();
    }

    final isDark = Theme.of(context).brightness == Brightness.dark;
    final avatar = _buildAvatar(context, size: avatarSize);
    final textScale = _profileTextScale(avatarSize);
    final storeNameSize = _lerpDouble(11.0, 13.0, textScale);
    final handleSize = _lerpDouble(14.0, 18.0, textScale);
    final storeColor = isDark
        ? AppColors.neutralWhite
        : AppColors.neutralGray900;
    final handleColor = isDark
        ? AppColors.neutralGray400
        : AppColors.neutralGray500;

    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      mainAxisSize: MainAxisSize.max,
      children: [
        avatar,
        const SizedBox(width: 12),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              if (storeName != null)
                Text(
                  storeName,
                  style: AppTypography.bodyMedium.copyWith(
                    color: storeColor,
                    fontWeight: FontWeight.w600,
                    fontSize: storeNameSize,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              if (handle != null) ...[
                if (storeName != null) const SizedBox(height: 2),
                Text(
                  handle,
                  style: AppTypography.bodySmall.copyWith(
                    color: handleColor,
                    fontSize: handleSize,
                  ),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              ],
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildAvatar(BuildContext context, {required double size}) {
    if (identity.isSeller) {
      return SellerDualAvatar(
        identity: identity,
        size: size,
        storeImageReloadToken: identity.storeImageReloadToken,
        onTap: onTap,
      );
    }

    return HybridAvatar(
      userId: identity.userId,
      size: size,
      savedAvatarUrl: identity.normalizedAvatarUrl,
      initials: UserInitialsHelper.fromName(identity.normalizedUsername),
      onTap: onTap,
      showOnlineStatus: true,
    );
  }

  Widget _buildDetail(BuildContext context, {required double avatarSize}) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final handle = identity.displayHandle;
    final storeName = identity.normalizedStoreName;
    final originLine = identity.publicOriginLine?.trim();

    if (handle == null && storeName == null && originLine == null) {
      return const SizedBox.shrink();
    }

    final avatar = _buildAvatar(context, size: avatarSize);

    return GestureDetector(
      onTap: onTap,
      behavior: HitTestBehavior.opaque,
      child: Row(
        children: [
          avatar,
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                if (storeName != null)
                  Text(
                    storeName,
                    style: AppTypography.bodyMedium.copyWith(
                      color: isDark
                          ? AppColors.neutralWhite
                          : AppColors.neutralGray900,
                      fontWeight: FontWeight.w600,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                if (handle != null) ...[
                  if (storeName != null) const SizedBox(height: 2),
                  Text(
                    handle,
                    style: AppTypography.bodySmall.copyWith(
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray600,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
                if (originLine != null) ...[
                  if (handle != null || storeName != null) const SizedBox(height: 2),
                  Text(
                    originLine,
                    style: AppTypography.bodySmall.copyWith(
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray600,
                    ),
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ],
              ],
            ),
          ),
        ],
      ),
    );
  }

  double _defaultAvatarSize() {
    switch (variant) {
      case SellerIdentityViewVariant.profile:
        return 80;
      case SellerIdentityViewVariant.drawer:
        return 56;
      case SellerIdentityViewVariant.detail:
        return 48;
    }
  }

  double _profileTextScale(double avatarSize) {
    const minAvatarSize = 40.0;
    const maxAvatarSize = 96.0;
    final clamped = avatarSize.clamp(minAvatarSize, maxAvatarSize);
    return (clamped - minAvatarSize) / (maxAvatarSize - minAvatarSize);
  }

  double _lerpDouble(double begin, double end, double t) {
    return begin + (end - begin) * t;
  }
}
