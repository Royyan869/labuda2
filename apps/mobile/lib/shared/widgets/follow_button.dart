import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/social/follow/follow.dart';

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

  const FollowButton({super.key, required this.userId});

  @override
  ConsumerState<FollowButton> createState() => _FollowButtonState();
}

class _FollowButtonState extends ConsumerState<FollowButton> {
  bool _isLoading = false;

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final authState = ref.watch(authControllerProvider);

    if (authState is AuthStateLoading) {
      return _buildPlaceholderButton(isDark, disabled: true);
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
          ref.read(followProvider.notifier).checkFollowStatus(
            followerId: currentUserId,
            followingId: widget.userId,
          );
        }
      });
      return _buildFollowButton(
        context,
        isDark,
        currentUserId,
        currentUserName,
        false,
      );
    }

    return _buildFollowButton(
      context,
      isDark,
      currentUserId,
      currentUserName,
      isFollowing,
    );
  }

  Widget _buildPlaceholderButton(bool isDark, {bool disabled = false}) {
    final bg = isDark ? AppColors.darkGray700 : AppColors.neutralGray100;
    final fg = isDark ? AppColors.neutralWhite : AppColors.neutralGray700;
    return Opacity(
      opacity: disabled ? 0.4 : 1.0,
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        decoration: BoxDecoration(
          color: bg,
          borderRadius: BorderRadius.circular(8),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.person_add_outlined, size: 16, color: fg),
            const SizedBox(width: 4),
            Text(
              'Follow',
              style: TextStyle(
                fontSize: 13,
                color: fg,
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildFollowButton(
    BuildContext context,
    bool isDark,
    String currentUserId,
    String currentUserName,
    bool isFollowing,
  ) {
    final bg = isFollowing
        ? (isDark ? AppColors.darkGray700 : AppColors.neutralGray100)
        : AppColors.primaryRed;
    final fg = isFollowing
        ? (isDark ? AppColors.neutralWhite : AppColors.neutralGray700)
        : AppColors.neutralWhite;

    return Opacity(
      opacity: _isLoading ? 0.6 : 1.0,
      child: Material(
        color: bg,
        borderRadius: BorderRadius.circular(8),
        child: InkWell(
          onTap: _isLoading
              ? null
              : () => _toggleFollow(currentUserId, currentUserName, isFollowing),
          borderRadius: BorderRadius.circular(8),
          child: Container(
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
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
                    isFollowing ? Icons.person_remove : Icons.person_add_outlined,
                    size: 16,
                    color: fg,
                  ),
                const SizedBox(width: 4),
                Text(
                  isFollowing ? 'Following' : 'Follow',
                  style: TextStyle(
                    fontSize: 13,
                    color: fg,
                    fontWeight: FontWeight.w500,
                  ),
                ),
              ],
            ),
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
        currentFollowStatus ? 'Unfollowed user' : 'Started following user',
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
