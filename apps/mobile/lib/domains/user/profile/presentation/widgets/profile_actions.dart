import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/shared/governance/content_lifecycle.dart';

/// Compact action buttons untuk Profile V2
///
/// Features:
/// - Own profile: Edit Profile, Share
/// - Other profile: Follow/Unfollow, Message
/// - Compact icon + text style
///
/// E5.2 — `lifecycle` gates target-user actions (follow / message) on
/// degraded identities. Own-profile buttons remain enabled regardless
/// (lifecycle is sourced from the target's identity card, not the viewer's).
class ProfileActions extends ConsumerWidget {
  final String userId;
  final bool isOwnProfile;
  final VoidCallback? onEditProfile;
  final VoidCallback? onShare;
  final VoidCallback? onMessage;
  final double opacity;
  final ContentLifecycle lifecycle;

  const ProfileActions({
    super.key,
    required this.userId,
    required this.isOwnProfile,
    this.onEditProfile,
    this.onShare,
    this.onMessage,
    this.opacity = 1.0,
    this.lifecycle = ContentLifecycle.active,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return Opacity(
      opacity: opacity,
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: isOwnProfile
            ? _buildOwnProfileButtons(context, isDark)
            : _buildOtherProfileButtons(context, isDark),
      ),
    );
  }

  List<Widget> _buildOwnProfileButtons(BuildContext context, bool isDark) {
    return [
      _CompactButton(
        icon: Icons.edit_outlined,
        label: 'Edit',
        onTap: onEditProfile,
        isDark: isDark,
      ),
      const SizedBox(width: 8),
      _CompactButton(
        icon: Icons.share_outlined,
        label: 'Share',
        onTap: onShare,
        isDark: isDark,
        isSecondary: true,
      ),
    ];
  }

  List<Widget> _buildOtherProfileButtons(BuildContext context, bool isDark) {
    final disabled = lifecycle.isDegraded;
    return [
      // Use existing FollowButton from shared module.
      // E5.2 — disabled on degraded; the FollowButton widget itself remains
      // untouched (separate convergence); we mute it via Opacity +
      // IgnorePointer at the call site so it cannot fire on a degraded
      // target.
      IgnorePointer(
        ignoring: disabled,
        child: Opacity(
          opacity: disabled ? 0.4 : 1.0,
          child: FollowButton(userId: userId),
        ),
      ),
      const SizedBox(width: 8),
      _CompactButton(
        icon: Icons.chat_bubble_outline,
        label: 'Message',
        onTap: disabled ? null : onMessage,
        isDark: isDark,
        isSecondary: true,
        disabled: disabled,
      ),
    ];
  }
}

/// Compact button dengan icon + label
class _CompactButton extends StatelessWidget {
  final IconData icon;
  final String label;
  final VoidCallback? onTap;
  final bool isDark;
  final bool isSecondary;
  final bool disabled;

  const _CompactButton({
    required this.icon,
    required this.label,
    this.onTap,
    required this.isDark,
    this.isSecondary = false,
    this.disabled = false,
  });

  @override
  Widget build(BuildContext context) {
    final backgroundColor = isSecondary
        ? (isDark ? AppColors.darkGray700 : AppColors.neutralGray100)
        : AppColors.primaryRed;

    final foregroundColor = isSecondary
        ? (isDark ? AppColors.neutralWhite : AppColors.neutralGray700)
        : AppColors.neutralWhite;

    final effectiveOpacity = disabled ? 0.4 : 1.0;

    return Opacity(
      opacity: effectiveOpacity,
      child: Material(
        color: backgroundColor,
        borderRadius: BorderRadius.circular(8),
        child: InkWell(
          onTap: disabled ? null : onTap,
          borderRadius: BorderRadius.circular(8),
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(icon, size: 16, color: foregroundColor),
                const SizedBox(width: 4),
                Text(
                  label,
                  style: TextStyle(
                    fontSize: 13,
                    fontWeight: FontWeight.w500,
                    color: foregroundColor,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
