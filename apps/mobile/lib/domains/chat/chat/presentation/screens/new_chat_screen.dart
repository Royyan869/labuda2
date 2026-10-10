import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:hishumi/core/core.dart';
import 'package:hishumi/shared/shared.dart';
import 'package:hishumi/features/search/search/domain/entities/user_search.dart';
import 'package:hishumi/domains/chat/chat/chat.dart';
import '../providers/new_chat_user_search_provider.dart';
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
    final authState = ref.watch(authControllerProvider);

    if (authState is! AuthStateAuthenticated) {
      return _buildUnauthorizedScreen();
    }

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
              _buildSearchBar(context),
              Expanded(
                child: _searchQuery.isEmpty
                    ? _buildEmptySearchState(context)
                    : _buildSearchResults(context),
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

  Widget _buildSearchBar(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.all(AppMetrics.p16),
      child: TextField(
        controller: _searchController,
        onChanged: (value) {
          setState(() {
            _searchQuery = value.trim();
          });
        },
        decoration: AppTheme.searchDecoration(
          scheme,
          hintText: 'Search name or username...',
        ).copyWith(
          prefixIcon: const Icon(Icons.search),
          suffixIcon: _searchQuery.isNotEmpty
              ? IconButton(
                  icon: const Icon(Icons.clear, semanticLabel: 'Bersihkan'),
                  onPressed: () {
                    _searchController.clear();
                    setState(() {
                      _searchQuery = '';
                    });
                  },
                )
              : null,
        ),
      ),
    );
  }

  Widget _buildEmptySearchState(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.search,
            size: AppIconSize.display,
            color: scheme.onSurfaceVariant,
          ),
          const SizedBox(height: 16),
          Text(
            'Search user to start a chat',
            style: context.typeRoles.titleProminent.copyWith(
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Type a name or username',
            style: context.typeRoles.bodyDense.copyWith(
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSearchResults(BuildContext context) {
    // Canonical search authority: the search domain's UserSearch projection
    // via /search/users. Self-exclusion is delegated to the provider (live
    // principal) — no client-side filter here.
    final searchAsync = ref.watch(newChatUserSearchProvider(_searchQuery));

    return searchAsync.when(
      loading: () => const Center(child: CircularProgressIndicator()),
      error: (error, stack) => _buildErrorState(context, error),
      data: (List<UserSearch> users) {
        if (users.isEmpty) {
          return _buildNoResultsState(context);
        }

        return ListView.builder(
          padding: const EdgeInsets.only(bottom: AppMetrics.p16),
          itemCount: users.length,
          itemBuilder: (context, index) {
            final user = users[index];
            return NewChatUserListWidget(
              user: user,
            );
          },
        );
      },
    );
  }

  Widget _buildErrorState(BuildContext context, Object error) {
    final scheme = Theme.of(context).colorScheme;
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(Icons.error_outline, size: AppIconSize.display, color: scheme.error),
          const SizedBox(height: 16),
          Text(
            'Failed to search users',
            style: context.typeRoles.titleProminent.copyWith(
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            error.toString(),
            style: context.typeRoles.bodyDense.copyWith(
              color: scheme.onSurfaceVariant,
            ),
            textAlign: TextAlign.center,
          ),
        ],
      ),
    );
  }

  Widget _buildNoResultsState(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Center(
      child: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        children: [
          Icon(
            Icons.search_off,
            size: AppIconSize.display,
            color: scheme.onSurfaceVariant,
          ),
          const SizedBox(height: 16),
          Text(
            'User not found',
            style: context.typeRoles.titleProminent.copyWith(
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 8),
          Text(
            'Try a different keyword',
            style: context.typeRoles.bodyDense.copyWith(
              color: scheme.onSurfaceVariant,
            ),
          ),
        ],
      ),
    );
  }
}
