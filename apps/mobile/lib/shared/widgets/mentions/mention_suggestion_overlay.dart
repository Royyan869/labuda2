import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/shared/widgets/hybrid_avatar.dart';
import 'package:hishumi/features/search/search/search.dart'; // R3.1: Import mention providers from search domain
import 'package:hishumi/core/core.dart';

/// Overlay widget untuk show user suggestions saat mention
///
/// Muncul di atas keyboard ketika user ketik @username
class MentionSuggestionOverlay extends ConsumerWidget {
  final String query;
  final List<String>? allowedUserIds;
  final Function(UserSearch user) onUserSelected;
  final VoidCallback onDismiss;
  final bool showSpecialMentions;

  const MentionSuggestionOverlay({
    super.key,
    required this.query,
    this.allowedUserIds,
    required this.onUserSelected,
    required this.onDismiss,
    this.showSpecialMentions = false,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final scheme = Theme.of(context).colorScheme;

    // Search users
    final searchParams = MentionSearchParams(
      query: query,
      allowedUserIds: allowedUserIds,
    );
    final usersAsync = ref.watch(mentionUserSearchProvider(searchParams));

    return Material(
      elevation: AppElevation.overlay,
      borderRadius: BorderRadius.circular(AppShape.r12),
      color: scheme.surfaceContainerHigh,
      child: Container(
        constraints: const BoxConstraints(maxHeight: 250, minHeight: 60),
        child: usersAsync.when(
          loading: () => const Center(
            child: Padding(
              padding: EdgeInsets.all(AppMetrics.p16),
              child: CircularProgressIndicator(),
            ),
          ),
          error: (error, _) => Center(
            child: Padding(
              padding: const EdgeInsets.all(AppMetrics.p16),
              child: Text(
                'Error loading users',
                style: TextStyle(color: scheme.onSurfaceVariant),
              ),
            ),
          ),
          data: (users) {
            if (users.isEmpty && !showSpecialMentions) {
              return Center(
                child: Padding(
                  padding: const EdgeInsets.all(AppMetrics.p16),
                  child: Text(
                    'No users found',
                    style: TextStyle(color: scheme.onSurfaceVariant),
                  ),
                ),
              );
            }

            return ListView(
              shrinkWrap: true,
              children: [
                // Special mentions (for group chat)
                if (showSpecialMentions &&
                    allowedUserIds != null &&
                    allowedUserIds!.length >= 3) ...[
                  _buildSpecialMentionTile(
                    context,
                    icon: Icons.group,
                    username: '@everyone',
                    subtitle: 'Notify all ${allowedUserIds!.length} members',
                    onTap: () {
                      // Create fake search projection for @everyone
                      final everyoneUser = UserSearch(
                        userId: 'everyone',
                        username: 'everyone',
                      );
                      onUserSelected(everyoneUser);
                    },
                  ),
                  if (users.isNotEmpty)
                    Divider(height: 1, color: scheme.outlineVariant),
                ],

                // User mentions
                ...users.map(
                  (user) => _buildUserMentionTile(
                    context,
                    user: user,
                    onTap: () => onUserSelected(user),
                  ),
                ),
              ],
            );
          },
        ),
      ),
    );
  }

  Widget _buildSpecialMentionTile(
    BuildContext context, {
    required IconData icon,
    required String username,
    required String subtitle,
    required VoidCallback onTap,
  }) {
    final scheme = Theme.of(context).colorScheme;
    return ListTile(
      dense: true,
      leading: CircleAvatar(
        backgroundColor: scheme.primary.withValues(alpha: 0.1),
        child: Icon(icon, color: scheme.primary, size: AppIconSize.action),
      ),
      title: Text(
        username,
        style: TextStyle(fontWeight: FontWeight.w600, color: scheme.primary),
      ),
      subtitle: Text(
        subtitle,
        style: context.typeRoles.labelMicro.copyWith(
          color: scheme.onSurfaceVariant,
        ),
      ),
      onTap: onTap,
    );
  }

  Widget _buildUserMentionTile(
    BuildContext context, {
    required UserSearch user,
    required VoidCallback onTap,
  }) {
    final scheme = Theme.of(context).colorScheme;
    return ListTile(
      dense: true,
      leading: HybridAvatar(
        userId: user.userId,
        size: 32,
        savedAvatarUrl: user.avatarUrl,
      ),
      title: Row(
        children: [
          Flexible(
            child: Text(
              '@${user.username}',
              style: TextStyle(
                fontWeight: FontWeight.w600,
                color: scheme.secondary,
              ),
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      ),
      subtitle: Text(
        '@${user.username}',
        style: context.typeRoles.labelMicro.copyWith(
          color: scheme.onSurfaceVariant,
        ),
        overflow: TextOverflow.ellipsis,
      ),
      onTap: onTap,
    );
  }
}
