import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/features/search/search/search.dart' show UserSearch;
import 'package:labuda/domains/user/profile/data/profile_providers.dart';
import 'package:labuda/shared/widgets/profile_avatar.dart';

/// Widget untuk menampilkan tagged users sebagai chips
/// Digunakan di CreateContentScreen dan CreateRequestScreen
///
/// **R2.3 MIGRATION:** Now uses UserLookupService from profile domain
/// instead of deprecated UserSearchApiService from shared.
class TaggedUsersChips extends ConsumerStatefulWidget {
  final List<String> taggedUserIds;
  final VoidCallback? onTap;
  final Function(String userId)? onRemove;
  final bool readOnly;

  const TaggedUsersChips({
    super.key,
    required this.taggedUserIds,
    this.onTap,
    this.onRemove,
    this.readOnly = false,
  });

  @override
  ConsumerState<TaggedUsersChips> createState() => _TaggedUsersChipsState();
}

class _TaggedUsersChipsState extends ConsumerState<TaggedUsersChips> {
  List<UserSearch> _users = [];
  bool _isLoading = true;

  @override
  void initState() {
    super.initState();
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    // Load users on first init
    if (_users.isEmpty && _isLoading) {
      _loadUsers();
    }
  }

  @override
  void didUpdateWidget(TaggedUsersChips oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.taggedUserIds != widget.taggedUserIds) {
      _loadUsers();
    }
  }

  Future<void> _loadUsers() async {
    if (widget.taggedUserIds.isEmpty) {
      setState(() {
        _users = [];
        _isLoading = false;
      });
      return;
    }

    setState(() {
      _isLoading = true;
    });

    try {
      // **R2.3 MIGRATION:** Use UserLookupService from profile domain
      // instead of deprecated UserSearchApiService
      final userLookupService = ref.read(userLookupServiceProvider);
      final users = await userLookupService.getUsersByIds(widget.taggedUserIds);

      if (mounted) {
        setState(() {
          _users = users;
          _isLoading = false;
        });
      }
    } catch (e) {
      if (mounted) {
        setState(() {
          _users = [];
          _isLoading = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    if (widget.taggedUserIds.isEmpty) {
      return _buildEmptyState(context);
    }

    if (_isLoading) {
      return const SizedBox(
        height: 40,
        child: Center(
          child: SizedBox(
            width: 20,
            height: 20,
            child: CircularProgressIndicator(strokeWidth: 2),
          ),
        ),
      );
    }

    return Wrap(
      spacing: 8,
      runSpacing: 8,
      children: [
        // Tagged user chips
        ..._users.map((user) => _buildUserChip(context, user)),

        // Add more button (if not read-only)
        if (!widget.readOnly && widget.onTap != null) _buildAddButton(context),
      ],
    );
  }

  Widget _buildEmptyState(BuildContext context) {
    if (widget.readOnly) {
      return const SizedBox.shrink();
    }
    final scheme = Theme.of(context).colorScheme;

    return InkWell(
      onTap: widget.onTap,
      borderRadius: BorderRadius.circular(20),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        decoration: BoxDecoration(
          color: scheme.surfaceContainerHigh,
          borderRadius: BorderRadius.circular(20),
          border: Border.all(
            color: scheme.outlineVariant,
            style: BorderStyle.solid,
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.person_add_outlined,
              size: 18,
              color: scheme.onSurfaceVariant,
            ),
            const SizedBox(width: 6),
            Text(
              'Tag People',
              style: TextStyle(
                fontSize: 14,
                color: scheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildUserChip(BuildContext context, UserSearch user) {
    final scheme = Theme.of(context).colorScheme;
    return Chip(
      avatar: ProfileAvatar(
        userId: user.userId,
        size: 28,
        imageUrl: user.avatarUrl,
        showShadow: false,
      ),
      label: Text(
        user.username,
        style: TextStyle(
          fontSize: 13,
          color: scheme.onSurface,
        ),
      ),
      deleteIcon: widget.readOnly
          ? null
          : Icon(
              Icons.close,
              size: 18,
              color: scheme.onSurfaceVariant,
            ),
      onDeleted: widget.readOnly
          ? null
          : () => widget.onRemove?.call(user.userId),
      backgroundColor: scheme.surfaceContainerHigh,
      side: BorderSide(
        color: scheme.outlineVariant,
      ),
      materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
    );
  }

  Widget _buildAddButton(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return InkWell(
      onTap: widget.onTap,
      borderRadius: BorderRadius.circular(20),
      child: Container(
        height: 32,
        padding: const EdgeInsets.symmetric(horizontal: 12),
        decoration: BoxDecoration(
          color: scheme.surfaceContainerHigh,
          borderRadius: BorderRadius.circular(20),
          border: Border.all(
            color: scheme.outlineVariant,
            style: BorderStyle.solid,
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.add,
              size: 18,
              color: scheme.secondary,
            ),
            const SizedBox(width: 4),
            Text(
              'Add',
              style: TextStyle(
                fontSize: 13,
                color: scheme.secondary,
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
