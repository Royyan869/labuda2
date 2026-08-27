import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/features/search/search/search.dart' show UserSearch;
import 'package:labuda/domains/user/profile/data/profile_providers.dart';
import 'package:labuda/shared/shared.dart';
import 'package:labuda/core/core.dart';

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
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    if (widget.taggedUserIds.isEmpty) {
      return _buildEmptyState(isDark);
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
        ..._users.map((user) => _buildUserChip(user, isDark)),

        // Add more button (if not read-only)
        if (!widget.readOnly && widget.onTap != null) _buildAddButton(isDark),
      ],
    );
  }

  Widget _buildEmptyState(bool isDark) {
    if (widget.readOnly) {
      return const SizedBox.shrink();
    }

    return InkWell(
      onTap: widget.onTap,
      borderRadius: BorderRadius.circular(20),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
        decoration: BoxDecoration(
          color: isDark
              ? AppColors.darkGray700.withValues(alpha: 0.5)
              : AppColors.neutralGray100,
          borderRadius: BorderRadius.circular(20),
          border: Border.all(
            color: isDark ? AppColors.neutralGray700 : AppColors.neutralGray300,
            style: BorderStyle.solid,
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.person_add_outlined,
              size: 18,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
            const SizedBox(width: 6),
            Text(
              'Tag People',
              style: TextStyle(
                fontSize: 14,
                color: isDark
                    ? AppColors.neutralGray400
                    : AppColors.neutralGray600,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildUserChip(UserSearch user, bool isDark) {
    return Chip(
      avatar: CircleAvatar(
        radius: 14,
        backgroundColor: AppColors.primaryBlue.withValues(alpha: 0.1),
        backgroundImage: user.avatarUrl != null
            ? NetworkImage(user.avatarUrl!)
            : null,
        child: user.avatarUrl == null
            ? Text(
                UserInitialsHelper.get(
                  name: user.username,
                  userId: user.userId,
                ),
                style: const TextStyle(
                  fontSize: 12,
                  color: AppColors.primaryBlue,
                  fontWeight: FontWeight.w600,
                ),
              )
            : null,
      ),
      label: Text(
        user.username,
        style: TextStyle(
          fontSize: 13,
          color: isDark ? AppColors.neutralWhite : AppColors.neutralBlack,
        ),
      ),
      deleteIcon: widget.readOnly
          ? null
          : Icon(
              Icons.close,
              size: 18,
              color: isDark
                  ? AppColors.neutralGray400
                  : AppColors.neutralGray600,
            ),
      onDeleted: widget.readOnly
          ? null
          : () => widget.onRemove?.call(user.userId),
      backgroundColor: isDark
          ? AppColors.darkGray700
          : AppColors.neutralGray100,
      side: BorderSide(
        color: isDark ? AppColors.neutralGray700 : AppColors.neutralGray300,
      ),
      materialTapTargetSize: MaterialTapTargetSize.shrinkWrap,
    );
  }

  Widget _buildAddButton(bool isDark) {
    return InkWell(
      onTap: widget.onTap,
      borderRadius: BorderRadius.circular(20),
      child: Container(
        height: 32,
        padding: const EdgeInsets.symmetric(horizontal: 12),
        decoration: BoxDecoration(
          color: isDark
              ? AppColors.darkGray700.withValues(alpha: 0.5)
              : AppColors.neutralGray100,
          borderRadius: BorderRadius.circular(20),
          border: Border.all(
            color: isDark ? AppColors.neutralGray700 : AppColors.neutralGray300,
            style: BorderStyle.solid,
          ),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(
              Icons.add,
              size: 18,
              color: isDark ? AppColors.primaryBlue : AppColors.primaryBlue,
            ),
            const SizedBox(width: 4),
            Text(
              'Add',
              style: TextStyle(
                fontSize: 13,
                color: isDark ? AppColors.primaryBlue : AppColors.primaryBlue,
                fontWeight: FontWeight.w500,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
