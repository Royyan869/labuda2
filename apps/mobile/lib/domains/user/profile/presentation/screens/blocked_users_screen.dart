import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/domains/system/shared/domain/services/time_format_service.dart';
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
            style: TextStyle(
              color: Theme.of(context).colorScheme.onSurfaceVariant,
            ),
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
              return _buildEmptyState(context);
            }

            return ListView.builder(
              padding: const EdgeInsets.symmetric(vertical: AppMetrics.p8),
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
                  size: AppIconSize.display,
                  color: Theme.of(context).colorScheme.error,
                ),
                const SizedBox(height: 16),
                Text(
                  'Failed to load blocked users',
                  style: context.typeRoles.titleCompact.copyWith(
                    color: Theme.of(context).colorScheme.onSurface,
                  ),
                ),
                const SizedBox(height: 8),
                Text(
                  error.toString(),
                  style: context.typeRoles.labelMicro.copyWith(
                    color: Theme.of(context).colorScheme.onSurfaceVariant,
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

  Widget _buildEmptyState(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(AppMetrics.p32),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.block_outlined,
              size: AppIconSize.display,
              color: scheme.onSurfaceVariant,
            ),
            const SizedBox(height: 16),
            Text(
              'No Blocked Users',
              style: context.typeRoles.titleSection.copyWith(
                fontWeight: FontWeight.bold,
                color: scheme.onSurface,
              ),
            ),
            const SizedBox(height: 8),
            Text(
              'Users you block will appear here.\nYou won\'t see their posts or messages.',
              style: context.typeRoles.bodyDense.copyWith(
                color: scheme.onSurfaceVariant,
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
              style: TextStyle(color: Theme.of(context).colorScheme.primary),
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
          AppSnackBar.showSuccess(context, 'Blokir pengguna dibuka');
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
    final scheme = Theme.of(context).colorScheme;

    return ListTile(
      contentPadding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p16,
        vertical: AppMetrics.p8,
      ),
      leading: ProfileAvatar(userId: userId, size: 48, imageUrl: avatarUrl),
      title: Text(
        '@$username',
        style: TextStyle(fontWeight: FontWeight.w600, color: scheme.onSurface),
      ),
      subtitle: Text(
        'Blocked ${const TimeFormatService().formatTimeAgo(blockedAt)}',
        style: context.typeRoles.labelMicro.copyWith(
          color: scheme.onSurfaceVariant,
        ),
      ),
      trailing: OutlinedButton(
        onPressed: onUnblock,
        style: OutlinedButton.styleFrom(
          padding: const EdgeInsets.symmetric(
            horizontal: AppMetrics.p16,
            vertical: AppMetrics.p8,
          ),
        ),
        child: Text(
          'Unblock',
          style: Theme.of(context).textTheme.labelLarge?.copyWith(
            color: scheme.primary,
            fontWeight: FontWeight.w600,
          ),
        ),
      ),
    );
  }
}
