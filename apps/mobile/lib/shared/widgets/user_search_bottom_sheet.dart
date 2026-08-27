import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/features/search/search/search.dart'; // R3.1: Full import for providers and extensions
import 'package:labuda/features/search/search/data/dto/search_dto.dart'; // R3.1: Import for UserSearchResultDto.toUserSearch() extension
import 'package:labuda/shared/shared.dart';
import 'package:labuda/core/core.dart';

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
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;
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
        color: isDark ? AppColors.darkGray800 : AppColors.neutralWhite,
        borderRadius: const BorderRadius.vertical(top: Radius.circular(16)),
      ),
      child: Column(
        children: [
          // Header
          _buildHeader(isDark),

          // Search bar
          _buildSearchBar(isDark),

          // Selected count
          if (_selectedUserIds.isNotEmpty) _buildSelectedCount(isDark),

          // Divider
          Divider(
            height: 1,
            color: isDark ? AppColors.neutralGray700 : AppColors.neutralGray200,
          ),

          // Results
          Expanded(child: _buildResults(isDark)),
        ],
      ),
    );
  }

  Widget _buildHeader(bool isDark) {
    return Padding(
      padding: const EdgeInsets.all(16),
      child: Row(
        children: [
          Expanded(
            child: Text(
              'Tag People',
              style: TextStyle(
                fontSize: 18,
                fontWeight: FontWeight.w600,
                color: isDark ? AppColors.neutralWhite : AppColors.neutralBlack,
              ),
            ),
          ),
          TextButton(
            onPressed: _selectedUserIds.isEmpty ? null : _done,
            child: Text(
              'Done',
              style: TextStyle(
                fontSize: 16,
                fontWeight: FontWeight.w600,
                color: _selectedUserIds.isEmpty
                    ? AppColors.neutralGray400
                    : AppColors.primaryBlue,
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSearchBar(bool isDark) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16),
      child: TextField(
        controller: _searchController,
        autofocus: true,
        decoration: InputDecoration(
          hintText: 'Search username...',
          prefixIcon: Icon(
            Icons.search,
            color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
          ),
          suffixIcon: _searchController.text.isNotEmpty
              ? IconButton(
                  icon: Icon(
                    Icons.clear,
                    color: isDark
                        ? AppColors.neutralGray400
                        : AppColors.neutralGray600,
                  ),
                  onPressed: () {
                    _searchController.clear();
                  },
                )
              : null,
          filled: true,
          fillColor: isDark ? AppColors.darkGray700 : AppColors.neutralGray100,
          border: OutlineInputBorder(
            borderRadius: BorderRadius.circular(8),
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

  Widget _buildSelectedCount(bool isDark) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
      child: Text(
        '${_selectedUserIds.length} / ${widget.maxSelections} selected',
        style: TextStyle(
          fontSize: 12,
          color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
        ),
      ),
    );
  }

  Widget _buildResults(bool isDark) {
    // Empty state - no search
    if (_searchQuery.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              Icons.person_search,
              size: 64,
              color: isDark
                  ? AppColors.neutralGray600
                  : AppColors.neutralGray300,
            ),
            const SizedBox(height: 16),
            Text(
              'Search for users to tag',
              style: TextStyle(
                fontSize: 16,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
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
              color: isDark
                  ? AppColors.neutralGray600
                  : AppColors.neutralGray300,
            ),
            const SizedBox(height: 16),
            Text(
              'No users found',
              style: TextStyle(
                fontSize: 16,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
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

        return _buildUserTile(user, isSelected, isDark);
      },
    );
  }

  Widget _buildUserTile(UserSearch user, bool isSelected, bool isDark) {
    return ListTile(
      leading: CircleAvatar(
        radius: 24,
        backgroundColor: AppColors.primaryBlue.withValues(alpha: 0.1),
        backgroundImage: user.avatarUrl != null
            ? NetworkImage(user.avatarUrl!)
            : null,
        child: user.avatarUrl == null
            ? Text(
                UserInitialsHelper.get(userId: user.userId),
                style: const TextStyle(
                  color: AppColors.primaryBlue,
                  fontWeight: FontWeight.w600,
                ),
              )
            : null,
      ),
      title: Text(
        user.username,
        style: TextStyle(
          fontWeight: FontWeight.w600,
          color: isDark ? AppColors.neutralWhite : AppColors.neutralBlack,
        ),
        overflow: TextOverflow.ellipsis,
      ),
      subtitle: Text(
        '@${user.username}',
        style: TextStyle(
          color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray600,
        ),
      ),
      trailing: isSelected
          ? const Icon(Icons.check_circle, color: AppColors.primaryBlue)
          : Icon(
              Icons.circle_outlined,
              color: isDark
                  ? AppColors.neutralGray600
                  : AppColors.neutralGray300,
            ),
      onTap: () => _toggleUser(user),
    );
  }
}
