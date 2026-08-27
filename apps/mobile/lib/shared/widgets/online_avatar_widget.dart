import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/widgets/profile_avatar.dart';

/// Avatar widget dengan online status indicator (green dot)
/// Menggunakan global presence provider untuk menampilkan status online
class OnlineAvatarWidget extends ConsumerWidget {
  final String userId;
  final String? imageUrl;
  final String? initials;
  final double size;
  final VoidCallback? onTap;

  /// Size of the online indicator dot relative to avatar size
  static const double _indicatorSizeRatio = 0.25;

  /// Minimum indicator size
  static const double _minIndicatorSize = 8.0;

  const OnlineAvatarWidget({
    super.key,
    required this.userId,
    this.imageUrl,
    this.initials,
    this.size = 40,
    this.onTap,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // Watch user's online status
    final onlineStatusAsync = ref.watch(userOnlineStatusProvider(userId));

    return GestureDetector(
      onTap: onTap,
      child: Stack(
        children: [
          // Avatar
          ProfileAvatar(
            userId: userId,
            size: size,
            imageUrl: imageUrl,
            initials: initials,
          ),

          // Online indicator
          onlineStatusAsync.when(
            data: (isOnline) => isOnline
                ? _buildOnlineIndicator(context)
                : const SizedBox.shrink(),
            loading: () => const SizedBox.shrink(),
            error: (_, _) => const SizedBox.shrink(),
          ),
        ],
      ),
    );
  }

  Widget _buildOnlineIndicator(BuildContext context) {
    final indicatorSize = (size * _indicatorSizeRatio).clamp(
      _minIndicatorSize,
      size * 0.35,
    );

    return Positioned(
      right: 0,
      bottom: 0,
      child: Container(
        width: indicatorSize,
        height: indicatorSize,
        decoration: BoxDecoration(
          color: AppColors.successGreen,
          shape: BoxShape.circle,
          border: Border.all(
            color: Theme.of(context).scaffoldBackgroundColor,
            width: 2,
          ),
        ),
      ),
    );
  }
}

/// Compact version untuk list items
class OnlineAvatarCompact extends ConsumerWidget {
  final String userId;
  final String? imageUrl;
  final String? initials;
  final double size;
  final VoidCallback? onTap;

  const OnlineAvatarCompact({
    super.key,
    required this.userId,
    this.imageUrl,
    this.initials,
    this.size = 32,
    this.onTap,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return OnlineAvatarWidget(
      userId: userId,
      imageUrl: imageUrl,
      initials: initials,
      size: size,
      onTap: onTap,
    );
  }
}

/// Widget untuk menampilkan online status text (e.g., "Online" or "Offline")
class OnlineStatusText extends ConsumerWidget {
  final String userId;
  final TextStyle? style;
  final TextStyle? onlineStyle;

  const OnlineStatusText({
    super.key,
    required this.userId,
    this.style,
    this.onlineStyle,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final onlineStatusAsync = ref.watch(userOnlineStatusProvider(userId));

    return onlineStatusAsync.when(
      data: (isOnline) => Text(
        isOnline ? 'Online' : 'Offline',
        style: isOnline
            ? (onlineStyle ??
                  style?.copyWith(color: AppColors.successGreen) ??
                  TextStyle(color: AppColors.successGreen, fontSize: 12))
            : (style ??
                  TextStyle(color: AppColors.neutralGray500, fontSize: 12)),
      ),
      loading: () => Text(
        'Memuat...',
        style:
            style ?? TextStyle(color: AppColors.neutralGray500, fontSize: 12),
      ),
      error: (_, _) => const SizedBox.shrink(),
    );
  }
}
