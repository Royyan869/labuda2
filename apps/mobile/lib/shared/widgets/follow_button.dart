import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/shared.dart';
import 'package:hishumi/domains/social/follow/follow.dart';

// Alias for easier access
final followProvider = followStatusProvider;

/// Canonical Follow Button — single authority for all follow surfaces.
///
/// Replaces the legacy mini outline variant (height 24, radius 12, transparent).
/// Now identical to ProfileActions _CompactButton: solid primaryRed for
/// "Follow", secondary gray for "Following", padding 12/8, font 13, icon 16,
/// radius 8. One size, one style, no variants.
class FollowButton extends ConsumerStatefulWidget {
  final String userId;

  /// Domain gate: a viewer may be forbidden from following a degraded
  /// identity. Disabled renders the ONE canonical disabled language
  /// (`surfaceContainerHighest` / `onSurfaceVariant`), never a local opacity.
  final bool enabled;

  const FollowButton({super.key, required this.userId, this.enabled = true});

  @override
  ConsumerState<FollowButton> createState() => _FollowButtonState();
}

class _FollowButtonState extends ConsumerState<FollowButton> {
  bool _isLoading = false;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final authState = ref.watch(authControllerProvider);

    if (authState is AuthStateLoading) {
      return _buildPlaceholderButton(scheme, disabled: true);
    }

    if (authState is! AuthStateAuthenticated) {
      return const SizedBox.shrink();
    }

    final currentUserId = authState.user.id;
    final currentUserName = authState.user.username;

    if (currentUserId == widget.userId) {
      return const SizedBox.shrink();
    }

    final followState = ref.watch(followProvider);
    final isFollowing = followState.followStatusMap[widget.userId] ?? false;

    if (!followState.followStatusMap.containsKey(widget.userId)) {
      WidgetsBinding.instance.addPostFrameCallback((_) {
        if (mounted) {
          ref
              .read(followProvider.notifier)
              .checkFollowStatus(
                followerId: currentUserId,
                followingId: widget.userId,
              );
        }
      });
      return _buildFollowButton(
        context,
        scheme,
        currentUserId,
        currentUserName,
        false,
      );
    }

    return _buildFollowButton(
      context,
      scheme,
      currentUserId,
      currentUserName,
      isFollowing,
    );
  }

  Widget _buildPlaceholderButton(ColorScheme scheme, {bool disabled = false}) {
    final bg = scheme.surfaceContainerHighest;
    final fg = scheme.onSurfaceVariant;
    return Container(
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p12,
        vertical: AppMetrics.p8,
      ),
      decoration: BoxDecoration(
        color: bg,
        borderRadius: BorderRadius.circular(AppShape.r8),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            Icons.person_add_outlined,
            size: AppIconSize.inlineGlyph,
            color: fg,
          ),
          const SizedBox(width: 4),
          Text(
            'Follow',
            style: Theme.of(context).textTheme.labelLarge?.copyWith(
              color: fg,
              fontWeight: FontWeight.w500,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildFollowButton(
    BuildContext context,
    ColorScheme scheme,
    String currentUserId,
    String currentUserName,
    bool isFollowing,
  ) {
    final isDisabled = !widget.enabled;
    final bg = isDisabled || isFollowing
        ? scheme.surfaceContainerHighest
        : scheme.primary;
    final fg = isDisabled
        ? scheme.onSurfaceVariant
        : isFollowing
        ? scheme.onSurface
        : scheme.onPrimary;

    return Material(
      color: bg,
      borderRadius: BorderRadius.circular(AppShape.r8),
      child: InkWell(
        onTap: (isDisabled || _isLoading)
            ? null
            : () => _toggleFollow(currentUserId, currentUserName, isFollowing),
        borderRadius: BorderRadius.circular(AppShape.r8),
        child: Container(
          padding: const EdgeInsets.symmetric(
            horizontal: AppMetrics.p12,
            vertical: AppMetrics.p8,
          ),
          child: Row(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (_isLoading)
                SizedBox(
                  width: 16,
                  height: 16,
                  child: CircularProgressIndicator(
                    strokeWidth: 1.5,
                    valueColor: AlwaysStoppedAnimation<Color>(fg),
                  ),
                )
              else
                Icon(
                  isFollowing
                      ? Icons.person_remove
                      : Icons.person_add_outlined,
                  size: AppIconSize.inlineGlyph,
                  color: fg,
                ),
              const SizedBox(width: 4),
              Text(
                isFollowing ? 'Following' : 'Follow',
                style: Theme.of(context).textTheme.labelLarge?.copyWith(
                  color: fg,
                  fontWeight: FontWeight.w500,
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Future<void> _toggleFollow(
    String currentUserId,
    String currentUserName,
    bool currentFollowStatus,
  ) async {
    setState(() => _isLoading = true);

    if (currentFollowStatus) {
      await ref
          .read(followProvider.notifier)
          .unfollowUser(followerId: currentUserId, followingId: widget.userId);
    } else {
      await ref
          .read(followProvider.notifier)
          .followUser(followerId: currentUserId, followingId: widget.userId);
    }

    if (!mounted) return;
    setState(() => _isLoading = false);

    final followState = ref.read(followProvider);
    if (followState.error == null) {
      AppSnackBar.showSuccess(
        context,
        currentFollowStatus ? 'Berhenti mengikuti' : 'Mulai mengikuti',
      );
    } else {
      AppSnackBar.showError(
        context,
        followState.error ??
            'Failed to ${currentFollowStatus ? 'unfollow' : 'follow'} user',
      );
    }
  }
}
