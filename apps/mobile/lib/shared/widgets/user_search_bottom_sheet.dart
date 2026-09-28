import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/features/search/search/search.dart'; // R3.1: Full import for providers and extensions
import 'package:labuda/features/search/search/data/dto/search_dto.dart'; // R3.1: Import for UserSearchResultDto.toUserSearch() extension
import 'package:labuda/shared/shared.dart';
import 'package:labuda/core/src/theme/app_theme.dart';

/// Bottom sheet untuk search dan select users (Instagram style)
/// Digunakan untuk tag people di create post/request
///
/// **R2.2 MIGRATED**: Now uses SearchApiService from search domain
/// instead of leaked UserSearchApiService from shared.
class UserSearchBottomSheet extends ConsumerStatefulWidget {
  final List<String> alreadyTaggedUserIds;
  final int maxSelections;

  const UserSearchBottomSheet({
    super.key,
    this.alreadyTaggedUserIds = const [],
    this.maxSelections = 50,
  });

  /// Show bottom sheet dan return selected user IDs
  static Future<List<String>?> show({
    required BuildContext context,
    List<String> alreadyTaggedUserIds = const [],
    int maxSelections = 50,
  }) async {
    return showModalBottomSheet<List<String>>(
      context: context,
      isScrollControlled: true,
      backgroundColor: Colors.transparent,
      builder: (context) => UserSearchBottomSheet(
        alreadyTaggedUserIds: alreadyTaggedUserIds,
        maxSelections: maxSelections,
      ),
    );
  }

  @override
  ConsumerState<UserSearchBottomSheet> createState() =>
      _UserSearchBottomSheetState();
}

class _UserSearchBottomSheetState extends ConsumerState<UserSearchBottomSheet> {
  final TextEditingController _searchController = TextEditingController();
  late final SearchApiService _searchService;
  final Set<String> _selectedUserIds = {};

  List<UserSearch> _searchResults = [];
  bool _isSearching = false;
  String _searchQuery = '';

  @override
  void initState() {
    super.initState();
    _selectedUserIds.addAll(widget.alreadyTaggedUserIds);
    _searchController.addListener(_onSearchChanged);
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    // Initialize service with search domain provider
    _searchService = ref.read(searchApiServiceProvider);
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  void _onSearchChanged() {
    final query = _searchController.text.trim();
    if (query == _searchQuery) return;

    setState(() {
      _searchQuery = query;
    });

    if (query.isEmpty) {
      setState(() {
        _searchResults = [];
        _isSearching = false;
      });
      return;
    }

    _performSearch(query);
  }

  Future<void> _performSearch(String query) async {
    setState(() {
      _isSearching = true;
    });

    try {
      final response = await _searchService.searchUsers(
        query: query,
        limit: 20,
      );

      // Map DTOs to UserSearch entities
      final results = response.users.map((dto) => dto.toUserSearch()).toList();

      // Only update if query hasn't changed
      if (query == _searchQuery && mounted) {
        setState(() {
          _searchResults = results;
          _isSearching = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _searchResults = [];
          _isSearching = false;
        });
      }
    }
  }

  void _toggleUser(UserSearch user) {
    setState(() {
      if (_selectedUserIds.contains(user.userId)) {
        _selectedUserIds.remove(user.userId);
      } else {
        if (_selectedUserIds.length >= widget.maxSelections) {
          AppSnackBar.showWarning(
            context,
            'Maksimal ${widget.maxSelections} user yang bisa di-tag',
          );
          return;
        }
        _selectedUserIds.add(user.userId);
      }
    });
  }

  void _done() {
    Navigator.pop(context, _selectedUserIds.toList());
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final mediaQuery = MediaQuery.of(context);
    final keyboardHeight = mediaQuery.viewInsets.bottom;

    // Calculate height: when keyboard is visible, use more space
    final modalHeight = keyboardHeight > 0
        ? mediaQuery.size.height *
              0.9 // 90% when keyboard active
        : mediaQuery.size.height * 0.75; // 75% when keyboard collapsed

    return Container(
      height: modalHeight,
      decoration: BoxDecoration(
        color: scheme.surfaceContainerLow,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(AppShape.r16)),
      ),
      child: Column(
        children: [
          // Header
          _buildHeader(context),

          // Search bar
          _buildSearchBar(context),

          // Selected count
          if (_selectedUserIds.isNotEmpty) _buildSelectedCount(context),

          // Divider
          Divider(
            height: 1,
            color: scheme.outlineVariant,
          ),

          // Results
          Expanded(child: _buildResults(context)),
        ],
      ),
    );
  }

  Widget _buildHeader(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.all(AppMetrics.p16),
      child: Row(
        children: [
          Expanded(
            child: Text(
              'Tag People',
              style: TextStyle(
                fontSize: AppType.s18,
                fontWeight: FontWeight.w600,
                color: scheme.onSurface,
              ),
            ),
          ),
          TextButton(
            onPressed: _selectedUserIds.isEmpty ? null : _done,
            child: Text(
              'Done',
              style: TextStyle(
                fontSize: AppType.s16,
                fontWeight: FontWeight.w600,
                color: _selectedUserIds.isEmpty
                    ? scheme.onSurfaceVariant
                    : scheme.secondary,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSearchBar(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16),
      child: TextField(
        controller: _searchController,
        autofocus: true,
        decoration: InputDecoration(
          hintText: 'Search username...',
          prefixIcon: Icon(
            Icons.search,
            color: scheme.onSurfaceVariant,
          ),
          suffixIcon: _searchController.text.isNotEmpty
              ? IconButton(
                  icon: Icon(
                    Icons.clear,
                    color: scheme.onSurfaceVariant,
                  ),
                  onPressed: () {
                    _searchController.clear();
                  },
                )
              : null,
          filled: true,
          fillColor: scheme.surfaceContainerHigh,
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(AppShape.r8),
            borderSide: BorderSide.none,
          ),
          contentPadding: const EdgeInsets.symmetric(
            horizontal: AppMetrics.p16,
            vertical: AppMetrics.p12,
          ),
        ),
      ),
    );
  }

  Widget _buildSelectedCount(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p16, vertical: AppMetrics.p8),
      child: Text(
        '${_selectedUserIds.length} / ${widget.maxSelections} selected',
        style: TextStyle(
          fontSize: AppType.s12,
          color: scheme.onSurfaceVariant,
        ),
      ),
    );
  }

  Widget _buildResults(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    // Empty state - no search
    if (_searchQuery.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.person_search,
              size: 64,
              color: scheme.outline,
            ),
            const SizedBox(height: 16),
            Text(
              'Search for users to tag',
              style: TextStyle(
                fontSize: AppType.s16,
                color: scheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
      );
    }

    // Loading
    if (_isSearching) {
      return const Center(child: CircularProgressIndicator());
    }

    // No results
    if (_searchResults.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.person_off_outlined,
              size: 64,
              color: scheme.outline,
            ),
            const SizedBox(height: 16),
            Text(
              'No users found',
              style: TextStyle(
                fontSize: AppType.s16,
                color: scheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
      );
    }

    // Results list
    return ListView.builder(
      itemCount: _searchResults.length,
      itemBuilder: (context, index) {
        final user = _searchResults[index];
        final isSelected = _selectedUserIds.contains(user.userId);

        return _buildUserTile(context, user, isSelected);
      },
    );
  }

  Widget _buildUserTile(BuildContext context, UserSearch user, bool isSelected) {
    final scheme = Theme.of(context).colorScheme;
    return ListTile(
      leading: ProfileAvatar(
        userId: user.userId,
        size: 48,
        imageUrl: user.avatarUrl,
      ),
      title: Text(
        user.username,
        style: TextStyle(
          fontWeight: FontWeight.w600,
          color: scheme.onSurface,
        ),
        overflow: TextOverflow.ellipsis,
      ),
      subtitle: Text(
        '@${user.username}',
        style: TextStyle(
          color: scheme.onSurfaceVariant,
        ),
      ),
      trailing: isSelected
          ? Icon(Icons.check_circle, color: scheme.secondary)
          : Icon(
              Icons.circle_outlined,
              color: scheme.outlineVariant,
            ),
      onTap: () => _toggleUser(user),
    );
  }
}
