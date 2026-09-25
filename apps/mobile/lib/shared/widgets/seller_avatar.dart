import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/models/seller_identity_data.dart';
import 'package:labuda/shared/widgets/hybrid_avatar.dart';
import 'package:labuda/shared/widgets/seller_dual_avatar.dart';

/// CANONICAL seller-aware avatar composite for profile surfaces.
///
/// Business truth (Owner decision 2026-09-24):
/// - Seller (`isSeller == true`) → [SellerDualAvatar]: store/farm circle with
///   the personal avatar overlaid. The dual layout is gated ONLY on the
///   seller flag — never on photo availability.
/// - Non-seller → single personal avatar via [HybridAvatar] (image or user
///   icon; no initials fallback exists anywhere).
class SellerAvatar extends StatelessWidget {
  final String userId;
  final String? avatarUrl;
  final String? storeImageUrl;
  final bool isSeller;
  final double size;
  final VoidCallback? onTap;

  const SellerAvatar({
    super.key,
    required this.userId,
    this.avatarUrl,
    this.storeImageUrl,
    this.isSeller = false,
    this.size = 80,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    if (isSeller) {
      return SellerDualAvatar(
        identity: SellerIdentityData(
          userId: userId,
          avatarUrl: avatarUrl,
          storeImageUrl: storeImageUrl,
          isSeller: true,
        ),
        size: size,
        onTap: onTap,
      );
    }

    return HybridAvatar(
      userId: userId,
      size: size,
      savedAvatarUrl: avatarUrl,
      onTap: onTap,
    );
  }
}

/// External online-presence badge wrapper.
///
/// Renders the green presence dot over [child] when the tracked user is
/// online. Use this INSTEAD of the avatar widgets' internal online handling
/// so the badge placement is uniform across single and dual layouts.
class OnlineBadge extends ConsumerWidget {
  final String userId;
  final bool enabled;
  final Widget child;

  const OnlineBadge({
    super.key,
    required this.userId,
    this.enabled = true,
    required this.child,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (!enabled) return child;

    final isOnline =
        ref.watch(userOnlineStatusProvider(userId)).value ?? false;
    if (!isOnline) return child;

    return Stack(
      clipBehavior: Clip.none,
      children: [
        child,
        Positioned(
          right: 0,
          bottom: 0,
          child: Container(
            width: 12,
            height: 12,
            decoration: BoxDecoration(
              color: AppColors.success,
              shape: BoxShape.circle,
              border: Border.all(color: AppColors.neutralWhite, width: 2),
            ),
          ),
        ),
      ],
    );
  }
}
