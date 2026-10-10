import 'package:flutter/material.dart';
import 'package:hishumi/shared/models/seller_identity_data.dart';
import 'package:hishumi/shared/widgets/hybrid_avatar.dart';
import 'package:hishumi/shared/widgets/seller_dual_avatar.dart';

/// The ONE identity composite: store name primary, handle secondary.
///
/// Surfaces that pair the two labels render this widget instead of composing
/// their own order. The former `drawer`/`detail` variants were dead renderers
/// (no consumer); the commerce detail surface has its own canonical authority
/// (`CommerceDetailSellerCard`), so only one renderer remains here.
class SellerIdentityView extends StatelessWidget {
  final SellerIdentityData identity;
  final double? size;
  final VoidCallback? onTap;

  const SellerIdentityView({
    super.key,
    required this.identity,
    this.size,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    return _buildIdentity(context, avatarSize: size ?? _defaultAvatarSize());
  }

  Widget _buildIdentity(BuildContext context, {required double avatarSize}) {
    final scheme = Theme.of(context).colorScheme;
    final handle = identity.displayHandle;
    final storeName = identity.normalizedStoreName;

    if (handle == null && storeName == null) {
      return const SizedBox.shrink();
    }

    final avatar = _buildAvatar(context, size: avatarSize);
    final textScale = _profileTextScale(avatarSize);

    // OWNER TRUTH — identity pairing: the store name is the primary line (top,
    // larger, emphasised) and the handle is secondary (below, smaller). A user
    // without a store has only a handle, and that handle takes the primary
    // treatment.
    final primarySize = _lerpDouble(15.0, 18.0, textScale);
    final secondarySize = _lerpDouble(12.0, 13.0, textScale);
    final textTheme = Theme.of(context).textTheme;
    final primaryStyle = textTheme.bodyMedium!.copyWith(
      color: scheme.onSurface,
      fontWeight: FontWeight.w600,
      fontSize: primarySize,
    );
    final secondaryStyle = textTheme.bodySmall!.copyWith(
      color: scheme.onSurfaceVariant,
      fontSize: secondarySize,
    );
    // The pairing rule lives in ONE place — the identity model — so every
    // surface orders the store name and the handle identically.
    final primaryLine = identity.primaryLabel;
    final secondaryLine = identity.secondaryLabel;

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
              if (primaryLine != null)
                Text(
                  primaryLine,
                  style: primaryStyle,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                ),
              if (secondaryLine != null) ...[
                const SizedBox(height: 2),
                Text(
                  secondaryLine,
                  style: secondaryStyle,
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
      onTap: onTap,
      showOnlineStatus: true,
    );
  }

  double _defaultAvatarSize() => 80;

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
