import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:hishumi/features/search/search/domain/entities/search_result.dart';
import 'package:hishumi/core/src/theme/app_theme.dart';

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
    final scheme = Theme.of(context).colorScheme;

    return TextField(
      controller: _controller,
      focusNode: _focusNode,
      autofocus: widget.autofocus,
      onTap: widget.onTap,
      onSubmitted: (_) => _onSubmit(),
      textInputAction: TextInputAction.search,
      decoration: AppTheme.searchDecoration(
        scheme,
        hintText: 'Cari koleksi, lelang, kontes...',
      ).copyWith(
        prefixIcon: Icon(Icons.search, color: scheme.onSurfaceVariant),
        suffixIcon: _controller.text.isNotEmpty
            ? IconButton(
                icon: Icon(
                  Icons.clear,
                  color: scheme.onSurfaceVariant,
                  semanticLabel: 'Bersihkan',
                ),
                onPressed: _onClear,
              )
            : null,
      ),
    );
  }

  Widget _buildCategoryChips(BuildContext context) {
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Row(
        children: [
          _buildChip(context, null, 'Semua'),
          const SizedBox(width: 8),
          _buildChip(context, SearchResultType.forSale, 'For Sale'),
          const SizedBox(width: 8),
          _buildChip(context, SearchResultType.auction, 'Lelang'),
          const SizedBox(width: 8),
          _buildChip(context, SearchResultType.user, 'User'),
          const SizedBox(width: 8),
          _buildChip(context, SearchResultType.content, 'Content'),
        ],
      ),
    );
  }

  Widget _buildChip(
    BuildContext context,
    SearchResultType? type,
    String label,
  ) {
    final scheme = Theme.of(context).colorScheme;
    final isSelected = _selectedType == type;

    return FilterChip(
      label: Text(label),
      selected: isSelected,
      onSelected: (selected) {
        setState(() {
          _selectedType = selected ? type : null;
        });
      },
      backgroundColor: scheme.surfaceContainerHigh,
      selectedColor: scheme.primary.withValues(alpha: 0.2),
      labelStyle: TextStyle(
        color: isSelected
            ? scheme.primary
            : scheme.onSurfaceVariant,
        fontWeight: isSelected ? FontWeight.w600 : FontWeight.normal,
      ),
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(AppShape.r20),
        side: BorderSide(
          color: isSelected ? scheme.primary : Colors.transparent,
        ),
      ),
    );
  }
}
