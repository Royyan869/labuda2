import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import '../providers/follow_stream_provider.dart';
import '../widgets/user_card.dart';

/// Follow List Screen - Display followers atau following list
///
/// Features:
/// - Tampilkan list followers atau following
/// - Search users
/// - Follow/Unfollow button per user
/// - Refresh data
/// - Empty state
///
/// Size: <300 lines (GUIDELINES compliant)
class FollowListScreen extends ConsumerStatefulWidget {
  final String userId;
  final FollowListType type;
  final String? username; // For title display

  const FollowListScreen({
    super.key,
    required this.userId,
    required this.type,
    this.username,
  });

  @override
  ConsumerState<FollowListScreen> createState() => _FollowListScreenState();
}

class _FollowListScreenState extends ConsumerState<FollowListScreen> {
  final _searchController = TextEditingController();
  String _searchQuery = '';

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;
    final authState = ref.watch(authControllerProvider);

    // Use stream providers for real-time updates
    final usersAsync = widget.type == FollowListType.followers
        ? ref.watch(followersStreamProvider(widget.userId))
        : ref.watch(followingStreamProvider(widget.userId));

    return Scaffold(
      appBar: AppBar(
        title: Text(
          widget.type == FollowListType.followers
              ? '${widget.username ?? 'User'}\'s Followers'
              : '${widget.username ?? 'User'}\'s Following',
        ),
        surfaceTintColor: Colors.transparent,
        scrolledUnderElevation: 0,
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(60),
          child: Padding(
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 8),
            child: TextField(
              controller: _searchController,
              decoration: InputDecoration(
                hintText: 'Search users...',
                prefixIcon: const Icon(Icons.search),
                suffixIcon: _searchQuery.isNotEmpty
                    ? IconButton(
                        icon: const Icon(Icons.clear),
                        onPressed: () {
                          _searchController.clear();
                          setState(() => _searchQuery = '');
                        },
                      )
                    : null,
                filled: true,
                fillColor: isDark
                    ? AppColors.neutralGray800
                    : AppColors.neutralGray100,
                border: OutlineInputBorder(
                  borderRadius: BorderRadius.circular(12),
                  borderSide: BorderSide.none,
                ),
                contentPadding: const EdgeInsets.symmetric(
                  horizontal: 16,
                  vertical: 12,
                ),
              ),
              onChanged: (value) {
                setState(() => _searchQuery = value);
              },
            ),
          ),
        ),
      ),
      body: usersAsync.when(
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, _) => _buildErrorState(),
        data: (users) {
          final filteredUsers = _searchQuery.isEmpty
              ? users
              : users
                    .where(
                      (u) =>
                          u.username.toLowerCase().contains(
                            _searchQuery.toLowerCase(),
                          ) ||
                          u.displayName.toLowerCase().contains(
                            _searchQuery.toLowerCase(),
                          ),
                    )
                    .toList();

          if (filteredUsers.isEmpty) {
            return _buildEmptyState();
          }

          return RefreshIndicator(
            onRefresh: () async {
              // Invalidate to force refresh
              if (widget.type == FollowListType.followers) {
                ref.invalidate(followersStreamProvider(widget.userId));
              } else {
                ref.invalidate(followingStreamProvider(widget.userId));
              }
            },
            child: ListView.separated(
              padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
              itemCount: filteredUsers.length,
              separatorBuilder: (_, index) => const SizedBox(height: 8),
              itemBuilder: (_, index) {
                final user = filteredUsers[index];

                return UserCard(
                  user: user,
                  showFollowButton:
                      authState is AuthStateAuthenticated &&
                      authState.user.id != user.id,
                  onTap: () {
                    // Navigate to user profile using NavigationHandler
                    ref
                        .read(navigationHandlerProvider)
                        .navigateToUserProfile(user.id);
                  },
                );
              },
            ),
          );
        },
      ),
    );
  }

  Widget _buildErrorState() {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.error_outline, size: 64, color: Colors.red),
          const SizedBox(height: 16),
          Text(
            'Data belum bisa dimuat.',
            style: Theme.of(context).textTheme.headlineMedium,
          ),
          const SizedBox(height: 8),
          Text(
            'Coba lagi beberapa saat.',
            style: Theme.of(context).textTheme.bodyMedium,
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 24),
          ElevatedButton.icon(
            onPressed: () {
              // Invalidate to retry
              if (widget.type == FollowListType.followers) {
                ref.invalidate(followersStreamProvider(widget.userId));
              } else {
                ref.invalidate(followingStreamProvider(widget.userId));
              }
            },
            icon: const Icon(Icons.refresh),
            label: const Text('Retry'),
          ),
        ],
      ),
    );
  }

  Widget _buildEmptyState() {
    final message = _searchQuery.isNotEmpty
        ? 'No users found'
        : widget.type == FollowListType.followers
        ? 'No followers yet'
        : 'Not following anyone yet';

    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            _searchQuery.isNotEmpty ? Icons.search_off : Icons.people_outline,
            size: 64,
            color: AppColors.neutralGray400,
          ),
          const SizedBox(height: 16),
          Text(
            message,
            style: Theme.of(
              context,
            ).textTheme.bodyLarge?.copyWith(color: Colors.grey),
          ),
        ],
      ),
    );
  }
}

/// Follow List Type
enum FollowListType { followers, following }
