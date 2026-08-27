import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/core/core.dart';

/// Reusable user header widget with avatar, name, username, and timeago
///
/// Used across cards and detail screens for consistent user information display
class UserHeaderWidget extends ConsumerWidget {
  final String userId;
  final String name;
  final String? username;
  final String? avatarUrl;
  final DateTime createdAt;
  final VoidCallback? onTap;
  final bool showTimeAgo;
  final UserHeaderSize size;
  final bool showOnlineStatus;

  const UserHeaderWidget({
    super.key,
    required this.userId,
    required this.name,
    this.username,
    this.avatarUrl,
    required this.createdAt,
    this.onTap,
    this.showTimeAgo = true,
    this.size = UserHeaderSize.medium,
    this.showOnlineStatus = false,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: [
        // User Avatar - Only this is tappable
        GestureDetector(onTap: onTap, child: _buildAvatar(ref)),
        SizedBox(width: _getSpacing()),

        // User Info - NOT tappable
        // Use Flexible instead of ConstrainedBox to allow dynamic width
        Flexible(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            mainAxisSize: MainAxisSize.min,
            children: [
              // Name
              Text(
                name,
                style: _getNameStyle(theme),
                overflow: TextOverflow.ellipsis,
              ),

              // Username and TimeAgo row
              if (username != null || showTimeAgo) ...[
                SizedBox(height: _getVerticalSpacing()),
                Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    // Username
                    if (username != null) ...[
                      Flexible(
                        child: Text(
                          '@$username',
                          style: _getSecondaryStyle(theme, isDark),
                          overflow: TextOverflow.ellipsis,
                        ),
                      ),
                      if (showTimeAgo) ...[
                        Text(' • ', style: _getSecondaryStyle(theme, isDark)),
                      ],
                    ],

                    // TimeAgo
                    if (showTimeAgo)
                      TimeAgoWidget.compact(
                        dateTime: createdAt,
                        color: isDark
                            ? AppColors.neutralGray400
                            : AppColors.neutralGray600,
                      ),
                  ],
                ),
              ],
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildAvatar(WidgetRef ref) {
    final avatarSize = switch (size) {
      UserHeaderSize.small => 24.0,
      UserHeaderSize.medium => 40.0,
      UserHeaderSize.large => 80.0,
    };

    final avatar = HybridAvatar(
      userId: userId,
      savedAvatarUrl: avatarUrl,
      size: avatarSize,
    );

    if (!showOnlineStatus) {
      return avatar;
    }

    // Wrap with online status indicator (green dot)
    final isOnlineAsync = ref.watch(userOnlineStatusProvider(userId));
    final isOnline = isOnlineAsync.value ?? false;

    final dotSize = (avatarSize * 0.25).clamp(6.0, 12.0);

    return Stack(
      children: [
        avatar,
        if (isOnline)
          Positioned(
            right: 0,
            bottom: 0,
            child: Container(
              width: dotSize,
              height: dotSize,
              decoration: BoxDecoration(
                color: AppColors.primaryGreen,
                shape: BoxShape.circle,
                border: Border.all(color: AppColors.neutralWhite, width: 1.5),
              ),
            ),
          ),
      ],
    );
  }

  double _getSpacing() {
    switch (size) {
      case UserHeaderSize.small:
        return 8;
      case UserHeaderSize.medium:
        return 12;
      case UserHeaderSize.large:
        return 16;
    }
  }

  double _getVerticalSpacing() {
    switch (size) {
      case UserHeaderSize.small:
        return 2;
      case UserHeaderSize.medium:
        return 4;
      case UserHeaderSize.large:
        return 6;
    }
  }

  TextStyle _getNameStyle(ThemeData theme) {
    switch (size) {
      case UserHeaderSize.small:
        return theme.textTheme.bodyMedium?.copyWith(
              fontWeight: FontWeight.w600,
            ) ??
            const TextStyle();
      case UserHeaderSize.medium:
        return theme.textTheme.titleMedium?.copyWith(
              fontWeight: FontWeight.w600,
            ) ??
            const TextStyle();
      case UserHeaderSize.large:
        return theme.textTheme.titleLarge?.copyWith(
              fontWeight: FontWeight.w600,
            ) ??
            const TextStyle();
    }
  }

  TextStyle _getSecondaryStyle(ThemeData theme, bool isDark) {
    final baseStyle = switch (size) {
      UserHeaderSize.small => theme.textTheme.bodySmall,
      UserHeaderSize.medium => theme.textTheme.bodySmall,
      UserHeaderSize.large => theme.textTheme.bodyMedium,
    };

    return baseStyle?.copyWith(
          color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
        ) ??
        const TextStyle();
  }
}

/// Size variants for UserHeaderWidget
enum UserHeaderSize {
  small, // For compact layouts
  medium, // Default size for cards
  large, // For detail screens or prominent displays
}
