import 'package:flutter/material.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

/// Card Header Widget
///
/// Reusable card header with avatar, name, subtitle, and online status.
/// Commonly used across multiple modules (profile, chat, etc).
class CardHeader extends StatelessWidget {
  final String avatarUrl;
  final String name;
  final String? subtitle;
  final bool isOnline;
  final String? timeAgo;
  final VoidCallback? onTap;
  final VoidCallback? onAvatarTap;
  final Widget? trailing;
  final double avatarSize;
  final double horizontalPadding;
  final bool showOnlineIndicator;

  const CardHeader({
    super.key,
    required this.avatarUrl,
    required this.name,
    this.subtitle,
    this.isOnline = false,
    this.timeAgo,
    this.onTap,
    this.onAvatarTap,
    this.trailing,
    this.avatarSize = 40,
    this.horizontalPadding = 16,
    this.showOnlineIndicator = true,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Material(
      color: Colors.transparent,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: EdgeInsets.symmetric(
            horizontal: horizontalPadding,
            vertical: 12,
          ),
          child: Row(
            children: [
              // Avatar with online indicator
              GestureDetector(
                onTap: onAvatarTap,
                child: Stack(
                  children: [
                    ProfileAvatar(
                      userId: name,
                      size: avatarSize,
                      imageUrl: avatarUrl.isEmpty ? null : avatarUrl,
                      initials: UserInitialsHelper.fromName(name),
                      showShadow: false,
                    ),
                    if (showOnlineIndicator && isOnline)
                      Positioned(
                        bottom: 0,
                        right: 0,
                        child: Container(
                          width: 12,
                          height: 12,
                          decoration: BoxDecoration(
                            color: AppColors.statusSuccess,
                            shape: BoxShape.circle,
                            border: Border.all(
                              color: isDark
                                  ? AppColors.neutralGray800
                                  : AppColors.neutralWhite,
                              width: 2,
                            ),
                          ),
                        ),
                      ),
                  ],
                ),
              ),
              const SizedBox(width: 12),

              // Name and subtitle
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      name,
                      style: AppTypography.username.copyWith(
                        color: isDark
                            ? AppColors.neutralWhite
                            : AppColors.neutralGray900,
                      ),
                    ),
                    if (subtitle != null) ...[
                      const SizedBox(height: 2),
                      Text(
                        subtitle!,
                        style: AppTypography.caption.copyWith(
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

              // Time ago or trailing widget
              if (timeAgo != null && trailing == null)
                Text(
                  timeAgo!,
                  style: AppTypography.timestamp.copyWith(
                    color: isDark
                        ? AppColors.neutralGray500
                        : AppColors.neutralGray400,
                  ),
                )
              else
                ?trailing,
            ],
          ),
        ),
      ),
    );
  }

  /// Compact variant with smaller avatar and padding
  factory CardHeader.compact({
    Key? key,
    required String avatarUrl,
    required String name,
    String? subtitle,
    bool isOnline = false,
    String? timeAgo,
    VoidCallback? onTap,
    VoidCallback? onAvatarTap,
    Widget? trailing,
  }) {
    return CardHeader(
      key: key,
      avatarUrl: avatarUrl,
      name: name,
      subtitle: subtitle,
      isOnline: isOnline,
      timeAgo: timeAgo,
      onTap: onTap,
      onAvatarTap: onAvatarTap,
      trailing: trailing,
      avatarSize: 32,
      horizontalPadding: 12,
    );
  }
}
