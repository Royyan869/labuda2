import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/features/search/search/domain/entities/search_result.dart';

/// Global search bar widget for unified search
class GlobalSearchBar extends ConsumerStatefulWidget {
  final String? initialQuery;
  final SearchResultType? initialType;
  final ValueChanged<String>? onSearch;
  final ValueChanged<String>? onQueryChanged;
  final VoidCallback? onTap;
  final bool autofocus;
  final bool showCategoryChips;

  const GlobalSearchBar({
    super.key,
    this.initialQuery,
    this.initialType,
    this.onSearch,
    this.onQueryChanged,
    this.onTap,
    this.autofocus = false,
    this.showCategoryChips = true,
  });

  @override
  ConsumerState<GlobalSearchBar> createState() => _GlobalSearchBarState();
}

class _GlobalSearchBarState extends ConsumerState<GlobalSearchBar> {
  late TextEditingController _controller;
  late FocusNode _focusNode;
  SearchResultType? _selectedType;

  @override
  void initState() {
    super.initState();
    _controller = TextEditingController(text: widget.initialQuery);
    _focusNode = FocusNode();
    _selectedType = widget.initialType;

    _controller.addListener(_onTextChanged);
  }

  @override
  void dispose() {
    _controller.removeListener(_onTextChanged);
    _controller.dispose();
    _focusNode.dispose();
    super.dispose();
  }

  void _onTextChanged() {
    widget.onQueryChanged?.call(_controller.text);
  }

  void _onSubmit() {
    if (_controller.text.trim().isNotEmpty) {
      widget.onSearch?.call(_controller.text.trim());
    }
  }

  void _onClear() {
    _controller.clear();
    widget.onQueryChanged?.call('');
  }

  @override
  Widget build(BuildContext context) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        _buildSearchField(context),
        if (widget.showCategoryChips) ...[
          const SizedBox(height: 12),
          _buildCategoryChips(context),
        ],
      ],
    );
  }

  Widget _buildSearchField(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return TextField(
      controller: _controller,
      focusNode: _focusNode,
      autofocus: widget.autofocus,
      onTap: widget.onTap,
      onSubmitted: (_) => _onSubmit(),
      textInputAction: TextInputAction.search,
      decoration: InputDecoration(
        hintText: 'Cari koleksi, lelang, kontes...',
        prefixIcon: Icon(
          Icons.search,
          color: isDark ? AppColors.neutralGray400 : AppColors.neutralGray500,
        ),
        suffixIcon: _controller.text.isNotEmpty
            ? IconButton(
                icon: Icon(
                  Icons.clear,
                  color: isDark
                      ? AppColors.neutralGray400
                      : AppColors.neutralGray500,
                ),
                onPressed: _onClear,
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
    );
  }

  Widget _buildCategoryChips(BuildContext context) {
    final isDark = Theme.of(context).brightness == Brightness.dark;

    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Row(
        children: [
          _buildChip(context, null, 'Semua', isDark),
          const SizedBox(width: 8),
          _buildChip(context, SearchResultType.listing, 'Listing', isDark),
          const SizedBox(width: 8),
          _buildChip(context, SearchResultType.auction, 'Lelang', isDark),
          const SizedBox(width: 8),
          _buildChip(context, SearchResultType.user, 'User', isDark),
          const SizedBox(width: 8),
          _buildChip(context, SearchResultType.content, 'Content', isDark),
        ],
      ),
    );
  }

  Widget _buildChip(
    BuildContext context,
    SearchResultType? type,
    String label,
    bool isDark,
  ) {
    final isSelected = _selectedType == type;

    return FilterChip(
      label: Text(label),
      selected: isSelected,
      onSelected: (selected) {
        setState(() {
          _selectedType = selected ? type : null;
        });
      },
      backgroundColor: isDark
          ? AppColors.darkGray700
          : AppColors.neutralGray100,
      selectedColor: AppColors.primary.withValues(alpha: 0.2),
      labelStyle: TextStyle(
        color: isSelected
            ? AppColors.primary
            : (isDark ? AppColors.neutralGray300 : AppColors.neutralGray600),
        fontWeight: isSelected ? FontWeight.w600 : FontWeight.normal,
      ),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(20),
        side: BorderSide(
          color: isSelected ? AppColors.primary : Colors.transparent,
        ),
      ),
    );
  }
}
