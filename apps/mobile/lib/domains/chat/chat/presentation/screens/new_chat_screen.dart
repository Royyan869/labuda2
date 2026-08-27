import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/domains/user/profile/profile.dart';
import 'package:labuda/domains/chat/chat/chat.dart';
import '../widgets/new_chat_user_list_widget.dart';

/// Screen untuk memilih user untuk memulai chat baru
class NewChatScreen extends ConsumerStatefulWidget {
  const NewChatScreen({super.key});

  @override
  ConsumerState<NewChatScreen> createState() => _NewChatScreenState();
}

class _NewChatScreenState extends ConsumerState<NewChatScreen> {
  final TextEditingController _searchController = TextEditingController();
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

    if (authState is! AuthStateAuthenticated) {
      return _buildUnauthorizedScreen();
    }

    final currentUserId = authState.user.id;

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) {
        if (didPop) return;
        if (context.canPop()) {
          context.pop();
        }
      },
      child: Scaffold(
        appBar: _buildAppBar(),
        body: SafeArea(
          child: Column(
            children: [
              _buildSearchBar(isDark),
              Expanded(
                child: _searchQuery.isEmpty
                    ? _buildEmptySearchState(isDark)
                    : _buildSearchResults(currentUserId, isDark),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildUnauthorizedScreen() {
    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) {
        if (didPop) return;
        if (context.canPop()) {
          context.pop();
        }
      },
      child: Scaffold(
        appBar: _buildAppBar(),
        body: const Center(child: Text('Please log in first')),
      ),
    );
  }

  PreferredSizeWidget _buildAppBar() {
    return const AppBarCustom(title: 'Select Contact');
  }

  Widget _buildSearchBar(bool isDark) {
    return Padding(
      padding: const EdgeInsets.all(16),
      child: TextField(
        controller: _searchController,
        onChanged: (value) {
          setState(() {
            _searchQuery = value.trim();
          });
        },
        decoration: InputDecoration(
          hintText: 'Search name or username...',
          prefixIcon: const Icon(Icons.search),
          suffixIcon: _searchQuery.isNotEmpty
              ? IconButton(
                  icon: const Icon(Icons.clear),
                  onPressed: () {
                    _searchController.clear();
                    setState(() {
                      _searchQuery = '';
                    });
                  },
                )
              : null,
          filled: true,
          fillColor: isDark ? AppColors.darkGray700 : AppColors.neutralGray100,
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(12),
            borderSide: BorderSide.none,
          ),
          contentPadding: const EdgeInsets.symmetric(
            horizontal: 16,
            vertical: 12,
          ),
        ),
      ),
    );
  }

  Widget _buildEmptySearchState(bool isDark) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.search,
            size: 64,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
          ),
          const SizedBox(height: 16),
          Text(
            'Search user to start a chat',
            style: TextStyle(
              fontSize: 16,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Type a name or username',
            style: TextStyle(
              fontSize: 14,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSearchResults(String currentUserId, bool isDark) {
    final searchAsync = ref.watch(searchProfilesProvider(_searchQuery));

    return searchAsync.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, stack) => _buildErrorState(error, isDark),
      data: (profiles) {
        final filteredProfiles = profiles
            .where((profile) => profile.userId != currentUserId)
            .toList();

        if (filteredProfiles.isEmpty) {
          return _buildNoResultsState(isDark);
        }

        return ListView.builder(
          padding: const EdgeInsets.only(bottom: 16),
          itemCount: filteredProfiles.length,
          itemBuilder: (context, index) {
            final profile = filteredProfiles[index];
            return NewChatUserListWidget(
              profile: profile,
              currentUserId: currentUserId,
              isDark: isDark,
            );
          },
        );
      },
    );
  }

  Widget _buildErrorState(Object error, bool isDark) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.error_outline, size: 64, color: AppColors.error),
          const SizedBox(height: 16),
          Text(
            'Failed to search users',
            style: TextStyle(
              fontSize: 16,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray700,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            error.toString(),
            style: TextStyle(
              fontSize: 12,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
            textAlign: TextAlign.center,
          ),
        ],
      ),
    );
  }

  Widget _buildNoResultsState(bool isDark) {
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.search_off,
            size: 64,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
          ),
          const SizedBox(height: 16),
          Text(
            'User not found',
            style: TextStyle(
              fontSize: 16,
              color: isDark
                  ? AppColors.neutralGray300
                  : AppColors.neutralGray600,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Try a different keyword',
            style: TextStyle(
              fontSize: 14,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray500,
            ),
          ),
        ],
      ),
    );
  }
}
