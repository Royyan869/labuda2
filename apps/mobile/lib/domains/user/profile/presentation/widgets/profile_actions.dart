import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/shared/shared.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

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
  final ContentLifecycle lifecycle;

  const ProfileActions({
    super.key,
    required this.userId,
    required this.isOwnProfile,
    this.onEditProfile,
    this.onShare,
    this.onMessage,
    this.lifecycle = ContentLifecycle.active,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    return Row(
      mainAxisSize: MainAxisSize.min,
      children: isOwnProfile
          ? _buildOwnProfileButtons(context)
          : _buildOtherProfileButtons(context),
    );
  }

  List<Widget> _buildOwnProfileButtons(BuildContext context) {
    return [
      _CompactButton(
        icon: Icons.edit_outlined,
        label: 'Edit',
        onTap: onEditProfile,
      ),
      const SizedBox(width: 8),
      _CompactButton(
        icon: Icons.share_outlined,
        label: 'Share',
        onTap: onShare,
        isSecondary: true,
      ),
    ];
  }

  List<Widget> _buildOtherProfileButtons(BuildContext context) {
    final disabled = lifecycle.isDegraded;
    return [
      // Single authority: FollowButton canonical. Degraded identities pass
      // `enabled: false`, so the component renders the ONE canonical disabled
      // language instead of a local opacity/ignore hack.
      FollowButton(userId: userId, enabled: !disabled),
      const SizedBox(width: 8),
      _CompactButton(
        icon: Icons.chat_bubble_outline,
        label: 'Message',
        onTap: disabled ? null : onMessage,
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
  final bool isSecondary;
  final bool disabled;

  const _CompactButton({
    required this.icon,
    required this.label,
    this.onTap,
    this.isSecondary = false,
    this.disabled = false,
  });

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    // One disabled language (owner decision 2026-10-05): neutral
    // surfaceContainerHighest fill + onSurfaceVariant content — never a local
    // opacity fade.
    final backgroundColor = disabled
        ? scheme.surfaceContainerHighest
        : isSecondary
        ? scheme.surfaceContainerHigh
        : scheme.primary;
    final foregroundColor = disabled
        ? scheme.onSurfaceVariant
        : isSecondary
        ? scheme.onSurface
        : scheme.onPrimary;

    return Material(
      color: backgroundColor,
      borderRadius: BorderRadius.circular(AppShape.r8),
      child: InkWell(
        onTap: disabled ? null : onTap,
        borderRadius: BorderRadius.circular(AppShape.r8),
        child: Container(
          padding: const EdgeInsets.symmetric(
            horizontal: AppMetrics.p12,
            vertical: AppMetrics.p8,
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                icon,
                size: AppIconSize.inlineGlyph,
                color: foregroundColor,
              ),
              const SizedBox(width: 4),
              Text(
                label,
                style: context.typeRoles.bodyDense.copyWith(
                  fontWeight: FontWeight.w500,
                  color: foregroundColor,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
