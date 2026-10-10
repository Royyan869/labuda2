import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/shared/widgets/app_image.dart';
import 'package:hishumi/shared/widgets/follow_button.dart';
import 'package:hishumi/shared/widgets/metadata_view.dart';
import 'package:hishumi/domains/social/follow/domain/entities/follow_entity.dart';
import 'package:hishumi/shared/governance/content_lifecycle.dart';
import 'package:hishumi/core/core.dart';

/// UserCard untuk Follow List Screen
///
/// Features:
/// - Compact design untuk list view
/// - Menggunakan shared FollowButton
/// - Responsive width (tidak overflow)
class UserCard extends ConsumerWidget {
  final FollowableUser user;
  final bool showFollowButton;
  final VoidCallback? onTap;

  const UserCard({
    super.key,
    required this.user,
    this.showFollowButton = true,
    this.onTap,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;
    final isDegraded = user.isDegraded;

    return Card(
      margin: EdgeInsets.zero,
      elevation: AppElevation.none,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppShape.r12),
        side: BorderSide(color: colorScheme.outline.withValues(alpha: 0.2)),
      ),
      child: InkWell(
        // Tap disabled for degraded users — profile is unavailable/removed.
        onTap: isDegraded ? null : onTap,
        borderRadius: BorderRadius.circular(AppShape.r12),
        child: Padding(
          padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p12, vertical: AppMetrics.p12),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.center,
            children: [
              _buildAvatar(context, isDegraded: isDegraded),
              const SizedBox(width: 12),

              Expanded(
                child: isDegraded
                    ? _buildDegradedInfo(context)
                    : _buildActiveInfo(context),
              ),

              const SizedBox(width: 12),

              // Follow button suppressed for degraded users.
              if (showFollowButton && !isDegraded)
                FollowButton(userId: user.id),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildActiveInfo(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(
          '@${user.username}',
          style: theme.textTheme.bodyMedium?.copyWith(
            fontWeight: FontWeight.w500,
          ),
          maxLines: 1,
          overflow: TextOverflow.ellipsis,
        ),
        const SizedBox(height: 2),
        // Standalone secondary fact (follow counts) → compact Metadata
        // text authority: one bounded line, truncates with ellipsis.
        MetadataText(
          text:
              '${_formatCount(user.followersCount)} followers • ${_formatCount(user.followingCount)} following',
          mode: MetadataMode.compact,
          style: theme.textTheme.bodySmall?.copyWith(
            color: colorScheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }

  Widget _buildDegradedInfo(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;
    // Canonical 2-string vocabulary via ContentLifecycleParse.publicRedactionLabel.
    // FollowableUser.lifecycle is a wire-string; fail-closed parsing means
    // unknown / null wire vocabulary degrades rather than silently rendering
    // as active.
    final label = ContentLifecycleParse.fromWire(
      user.lifecycle,
    ).publicRedactionLabel;
    return Text(
      label,
      style: theme.textTheme.bodyMedium?.copyWith(
        fontStyle: FontStyle.italic,
        color: colorScheme.onSurfaceVariant,
      ),
      maxLines: 1,
      overflow: TextOverflow.ellipsis,
    );
  }

  Widget _buildAvatar(BuildContext context, {required bool isDegraded}) {
    final colorScheme = Theme.of(context).colorScheme;

    return Container(
      width: 48,
      height: 48,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        border: Border.all(color: colorScheme.outline.withValues(alpha: 0.2)),
      ),
      child: ClipOval(
        // Degraded users always show the placeholder — no avatar leaked.
        child: user.avatar != null && !isDegraded
            ? AppImage(
                imageUrl: user.avatar,
                fit: BoxFit.cover,
                isCircle: true,
                width: 48,
                height: 48,
                backgroundColor:
                    Theme.of(context).colorScheme.surfaceContainerHighest,
                errorWidget: _buildAvatarPlaceholder(context),
              )
            : _buildAvatarPlaceholder(context),
      ),
    );
  }

  Widget _buildAvatarPlaceholder(BuildContext context) {
    final colorScheme = Theme.of(context).colorScheme;
    return Container(
      color: colorScheme.surfaceContainerHighest,
      child: Icon(Icons.person, size: AppIconSize.header, color: colorScheme.onSurfaceVariant),
    );
  }

  String _formatCount(int count) {
    if (count >= 1000000) {
      return '${(count / 1000000).toStringAsFixed(1)}M';
    } else if (count >= 1000) {
      return '${(count / 1000).toStringAsFixed(1)}K';
    } else {
      return count.toString();
    }
  }
}
