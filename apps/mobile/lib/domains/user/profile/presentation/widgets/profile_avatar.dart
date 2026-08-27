import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

/// Animated avatar widget untuk Profile V2
///
/// Features:
/// - Support DualAvatar (seller) dan HybridAvatar (buyer)
/// - Animated size berdasarkan scroll progress
/// - Real-time online status indicator from Firebase
class ProfileAvatar extends ConsumerWidget {
  final String userId;
  final String? avatarUrl;
  final String? farmPhotoUrl;
  final String initials;
  final bool isSeller;
  final double size;
  final bool showOnlineStatus;
  final VoidCallback? onTap;

  const ProfileAvatar({
    super.key,
    required this.userId,
    this.avatarUrl,
    this.farmPhotoUrl,
    required this.initials,
    this.isSeller = false,
    this.size = 80,
    this.showOnlineStatus = true,
    this.onTap,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    // Watch real-time online status from Firebase
    final onlineStatusAsync = showOnlineStatus
        ? ref.watch(userOnlineStatusProvider(userId))
        : const AsyncValue.data(false);

    return GestureDetector(
      onTap: onTap,
      child: SizedBox(
        width: size,
        height: size,
        child: _buildAvatar(isDark, onlineStatusAsync),
      ),
    );
  }

  Widget _buildAvatar(bool isDark, AsyncValue<bool> onlineStatusAsync) {
    // Get online status value (default to false if loading/error)
    final isOnline = onlineStatusAsync.when(
      data: (value) => value,
      loading: () => false,
      error: (_, _) => false,
    );

    // Seller dengan farm photo -> dual avatar
    if (isSeller && farmPhotoUrl != null && farmPhotoUrl!.isNotEmpty) {
      return _buildDualAvatar(isDark, isOnline);
    }

    // Regular avatar - wrap with Stack to add online indicator on top
    if (showOnlineStatus && isOnline) {
      return Stack(
        children: [
          HybridAvatar(
            userId: userId,
            savedAvatarUrl: avatarUrl,
            size: size,
            initials: initials,
            showOnlineStatus: false, // We handle it ourselves
          ),
          Positioned(right: 0, bottom: 0, child: _buildOnlineIndicator()),
        ],
      );
    }

    return HybridAvatar(
      userId: userId,
      savedAvatarUrl: avatarUrl,
      size: size,
      initials: initials,
      showOnlineStatus: false,
    );
  }

  Widget _buildDualAvatar(bool isDark, bool isOnline) {
    final personalSize = size * 0.4;

    return Stack(
      children: [
        // Farm photo sebagai background (main avatar)
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
            child: Image.network(
              farmPhotoUrl!,
              fit: BoxFit.cover,
              errorBuilder: (context, error, stackTrace) => Container(
                color: isDark
                    ? AppColors.darkGray700
                    : AppColors.neutralGray200,
                child: Icon(
                  Icons.storefront,
                  size: size * 0.4,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray500,
                ),
              ),
            ),
          ),
        ),

        // Personal photo overlay di bottom-right
        Positioned(
          right: 0,
          bottom: 0,
          child: Container(
            width: personalSize,
            height: personalSize,
            decoration: BoxDecoration(
              shape: BoxShape.circle,
              boxShadow: [
                BoxShadow(
                  color: Colors.black.withValues(alpha: 0.15),
                  blurRadius: 4,
                  offset: const Offset(0, 1),
                ),
              ],
            ),
            child: ClipOval(
              child: avatarUrl != null && avatarUrl!.isNotEmpty
                  ? Image.network(
                      avatarUrl!,
                      fit: BoxFit.cover,
                      errorBuilder: (context, error, stackTrace) =>
                          _buildInitialsAvatar(personalSize, isDark),
                    )
                  : _buildInitialsAvatar(personalSize, isDark),
            ),
          ),
        ),

        // Online status indicator - only show if user is actually online
        if (showOnlineStatus && isOnline)
          Positioned(right: 0, bottom: 0, child: _buildOnlineIndicator()),
      ],
    );
  }

  Widget _buildInitialsAvatar(double avatarSize, bool isDark) {
    return Container(
      width: avatarSize,
      height: avatarSize,
      color: isDark ? AppColors.darkGray600 : AppColors.neutralGray300,
      child: Center(
        child: Text(
          initials,
          style: TextStyle(
            fontSize: avatarSize * 0.4,
            fontWeight: FontWeight.bold,
            color: isDark ? AppColors.neutralWhite : AppColors.neutralGray700,
          ),
        ),
      ),
    );
  }

  /// Online indicator badge (green dot)
  /// Only shown when user is actually online (from Firebase real-time presence)
  /// Fixed 12px size for consistency (matches CardHeader reference)
  Widget _buildOnlineIndicator() {
    return Container(
      width: 12,
      height: 12,
      decoration: BoxDecoration(
        color: AppColors.success,
        shape: BoxShape.circle,
        border: Border.all(color: AppColors.neutralWhite, width: 2),
      ),
    );
  }
}
