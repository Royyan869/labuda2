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
    return AppBottomSheetBase.show<List<String>>(
      context: context,
      title: 'Tag People',
      padding: EdgeInsets.zero,
      content: UserSearchBottomSheet(
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
  // One-shot init: `didChangeDependencies` re-fires on ANY inherited
  // dependency change (keyboard, insets, theme) — assigning a `late final`
  // there threw a LateInitializationError mid-rebuild and froze/broke the
  // body (BOTTOMSHEET-04, geometry-proven). The lazy field initializer
  // runs exactly once, on first use.
  late final SearchApiService _searchService = ref.read(
    searchApiServiceProvider,
  );
  final Set<String> _selectedUserIds = {};

  List<UserSearch> _searchResults = [];
  bool _isSearching = false;
  String _searchQuery = '';

  /// Share of the sheet's available height this body asks for: a tall sheet
  /// that still leaves room for the sheet's own chrome (handle + title) and
  /// never exceeds the sheet's ceiling.
  static const double _bodyShare = 0.75;

  @override
  void initState() {
    super.initState();
    _selectedUserIds.addAll(widget.alreadyTaggedUserIds);
    _searchController.addListener(_onSearchChanged);
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

    // Body height — a share of the space the sheet ACTUALLY has, asked of
    // the sheet authority itself ([AppBottomSheetBase.contentAllocationOf]:
    // the live content region, with the sheet chrome and the system spacer
    // already spent). Denominating the share on the raw CEILING
    // (`availableHeight × share`) re-derived the sheet fit from the wrong
    // budget: the chrome sharing the ceiling pushed this slot past the
    // content region at larger insets (BOTTOMSHEET-04, geometry-proven).
    final modalHeight =
        AppBottomSheetBase.contentAllocationOf(context) * _bodyShare;

    // Surface, shape, handle and scroll come from the base.
    return SizedBox(
      height: modalHeight,
      child: Column(
        children: [
          // Search bar
          _buildSearchBar(context),
          Align(
            alignment: Alignment.centerRight,
            child: Padding(
              padding: const EdgeInsets.symmetric(horizontal: AppMetrics.p8),
              child: _buildDoneAction(context),
            ),
          ),

          // Selected count
          if (_selectedUserIds.isNotEmpty) _buildSelectedCount(context),

          // Divider
          Divider(height: 1, color: scheme.outlineVariant),

          // Results
          Expanded(child: _buildResults(context)),
        ],
      ),
    );
  }

  Widget _buildDoneAction(BuildContext context) {
    return TextButton(
      onPressed: _selectedUserIds.isEmpty ? null : _done,
      child: Text(
        'Done',
        style: Theme.of(
          context,
        ).textTheme.labelLarge?.copyWith(fontWeight: FontWeight.w600),
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
        decoration:
            AppTheme.searchDecoration(
              scheme,
              hintText: 'Search username...',
            ).copyWith(
              prefixIcon: Icon(Icons.search, color: scheme.onSurfaceVariant),
              suffixIcon: _searchController.text.isNotEmpty
                  ? IconButton(
                      icon: Icon(
                        Icons.clear,
                        color: scheme.onSurfaceVariant,
                        semanticLabel: 'Bersihkan',
                      ),
                      onPressed: () {
                        _searchController.clear();
                      },
                    )
                  : null,
            ),
      ),
    );
  }

  Widget _buildSelectedCount(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Padding(
      padding: const EdgeInsets.symmetric(
        horizontal: AppMetrics.p16,
        vertical: AppMetrics.p8,
      ),
      child: Text(
        '${_selectedUserIds.length} / ${widget.maxSelections} selected',
        style: context.typeRoles.labelMicro.copyWith(
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
              size: AppIconSize.display,
              color: scheme.outline,
            ),
            const SizedBox(height: 16),
            Text(
              'Search for users to tag',
              style: context.typeRoles.titleProminent.copyWith(
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
              size: AppIconSize.display,
              color: scheme.outline,
            ),
            const SizedBox(height: 16),
            Text(
              'No users found',
              style: context.typeRoles.titleProminent.copyWith(
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

  Widget _buildUserTile(
    BuildContext context,
    UserSearch user,
    bool isSelected,
  ) {
    final scheme = Theme.of(context).colorScheme;
    return ListTile(
      leading: ProfileAvatar(
        userId: user.userId,
        size: 48,
        imageUrl: user.avatarUrl,
      ),
      title: Text(
        user.username,
        style: TextStyle(fontWeight: FontWeight.w600, color: scheme.onSurface),
        overflow: TextOverflow.ellipsis,
      ),
      subtitle: Text(
        '@${user.username}',
        style: TextStyle(color: scheme.onSurfaceVariant),
      ),
      trailing: isSelected
          ? Icon(Icons.check_circle, color: scheme.secondary)
          : Icon(Icons.circle_outlined, color: scheme.outlineVariant),
      onTap: () => _toggleUser(user),
    );
  }
}
