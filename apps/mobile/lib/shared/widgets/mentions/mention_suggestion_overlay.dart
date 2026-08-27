import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/features/search/search/search.dart'; // R3.1: Import mention providers from search domain

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
    final isDark = Theme.of(context).brightness == Brightness.dark;

    // Search users
    final searchParams = MentionSearchParams(
      query: query,
      allowedUserIds: allowedUserIds,
    );
    final usersAsync = ref.watch(mentionUserSearchProvider(searchParams));

    return Material(
      elevation: 8,
      borderRadius: BorderRadius.circular(12),
      color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
      child: Container(
        constraints: const BoxConstraints(maxHeight: 250, minHeight: 60),
        child: usersAsync.when(
          loading: () => const Center(
            child: Padding(
              padding: EdgeInsets.all(16),
              child: CircularProgressIndicator(),
            ),
          ),
          error: (error, _) => Center(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Text(
                'Error loading users',
                style: TextStyle(
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray600,
                ),
              ),
            ),
          ),
          data: (users) {
            if (users.isEmpty && !showSpecialMentions) {
              return Center(
                child: Padding(
                  padding: const EdgeInsets.all(16),
                  child: Text(
                    'No users found',
                    style: TextStyle(
                      color: isDark
                          ? AppColors.neutralGray400
                          : AppColors.neutralGray600,
                    ),
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
                    isDark: isDark,
                  ),
                  if (users.isNotEmpty)
                    Divider(
                      height: 1,
                      color: isDark
                          ? AppColors.darkGray700
                          : AppColors.neutralGray200,
                    ),
                ],

                // User mentions
                ...users.map(
                  (user) => _buildUserMentionTile(
                    context,
                    user: user,
                    onTap: () => onUserSelected(user),
                    isDark: isDark,
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
    required bool isDark,
  }) {
    return ListTile(
      dense: true,
      leading: CircleAvatar(
        backgroundColor: AppColors.primaryRed.withValues(alpha: 0.1),
        child: Icon(icon, color: AppColors.primaryRed, size: 20),
      ),
      title: Text(
        username,
        style: const TextStyle(
          fontWeight: FontWeight.w600,
          color: AppColors.primaryRed,
        ),
      ),
      subtitle: Text(
        subtitle,
        style: TextStyle(
          fontSize: 12,
          color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
        ),
      ),
      onTap: onTap,
    );
  }

  Widget _buildUserMentionTile(
    BuildContext context, {
    required UserSearch user,
    required VoidCallback onTap,
    required bool isDark,
  }) {
    return ListTile(
      dense: true,
      leading: HybridAvatar(
        userId: user.userId,
        size: 32,
        initials: UserInitialsHelper.get(
          name: user.username,
          userId: user.userId,
        ),
      ),
      title: Row(
        children: [
          Flexible(
            child: Text(
              '@${user.username}',
              style: const TextStyle(
                fontWeight: FontWeight.w600,
                color: AppColors.primaryBlue,
              ),
              overflow: TextOverflow.ellipsis,
            ),
          ),
        ],
      ),
      subtitle: Text(
        '@${user.username}',
        style: TextStyle(
          fontSize: 12,
          color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
        ),
        overflow: TextOverflow.ellipsis,
      ),
      onTap: onTap,
    );
  }
}
