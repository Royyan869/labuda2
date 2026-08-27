import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/user/profile/presentation/providers/blocked_users_provider.dart';

/// Blocked Users Screen
/// Displays list of users that have been blocked by the current user
///
/// Features:
/// - View blocked users list
/// - Unblock users
/// - Empty state
///
/// Size: < 250 lines (per GUIDELINES)
class BlockedUsersScreen extends ConsumerWidget {
  const BlockedUsersScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final authState = ref.watch(authControllerProvider);

    if (authState is! AuthStateAuthenticated) {
      return Scaffold(
        appBar: const AppBarCustom(title: 'Blocked Users'),
        body: Center(
          child: Text(
            'Please login to view blocked users',
            style: TextStyle(color: AppColors.neutralGray600),
          ),
        ),
      );
    }

    final blockedUsersAsync = ref.watch(
      blockedUsersProvider(authState.user.id),
    );

    return Scaffold(
      appBar: const AppBarCustom(title: 'Blocked Users'),
      body: SafeArea(
        child: blockedUsersAsync.when(
          data: (blockedUsers) {
            if (blockedUsers.isEmpty) {
              return _buildEmptyState();
            }

            return ListView.builder(
              padding: const EdgeInsets.symmetric(vertical: 8),
              itemCount: blockedUsers.length,
              itemBuilder: (context, index) {
                final blockedUser = blockedUsers[index];
                return _BlockedUserTile(
                  userId: blockedUser.id,
                  username: blockedUser.username,
                  avatarUrl: blockedUser.avatarUrl,
                  blockedAt: blockedUser.blockedAt,
                  onUnblock: () => _handleUnblock(
                    context,
                    ref,
                    authState.user.id,
                    blockedUser.id,
                    blockedUser.username,
                  ),
                );
              },
            );
          },
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, stack) => Center(
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(
                  Icons.error_outline,
                  size: 48,
                  color: AppColors.primaryRed,
                ),
                const SizedBox(height: 16),
                Text(
                  'Failed to load blocked users',
                  style: TextStyle(
                    fontSize: 16,
                    color: AppColors.neutralGray700,
                  ),
                ),
                const SizedBox(height: 8),
                Text(
                  error.toString(),
                  style: TextStyle(
                    fontSize: 12,
                    color: AppColors.neutralGray500,
                  ),
                  textAlign: TextAlign.center,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildEmptyState() {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.block_outlined,
              size: 64,
              color: AppColors.neutralGray400,
            ),
            const SizedBox(height: 16),
            Text(
              'No Blocked Users',
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.bold,
                color: AppColors.neutralGray700,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              'Users you block will appear here.\nYou won\'t see their posts or messages.',
              style: TextStyle(
                fontSize: 14,
                color: AppColors.neutralGray600,
                height: 1.5,
              ),
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }

  Future<void> _handleUnblock(
    BuildContext context,
    WidgetRef ref,
    String currentUserId,
    String blockedUserId,
    String username,
  ) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: const Text('Unblock User'),
        content: Text(
          'Are you sure you want to unblock @$username? They will be able to interact with you again.',
        ),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: const Text('Cancel'),
          ),
          TextButton(
            onPressed: () => Navigator.of(context).pop(true),
            child: Text(
              'Unblock',
              style: TextStyle(color: AppColors.primaryRed),
            ),
          ),
        ],
      ),
    );

    if (confirmed == true && context.mounted) {
      try {
        await ref
            .read(blockedUsersActionsProvider)
            .unblockUser(
              currentUserId: currentUserId,
              blockedUserId: blockedUserId,
            );
        if (context.mounted) {
          AppSnackBar.showSuccess(context, 'User unblocked successfully');
        }
      } catch (e) {
        if (context.mounted) {
          AppSnackBar.showError(context, 'Aksi belum berhasil. Coba lagi.');
        }
      }
    }
  }
}

/// Blocked User List Tile Widget
class _BlockedUserTile extends StatelessWidget {
  final String userId;
  final String username;
  final String? avatarUrl;
  final DateTime blockedAt;
  final VoidCallback onUnblock;

  const _BlockedUserTile({
    required this.userId,
    required this.username,
    this.avatarUrl,
    required this.blockedAt,
    required this.onUnblock,
  });

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return ListTile(
      contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      leading: CircleAvatar(
        radius: 24,
        backgroundImage: avatarUrl != null && avatarUrl!.isNotEmpty
            ? NetworkImage(avatarUrl!)
            : null,
        child: avatarUrl == null || avatarUrl!.isEmpty
            ? Icon(
                Icons.person,
                color: isDark
                    ? AppColors.neutralGray600
                    : AppColors.neutralGray400,
              )
            : null,
      ),
      title: Text(
        '@$username',
        style: TextStyle(
          fontWeight: FontWeight.w600,
          color: isDark ? AppColors.neutralWhite : AppColors.neutralGray900,
        ),
      ),
      subtitle: Text(
        'Blocked ${_formatBlockedDate(blockedAt)}',
        style: TextStyle(
          fontSize: 12,
          color: isDark ? AppColors.neutralGray500 : AppColors.neutralGray600,
        ),
      ),
      trailing: OutlinedButton(
        onPressed: onUnblock,
        style: OutlinedButton.styleFrom(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          side: BorderSide(color: AppColors.primaryRed),
        ),
        child: Text(
          'Unblock',
          style: TextStyle(
            color: AppColors.primaryRed,
            fontSize: 13,
            fontWeight: FontWeight.w600,
          ),
        ),
      ),
    );
  }

  String _formatBlockedDate(DateTime date) {
    final now = DateTime.now();
    final difference = now.difference(date);

    if (difference.inDays > 365) {
      final years = (difference.inDays / 365).floor();
      return '$years ${years == 1 ? 'year' : 'years'} ago';
    } else if (difference.inDays > 30) {
      final months = (difference.inDays / 30).floor();
      return '$months ${months == 1 ? 'month' : 'months'} ago';
    } else if (difference.inDays > 0) {
      return '${difference.inDays} ${difference.inDays == 1 ? 'day' : 'days'} ago';
    } else if (difference.inHours > 0) {
      return '${difference.inHours} ${difference.inHours == 1 ? 'hour' : 'hours'} ago';
    } else if (difference.inMinutes > 0) {
      return '${difference.inMinutes} ${difference.inMinutes == 1 ? 'minute' : 'minutes'} ago';
    } else {
      return 'just now';
    }
  }
}
