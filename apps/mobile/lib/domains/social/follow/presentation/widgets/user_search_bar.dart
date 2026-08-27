import 'package:flutter/material.dart';
import 'package:labuda/domains/social/follow/domain/entities/follow_entity.dart';

class UserSearchBar extends StatelessWidget {
  final String query;
  final UserType? selectedFilter;
  final ValueChanged<String> onQueryChanged;
  final ValueChanged<UserType?> onFilterChanged;
  final VoidCallback? onClear;
  final bool isSearching;

  const UserSearchBar({
    super.key,
    required this.query,
    this.selectedFilter,
    required this.onQueryChanged,
    required this.onFilterChanged,
    this.onClear,
    this.isSearching = false,
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        // Search field
        Container(
          margin: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: colorScheme.surfaceContainerHighest,
            borderRadius: BorderRadius.circular(28),
            border: Border.all(
              color: colorScheme.outline.withValues(alpha: 0.3),
            ),
          ),
          child: Row(
            children: [
              const SizedBox(width: 16),
              Icon(Icons.search, color: colorScheme.onSurfaceVariant, size: 20),
              const SizedBox(width: 12),
              Expanded(
                child: TextField(
                  onChanged: onQueryChanged,
                  decoration: InputDecoration(
                    hintText: 'Cari pengguna...',
                    hintStyle: theme.textTheme.bodyMedium?.copyWith(
                      color: colorScheme.onSurfaceVariant,
                    ),
                    border: InputBorder.none,
                    contentPadding: const EdgeInsets.symmetric(vertical: 14),
                  ),
                  style: theme.textTheme.bodyMedium,
                ),
              ),
              if (isSearching)
                Padding(
                  padding: const EdgeInsets.only(right: 16),
                  child: SizedBox(
                    width: 16,
                    height: 16,
                    child: CircularProgressIndicator(
                      strokeWidth: 2,
                      color: colorScheme.primary,
                    ),
                  ),
                )
              else if (query.isNotEmpty && onClear != null)
                IconButton(
                  onPressed: onClear,
                  icon: Icon(
                    Icons.clear,
                    color: colorScheme.onSurfaceVariant,
                    size: 20,
                  ),
                ),
            ],
          ),
        ),

        // Filter chips
        SingleChildScrollView(
          scrollDirection: Axis.horizontal,
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: Row(
            children: [
              // All users
              _buildFilterChip(
                context,
                label: 'Semua',
                isSelected: selectedFilter == null,
                onTap: () => onFilterChanged(null),
              ),
              const SizedBox(width: 8),

              // User types
              ...UserType.values.map(
                (type) => Padding(
                  padding: const EdgeInsets.only(right: 8),
                  child: _buildFilterChip(
                    context,
                    label: _getUserTypeLabel(type),
                    isSelected: selectedFilter == type,
                    onTap: () =>
                        onFilterChanged(selectedFilter == type ? null : type),
                  ),
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }

  Widget _buildFilterChip(
    BuildContext context, {
    required String label,
    required bool isSelected,
    required VoidCallback onTap,
  }) {
    final theme = Theme.of(context);
    final colorScheme = theme.colorScheme;

    return FilterChip(
      label: Text(label),
      selected: isSelected,
      onSelected: (_) => onTap(),
      backgroundColor: colorScheme.surface,
      selectedColor: colorScheme.primary.withValues(alpha: 0.2),
      checkmarkColor: colorScheme.primary,
      labelStyle: theme.textTheme.labelMedium?.copyWith(
        color: isSelected ? colorScheme.primary : colorScheme.onSurface,
        fontWeight: isSelected ? FontWeight.w600 : FontWeight.normal,
      ),
      side: BorderSide(
        color: isSelected
            ? colorScheme.primary
            : colorScheme.outline.withValues(alpha: 0.3),
      ),
    );
  }

  String _getUserTypeLabel(UserType type) {
    switch (type) {
      case UserType.buyer:
        return 'Buyer';
      case UserType.seller:
        return 'Seller';
      case UserType.breeder:
        return 'Breeder';
      case UserType.enthusiast:
        return 'Enthusiast';
      case UserType.judge:
        return 'Judge';
    }
  }
}
