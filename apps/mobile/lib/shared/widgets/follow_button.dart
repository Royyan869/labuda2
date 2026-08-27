import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/user/identity/authentication/presentation/widgets/blocked_action_gate.dart';
import 'package:labuda/domains/social/follow/follow.dart';

// Alias for easier access
final followProvider = followStatusProvider;

/// Compact Follow Button untuk ContentCard dan RequestCard headers
///
/// Features:
/// - Real follow functionality dengan Firebase
/// - Loading states dan error handling
/// - Theme-aware design
/// - Compact size untuk headers
class FollowButton extends ConsumerStatefulWidget {
  final String userId; // User yang akan di-follow
  final double? buttonSize;
  final double? iconSize;
  final double? fontSize;

  const FollowButton({
    super.key,
    required this.userId,
    this.buttonSize,
    this.iconSize = 14,
    this.fontSize = 12,
  });

  @override
  ConsumerState<FollowButton> createState() => _FollowButtonState();
}

class _FollowButtonState extends ConsumerState<FollowButton> {
  bool _isLoading = false;

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final authState = ref.watch(authControllerProvider);

    // Show placeholder while auth is loading
    if (authState is AuthStateLoading) {
      return _buildPlaceholderButton(isDark);
    }

    // Don't show if not authenticated
    if (authState is! AuthStateAuthenticated) {
      return const SizedBox.shrink();
    }

    final currentUserId = authState.user.id;
    final currentUserName = authState.user.username;

    // Don't show if trying to follow yourself
    if (currentUserId == widget.userId) {
      return const SizedBox.shrink();
    }

    // Watch follow state dari follows module
    final followState = ref.watch(followProvider);
    final isFollowing = followState.followStatusMap[widget.userId] ?? false;

    // Always show button, even if status is not loaded yet
    // This prevents flickering and provides better UX
    if (!followState.followStatusMap.containsKey(widget.userId)) {
      // Trigger initial status check without blocking UI
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
      // Show default follow button instead of loading
      return _buildFollowButton(
        context,
        isDark,
        currentUserId,
        currentUserName,
        false, // Default to not following
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

  Widget _buildPlaceholderButton(bool isDark) {
    return Container(
      height: widget.buttonSize ?? 24,
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
      decoration: BoxDecoration(
        border: Border.all(
          color: isDark ? AppColors.neutralGray500 : AppColors.neutralGray400,
          width: 1,
        ),
        borderRadius: BorderRadius.circular(12),
        color: Colors.transparent,
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            Icons.person_add,
            size: widget.iconSize ?? 14,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
          ),
          const SizedBox(width: 4),
          Text(
            'Follow',
            style: TextStyle(
              fontSize: widget.fontSize ?? 12,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
              fontWeight: FontWeight.w500,
            ),
          ),
        ],
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
    return GestureDetector(
      onTap: _isLoading
          ? null
          : () => _toggleFollow(currentUserId, currentUserName, isFollowing),
      child: Container(
        height: widget.buttonSize ?? 24,
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
        decoration: BoxDecoration(
          border: Border.all(
            color: isFollowing
                ? AppColors.primaryRed
                : (isDark
                      ? AppColors.neutralGray500
                      : AppColors.neutralGray400),
            width: 1,
          ),
          borderRadius: BorderRadius.circular(12),
          color: isFollowing
              ? AppColors.primaryRed.withValues(alpha: 0.1)
              : Colors.transparent,
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            if (_isLoading)
              SizedBox(
                width: widget.iconSize ?? 14,
                height: widget.iconSize ?? 14,
                child: CircularProgressIndicator(
                  strokeWidth: 1.5,
                  valueColor: AlwaysStoppedAnimation<Color>(
                    isFollowing
                        ? AppColors.primaryRed
                        : (isDark
                              ? AppColors.neutralWhite
                              : AppColors.neutralGray700),
                  ),
                ),
              )
            else
              Icon(
                isFollowing ? Icons.person_remove : Icons.person_add,
                size: widget.iconSize ?? 14,
                color: isFollowing
                    ? AppColors.primaryRed
                    : (isDark
                          ? AppColors.neutralWhite
                          : AppColors.neutralGray700),
              ),
            const SizedBox(width: 4),
            Text(
              isFollowing ? 'Unfollow' : 'Follow',
              style: TextStyle(
                fontSize: widget.fontSize ?? 12,
                color: isFollowing
                    ? AppColors.primaryRed
                    : (isDark
                          ? AppColors.neutralWhite
                          : AppColors.neutralGray700),
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _toggleFollow(
    String currentUserId,
    String currentUserName,
    bool currentFollowStatus,
  ) async {
    // Preflight for FOLLOW only: backend gates POST /users/:id/follow with
    // RequireInteractionAuthority (email-verified). Unfollow stays open by
    // doctrine so reducing a relationship is never blocked by an interaction
    // gate. The backend remains authoritative — this preflight just avoids
    // a guaranteed 403 round-trip.
    if (!currentFollowStatus) {
      final authState = ref.read(authControllerProvider);
      if (authState is AuthStateAuthenticated && !authState.emailVerified) {
        await showBlockedActionGate(
          context,
          actionDescription: 'mengikuti pengguna',
        );
        return;
      }
    }

    setState(() {
      _isLoading = true;
    });

    // Use follows module provider - error handling is done in Provider
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

    setState(() {
      _isLoading = false;
    });

    // Check if operation was successful by checking error state
    final followState = ref.read(followProvider);
    if (followState.error == null) {
      AppSnackBar.showSuccess(
        context,
        currentFollowStatus ? 'Unfollowed user' : 'Started following user',
      );
    } else {
      // Error is handled by Provider, show it here
      AppSnackBar.showError(
        context,
        followState.error ??
            'Failed to ${currentFollowStatus ? 'unfollow' : 'follow'} user',
      );
    }
  }
}
