/// Village Search Dropdown Widget
///
/// Searchable dropdown untuk desa/kelurahan dengan autocomplete functionality
library;

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:labuda/core/core.dart';
import 'package:labuda/shared/shared.dart';

class VillageSearchDropdown extends ConsumerStatefulWidget {
  final Village? selectedVillage;
  final District? selectedDistrict;
  final ValueChanged<Village?> onChanged;
  final String? labelText;
  final String? hintText;
  final IconData? prefixIcon;
  final String? Function(Village?)? validator;

  const VillageSearchDropdown({
    super.key,
    required this.selectedVillage,
    required this.selectedDistrict,
    required this.onChanged,
    this.labelText,
    this.hintText,
    this.prefixIcon,
    this.validator,
  });

  @override
  ConsumerState<VillageSearchDropdown> createState() =>
      _VillageSearchDropdownState();
}

class _VillageSearchDropdownState extends ConsumerState<VillageSearchDropdown> {
  final TextEditingController _searchController = TextEditingController();
  bool _isDropdownOpen = false;
  List<Village> _filteredVillages = [];

  @override
  void initState() {
    super.initState();
    if (widget.selectedVillage != null) {
      _searchController.text = widget.selectedVillage!.name;
    }
  }

  @override
  void didUpdateWidget(VillageSearchDropdown oldWidget) {
    super.didUpdateWidget(oldWidget);

    // Reset ketika district berubah
    if (oldWidget.selectedDistrict?.id != widget.selectedDistrict?.id) {
      _searchController.clear();
      setState(() {
        _isDropdownOpen = false;
        _filteredVillages = [];
      });
      widget.onChanged(null);
    }

    // Update text ketika selectedVillage berubah dari luar
    if (oldWidget.selectedVillage?.id != widget.selectedVillage?.id) {
      _searchController.text = widget.selectedVillage?.name ?? '';
    }
  }

  @override
  void dispose() {
    _searchController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final villagesAsync = ref.watch(
      villagesProvider(widget.selectedDistrict?.id),
    );

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        if (widget.labelText != null) ...[
          Text(
            widget.labelText!,
            style: TextStyle(
              fontSize: AppType.s14,
              fontWeight: FontWeight.w500,
              color: scheme.onSurface,
            ),
          ),
          const SizedBox(height: 8),
        ],
        widget.selectedDistrict == null
            ? _buildDisabledField(context)
            : villagesAsync.when(
                data: (villages) =>
                    _buildSearchableDropdown(context, villages),
                loading: () => _buildLoadingField(context),
                error: (error, stack) => _buildErrorField(context),
              ),
      ],
    );
  }

  Widget _buildDisabledField(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return DropdownStateBuilders.fieldRow(
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: scheme.outlineVariant),
        color: scheme.surfaceContainerHighest,
      ),
      children: [
        if (widget.prefixIcon != null) ...[
          Icon(widget.prefixIcon, color: scheme.onSurfaceVariant),
          const SizedBox(width: AppMetrics.p12),
        ],
        Text(
          'Pilih kecamatan dulu',
          style: TextStyle(
            color: scheme.onSurfaceVariant,
            fontSize: AppType.s16,
          ),
        ),
      ],
    );
  }

  Widget _buildLoadingField(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return DropdownStateBuilders.fieldRow(
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: scheme.outlineVariant),
        color: scheme.surface,
      ),
      children: [
        if (widget.prefixIcon != null) ...[
          Icon(widget.prefixIcon, color: scheme.onSurfaceVariant),
          const SizedBox(width: AppMetrics.p12),
        ],
        const SizedBox(
          width: AppIconSize.action,
          height: AppIconSize.action,
          child: CircularProgressIndicator(strokeWidth: 2),
        ),
        const SizedBox(width: AppMetrics.p12),
        Text(
          'Loading desa...',
          style: TextStyle(color: scheme.onSurfaceVariant),
        ),
      ],
    );
  }

  Widget _buildErrorField(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return DropdownStateBuilders.fieldRow(
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(AppShape.r12),
        border: Border.all(color: context.statusColors.error),
        color: scheme.surface,
      ),
      children: [
        Icon(
          Icons.error_outline,
          color: context.statusColors.error,
          size: AppIconSize.action,
        ),
        const SizedBox(width: AppMetrics.p8),
        Text(
          'Error loading desa',
          style: TextStyle(color: context.statusColors.error),
        ),
      ],
    );
  }

  Widget _buildSearchableDropdown(
    BuildContext context,
    List<Village> villages,
  ) {
    final scheme = Theme.of(context).colorScheme;
    return GestureDetector(
      onTap: () {
        // Close dropdown ketika tap di luar
        if (_isDropdownOpen) {
          setState(() {
            _isDropdownOpen = false;
          });
        }
      },
      child: Column(
        children: [
          TextFormField(
            controller: _searchController,
            validator: (value) =>
                widget.validator?.call(widget.selectedVillage),
            decoration: InputDecoration(
              hintText: widget.hintText ?? 'Cari atau pilih desa/kelurahan',
              prefixIcon: widget.prefixIcon != null
                  ? Icon(
                      widget.prefixIcon,
                      color: scheme.onSurfaceVariant,
                    )
                  : null,
              suffixIcon: GestureDetector(
                onTap: () {
                  setState(() {
                    if (_isDropdownOpen) {
                      _isDropdownOpen = false;
                    } else {
                      _filteredVillages = villages;
                      _isDropdownOpen = true;
                    }
                  });
                },
                child: Icon(
                  _isDropdownOpen
                      ? Icons.keyboard_arrow_up
                      : Icons.keyboard_arrow_down,
                  color: scheme.onSurfaceVariant,
                ),
              ),
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(AppShape.r12),
                borderSide: BorderSide(color: scheme.outlineVariant),
              ),
              enabledBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(AppShape.r12),
                borderSide: BorderSide(color: scheme.outlineVariant),
              ),
              focusedBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(AppShape.r12),
                borderSide: BorderSide(color: scheme.primary),
              ),
              filled: true,
              fillColor: scheme.surface,
              contentPadding: const EdgeInsets.symmetric(
                horizontal: AppMetrics.p16,
                vertical: AppMetrics.p16,
              ),
            ),
            style: TextStyle(
              color: scheme.onSurface,
              fontSize: AppType.s16,
            ),
            onChanged: (query) {
              setState(() {
                _filteredVillages = villages
                    .where(
                      (village) => village.name.toLowerCase().contains(
                        query.toLowerCase(),
                      ),
                    )
                    .toList();
                _isDropdownOpen =
                    query.isNotEmpty || _filteredVillages.isNotEmpty;
              });

              // Reset selected village jika text berubah tapi tidak match dengan selected
              if (widget.selectedVillage != null &&
                  widget.selectedVillage!.name != query) {
                widget.onChanged(null);
              }
            },
            onTap: () {
              setState(() {
                _filteredVillages = villages;
                _isDropdownOpen = true;
              });
            },
          ),
          if (_isDropdownOpen && _filteredVillages.isNotEmpty) ...[
            const SizedBox(height: 4),
            Container(
              constraints: const BoxConstraints(maxHeight: 200),
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(AppShape.r12),
                border: Border.all(color: scheme.outlineVariant),
                color: scheme.surfaceContainerHigh,
                boxShadow: [
                  BoxShadow(
                    color: scheme.shadow.withValues(alpha: 0.15),
                    blurRadius: 8,
                    offset: const Offset(0, 2),
                  ),
                ],
              ),
              child: ListView.builder(
                shrinkWrap: true,
                itemCount: _filteredVillages.length,
                itemBuilder: (context, index) {
                  final village = _filteredVillages[index];
                  final isSelected = widget.selectedVillage?.id == village.id;

                  return Container(
                    decoration: BoxDecoration(
                      color: isSelected
                          ? scheme.primary.withValues(alpha: 0.12)
                          : Colors.transparent,
                    ),
                    child: ListTile(
                      title: Text(
                        village.name,
                        style: TextStyle(
                          color: isSelected
                              ? scheme.primary
                              : scheme.onSurface,
                          fontSize: AppType.s14,
                          fontWeight: isSelected
                              ? FontWeight.w600
                              : FontWeight.normal,
                        ),
                      ),
                      onTap: () {
                        widget.onChanged(village);
                        _searchController.text = village.name;
                        setState(() {
                          _isDropdownOpen = false;
                        });
                      },
                    ),
                  );
                },
              ),
            ),
          ],
          if (_isDropdownOpen &&
              _filteredVillages.isEmpty &&
              _searchController.text.isNotEmpty) ...[
            const SizedBox(height: 4),
            DropdownStateBuilders.fieldRow(
              decoration: BoxDecoration(
                borderRadius: BorderRadius.circular(AppShape.r12),
                border: Border.all(color: scheme.outlineVariant),
                color: scheme.surface,
              ),
              children: [
                Icon(
                  Icons.search_off,
                  color: scheme.onSurfaceVariant,
                  size: AppIconSize.action,
                ),
                const SizedBox(width: AppMetrics.p8),
                Text(
                  'Tidak ditemukan hasil pencarian',
                  style: TextStyle(
                    color: scheme.onSurfaceVariant,
                    fontSize: AppType.s14,
                  ),
                ),
              ],
            ),
          ],
        ],
      ),
    );
  }
}
