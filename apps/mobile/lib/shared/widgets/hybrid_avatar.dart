import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/user/profile/data/profile_providers.dart';
import 'package:labuda/shared/shared.dart';

// R3.1: Removed deprecated userAvatarServiceProvider - UserAvatarApiService no longer exists
// Use avatarCacheServiceProvider from profile domain instead

/// Hybrid Avatar Widget dengan 3-tier fallback strategy
///
/// Priority:
/// 1. Current user avatar (jika own post/request) - INSTANT
/// 2. Saved avatar URL in post/request - FAST
/// 3. Fresh user avatar from API - FRESH
///
/// **R2.3 MIGRATION:** Now uses AvatarCacheService from profile domain
/// instead of deprecated UserAvatarApiService from shared.
///
/// Widget ini otomatis handle semua fallback dan caching
class HybridAvatar extends ConsumerWidget {
  final String userId;
  final String?
  savedAvatarUrl; // dari post.authorAvatarUrl atau request.userAvatarUrl
  final double size;
  final String? initials;
  final VoidCallback? onTap;
  final bool showOnlineStatus;

  const HybridAvatar({
    super.key,
    required this.userId,
    this.savedAvatarUrl,
    this.size = 36,
    this.initials,
    this.onTap,
    this.showOnlineStatus = false,
  });

  /// Factory untuk post header
  factory HybridAvatar.postHeader({
    required String userId,
    String? savedAvatarUrl,
    String? initials,
    VoidCallback? onTap,
  }) {
    return HybridAvatar(
      userId: userId,
      savedAvatarUrl: savedAvatarUrl,
      size: 36,
      initials: initials,
      onTap: onTap,
    );
  }

  /// Factory untuk medium avatar
  factory HybridAvatar.medium({
    required String userId,
    String? savedAvatarUrl,
    String? initials,
    VoidCallback? onTap,
  }) {
    return HybridAvatar(
      userId: userId,
      savedAvatarUrl: savedAvatarUrl,
      size: 40,
      initials: initials,
      onTap: onTap,
    );
  }

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authState = ref.watch(authControllerProvider);
    // **R2.3 MIGRATION:** Use AvatarCacheService from profile domain
    final avatarCacheService = ref.read(avatarCacheServiceProvider);

    // Watch online status if enabled
    final isOnline = showOnlineStatus
        ? (ref.watch(userOnlineStatusProvider(userId)).value ?? false)
        : false;

    // Tier 1: Current user avatar (INSTANT)
    String? currentUserAvatar;
    if (authState is AuthStateAuthenticated && authState.user.id == userId) {
      currentUserAvatar = authState.user.avatarUrl;
    }

    // Tier 2: Use saved avatar or current user avatar
    final fallbackAvatarUrl = currentUserAvatar ?? savedAvatarUrl;

    return FutureBuilder<String?>(
      // Tier 3: Background fetch fresh avatar (FRESH)
      future: _shouldFetchFreshAvatar(currentUserAvatar, savedAvatarUrl)
          ? avatarCacheService.getUserAvatarUrl(userId)
          : Future.value(null),
      builder: (context, snapshot) {
        // Priority logic:
        // 1. Fresh avatar dari API (jika ada dan berbeda)
        // 2. Fallback avatar (current user atau saved)
        // 3. Null (akan fallback ke initials)

        String? finalAvatarUrl;

        if (snapshot.hasData &&
            snapshot.data != null &&
            snapshot.data!.isNotEmpty) {
          // Use fresh avatar jika available dan berbeda dari fallback
          finalAvatarUrl = snapshot.data;
        } else {
          // Use fallback (instant/fast)
          finalAvatarUrl = fallbackAvatarUrl;
        }

        final avatar = ProfileAvatar(
          userId: userId,
          size: size,
          imageUrl: finalAvatarUrl,
          initials: initials,
          onTap: onTap,
        );

        // Return with online status dot if enabled
        if (!showOnlineStatus) return avatar;

        final dotSize = (size * 0.25).clamp(8.0, 14.0);
        // Calculate offset to position dot on circle edge at ~45°
        // Formula: size * 0.146 (distance from corner to circle edge) - dotSize/2
        final dotOffset = (size * 0.146 - dotSize / 2).clamp(0.0, size * 0.15);

        return SizedBox(
          width: size,
          height: size,
          child: Stack(
            clipBehavior: Clip.none,
            children: [
              avatar,
              if (isOnline)
                Positioned(
                  right: dotOffset,
                  bottom: dotOffset,
                  child: Container(
                    width: dotSize,
                    height: dotSize,
                    decoration: BoxDecoration(
                      color: AppColors.primaryGreen,
                      shape: BoxShape.circle,
                      border: Border.all(
                        color: AppColors.neutralWhite,
                        width: 2,
                      ),
                    ),
                  ),
                ),
            ],
          ),
        );
      },
    );
  }

  /// Determine apakah perlu fetch fresh avatar atau tidak
  ///
  /// Rules:
  /// - Jika current user: tidak perlu fetch (sudah pasti fresh)
  /// - Jika ada saved avatar: fetch background untuk update
  /// - Jika tidak ada saved avatar: fetch immediately
  bool _shouldFetchFreshAvatar(
    String? currentUserAvatar,
    String? savedAvatarUrl,
  ) {
    // Jika current user, tidak perlu fetch
    if (currentUserAvatar != null) {
      return false;
    }

    // Untuk user lain, selalu fetch untuk freshness
    return true;
  }
}
